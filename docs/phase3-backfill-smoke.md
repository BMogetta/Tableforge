# Phase 3 — Ranked Bot Backfill Smoke Test

One-off recipe to verify that a lone human in the ranked queue triggers a bot
activation via `match-service` + `bot-runner --mode=backfill`.

**Run from the repo root** (`/home/bruno/Github/tableforge`).

---

## 1. Prepare env and temporary redis override

Append the backfill vars to `.env`. `BOT_SERVICE_SECRET` is optional —
set it to exercise the production-style `/auth/bot-login` endpoint,
leave it unset to fall back to `/auth/test-login` (TEST_MODE only).

```bash
cat >> .env <<'EOF'

# --- Smoke Phase 3 ---
BACKFILL_ENABLED=true
BACKFILL_THRESHOLD_SECS=5
BACKFILL_MAX_ACTIVE=3
RANKED_GAME_ID=rootaccess
BOT_SERVICE_SECRET=smoke-phase3-secret
EOF
```

Expose Redis on the host so `bot-runner` (which runs outside the compose
network) can subscribe:

```bash
cat > docker-compose.smoke.yml <<'EOF'
services:
  redis:
    ports:
      - "127.0.0.1:6379:6379"
EOF
```

---

## 2. Bring up the test stack

```bash
TEST_MODE=true MATCHMAKER_TICK_INTERVAL=1s docker compose -f docker-compose.yml -f docker-compose.override.yml -f docker-compose.services.yml -f docker-compose.smoke.yml --profile app up --build -d
```

Wait until everything is healthy:

```bash
docker compose ps
```

---

## 3. Seed players (creates bot slots with `is_bot=TRUE`)

```bash
make seed-test
```

---

## 4. Capture UUIDs

```bash
BOTS=$(docker exec postgres psql -U recess -d recess -tAc "SELECT string_agg(id::text || ':' || split_part(username,'_',2), ',') FROM players WHERE is_bot = TRUE AND username IN ('bot_easy_1','bot_medium_1','bot_hard_1')")
echo "BOTS=$BOTS"
```

Expected: `<uuid>:easy,<uuid>:medium,<uuid>:hard`

```bash
HUMAN_ID=$(docker exec postgres psql -U recess -d recess -tAc "SELECT id FROM players WHERE username='test_player_1'")
echo "HUMAN_ID=$HUMAN_ID"
```

---

## 5. Terminal A — start `bot-runner` in backfill mode

Export `BOT_SERVICE_SECRET` to use `/auth/bot-login` (must match the value
in `.env`). Leave it unset to fall back to `/auth/test-login`.

```bash
REDIS_URL=redis://:recess@localhost:6379 \
BOT_SERVICE_SECRET=smoke-phase3-secret \
  go run ./services/game-server/cmd/bot-runner \
    --mode=backfill --base-url http://localhost \
    --game-id rootaccess --bots "$BOTS"
```

Expected output (one block per bot):

```
INFO authenticated (backfill mode) bot=...
INFO awaiting activation channel=bot.activate
```

---

## 6. Terminal B — follow `match-service` logs

```bash
docker logs -f match-service 2>&1 | grep -iE "backfill|queue:|match"
```

Expected once on boot:

```
INFO backfill config enabled=true threshold=5s max_active=3
```

---

## 7. Terminal C — push a lone human into the queue

```bash
COOKIE=$(mktemp)
curl -sS -c "$COOKIE" "http://localhost/auth/test-login?player_id=$HUMAN_ID"
curl -sS -b "$COOKIE" -X POST "http://localhost/api/v1/queue"
echo "joined queue at $(date +%T)"
```

---

## 8. What you should see

**Terminal B (match-service), ~5–7 s after enqueue:**

```
INFO backfill: bot activated bot=<uuid> human=<HUMAN_ID> human_mmr=1000 waited_secs=5
INFO queue: proposed match match_id=... player_a=<human> player_b=<bot>
INFO queue: match started room_id=... session_id=...
```

**Terminal A (bot-runner):**

```
INFO activated — joining queue bot=bot_medium_1
INFO joined queue
INFO match found match_id=...
INFO match ready room_id=... session_id=...
INFO game over outcome=win|loss|draw
INFO awaiting activation
```

---

## 9. Edge cases to exercise

### A. Two humans before threshold → backfill must NOT fire

```bash
HUMAN2_ID=$(docker exec postgres psql -U recess -d recess -tAc "SELECT id FROM players WHERE username='test_player_2'")
COOKIE2=$(mktemp)
curl -sS -c "$COOKIE2" "http://localhost/auth/test-login?player_id=$HUMAN2_ID"
curl -sS -b "$COOKIE"  -X POST "http://localhost/api/v1/queue"
curl -sS -b "$COOKIE2" -X POST "http://localhost/api/v1/queue"
```

Expected: `queue: proposed match` for the human/human pair, no `backfill:` line.

### B. Backfill while some bots are mid-game

Run edge case A to keep 2 humans busy. With the 3rd bot still idle, push a
third human alone:

```bash
HUMAN3_ID=$(docker exec postgres psql -U recess -d recess -tAc "SELECT id FROM players WHERE username='test_player_3'")
COOKIE3=$(mktemp)
curl -sS -c "$COOKIE3" "http://localhost/auth/test-login?player_id=$HUMAN3_ID"
curl -sS -b "$COOKIE3" -X POST "http://localhost/api/v1/queue"
```

Expected: backfill fires, picks one of the remaining idle bots.

---

## 10. Diagnostics

```bash
docker logs match-service 2>&1 | grep "backfill config"
```

```bash
docker exec redis redis-cli -a recess SMEMBERS bot:known
docker exec redis redis-cli -a recess SMEMBERS bot:available
docker exec redis redis-cli -a recess ZRANGE queue:ranked 0 -1 WITHSCORES
```

Live-subscribe to the activation channel from another shell:

```bash
docker exec -it redis redis-cli -a recess SUBSCRIBE bot.activate
```

---

## 11. Cleanup

Ctrl+C the `bot-runner` process in Terminal A. It should `SREM` itself from
both sets. Verify:

```bash
docker exec redis redis-cli -a recess SMEMBERS bot:known
docker exec redis redis-cli -a recess SMEMBERS bot:available
```

Both should return empty arrays.

```bash
make down
rm docker-compose.smoke.yml
```

Then manually remove the `# --- Smoke Phase 3 ---` block from `.env`
(including `BOT_SERVICE_SECRET`).

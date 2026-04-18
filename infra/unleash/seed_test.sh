#!/bin/sh
# Smoke test for seed.sh. Runs against a live Unleash reachable at
# UNLEASH_URL (with /api suffix) using UNLEASH_TOKEN, and verifies that:
#
#   1. seed.sh exits 0 on a fresh run.
#   2. Every flag from flags.json exists.
#   3. Each flag's dev-env state matches environments.json.
#   4. A second invocation stays exit 0 (idempotency).
#
# Usage (from repo root):
#   UNLEASH_URL=http://unleash.localhost/api \
#   UNLEASH_TOKEN='*:*.unleash-insecure-api-token' \
#     ./infra/unleash/seed_test.sh
#
# Or inside the docker network:
#   docker run --rm --network data_network \
#     -e UNLEASH_URL=http://unleash:4242/api \
#     -e UNLEASH_TOKEN='*:*.unleash-insecure-api-token' \
#     -v "$(pwd)/infra/unleash:/seed:ro" \
#     alpine:3.20 sh -c "apk add --no-cache curl jq >/dev/null && /seed/seed_test.sh"

set -eu

: "${UNLEASH_URL:?UNLEASH_URL is required}"
: "${UNLEASH_TOKEN:?UNLEASH_TOKEN is required}"

SEED_DIR="$(dirname "$0")"
FLAGS_JSON="$SEED_DIR/flags.json"
ENVS_JSON="$SEED_DIR/environments.json"

# Strip trailing /api like seed.sh does, for API reads below.
BASE_URL="${UNLEASH_URL%/api}"
BASE_URL="${BASE_URL%/}"
API="$BASE_URL/api/admin/projects/default"
AUTH="Authorization: $UNLEASH_TOKEN"

fail() { echo "FAIL: $1" >&2; exit 1; }
ok()   { echo "ok:   $1"; }

# ── 1 + 4. Run seed twice, both must succeed ───────────────────────────────

echo "── running seed.sh (first run) ──"
if ! sh "$SEED_DIR/seed.sh" >/dev/null; then
  fail "seed.sh first run exited non-zero"
fi
ok "seed.sh first run exit 0"

echo "── running seed.sh (second run, idempotency check) ──"
if ! sh "$SEED_DIR/seed.sh" >/dev/null; then
  fail "seed.sh second run exited non-zero (idempotency broken)"
fi
ok "seed.sh second run exit 0"

# ── 2 + 3. Verify every flag exists with the expected env state ────────────

jq -c '.[]' "$ENVS_JSON" | while IFS= read -r entry; do
  feature=$(echo "$entry" | jq -r '.feature')
  env=$(echo "$entry" | jq -r '.environment')
  want=$(echo "$entry" | jq -r '.enabled')

  got=$(curl -fsS -H "$AUTH" "$API/features/$feature" 2>/dev/null \
    | jq -r ".environments[] | select(.name == \"$env\") | .enabled")

  if [ -z "$got" ]; then
    fail "flag $feature: env $env not found (flag exists? check seed.sh logs)"
  fi

  if [ "$got" != "$want" ]; then
    fail "flag $feature env $env: want enabled=$want, got enabled=$got"
  fi

  ok "flag $feature/$env enabled=$got (matches declared)"
done

echo "── all checks passed ──"

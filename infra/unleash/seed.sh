#!/bin/sh
# Idempotent Unleash seeder: reads flags.json + environments.json from the
# same directory as this script, POSTs them to the Unleash admin API, and
# leaves the instance in the declared state. Safe to re-run; 409 Conflict
# on existing flags is treated as success.
#
# Requires: curl, jq (both in alpine:3.20).
#
# Env vars:
#   UNLEASH_URL   — admin base URL (e.g. http://unleash:4242). Required.
#   UNLEASH_TOKEN — admin API token (e.g. *:*.unleash-insecure-api-token). Required.
#
# Exit codes:
#   0  success (idempotent, all flags + env states reconciled)
#   1  config error (missing env var, JSON parse error)
#   2  Unleash never became healthy within the wait budget
#   3  a flag create/enable POST returned an unexpected 5xx

set -eu

# ── Config ───────────────────────────────────────────────────────────────────

: "${UNLEASH_URL:?UNLEASH_URL is required}"
: "${UNLEASH_TOKEN:?UNLEASH_TOKEN is required}"

# The Go SDK convention points UNLEASH_URL at http://unleash:4242/api but the
# admin endpoints here are /api/admin/... and the health check is /health
# (outside /api). Normalize by stripping a trailing /api if present so the
# same env var can be shared across services.
UNLEASH_URL="${UNLEASH_URL%/api}"
UNLEASH_URL="${UNLEASH_URL%/}"

SEED_DIR="$(dirname "$0")"
FLAGS_JSON="$SEED_DIR/flags.json"
ENVS_JSON="$SEED_DIR/environments.json"

if [ ! -f "$FLAGS_JSON" ] || [ ! -f "$ENVS_JSON" ]; then
  echo "ERROR: missing $FLAGS_JSON or $ENVS_JSON" >&2
  exit 1
fi

# ── Wait for Unleash ────────────────────────────────────────────────────────

WAIT_BUDGET_SECS=60
waited=0
while [ "$waited" -lt "$WAIT_BUDGET_SECS" ]; do
  if curl -fsS "$UNLEASH_URL/health" >/dev/null 2>&1; then
    break
  fi
  sleep 2
  waited=$((waited + 2))
done

if [ "$waited" -ge "$WAIT_BUDGET_SECS" ]; then
  echo "ERROR: Unleash at $UNLEASH_URL never became healthy after ${WAIT_BUDGET_SECS}s" >&2
  exit 2
fi

echo "✓ Unleash healthy at $UNLEASH_URL"

# ── Helper ──────────────────────────────────────────────────────────────────

AUTH_HEADER="Authorization: $UNLEASH_TOKEN"
PROJECT_URL="$UNLEASH_URL/api/admin/projects/default"

# post_flag creates a flag; 409 (already exists) and 200/201 are success.
post_flag() {
  flag_json="$1"
  name=$(echo "$flag_json" | jq -r '.name')

  http_code=$(curl -s -o /tmp/seed_resp -w "%{http_code}" \
    -X POST "$PROJECT_URL/features" \
    -H "$AUTH_HEADER" \
    -H "Content-Type: application/json" \
    -d "$flag_json")

  case "$http_code" in
    200|201) echo "  + created $name" ;;
    409)     echo "  = exists $name" ;;
    *)
      echo "ERROR: POST feature $name returned $http_code: $(cat /tmp/seed_resp)" >&2
      return 3
      ;;
  esac
}

# ensure_env_state sets enabled state in a given env and registers the
# default strategy (Unleash rejects enabled=true without a strategy).
ensure_env_state() {
  feature="$1"
  env="$2"
  enabled="$3"

  # Upsert default strategy. Response 200 = reused, 201 = new, 409 = exists.
  http_code=$(curl -s -o /tmp/seed_resp -w "%{http_code}" \
    -X POST "$PROJECT_URL/features/$feature/environments/$env/strategies" \
    -H "$AUTH_HEADER" \
    -H "Content-Type: application/json" \
    -d '{"name":"default","constraints":[],"parameters":{}}')

  case "$http_code" in
    200|201|409) : ;;
    *)
      echo "ERROR: strategy POST for $feature/$env returned $http_code: $(cat /tmp/seed_resp)" >&2
      return 3
      ;;
  esac

  # Toggle on/off. Idempotent: POSTing /on when already on returns 200.
  if [ "$enabled" = "true" ]; then
    action=on
  else
    action=off
  fi

  http_code=$(curl -s -o /tmp/seed_resp -w "%{http_code}" \
    -X POST "$PROJECT_URL/features/$feature/environments/$env/$action" \
    -H "$AUTH_HEADER")

  case "$http_code" in
    200|204) echo "  · $feature/$env → $action" ;;
    *)
      echo "ERROR: toggle $action for $feature/$env returned $http_code: $(cat /tmp/seed_resp)" >&2
      return 3
      ;;
  esac
}

# ── Apply flags ─────────────────────────────────────────────────────────────

echo "Seeding flags..."
jq -c '.[]' "$FLAGS_JSON" | while IFS= read -r flag; do
  post_flag "$flag"
done

echo "Applying environment states..."
jq -c '.[]' "$ENVS_JSON" | while IFS= read -r entry; do
  feature=$(echo "$entry" | jq -r '.feature')
  env=$(echo "$entry" | jq -r '.environment')
  enabled=$(echo "$entry" | jq -r '.enabled')
  ensure_env_state "$feature" "$env" "$enabled"
done

echo "✓ Seed complete"

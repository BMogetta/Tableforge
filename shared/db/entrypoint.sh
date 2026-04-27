#!/usr/bin/env bash
# =============================================================================
# recess-db-migrate entrypoint
# =============================================================================
# Steps:
#   1. Run `migrate up` — applies any pending .up.sql migrations. Idempotent
#      (golang-migrate tracks applied versions in `schema_migrations`).
#   2. If APPLY_SEED=prod, also apply /seeds/prod.sql via psql. This file is
#      idempotent (every INSERT uses ON CONFLICT DO NOTHING), so re-running
#      on every sync is safe.
#
# Required env:
#   DATABASE_URL  — postgres://user:pass@host:port/db (sslmode appended below)
#
# Optional env:
#   APPLY_SEED         — 'prod' to apply prod seed after migrate; default off.
#                        'dev' is intentionally not supported here — dev seed
#                        runs only in docker-compose via 999_apply_dev_seed.sh.
#
#   INIT_FORCE_VERSION — Set to a version number (e.g. "9") to mark all
#                        migrations up to and including that version as
#                        already applied WITHOUT running them. One-time use
#                        when bootstrapping golang-migrate against a database
#                        whose schema was applied manually before this Job
#                        existed. Remove the env after the first sync; on
#                        subsequent runs `migrate up` is a no-op.
set -euo pipefail

if [ -z "${DATABASE_URL:-}" ]; then
  echo "ERROR: DATABASE_URL not set" >&2
  exit 1
fi

# CNPG's pg-app.uri secret key omits sslmode; append `sslmode=require` when
# missing so connections to the TLS-enabled cluster don't fall back to plain.
if [[ "$DATABASE_URL" != *"sslmode="* ]]; then
  if [[ "$DATABASE_URL" == *"?"* ]]; then
    DATABASE_URL="${DATABASE_URL}&sslmode=require"
  else
    DATABASE_URL="${DATABASE_URL}?sslmode=require"
  fi
fi

MASKED=$(echo "$DATABASE_URL" | sed -E 's|://[^:]+:[^@]+@|://***:***@|')

if [ -n "${INIT_FORCE_VERSION:-}" ]; then
  echo "==> INIT_FORCE_VERSION=$INIT_FORCE_VERSION — marking schema_migrations as applied without running"
  migrate -path /migrations -database "$DATABASE_URL" force "$INIT_FORCE_VERSION"
fi

echo "==> Running migrate up against $MASKED"
migrate -path /migrations -database "$DATABASE_URL" up

case "${APPLY_SEED:-}" in
  prod)
    echo "==> Applying prod seed (idempotent)"
    psql -v ON_ERROR_STOP=1 "$DATABASE_URL" -f /seeds/prod.sql
    ;;
  '')
    echo "==> APPLY_SEED unset; skipping seed application"
    ;;
  *)
    echo "ERROR: unsupported APPLY_SEED='$APPLY_SEED' (only 'prod' is valid here)" >&2
    exit 1
    ;;
esac

echo "==> Done."

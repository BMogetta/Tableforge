#!/usr/bin/env bash
# =============================================================================
# 999_apply_dev_seed.sh — Apply dev seed in docker-compose only
# =============================================================================
# postgres' /docker-entrypoint-initdb.d runs *.sh files alphabetically. This
# is the LAST step (999_) so it runs after every DDL migration has applied.
#
# k8s never sees this file: the migrator image at shared/db/Dockerfile only
# bundles *.up.sql and the prod seed. .sh init scripts are docker-compose
# specific (CNPG handles initdb separately).
#
# Gated on TEST_MODE=true so non-test docker-compose runs don't get dev data.
set -euo pipefail

if [ "${TEST_MODE:-false}" != "true" ]; then
  echo "TEST_MODE != true; skipping dev seed."
  exit 0
fi

echo "TEST_MODE=true; applying dev seed from /seeds/dev.sql"
psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f /seeds/dev.sql

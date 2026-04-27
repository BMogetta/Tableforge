# =============================================================================
# recess-db-migrate — bundled migrator + seed runner
# =============================================================================
# Stage 1: pull the official golang-migrate binary (no build, just COPY).
# Stage 2: alpine + psql client + bundled migrations + seeds + entrypoint.
#
# Build context: ./shared/db (so COPY . . brings migrations/ + seeds/).
# Built and pushed by .github/workflows/db-migrate-image.yml on changes
# under shared/db/**.
#
# Image is consumed by the ArgoCD-managed Job at
#   recess-deploy/infra/k8s/data/db-migrations/job.yaml
# with sync-wave -10 so it runs before service Apps on every sync.
# `migrate up` is idempotent (tracks state in `schema_migrations` table),
# so re-runs on every sync are safe no-ops when there's nothing pending.

<<<<<<< fix/db-migrate-arm64
# Stage 1: extract migrate binary. Multi-arch image — let buildkit resolve
# the manifest matching $TARGETPLATFORM (arm64 on the Pi). Pinning to
# $BUILDPLATFORM here was a bug: it pulled the runner-native amd64 binary
# even when targeting arm64, producing "exec format error" at runtime.
FROM migrate/migrate:v4.19.1@sha256:cc4ad8e19d66791e3689405d9a028ce6e9614f32032db14acda1469f7201d6e4 AS migrate-bin
=======
# Stage 1: extract migrate binary from official image (multi-arch).
FROM --platform=$BUILDPLATFORM migrate/migrate:v4.19.1@sha256:cc4ad8e19d66791e3689405d9a028ce6e9614f32032db14acda1469f7201d6e4 AS migrate-bin
>>>>>>> main

# Stage 2: minimal runtime with psql + the migrate binary.
FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11
RUN apk --no-cache add postgresql16-client bash ca-certificates
COPY --from=migrate-bin /usr/local/bin/migrate /usr/local/bin/migrate

WORKDIR /work
# Only DDL .up.sql files end up in /migrations — the .sh init scripts
# (000_init-databases, 998_grant_permissions, 999_apply_dev_seed) are
# docker-compose-only and excluded by .dockerignore.
COPY migrations/*.up.sql /migrations/
COPY seeds/ /seeds/
COPY entrypoint.sh /usr/local/bin/recess-db-migrate
RUN chmod +x /usr/local/bin/recess-db-migrate

ENTRYPOINT ["/usr/local/bin/recess-db-migrate"]

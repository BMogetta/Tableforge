#!/bin/bash
set -e

# Runs after all SQL migrations (sorted after 009 *.up.sql, before
# 999_apply_dev_seed.sh). Grants the application user ownership of all
# objects created by the postgres superuser during migration.
# docker-compose only — k8s never sees this file (.sh files excluded
# from the migrator image via shared/db/.dockerignore).

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname recess <<-EOSQL
    -- Grant schema usage
    GRANT USAGE ON SCHEMA public, users, ratings, admin TO recess;

    -- Transfer ownership of all tables, sequences, and functions
    -- Read-only user for Claude MCP (no inserts, updates, or deletes)
    CREATE USER claude_mcp_ro WITH PASSWORD '${CLAUDE_MCP_RO_PASSWORD:-claude_ro}';
    GRANT USAGE ON SCHEMA public, users, ratings, admin TO claude_mcp_ro;

    DO \$\$
    DECLARE
        s TEXT;
    BEGIN
        FOR s IN SELECT unnest(ARRAY['public', 'users', 'ratings', 'admin'])
        LOOP
            -- Application user: full access
            EXECUTE format('GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA %I TO recess', s);
            EXECUTE format('GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA %I TO recess', s);
            EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I GRANT ALL PRIVILEGES ON TABLES TO recess', s);
            EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I GRANT ALL PRIVILEGES ON SEQUENCES TO recess', s);

            -- MCP user: read-only
            EXECUTE format('GRANT SELECT ON ALL TABLES IN SCHEMA %I TO claude_mcp_ro', s);
            EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I GRANT SELECT ON TABLES TO claude_mcp_ro', s);
        END LOOP;
    END
    \$\$;
EOSQL

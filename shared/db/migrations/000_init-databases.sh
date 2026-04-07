#!/bin/bash
set -e

# This script runs once during postgres first initialization (docker-entrypoint-initdb.d).
# It creates application users and the unleash database.
# The recess database is already created by POSTGRES_DB=recess.

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<-EOSQL
    -- Application user (owns the recess database)
    CREATE USER recess WITH PASSWORD '${RECESS_DB_PASSWORD}';
    ALTER DATABASE recess OWNER TO recess;
    GRANT ALL PRIVILEGES ON DATABASE recess TO recess;

    -- Unleash user and database (feature flags)
    CREATE USER unleash WITH PASSWORD '${UNLEASH_DB_PASSWORD}';
    CREATE DATABASE unleash OWNER unleash;
    GRANT ALL PRIVILEGES ON DATABASE unleash TO unleash;
EOSQL

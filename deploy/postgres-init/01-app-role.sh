#!/bin/bash
# Runs once, on first init of the postgres data volume (docker-entrypoint-initdb.d).
# Creates the non-superuser, non-owner application role. Table ownership stays with
# POSTGRES_USER (the migration role) so RLS cannot be silently bypassed by the app
# connection (PRD 6.1 point 6).
set -euo pipefail

: "${APP_DB_PASSWORD:?APP_DB_PASSWORD must be set}"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
  DO \$\$
  BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'app_user') THEN
      CREATE ROLE app_user LOGIN PASSWORD '${APP_DB_PASSWORD}' NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;
    END IF;
  END
  \$\$;
EOSQL

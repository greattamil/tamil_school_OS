-- Migrations run as the postgres superuser and therefore own every table created
-- above. Table owners bypass RLS by default, so the application must connect as a
-- separate, non-owning role with no BYPASSRLS (PRD 6.1 point 6). That role
-- (app_user) is created by deploy/postgres-init/01-app-role.sh on first container
-- init; this migration only grants it the privileges it needs on the objects that
-- now exist.
DO $$
BEGIN
  EXECUTE format('GRANT CONNECT ON DATABASE %I TO app_user', current_database());
END
$$;
GRANT USAGE ON SCHEMA public TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_user;
GRANT EXECUTE ON FUNCTION current_school_id() TO app_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_user;

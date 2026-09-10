-- Same pattern as 000008_rls_policies.up.sql, extended to the tenant tables added
-- in Phase 2. jobs is deliberately excluded -- see the comment on that table in
-- 000012_jobs.up.sql.
DO $$
DECLARE
  t text;
  tenant_tables CONSTANT text[] := ARRAY[
    'attendance_registers', 'attendance_entries', 'attendance_entry_device_counters',
    'attendance_sync_rejections',
    'school_settings', 'notifications', 'notification_recipients', 'device_push_tokens',
    'fee_dues_snapshot'
  ];
BEGIN
  FOREACH t IN ARRAY tenant_tables LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format(
      'CREATE POLICY tenant_isolation ON %I USING (school_id = current_school_id()) WITH CHECK (school_id = current_school_id())',
      t
    );
  END LOOP;
END
$$;

-- app_user needs privileges on the tables just created, same as
-- 000009_app_role_grants.up.sql did for Phase 1.
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_user;

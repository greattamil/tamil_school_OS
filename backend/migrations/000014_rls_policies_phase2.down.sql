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
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', t);
    EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', t);
  END LOOP;
END
$$;

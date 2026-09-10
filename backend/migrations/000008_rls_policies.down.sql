DO $$
DECLARE
  t text;
  tenant_tables CONSTANT text[] := ARRAY[
    'academic_years', 'classes', 'sections',
    'guardians', 'students', 'student_guardians', 'enrollments',
    'staff', 'section_teachers',
    'audit_log'
  ];
BEGIN
  FOREACH t IN ARRAY tenant_tables LOOP
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', t);
    EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', t);
  END LOOP;
END
$$;

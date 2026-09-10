-- Row-level security is the backstop, not the only control (PRD 6.1). Every table
-- listed here carries school_id; the policy rejects UPDATE/DELETE/SELECT/INSERT
-- against rows outside the caller's tenant context, derived exclusively from
-- current_school_id() -- which in turn reads a value only ever set via
-- SET LOCAL app.current_school_id inside an explicit transaction (see internal/db).
--
-- schools, users, credentials, sessions, otp_codes and user_school_roles are the
-- global identity tables (PRD 3.2.1) and deliberately carry no RLS.
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
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format(
      'CREATE POLICY tenant_isolation ON %I USING (school_id = current_school_id()) WITH CHECK (school_id = current_school_id())',
      t
    );
  END LOOP;
END
$$;

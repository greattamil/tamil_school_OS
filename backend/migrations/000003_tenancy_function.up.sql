-- Single helper function referenced by every RLS policy (PRD 6.1 point 5), rather than
-- inlining current_setting() in each policy. missing_ok=true means an unset tenant
-- context yields NULL, so policies evaluate to zero rows rather than raising an error.
CREATE FUNCTION current_school_id() RETURNS uuid AS $$
  SELECT NULLIF(current_setting('app.current_school_id', true), '')::uuid;
$$ LANGUAGE sql STABLE PARALLEL SAFE;

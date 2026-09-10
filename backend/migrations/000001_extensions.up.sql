-- Extensions required by the schema.
-- pgcrypto: gen_random_uuid() for UUID primary keys (no sequential integers, per PRD 3.3).
-- btree_gist: required for the GiST exclusion constraint on enrollments.period, which
-- needs an equality operator class for uuid columns inside a GiST index.
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "btree_gist";

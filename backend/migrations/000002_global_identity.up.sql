-- Global identity tables (PRD 3.2.1): users, credentials, sessions, otp_codes and
-- user_school_roles carry NO school_id and NO row-level security. A person is one
-- identity across every school they hold a role at. Kept deliberately small.

CREATE TABLE schools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    short_code TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mobile TEXT UNIQUE,
    email TEXT UNIQUE,
    display_name TEXT NOT NULL,
    -- Revoking access, forcing logout, or resetting a password sets this to now(),
    -- invalidating every access/refresh token issued before that moment (PRD 6.2).
    tokens_valid_after TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT users_mobile_or_email CHECK (mobile IS NOT NULL OR email IS NOT NULL)
);

CREATE TABLE credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_credentials_user_id ON credentials(user_id);

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    refresh_token_hash TEXT NOT NULL UNIQUE,
    -- Set once a school is selected; a token exchange (not a parameter change) is
    -- required to switch schools (PRD 3.2.1).
    school_id UUID REFERENCES schools(id),
    device_id TEXT,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);

CREATE TABLE otp_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mobile TEXT NOT NULL,
    code_hash TEXT NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_otp_codes_mobile ON otp_codes(mobile);

CREATE TYPE user_role AS ENUM ('correspondent', 'office_admin', 'teacher', 'parent');

CREATE TABLE user_school_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    school_id UUID NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    role user_role NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (user_id, school_id, role)
);
CREATE INDEX idx_user_school_roles_user ON user_school_roles(user_id);
CREATE INDEX idx_user_school_roles_school ON user_school_roles(school_id);

-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id uuid PRIMARY KEY,
    email citext NOT NULL UNIQUE,
    password_hash text NOT NULL,
    name text NOT NULL,
    phone text,
    role text NOT NULL DEFAULT 'CUSTOMER' CHECK (role IN ('CUSTOMER', 'ADMIN')),
    status text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'BLOCKED')),
    email_verified_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX users_role_status_idx ON users (role, status);

CREATE TABLE email_verifications (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash text NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    attempts int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX email_verifications_user_expires_idx ON email_verifications (user_id, expires_at DESC);

CREATE TABLE refresh_sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    family_id uuid NOT NULL,
    user_agent text,
    ip inet,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    replaced_by uuid REFERENCES refresh_sessions(id),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX refresh_sessions_user_expires_idx ON refresh_sessions (user_id, expires_at);
CREATE INDEX refresh_sessions_family_idx ON refresh_sessions (family_id);

-- +goose Down
DROP TABLE refresh_sessions;
DROP TABLE email_verifications;
DROP TABLE users;
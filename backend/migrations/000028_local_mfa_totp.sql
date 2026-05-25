-- +goose Up
ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS mfa_required BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS mfa_verified_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS user_local_totp (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    encrypted_secret BYTEA NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_mfa_recovery_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash BYTEA NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, code_hash)
);

ALTER TABLE admin_settings
    ADD COLUMN IF NOT EXISTS force_local_mfa BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE admin_settings
    DROP COLUMN IF EXISTS force_local_mfa;

DROP TABLE IF EXISTS user_mfa_recovery_codes;
DROP TABLE IF EXISTS user_local_totp;

ALTER TABLE sessions
    DROP COLUMN IF EXISTS mfa_verified_at,
    DROP COLUMN IF EXISTS mfa_required;

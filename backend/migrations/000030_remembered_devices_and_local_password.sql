-- +goose Up
CREATE TABLE IF NOT EXISTS remembered_devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    selector_hash BYTEA NOT NULL UNIQUE,
    verifier_hash BYTEA NOT NULL,
    user_agent TEXT,
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS remembered_devices_user_active_idx
ON remembered_devices (user_id, expires_at DESC)
WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS user_local_passwords (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS user_local_passwords;
DROP TABLE IF EXISTS remembered_devices;

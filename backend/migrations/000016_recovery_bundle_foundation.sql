-- +goose Up
CREATE TABLE user_recovery_keys (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    public_recipient TEXT NOT NULL,
    encrypted_private_identity BYTEA NOT NULL,
    encryption_scheme TEXT NOT NULL DEFAULT 'aes-256-gcm',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE recovery_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    snapshot_version INTEGER NOT NULL,
    manifest_schema TEXT NOT NULL,
    manifest_sha256 BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, snapshot_version)
);

CREATE INDEX recovery_snapshots_user_created_idx ON recovery_snapshots (user_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS recovery_snapshots;
DROP TABLE IF EXISTS user_recovery_keys;

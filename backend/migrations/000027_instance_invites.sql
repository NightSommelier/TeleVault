-- +goose Up
CREATE TABLE instance_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash BYTEA NOT NULL UNIQUE,
    invited_telegram_id BIGINT,
    max_uses INTEGER NOT NULL DEFAULT 1,
    used_count INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    expires_at TIMESTAMPTZ NOT NULL,
    creator_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    consumed_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT instance_invites_status_check CHECK (status IN ('active', 'revoked', 'consumed')),
    CONSTRAINT instance_invites_max_uses_positive CHECK (max_uses > 0),
    CONSTRAINT instance_invites_used_count_nonnegative CHECK (used_count >= 0),
    CONSTRAINT instance_invites_used_count_range CHECK (used_count <= max_uses)
);

CREATE INDEX instance_invites_active_idx
ON instance_invites (status, expires_at, created_at DESC)
WHERE status = 'active';

-- +goose Down
DROP TABLE IF EXISTS instance_invites;

-- +goose Up
CREATE TABLE IF NOT EXISTS user_webauthn_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id BYTEA NOT NULL UNIQUE,
    credential_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS user_webauthn_credentials_user_idx
ON user_webauthn_credentials (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS user_webauthn_challenges (
    id TEXT PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    challenge_kind TEXT NOT NULL,
    session_data_json JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT user_webauthn_challenges_kind_check CHECK (challenge_kind IN ('registration', 'authentication'))
);

CREATE INDEX IF NOT EXISTS user_webauthn_challenges_active_idx
ON user_webauthn_challenges (user_id, challenge_kind, expires_at DESC)
WHERE consumed_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS user_webauthn_challenges;
DROP TABLE IF EXISTS user_webauthn_credentials;

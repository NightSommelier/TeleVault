-- +goose Up
CREATE TABLE auth_challenges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_hash BYTEA NOT NULL,
    phone_code_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ
);

CREATE INDEX auth_challenges_phone_active_idx ON auth_challenges (phone_hash, expires_at)
WHERE consumed_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS auth_challenges;

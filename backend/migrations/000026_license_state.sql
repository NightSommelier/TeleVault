-- +goose Up
CREATE TABLE license_state (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE,
    raw_license_json TEXT,
    status TEXT NOT NULL DEFAULT 'missing',
    tier TEXT NOT NULL DEFAULT 'community',
    license_id TEXT,
    schema_version INTEGER,
    key_id TEXT,
    instance_id TEXT,
    issued_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    grace_days INTEGER,
    limits JSONB NOT NULL DEFAULT '{}'::jsonb,
    validation_error TEXT,
    installed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    installed_at TIMESTAMPTZ,
    validated_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT license_state_singleton CHECK (id),
    CONSTRAINT license_state_status_check CHECK (status IN ('missing', 'valid', 'invalid', 'expired', 'grace', 'instance_mismatch')),
    CONSTRAINT license_state_tier_check CHECK (tier IN ('community', 'pro', 'team')),
    CONSTRAINT license_state_grace_nonnegative CHECK (grace_days IS NULL OR grace_days >= 0),
    CONSTRAINT license_state_schema_positive CHECK (schema_version IS NULL OR schema_version > 0)
);

INSERT INTO license_state (id, status, tier, limits)
VALUES (TRUE, 'missing', 'community', '{}'::jsonb);

-- +goose Down
DROP TABLE IF EXISTS license_state;

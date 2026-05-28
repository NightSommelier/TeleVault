-- +goose Up
ALTER TABLE user_webauthn_credentials
    ADD COLUMN IF NOT EXISTS display_name TEXT NOT NULL DEFAULT 'Passkey',
    ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ;

UPDATE user_webauthn_credentials
SET display_name = 'Passkey'
WHERE btrim(display_name) = '';

-- +goose Down
ALTER TABLE user_webauthn_credentials
    DROP COLUMN IF EXISTS last_used_at,
    DROP COLUMN IF EXISTS display_name;

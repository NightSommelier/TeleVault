-- +goose Up
ALTER TABLE auth_challenges
ADD COLUMN encrypted_client_session BYTEA;

-- +goose Down
ALTER TABLE auth_challenges
DROP COLUMN IF EXISTS encrypted_client_session;

-- +goose Up
ALTER TABLE admin_settings
ADD COLUMN IF NOT EXISTS community_owner_telegram_id BIGINT;

-- +goose Down
ALTER TABLE admin_settings
DROP COLUMN IF EXISTS community_owner_telegram_id;

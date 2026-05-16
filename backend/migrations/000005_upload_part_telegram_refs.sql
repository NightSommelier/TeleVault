-- +goose Up
ALTER TABLE upload_parts
    ADD COLUMN telegram_peer TEXT,
    ADD COLUMN telegram_message_id BIGINT;

-- +goose Down
ALTER TABLE upload_parts
    DROP COLUMN IF EXISTS telegram_message_id,
    DROP COLUMN IF EXISTS telegram_peer;

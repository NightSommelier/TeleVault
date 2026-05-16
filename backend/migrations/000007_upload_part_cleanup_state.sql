-- +goose Up
ALTER TABLE upload_parts
    ADD COLUMN telegram_deleted_at TIMESTAMPTZ,
    ADD COLUMN telegram_delete_error TEXT;

CREATE INDEX upload_parts_telegram_cleanup_idx ON upload_parts (upload_id)
WHERE telegram_message_id IS NOT NULL
  AND telegram_deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS upload_parts_telegram_cleanup_idx;

ALTER TABLE upload_parts
    DROP COLUMN IF EXISTS telegram_delete_error,
    DROP COLUMN IF EXISTS telegram_deleted_at;

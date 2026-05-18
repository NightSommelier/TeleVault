-- +goose Up
ALTER TABLE file_parts
    ADD COLUMN telegram_deleted_at TIMESTAMPTZ,
    ADD COLUMN telegram_delete_error TEXT,
    ADD COLUMN telegram_delete_available_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE INDEX file_parts_telegram_cleanup_idx ON file_parts (telegram_delete_available_at, created_at)
WHERE telegram_deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS file_parts_telegram_cleanup_idx;

ALTER TABLE file_parts
    DROP COLUMN IF EXISTS telegram_delete_available_at,
    DROP COLUMN IF EXISTS telegram_delete_error,
    DROP COLUMN IF EXISTS telegram_deleted_at;

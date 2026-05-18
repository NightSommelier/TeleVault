-- +goose Up
ALTER TABLE upload_parts
    ADD COLUMN storage_backend TEXT,
    ADD COLUMN storage_key TEXT,
    ADD COLUMN available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN leased_until TIMESTAMPTZ,
    ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN last_error TEXT,
    ADD COLUMN worker_id TEXT,
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE INDEX upload_parts_queue_idx ON upload_parts (available_at, created_at)
WHERE status = 'pending'
  AND storage_backend IS NOT NULL
  AND storage_key IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS upload_parts_queue_idx;

ALTER TABLE upload_parts
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS worker_id,
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS attempts,
    DROP COLUMN IF EXISTS leased_until,
    DROP COLUMN IF EXISTS available_at,
    DROP COLUMN IF EXISTS storage_key,
    DROP COLUMN IF EXISTS storage_backend;

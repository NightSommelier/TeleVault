-- +goose Up
ALTER TABLE telegram_account_limits
    ADD COLUMN detected_document_limit_bytes BIGINT,
    ADD COLUMN last_probe_status TEXT,
    ADD COLUMN last_probe_error TEXT,
    ADD COLUMN last_probed_at TIMESTAMPTZ,
    ADD COLUMN next_probe_at TIMESTAMPTZ,
    ADD CONSTRAINT telegram_account_limits_positive_detected_limit CHECK (
        detected_document_limit_bytes IS NULL OR detected_document_limit_bytes > 0
    ),
    ADD CONSTRAINT telegram_account_limits_probe_status_check CHECK (
        last_probe_status IS NULL OR last_probe_status IN ('pending', 'success', 'failed', 'manual')
    );

-- +goose Down
ALTER TABLE telegram_account_limits
    DROP CONSTRAINT IF EXISTS telegram_account_limits_probe_status_check,
    DROP CONSTRAINT IF EXISTS telegram_account_limits_positive_detected_limit,
    DROP COLUMN IF EXISTS next_probe_at,
    DROP COLUMN IF EXISTS last_probed_at,
    DROP COLUMN IF EXISTS last_probe_error,
    DROP COLUMN IF EXISTS last_probe_status,
    DROP COLUMN IF EXISTS detected_document_limit_bytes;

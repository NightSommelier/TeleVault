-- +goose Up
ALTER TABLE admin_settings
    ADD COLUMN max_parallel_uploads INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN target_upload_bytes_per_second BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN cooldown_between_parts_ms INTEGER NOT NULL DEFAULT 0,
    ADD CONSTRAINT admin_settings_positive_parallel_uploads CHECK (max_parallel_uploads > 0),
    ADD CONSTRAINT admin_settings_nonnegative_target_upload_rate CHECK (target_upload_bytes_per_second >= 0),
    ADD CONSTRAINT admin_settings_nonnegative_upload_cooldown CHECK (cooldown_between_parts_ms >= 0);

ALTER TABLE telegram_account_limits
    ADD COLUMN max_parallel_uploads INTEGER,
    ADD COLUMN target_upload_bytes_per_second BIGINT,
    ADD COLUMN cooldown_between_parts_ms INTEGER,
    ADD CONSTRAINT telegram_account_limits_positive_parallel_uploads CHECK (max_parallel_uploads IS NULL OR max_parallel_uploads > 0),
    ADD CONSTRAINT telegram_account_limits_nonnegative_target_upload_rate CHECK (target_upload_bytes_per_second IS NULL OR target_upload_bytes_per_second >= 0),
    ADD CONSTRAINT telegram_account_limits_nonnegative_upload_cooldown CHECK (cooldown_between_parts_ms IS NULL OR cooldown_between_parts_ms >= 0);

-- +goose Down
ALTER TABLE telegram_account_limits
    DROP CONSTRAINT IF EXISTS telegram_account_limits_nonnegative_upload_cooldown,
    DROP CONSTRAINT IF EXISTS telegram_account_limits_nonnegative_target_upload_rate,
    DROP CONSTRAINT IF EXISTS telegram_account_limits_positive_parallel_uploads,
    DROP COLUMN IF EXISTS cooldown_between_parts_ms,
    DROP COLUMN IF EXISTS target_upload_bytes_per_second,
    DROP COLUMN IF EXISTS max_parallel_uploads;

ALTER TABLE admin_settings
    DROP CONSTRAINT IF EXISTS admin_settings_nonnegative_upload_cooldown,
    DROP CONSTRAINT IF EXISTS admin_settings_nonnegative_target_upload_rate,
    DROP CONSTRAINT IF EXISTS admin_settings_positive_parallel_uploads,
    DROP COLUMN IF EXISTS cooldown_between_parts_ms,
    DROP COLUMN IF EXISTS target_upload_bytes_per_second,
    DROP COLUMN IF EXISTS max_parallel_uploads;

-- +goose Up
CREATE TABLE admin_settings (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE,
    upload_part_size_bytes BIGINT NOT NULL,
    telegram_document_limit_bytes BIGINT NOT NULL,
    upload_safety_margin_bytes BIGINT NOT NULL,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT admin_settings_singleton CHECK (id),
    CONSTRAINT admin_settings_positive_part_size CHECK (upload_part_size_bytes > 0),
    CONSTRAINT admin_settings_positive_document_limit CHECK (telegram_document_limit_bytes > 0),
    CONSTRAINT admin_settings_nonnegative_margin CHECK (upload_safety_margin_bytes >= 0),
    CONSTRAINT admin_settings_safe_part_size CHECK (upload_part_size_bytes + upload_safety_margin_bytes <= telegram_document_limit_bytes)
);

INSERT INTO admin_settings (
    id,
    upload_part_size_bytes,
    telegram_document_limit_bytes,
    upload_safety_margin_bytes
)
VALUES (TRUE, 67108864, 2147483648, 67108864);

CREATE TABLE telegram_account_limits (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    telegram_document_limit_bytes BIGINT NOT NULL,
    upload_safety_margin_bytes BIGINT NOT NULL,
    is_premium BOOLEAN NOT NULL DEFAULT FALSE,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT telegram_account_limits_positive_document_limit CHECK (telegram_document_limit_bytes > 0),
    CONSTRAINT telegram_account_limits_nonnegative_margin CHECK (upload_safety_margin_bytes >= 0),
    CONSTRAINT telegram_account_limits_safe_margin CHECK (upload_safety_margin_bytes < telegram_document_limit_bytes)
);

-- +goose Down
DROP TABLE IF EXISTS telegram_account_limits;
DROP TABLE IF EXISTS admin_settings;

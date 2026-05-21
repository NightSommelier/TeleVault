-- +goose Up
ALTER TABLE admin_settings
    ADD COLUMN public_link_password_min_length INTEGER NOT NULL DEFAULT 8,
    ADD CONSTRAINT admin_settings_public_link_password_min_length_check
        CHECK (public_link_password_min_length >= 1 AND public_link_password_min_length <= 1024);

-- +goose Down
ALTER TABLE admin_settings
    DROP CONSTRAINT IF EXISTS admin_settings_public_link_password_min_length_check,
    DROP COLUMN IF EXISTS public_link_password_min_length;

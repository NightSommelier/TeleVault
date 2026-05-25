-- +goose Up
ALTER TABLE admin_settings
    ADD COLUMN instance_id UUID NOT NULL DEFAULT gen_random_uuid();

-- +goose Down
ALTER TABLE admin_settings
    DROP COLUMN IF EXISTS instance_id;

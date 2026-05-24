-- +goose Up
ALTER TABLE public_links
    ADD COLUMN IF NOT EXISTS show_checksum BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE public_links
    DROP COLUMN IF EXISTS show_checksum;

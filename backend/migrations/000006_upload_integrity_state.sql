-- +goose Up
ALTER TABLE uploads
    ADD COLUMN checksum_state BYTEA,
    ADD COLUMN plaintext_uploaded_size BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN next_part_number INTEGER NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE uploads
    DROP COLUMN IF EXISTS next_part_number,
    DROP COLUMN IF EXISTS plaintext_uploaded_size,
    DROP COLUMN IF EXISTS checksum_state;

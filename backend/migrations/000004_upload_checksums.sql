-- +goose Up
ALTER TABLE uploads
    ADD COLUMN checksum_algorithm TEXT,
    ADD COLUMN checksum BYTEA;

ALTER TABLE uploads
    ADD CONSTRAINT uploads_checksum_algorithm_check
    CHECK (checksum_algorithm IS NULL OR checksum_algorithm IN ('sha256'));

-- +goose Down
ALTER TABLE uploads
    DROP CONSTRAINT IF EXISTS uploads_checksum_algorithm_check;

ALTER TABLE uploads
    DROP COLUMN IF EXISTS checksum,
    DROP COLUMN IF EXISTS checksum_algorithm;

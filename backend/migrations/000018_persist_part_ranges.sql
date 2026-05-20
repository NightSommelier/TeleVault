-- +goose Up
ALTER TABLE upload_parts
    ADD COLUMN plaintext_start BIGINT,
    ADD COLUMN plaintext_end BIGINT;

ALTER TABLE file_parts
    ADD COLUMN plaintext_start BIGINT,
    ADD COLUMN plaintext_end BIGINT,
    ADD COLUMN plaintext_size BIGINT;

WITH planned AS (
    SELECT
        id,
        COALESCE(
            SUM(COALESCE(plaintext_size, 0)) OVER (
                PARTITION BY upload_id
                ORDER BY part_number
                ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING
            ),
            0
        ) AS computed_start,
        COALESCE(
            SUM(COALESCE(plaintext_size, 0)) OVER (
                PARTITION BY upload_id
                ORDER BY part_number
                ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
            ),
            0
        ) AS computed_end
    FROM upload_parts
    WHERE plaintext_size IS NOT NULL
)
UPDATE upload_parts p
SET plaintext_start = planned.computed_start,
    plaintext_end = planned.computed_end
FROM planned
WHERE p.id = planned.id;

-- +goose Down
ALTER TABLE file_parts
    DROP COLUMN IF EXISTS plaintext_size,
    DROP COLUMN IF EXISTS plaintext_end,
    DROP COLUMN IF EXISTS plaintext_start;

ALTER TABLE upload_parts
    DROP COLUMN IF EXISTS plaintext_end,
    DROP COLUMN IF EXISTS plaintext_start;

-- +goose Up
UPDATE admin_settings
SET upload_part_size_bytes = 402653184,
    updated_at = now()
WHERE id = TRUE
  AND upload_part_size_bytes = 67108864;

-- +goose Down
UPDATE admin_settings
SET upload_part_size_bytes = 67108864,
    updated_at = now()
WHERE id = TRUE
  AND upload_part_size_bytes = 402653184;

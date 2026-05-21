-- +goose Up
ALTER TABLE public_links
    ADD COLUMN max_downloads BIGINT,
    ADD COLUMN download_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN active_download_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN download_limit_mode TEXT NOT NULL DEFAULT 'hard',
    ADD CONSTRAINT public_links_max_downloads_check CHECK (max_downloads IS NULL OR max_downloads > 0),
    ADD CONSTRAINT public_links_download_count_check CHECK (download_count >= 0),
    ADD CONSTRAINT public_links_active_download_count_check CHECK (active_download_count >= 0),
    ADD CONSTRAINT public_links_download_limit_mode_check CHECK (download_limit_mode IN ('hard', 'soft'));

-- +goose Down
ALTER TABLE public_links
    DROP CONSTRAINT IF EXISTS public_links_download_limit_mode_check,
    DROP CONSTRAINT IF EXISTS public_links_active_download_count_check,
    DROP CONSTRAINT IF EXISTS public_links_download_count_check,
    DROP CONSTRAINT IF EXISTS public_links_max_downloads_check,
    DROP COLUMN IF EXISTS download_limit_mode,
    DROP COLUMN IF EXISTS active_download_count,
    DROP COLUMN IF EXISTS download_count,
    DROP COLUMN IF EXISTS max_downloads;

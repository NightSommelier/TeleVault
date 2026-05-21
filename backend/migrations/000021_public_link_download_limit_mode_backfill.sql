-- +goose Up
ALTER TABLE public_links
    ADD COLUMN IF NOT EXISTS active_download_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS download_limit_mode TEXT NOT NULL DEFAULT 'hard';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'public_links_active_download_count_check'
    ) THEN
        ALTER TABLE public_links
            ADD CONSTRAINT public_links_active_download_count_check CHECK (active_download_count >= 0);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'public_links_download_limit_mode_check'
    ) THEN
        ALTER TABLE public_links
            ADD CONSTRAINT public_links_download_limit_mode_check CHECK (download_limit_mode IN ('hard', 'soft'));
    END IF;
END $$;

-- +goose Down
ALTER TABLE public_links
    DROP CONSTRAINT IF EXISTS public_links_download_limit_mode_check,
    DROP CONSTRAINT IF EXISTS public_links_active_download_count_check,
    DROP COLUMN IF EXISTS download_limit_mode,
    DROP COLUMN IF EXISTS active_download_count;

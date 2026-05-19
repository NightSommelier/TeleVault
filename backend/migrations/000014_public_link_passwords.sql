-- +goose Up
ALTER TABLE public_links
    ADD COLUMN password_kdf TEXT,
    ADD COLUMN password_salt BYTEA,
    ADD COLUMN password_hash BYTEA,
    ADD COLUMN password_argon_time INTEGER,
    ADD COLUMN password_argon_memory_kib INTEGER,
    ADD COLUMN password_argon_threads INTEGER,
    ADD CONSTRAINT public_links_password_kdf_check CHECK (password_kdf IS NULL OR password_kdf = 'argon2id'),
    ADD CONSTRAINT public_links_password_fields_check CHECK (
        (
            password_kdf IS NULL
            AND password_salt IS NULL
            AND password_hash IS NULL
            AND password_argon_time IS NULL
            AND password_argon_memory_kib IS NULL
            AND password_argon_threads IS NULL
        )
        OR
        (
            password_kdf IS NOT NULL
            AND password_salt IS NOT NULL
            AND password_hash IS NOT NULL
            AND password_argon_time IS NOT NULL
            AND password_argon_memory_kib IS NOT NULL
            AND password_argon_threads IS NOT NULL
        )
    );

-- +goose Down
ALTER TABLE public_links
    DROP CONSTRAINT IF EXISTS public_links_password_fields_check,
    DROP CONSTRAINT IF EXISTS public_links_password_kdf_check,
    DROP COLUMN IF EXISTS password_argon_threads,
    DROP COLUMN IF EXISTS password_argon_memory_kib,
    DROP COLUMN IF EXISTS password_argon_time,
    DROP COLUMN IF EXISTS password_hash,
    DROP COLUMN IF EXISTS password_salt,
    DROP COLUMN IF EXISTS password_kdf;

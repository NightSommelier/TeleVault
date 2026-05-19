-- +goose Up
CREATE TABLE public_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    permission TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT public_links_permission_check CHECK (permission IN ('read'))
);

CREATE INDEX public_links_owner_file_idx ON public_links (owner_id, file_id, created_at DESC);

CREATE INDEX public_links_active_lookup_idx ON public_links (token_hash)
WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS public_links;

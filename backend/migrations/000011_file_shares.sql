-- +goose Up
CREATE TABLE file_shares (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    grantee_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT file_shares_permission_check CHECK (permission IN ('read')),
    CONSTRAINT file_shares_no_self_share_check CHECK (owner_id <> grantee_user_id)
);

CREATE UNIQUE INDEX file_shares_active_unique_idx ON file_shares (file_id, grantee_user_id)
WHERE revoked_at IS NULL;

CREATE INDEX file_shares_grantee_active_idx ON file_shares (grantee_user_id, created_at DESC)
WHERE revoked_at IS NULL;

CREATE INDEX file_shares_owner_file_idx ON file_shares (owner_id, file_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS file_shares;

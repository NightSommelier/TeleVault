-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_id BIGINT NOT NULL UNIQUE,
    username TEXT,
    display_name TEXT,
    role TEXT NOT NULL DEFAULT 'user',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash BYTEA NOT NULL,
    user_agent TEXT,
    ip_hash BYTEA,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sessions_active_idx ON sessions (user_id, expires_at)
WHERE revoked_at IS NULL;

CREATE TABLE telegram_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    encrypted_session BYTEA NOT NULL,
    storage_peer TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX telegram_sessions_user_id_idx ON telegram_sessions (user_id);

CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES files(id) ON DELETE SET NULL,
    name_plain TEXT,
    name_encrypted BYTEA,
    mime_type TEXT,
    plaintext_size BIGINT,
    ciphertext_size BIGINT,
    type TEXT NOT NULL,
    status TEXT NOT NULL,
    encryption_scheme TEXT,
    checksum BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT files_type_check CHECK (type IN ('file', 'folder')),
    CONSTRAINT files_status_check CHECK (status IN ('pending', 'ready', 'deleted', 'failed'))
);

CREATE INDEX files_owner_parent_active_idx ON files (owner_id, parent_id, name_plain)
WHERE deleted_at IS NULL;

CREATE TABLE file_parts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    part_number INTEGER NOT NULL,
    telegram_peer TEXT NOT NULL,
    telegram_message_id BIGINT NOT NULL,
    ciphertext_size BIGINT NOT NULL,
    checksum BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (file_id, part_number)
);

CREATE TABLE uploads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES files(id) ON DELETE SET NULL,
    name_plain TEXT NOT NULL,
    mime_type TEXT,
    plaintext_size BIGINT,
    part_size BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    idempotency_key TEXT,
    error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT uploads_status_check CHECK (status IN ('pending', 'uploading', 'complete', 'failed', 'expired'))
);

CREATE UNIQUE INDEX uploads_owner_idempotency_idx ON uploads (owner_id, idempotency_key)
WHERE idempotency_key IS NOT NULL;

CREATE INDEX uploads_expiry_idx ON uploads (expires_at)
WHERE status IN ('pending', 'uploading');

CREATE TABLE upload_parts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    upload_id UUID NOT NULL REFERENCES uploads(id) ON DELETE CASCADE,
    part_number INTEGER NOT NULL,
    plaintext_size BIGINT,
    ciphertext_size BIGINT,
    checksum BYTEA,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (upload_id, part_number),
    CONSTRAINT upload_parts_status_check CHECK (status IN ('pending', 'complete', 'failed'))
);

CREATE TABLE file_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    recipient_type TEXT NOT NULL,
    recipient_id TEXT,
    encrypted_key BYTEA NOT NULL,
    algorithm TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX file_keys_file_id_idx ON file_keys (file_id);

CREATE TABLE audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    resource_type TEXT,
    resource_id UUID,
    ip_hash BYTEA,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX audit_events_actor_created_idx ON audit_events (actor_user_id, created_at DESC);
CREATE INDEX audit_events_resource_idx ON audit_events (resource_type, resource_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS file_keys;
DROP TABLE IF EXISTS upload_parts;
DROP TABLE IF EXISTS uploads;
DROP TABLE IF EXISTS file_parts;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS telegram_sessions;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;

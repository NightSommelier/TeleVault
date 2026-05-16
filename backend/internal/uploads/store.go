package uploads

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrParentNotFound = errors.New("parent folder not found")
var ErrUploadNotFound = errors.New("upload not found")
var ErrUploadExpired = errors.New("upload expired")
var ErrTelegramSessionNotFound = errors.New("telegram session not found")
var ErrUploadIncomplete = errors.New("upload incomplete")
var ErrUploadSizeMismatch = errors.New("upload size mismatch")
var ErrUploadChecksumMismatch = errors.New("upload checksum mismatch")
var ErrMultipartChecksumUnsupported = errors.New("multipart checksum verification is not supported yet")

const (
	StatusPending   = "pending"
	StatusUploading = "uploading"
	StatusComplete  = "complete"
)

type Upload struct {
	ID                string
	OwnerID           string
	ParentID          sql.NullString
	NamePlain         string
	MimeType          sql.NullString
	PlaintextSize     sql.NullInt64
	PartSize          int64
	Status            string
	IdempotencyKey    sql.NullString
	ChecksumAlgorithm sql.NullString
	Checksum          []byte
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ExpiresAt         time.Time
}

type UploadPart struct {
	ID             string
	UploadID       string
	PartNumber     int
	PlaintextSize  sql.NullInt64
	CiphertextSize sql.NullInt64
	Checksum       []byte
	TelegramPeer   sql.NullString
	MessageID      sql.NullInt64
	Status         string
	CreatedAt      time.Time
}

type TelegramSession struct {
	EncryptedSession []byte
	StoragePeer      sql.NullString
}

type File struct {
	ID             string
	OwnerID        string
	ParentID       sql.NullString
	NamePlain      sql.NullString
	MimeType       sql.NullString
	PlaintextSize  sql.NullInt64
	CiphertextSize sql.NullInt64
	Type           string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateUploadParams struct {
	OwnerID           string
	ParentID          string
	Name              string
	MimeType          string
	PlaintextSize     int64
	PartSize          int64
	IdempotencyKey    string
	ChecksumAlgorithm string
	Checksum          []byte
	ExpiresAt         time.Time
}

type CompletePartParams struct {
	OwnerID        string
	UploadID       string
	PartNumber     int
	PlaintextSize  int64
	CiphertextSize int64
	Checksum       []byte
	TelegramPeer   string
	MessageID      int64
	Now            time.Time
}

type CompleteUploadParams struct {
	OwnerID  string
	UploadID string
	Now      time.Time
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(ctx context.Context, params CreateUploadParams) (Upload, error) {
	if params.ParentID != "" {
		if err := s.ensureParentFolder(ctx, params.OwnerID, params.ParentID); err != nil {
			return Upload{}, err
		}
	}

	if params.IdempotencyKey != "" {
		return s.createIdempotent(ctx, params)
	}

	return scanUpload(s.db.QueryRowContext(ctx, `
INSERT INTO uploads (
    owner_id, parent_id, name_plain, mime_type, plaintext_size, part_size,
    status, checksum_algorithm, checksum, expires_at
)
VALUES ($1, NULLIF($2, '')::uuid, $3, NULLIF($4, ''), $5, $6, 'pending', NULLIF($7, ''), $8, $9)
RETURNING id, owner_id, parent_id, name_plain, mime_type, plaintext_size, part_size, status,
          idempotency_key, checksum_algorithm, checksum, created_at, updated_at, expires_at`,
		params.OwnerID,
		params.ParentID,
		params.Name,
		params.MimeType,
		params.PlaintextSize,
		params.PartSize,
		params.ChecksumAlgorithm,
		nullableBytes(params.Checksum),
		params.ExpiresAt,
	))
}

func (s *Store) CompletePart(ctx context.Context, params CompletePartParams) (UploadPart, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return UploadPart{}, err
	}
	defer tx.Rollback()

	var expiresAt time.Time
	err = tx.QueryRowContext(ctx, `
SELECT expires_at
FROM uploads
WHERE id = $1
  AND owner_id = $2
  AND status IN ('pending', 'uploading')`,
		params.UploadID,
		params.OwnerID,
	).Scan(&expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return UploadPart{}, ErrUploadNotFound
	}
	if err != nil {
		return UploadPart{}, err
	}
	if !expiresAt.After(params.Now) {
		if _, err := tx.ExecContext(ctx, `
UPDATE uploads
SET status = 'expired', updated_at = now()
WHERE id = $1
  AND owner_id = $2
  AND status IN ('pending', 'uploading')`,
			params.UploadID,
			params.OwnerID,
		); err != nil {
			return UploadPart{}, err
		}
		return UploadPart{}, ErrUploadExpired
	}

	part, err := scanUploadPart(tx.QueryRowContext(ctx, `
INSERT INTO upload_parts (upload_id, part_number, plaintext_size, ciphertext_size, checksum, telegram_peer, telegram_message_id, status)
VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7, 'complete')
ON CONFLICT (upload_id, part_number)
DO UPDATE SET
    plaintext_size = EXCLUDED.plaintext_size,
    ciphertext_size = EXCLUDED.ciphertext_size,
    checksum = EXCLUDED.checksum,
    telegram_peer = EXCLUDED.telegram_peer,
    telegram_message_id = EXCLUDED.telegram_message_id,
    status = 'complete'
RETURNING id, upload_id, part_number, plaintext_size, ciphertext_size, checksum, telegram_peer, telegram_message_id, status, created_at`,
		params.UploadID,
		params.PartNumber,
		params.PlaintextSize,
		params.CiphertextSize,
		nullableBytes(params.Checksum),
		params.TelegramPeer,
		nullableInt64Param(params.MessageID),
	))
	if err != nil {
		return UploadPart{}, err
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE uploads
SET status = 'uploading', updated_at = now()
WHERE id = $1
  AND owner_id = $2
  AND status = 'pending'`,
		params.UploadID,
		params.OwnerID,
	); err != nil {
		return UploadPart{}, err
	}

	if err := tx.Commit(); err != nil {
		return UploadPart{}, err
	}

	return part, nil
}

func (s *Store) CompleteUpload(ctx context.Context, params CompleteUploadParams) (File, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return File{}, err
	}
	defer tx.Rollback()

	upload, err := scanUpload(tx.QueryRowContext(ctx, `
SELECT id, owner_id, parent_id, name_plain, mime_type, plaintext_size, part_size, status,
       idempotency_key, checksum_algorithm, checksum, created_at, updated_at, expires_at
FROM uploads
WHERE id = $1
  AND owner_id = $2
  AND status IN ('pending', 'uploading')
FOR UPDATE`,
		params.UploadID,
		params.OwnerID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, ErrUploadNotFound
	}
	if err != nil {
		return File{}, err
	}
	if !upload.ExpiresAt.After(params.Now) {
		if _, err := tx.ExecContext(ctx, `
UPDATE uploads
SET status = 'expired', updated_at = now()
WHERE id = $1
  AND owner_id = $2`,
			params.UploadID,
			params.OwnerID,
		); err != nil {
			return File{}, err
		}
		return File{}, ErrUploadExpired
	}

	plaintextSize := nullableInt64(upload.PlaintextSize)
	expectedParts := partCount(plaintextSize, upload.PartSize)
	parts, err := completeParts(ctx, tx, upload.ID)
	if err != nil {
		return File{}, err
	}
	if int64(len(parts)) != expectedParts {
		return File{}, ErrUploadIncomplete
	}

	var plaintextTotal int64
	var ciphertextTotal int64
	for i, part := range parts {
		expectedPartNumber := i + 1
		if part.PartNumber != expectedPartNumber || !part.PlaintextSize.Valid || !part.CiphertextSize.Valid || !part.TelegramPeer.Valid || !part.MessageID.Valid {
			return File{}, ErrUploadIncomplete
		}
		plaintextTotal += part.PlaintextSize.Int64
		ciphertextTotal += part.CiphertextSize.Int64
	}
	if plaintextTotal != plaintextSize {
		return File{}, ErrUploadSizeMismatch
	}

	if len(upload.Checksum) > 0 {
		if expectedParts != 1 {
			return File{}, ErrMultipartChecksumUnsupported
		}
		if len(parts) != 1 || !bytesEqual(parts[0].Checksum, upload.Checksum) {
			return File{}, ErrUploadChecksumMismatch
		}
	}

	file, err := scanFile(tx.QueryRowContext(ctx, `
INSERT INTO files (
    owner_id, parent_id, name_plain, mime_type, plaintext_size, ciphertext_size,
    type, status, encryption_scheme, checksum
)
VALUES ($1, $2, $3, $4, $5, $6, 'file', 'ready', 'age-x25519', $7)
RETURNING id, owner_id, parent_id, name_plain, mime_type, plaintext_size, ciphertext_size, type, status, created_at, updated_at`,
		upload.OwnerID,
		upload.ParentID,
		upload.NamePlain,
		upload.MimeType,
		plaintextSize,
		ciphertextTotal,
		nullableBytes(upload.Checksum),
	))
	if err != nil {
		return File{}, err
	}

	for _, part := range parts {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO file_parts (file_id, part_number, telegram_peer, telegram_message_id, ciphertext_size, checksum)
VALUES ($1, $2, $3, $4, $5, $6)`,
			file.ID,
			part.PartNumber,
			part.TelegramPeer.String,
			part.MessageID.Int64,
			part.CiphertextSize.Int64,
			nullableBytes(part.Checksum),
		); err != nil {
			return File{}, err
		}
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE uploads
SET status = 'complete', updated_at = now()
WHERE id = $1
  AND owner_id = $2`,
		upload.ID,
		upload.OwnerID,
	); err != nil {
		return File{}, err
	}

	if err := tx.Commit(); err != nil {
		return File{}, err
	}

	return file, nil
}

func (s *Store) EnsureActiveUpload(ctx context.Context, ownerID string, uploadID string, now time.Time) error {
	var expiresAt time.Time
	err := s.db.QueryRowContext(ctx, `
SELECT expires_at
FROM uploads
WHERE id = $1
  AND owner_id = $2
  AND status IN ('pending', 'uploading')`,
		uploadID,
		ownerID,
	).Scan(&expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUploadNotFound
	}
	if err != nil {
		return err
	}
	if !expiresAt.After(now) {
		return ErrUploadExpired
	}
	return nil
}

func (s *Store) TelegramSession(ctx context.Context, ownerID string) (TelegramSession, error) {
	var session TelegramSession
	err := s.db.QueryRowContext(ctx, `
SELECT encrypted_session, storage_peer
FROM telegram_sessions
WHERE user_id = $1`,
		ownerID,
	).Scan(&session.EncryptedSession, &session.StoragePeer)
	if errors.Is(err, sql.ErrNoRows) {
		return TelegramSession{}, ErrTelegramSessionNotFound
	}
	if err != nil {
		return TelegramSession{}, err
	}
	return session, nil
}

func (s *Store) createIdempotent(ctx context.Context, params CreateUploadParams) (Upload, error) {
	return scanUpload(s.db.QueryRowContext(ctx, `
INSERT INTO uploads (
    owner_id, parent_id, name_plain, mime_type, plaintext_size, part_size,
    status, idempotency_key, checksum_algorithm, checksum, expires_at
)
VALUES ($1, NULLIF($2, '')::uuid, $3, NULLIF($4, ''), $5, $6, 'pending', $7, NULLIF($8, ''), $9, $10)
ON CONFLICT (owner_id, idempotency_key) WHERE idempotency_key IS NOT NULL
DO UPDATE SET updated_at = uploads.updated_at
RETURNING id, owner_id, parent_id, name_plain, mime_type, plaintext_size, part_size, status,
          idempotency_key, checksum_algorithm, checksum, created_at, updated_at, expires_at`,
		params.OwnerID,
		params.ParentID,
		params.Name,
		params.MimeType,
		params.PlaintextSize,
		params.PartSize,
		params.IdempotencyKey,
		params.ChecksumAlgorithm,
		nullableBytes(params.Checksum),
		params.ExpiresAt,
	))
}

func (s *Store) ensureParentFolder(ctx context.Context, ownerID string, parentID string) error {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM files
    WHERE owner_id = $1
      AND id = $2
      AND type = 'folder'
      AND deleted_at IS NULL
)`,
		ownerID,
		parentID,
	).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrParentNotFound
	}

	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func scanUpload(row rowScanner) (Upload, error) {
	var upload Upload
	err := row.Scan(
		&upload.ID,
		&upload.OwnerID,
		&upload.ParentID,
		&upload.NamePlain,
		&upload.MimeType,
		&upload.PlaintextSize,
		&upload.PartSize,
		&upload.Status,
		&upload.IdempotencyKey,
		&upload.ChecksumAlgorithm,
		&upload.Checksum,
		&upload.CreatedAt,
		&upload.UpdatedAt,
		&upload.ExpiresAt,
	)
	if err != nil {
		return Upload{}, err
	}
	return upload, nil
}

func scanUploadPart(row rowScanner) (UploadPart, error) {
	var part UploadPart
	err := row.Scan(
		&part.ID,
		&part.UploadID,
		&part.PartNumber,
		&part.PlaintextSize,
		&part.CiphertextSize,
		&part.Checksum,
		&part.TelegramPeer,
		&part.MessageID,
		&part.Status,
		&part.CreatedAt,
	)
	if err != nil {
		return UploadPart{}, err
	}
	return part, nil
}

func completeParts(ctx context.Context, q queryer, uploadID string) ([]UploadPart, error) {
	rows, err := q.QueryContext(ctx, `
SELECT id, upload_id, part_number, plaintext_size, ciphertext_size, checksum, telegram_peer, telegram_message_id, status, created_at
FROM upload_parts
WHERE upload_id = $1
  AND status = 'complete'
ORDER BY part_number ASC`,
		uploadID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var parts []UploadPart
	for rows.Next() {
		part, err := scanUploadPart(rows)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return parts, nil
}

func scanFile(row rowScanner) (File, error) {
	var file File
	err := row.Scan(
		&file.ID,
		&file.OwnerID,
		&file.ParentID,
		&file.NamePlain,
		&file.MimeType,
		&file.PlaintextSize,
		&file.CiphertextSize,
		&file.Type,
		&file.Status,
		&file.CreatedAt,
		&file.UpdatedAt,
	)
	if err != nil {
		return File{}, err
	}
	return file, nil
}

func nullableBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func nullableInt64Param(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}

func partCount(size int64, partSize int64) int64 {
	if size == 0 {
		return 0
	}
	return (size + partSize - 1) / partSize
}

func nullableInt64(value sql.NullInt64) int64 {
	if !value.Valid {
		return 0
	}
	return value.Int64
}

func bytesEqual(a []byte, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := range a {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

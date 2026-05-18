package uploads

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding"
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
var ErrUploadPartOutOfOrder = errors.New("upload part out of order")
var ErrUploadPartNotFound = errors.New("upload part not found")

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
	ChecksumState     []byte
	UploadedSize      int64
	NextPartNumber    int
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
	StorageBackend sql.NullString
	StorageKey     sql.NullString
	AvailableAt    time.Time
	LeasedUntil    sql.NullTime
	Attempts       int
	LastError      sql.NullString
	WorkerID       sql.NullString
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type TelegramCleanupArtifact struct {
	Source           string
	PartID           string
	ResourceID       string
	OwnerID          string
	OwnerTelegramID  int64
	EncryptedSession []byte
	StoragePeer      sql.NullString
	TelegramPeer     sql.NullString
	MessageID        int64
}

type LocalStagingCleanupArtifact struct {
	PartID     string
	UploadID   string
	StorageKey string
}

type TelegramSession struct {
	EncryptedSession []byte
	StoragePeer      sql.NullString
}

type QueuedPartWork struct {
	Part             UploadPart
	OwnerID          string
	OwnerTelegramID  int64
	EncryptedSession []byte
	StoragePeer      sql.NullString
	UploadName       string
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
	ChecksumState  []byte
	UploadedSize   int64
	TelegramPeer   string
	MessageID      int64
	Now            time.Time
}

type StagePartParams struct {
	OwnerID        string
	UploadID       string
	PartNumber     int
	PlaintextSize  int64
	CiphertextSize int64
	Checksum       []byte
	ChecksumState  []byte
	UploadedSize   int64
	StorageBackend string
	StorageKey     string
	AvailableAt    time.Time
	Now            time.Time
}

type ClaimQueuedPartParams struct {
	WorkerID      string
	Now           time.Time
	LeaseDuration time.Duration
}

type RetryPartParams struct {
	PartID      string
	LastError   string
	AvailableAt time.Time
}

type MarkStagedPartUploadedParams struct {
	PartID       string
	TelegramPeer string
	MessageID    int64
}

type CompleteUploadParams struct {
	OwnerID  string
	UploadID string
	Now      time.Time
}

type CleanupResult struct {
	ExpiredUploads           int64
	TelegramArtifactsDeleted int64
	TelegramArtifactsFailed  int64
	LocalStagingDeleted      int64
	LocalStagingFailed       int64
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
          idempotency_key, checksum_algorithm, checksum, checksum_state, plaintext_uploaded_size,
          next_part_number, created_at, updated_at, expires_at`,
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
	var nextPartNumber int
	err = tx.QueryRowContext(ctx, `
SELECT expires_at, next_part_number
FROM uploads
WHERE id = $1
  AND owner_id = $2
  AND status IN ('pending', 'uploading')`,
		params.UploadID,
		params.OwnerID,
	).Scan(&expiresAt, &nextPartNumber)
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
	if nextPartNumber != params.PartNumber {
		return UploadPart{}, ErrUploadPartOutOfOrder
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
    status = 'complete',
    leased_until = NULL,
    last_error = NULL,
    worker_id = NULL,
    updated_at = now()
WHERE upload_parts.status != 'complete'
RETURNING id, upload_id, part_number, plaintext_size, ciphertext_size, checksum, telegram_peer, telegram_message_id,
          status, storage_backend, storage_key, available_at, leased_until, attempts, last_error, worker_id, created_at, updated_at`,
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

	result, err := tx.ExecContext(ctx, `
UPDATE uploads
SET status = 'uploading',
    checksum_state = $3,
    plaintext_uploaded_size = $4,
    next_part_number = $5,
    updated_at = now()
WHERE id = $1
  AND owner_id = $2
  AND status IN ('pending', 'uploading')
  AND next_part_number = $6`,
		params.UploadID,
		params.OwnerID,
		nullableBytes(params.ChecksumState),
		params.UploadedSize,
		params.PartNumber+1,
		params.PartNumber,
	)
	if err != nil {
		return UploadPart{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return UploadPart{}, err
	}
	if rows == 0 {
		return UploadPart{}, ErrUploadPartOutOfOrder
	}

	if err := tx.Commit(); err != nil {
		return UploadPart{}, err
	}

	return part, nil
}

func (s *Store) StagePart(ctx context.Context, params StagePartParams) (UploadPart, error) {
	if params.StorageBackend == "" || params.StorageKey == "" {
		return UploadPart{}, errors.New("storage backend and key are required")
	}
	if params.AvailableAt.IsZero() {
		params.AvailableAt = params.Now
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return UploadPart{}, err
	}
	defer tx.Rollback()

	var expiresAt time.Time
	var nextPartNumber int
	err = tx.QueryRowContext(ctx, `
SELECT expires_at, next_part_number
FROM uploads
WHERE id = $1
  AND owner_id = $2
  AND status IN ('pending', 'uploading')`,
		params.UploadID,
		params.OwnerID,
	).Scan(&expiresAt, &nextPartNumber)
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
	if nextPartNumber != params.PartNumber {
		return UploadPart{}, ErrUploadPartOutOfOrder
	}
	part, err := scanUploadPart(tx.QueryRowContext(ctx, `
INSERT INTO upload_parts (
    upload_id, part_number, plaintext_size, ciphertext_size, checksum, status,
    storage_backend, storage_key, available_at
)
VALUES ($1, $2, $3, $4, $5, 'pending', $6, $7, $8)
ON CONFLICT (upload_id, part_number)
DO UPDATE SET
    plaintext_size = EXCLUDED.plaintext_size,
    ciphertext_size = EXCLUDED.ciphertext_size,
    checksum = EXCLUDED.checksum,
    status = 'pending',
    storage_backend = EXCLUDED.storage_backend,
    storage_key = EXCLUDED.storage_key,
    available_at = EXCLUDED.available_at,
    leased_until = NULL,
    last_error = NULL,
    worker_id = NULL,
    updated_at = now()
WHERE upload_parts.status != 'complete'
RETURNING id, upload_id, part_number, plaintext_size, ciphertext_size, checksum, telegram_peer, telegram_message_id,
          status, storage_backend, storage_key, available_at, leased_until, attempts, last_error, worker_id, created_at, updated_at`,
		params.UploadID,
		params.PartNumber,
		params.PlaintextSize,
		params.CiphertextSize,
		nullableBytes(params.Checksum),
		params.StorageBackend,
		params.StorageKey,
		params.AvailableAt,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return UploadPart{}, ErrUploadPartOutOfOrder
	}
	if err != nil {
		return UploadPart{}, err
	}

	result, err := tx.ExecContext(ctx, `
UPDATE uploads
SET status = 'uploading',
    checksum_state = $3,
    plaintext_uploaded_size = $4,
    next_part_number = $5,
    updated_at = now()
WHERE id = $1
  AND owner_id = $2
  AND status IN ('pending', 'uploading')
  AND next_part_number = $6`,
		params.UploadID,
		params.OwnerID,
		nullableBytes(params.ChecksumState),
		params.UploadedSize,
		params.PartNumber+1,
		params.PartNumber,
	)
	if err != nil {
		return UploadPart{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return UploadPart{}, err
	}
	if rows == 0 {
		return UploadPart{}, ErrUploadPartOutOfOrder
	}

	if err := tx.Commit(); err != nil {
		return UploadPart{}, err
	}

	return part, nil
}

func (s *Store) MarkStagedPartUploaded(ctx context.Context, params MarkStagedPartUploadedParams) (UploadPart, error) {
	part, err := scanUploadPart(s.db.QueryRowContext(ctx, `
UPDATE upload_parts
SET status = 'complete',
    telegram_peer = NULLIF($2, ''),
    telegram_message_id = $3,
    leased_until = NULL,
    last_error = NULL,
    worker_id = NULL,
    updated_at = now()
WHERE id = $1
  AND status = 'pending'
RETURNING id, upload_id, part_number, plaintext_size, ciphertext_size, checksum, telegram_peer, telegram_message_id,
          status, storage_backend, storage_key, available_at, leased_until, attempts, last_error, worker_id, created_at, updated_at`,
		params.PartID,
		params.TelegramPeer,
		nullableInt64Param(params.MessageID),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return UploadPart{}, ErrUploadPartNotFound
	}
	if err != nil {
		return UploadPart{}, err
	}
	return part, nil
}

func (s *Store) ClaimQueuedPart(ctx context.Context, params ClaimQueuedPartParams) (UploadPart, error) {
	if params.WorkerID == "" {
		return UploadPart{}, errors.New("worker id is required")
	}
	if params.LeaseDuration <= 0 {
		return UploadPart{}, errors.New("lease duration must be positive")
	}

	part := UploadPart{}
	err := s.db.QueryRowContext(ctx, `
WITH next_part AS (
    SELECT p.id
    FROM upload_parts p
    JOIN uploads u ON u.id = p.upload_id
    WHERE p.status = 'pending'
      AND p.storage_backend IS NOT NULL
      AND p.storage_key IS NOT NULL
      AND p.available_at <= $1
      AND (p.leased_until IS NULL OR p.leased_until <= $1)
      AND u.status IN ('pending', 'uploading')
      AND u.expires_at > $1
    ORDER BY p.available_at ASC, p.created_at ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE upload_parts p
SET leased_until = $2,
    attempts = p.attempts + 1,
    last_error = NULL,
    worker_id = $3,
    updated_at = now()
FROM next_part
WHERE p.id = next_part.id
RETURNING p.id, p.upload_id, p.part_number, p.plaintext_size, p.ciphertext_size, p.checksum,
          p.telegram_peer, p.telegram_message_id, p.status, p.storage_backend, p.storage_key,
          p.available_at, p.leased_until, p.attempts, p.last_error, p.worker_id, p.created_at, p.updated_at`,
		params.Now,
		params.Now.Add(params.LeaseDuration),
		params.WorkerID,
	).Scan(
		&part.ID,
		&part.UploadID,
		&part.PartNumber,
		&part.PlaintextSize,
		&part.CiphertextSize,
		&part.Checksum,
		&part.TelegramPeer,
		&part.MessageID,
		&part.Status,
		&part.StorageBackend,
		&part.StorageKey,
		&part.AvailableAt,
		&part.LeasedUntil,
		&part.Attempts,
		&part.LastError,
		&part.WorkerID,
		&part.CreatedAt,
		&part.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return UploadPart{}, ErrUploadPartNotFound
	}
	if err != nil {
		return UploadPart{}, err
	}
	return part, nil
}

func (s *Store) ClaimQueuedPartWork(ctx context.Context, params ClaimQueuedPartParams) (QueuedPartWork, error) {
	if params.WorkerID == "" {
		return QueuedPartWork{}, errors.New("worker id is required")
	}
	if params.LeaseDuration <= 0 {
		return QueuedPartWork{}, errors.New("lease duration must be positive")
	}

	var work QueuedPartWork
	err := s.db.QueryRowContext(ctx, `
WITH next_part AS (
    SELECT p.id
    FROM upload_parts p
    JOIN uploads u ON u.id = p.upload_id
    WHERE p.status = 'pending'
      AND p.storage_backend IS NOT NULL
      AND p.storage_key IS NOT NULL
      AND p.available_at <= $1
      AND (p.leased_until IS NULL OR p.leased_until <= $1)
      AND u.status IN ('pending', 'uploading')
      AND u.expires_at > $1
    ORDER BY p.available_at ASC, p.created_at ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
),
claimed AS (
    UPDATE upload_parts p
    SET leased_until = $2,
        attempts = p.attempts + 1,
        last_error = NULL,
        worker_id = $3,
        updated_at = now()
    FROM next_part
    WHERE p.id = next_part.id
    RETURNING p.id, p.upload_id, p.part_number, p.plaintext_size, p.ciphertext_size, p.checksum,
              p.telegram_peer, p.telegram_message_id, p.status, p.storage_backend, p.storage_key,
              p.available_at, p.leased_until, p.attempts, p.last_error, p.worker_id, p.created_at, p.updated_at
)
SELECT claimed.id, claimed.upload_id, claimed.part_number, claimed.plaintext_size, claimed.ciphertext_size,
       claimed.checksum, claimed.telegram_peer, claimed.telegram_message_id, claimed.status,
       claimed.storage_backend, claimed.storage_key, claimed.available_at, claimed.leased_until,
       claimed.attempts, claimed.last_error, claimed.worker_id, claimed.created_at, claimed.updated_at,
       u.owner_id, users.telegram_id, ts.encrypted_session, ts.storage_peer, u.name_plain
FROM claimed
JOIN uploads u ON u.id = claimed.upload_id
JOIN users ON users.id = u.owner_id
LEFT JOIN telegram_sessions ts ON ts.user_id = u.owner_id`,
		params.Now,
		params.Now.Add(params.LeaseDuration),
		params.WorkerID,
	).Scan(
		&work.Part.ID,
		&work.Part.UploadID,
		&work.Part.PartNumber,
		&work.Part.PlaintextSize,
		&work.Part.CiphertextSize,
		&work.Part.Checksum,
		&work.Part.TelegramPeer,
		&work.Part.MessageID,
		&work.Part.Status,
		&work.Part.StorageBackend,
		&work.Part.StorageKey,
		&work.Part.AvailableAt,
		&work.Part.LeasedUntil,
		&work.Part.Attempts,
		&work.Part.LastError,
		&work.Part.WorkerID,
		&work.Part.CreatedAt,
		&work.Part.UpdatedAt,
		&work.OwnerID,
		&work.OwnerTelegramID,
		&work.EncryptedSession,
		&work.StoragePeer,
		&work.UploadName,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return QueuedPartWork{}, ErrUploadPartNotFound
	}
	if err != nil {
		return QueuedPartWork{}, err
	}
	return work, nil
}

func (s *Store) RetryQueuedPart(ctx context.Context, params RetryPartParams) error {
	message := truncateError(params.LastError)
	_, err := s.db.ExecContext(ctx, `
UPDATE upload_parts
SET available_at = $2,
    leased_until = NULL,
    last_error = NULLIF($3, ''),
    worker_id = NULL,
    updated_at = now()
WHERE id = $1
  AND status = 'pending'`,
		params.PartID,
		params.AvailableAt,
		message,
	)
	return err
}

func (s *Store) FailQueuedPart(ctx context.Context, partID string, failure error) error {
	message := ""
	if failure != nil {
		message = failure.Error()
	}
	message = truncateError(message)
	_, err := s.db.ExecContext(ctx, `
UPDATE upload_parts
SET status = 'failed',
    leased_until = NULL,
    last_error = NULLIF($2, ''),
    worker_id = NULL,
    updated_at = now()
WHERE id = $1`,
		partID,
		message,
	)
	return err
}

func (s *Store) MarkPartFailed(ctx context.Context, ownerID string, uploadID string, partNumber int) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO upload_parts (upload_id, part_number, status)
SELECT id, $3, 'failed'
FROM uploads
WHERE id = $1
  AND owner_id = $2
  AND status IN ('pending', 'uploading')
ON CONFLICT (upload_id, part_number)
DO UPDATE SET
    status = 'failed',
    leased_until = NULL,
    worker_id = NULL,
    updated_at = now()`,
		uploadID,
		ownerID,
		partNumber,
	)
	return err
}

func (s *Store) ExpireAbandoned(ctx context.Context, now time.Time) (CleanupResult, error) {
	result, err := s.db.ExecContext(ctx, `
UPDATE uploads
SET status = 'expired', updated_at = now()
WHERE status IN ('pending', 'uploading')
  AND expires_at <= $1`,
		now,
	)
	if err != nil {
		return CleanupResult{}, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return CleanupResult{}, err
	}

	return CleanupResult{ExpiredUploads: rows}, nil
}

func (s *Store) PendingTelegramCleanupArtifacts(ctx context.Context, limit int) ([]TelegramCleanupArtifact, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT cleanup_source, part_id, resource_id, owner_id, telegram_id, encrypted_session, storage_peer, telegram_peer, telegram_message_id
FROM (
    SELECT 'upload_part' AS cleanup_source, p.id AS part_id, p.upload_id AS resource_id,
           u.owner_id, users.telegram_id, ts.encrypted_session, ts.storage_peer,
           p.telegram_peer, p.telegram_message_id, p.created_at
    FROM upload_parts p
    JOIN uploads u ON u.id = p.upload_id
    JOIN users ON users.id = u.owner_id
    JOIN telegram_sessions ts ON ts.user_id = u.owner_id
    WHERE u.status = 'expired'
      AND p.telegram_message_id IS NOT NULL
      AND p.telegram_deleted_at IS NULL
    UNION ALL
    SELECT 'file_part' AS cleanup_source, p.id AS part_id, p.file_id AS resource_id,
           f.owner_id, users.telegram_id, ts.encrypted_session, ts.storage_peer,
           p.telegram_peer, p.telegram_message_id, p.created_at
    FROM file_parts p
    JOIN files f ON f.id = p.file_id
    JOIN users ON users.id = f.owner_id
    JOIN telegram_sessions ts ON ts.user_id = f.owner_id
    WHERE f.deleted_at IS NOT NULL
      AND p.telegram_deleted_at IS NULL
      AND p.telegram_delete_available_at <= now()
) cleanup
ORDER BY created_at ASC
LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artifacts []TelegramCleanupArtifact
	for rows.Next() {
		var artifact TelegramCleanupArtifact
		if err := rows.Scan(
			&artifact.Source,
			&artifact.PartID,
			&artifact.ResourceID,
			&artifact.OwnerID,
			&artifact.OwnerTelegramID,
			&artifact.EncryptedSession,
			&artifact.StoragePeer,
			&artifact.TelegramPeer,
			&artifact.MessageID,
		); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return artifacts, nil
}

func (s *Store) PendingLocalStagingCleanupArtifacts(ctx context.Context, limit int) ([]LocalStagingCleanupArtifact, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT p.id, p.upload_id, p.storage_key
FROM upload_parts p
JOIN uploads u ON u.id = p.upload_id
WHERE p.storage_backend = $1
  AND p.storage_key IS NOT NULL
  AND p.status != 'complete'
  AND (u.status IN ('expired', 'failed') OR p.status = 'failed')
ORDER BY p.created_at ASC
LIMIT $2`,
		LocalStagingBackend,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artifacts []LocalStagingCleanupArtifact
	for rows.Next() {
		var artifact LocalStagingCleanupArtifact
		if err := rows.Scan(&artifact.PartID, &artifact.UploadID, &artifact.StorageKey); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return artifacts, nil
}

func (s *Store) MarkTelegramArtifactDeleted(ctx context.Context, partID string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE upload_parts
SET telegram_deleted_at = $2,
    telegram_delete_error = NULL
WHERE id = $1`,
		partID,
		now,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows > 0 {
		return nil
	}

	_, err = s.db.ExecContext(ctx, `
UPDATE file_parts
SET telegram_deleted_at = $2,
    telegram_delete_error = NULL
WHERE id = $1`,
		partID,
		now,
	)
	return err
}

func (s *Store) MarkLocalStagingDeleted(ctx context.Context, partID string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE upload_parts
SET storage_backend = NULL,
    storage_key = NULL,
    last_error = NULL,
    updated_at = now()
WHERE id = $1`,
		partID,
	)
	return err
}

func (s *Store) MarkLocalStagingDeleteFailed(ctx context.Context, partID string, deleteErr error) error {
	message := ""
	if deleteErr != nil {
		message = deleteErr.Error()
	}
	message = truncateError(message)

	_, err := s.db.ExecContext(ctx, `
UPDATE upload_parts
SET last_error = NULLIF($2, ''),
    updated_at = now()
WHERE id = $1`,
		partID,
		message,
	)
	return err
}

func (s *Store) MarkTelegramArtifactDeleteFailed(ctx context.Context, partID string, deleteErr error) error {
	message := ""
	if deleteErr != nil {
		message = deleteErr.Error()
	}
	if len(message) > 1000 {
		message = message[:1000]
	}

	result, err := s.db.ExecContext(ctx, `
UPDATE upload_parts
SET telegram_delete_error = NULLIF($2, '')
WHERE id = $1`,
		partID,
		message,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows > 0 {
		return nil
	}

	_, err = s.db.ExecContext(ctx, `
UPDATE file_parts
SET telegram_delete_error = NULLIF($2, ''),
    telegram_delete_available_at = now() + interval '5 minutes'
WHERE id = $1`,
		partID,
		message,
	)
	return err
}

func (s *Store) CompleteUpload(ctx context.Context, params CompleteUploadParams) (File, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return File{}, err
	}
	defer tx.Rollback()

	upload, err := scanUpload(tx.QueryRowContext(ctx, `
SELECT id, owner_id, parent_id, name_plain, mime_type, plaintext_size, part_size, status,
       idempotency_key, checksum_algorithm, checksum, checksum_state, plaintext_uploaded_size,
       next_part_number, created_at, updated_at, expires_at
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
		finalChecksum, err := checksumFromState(upload.ChecksumState)
		if err != nil {
			return File{}, err
		}
		if !bytesEqual(finalChecksum, upload.Checksum) {
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

func (s *Store) GetWithParts(ctx context.Context, ownerID string, uploadID string) (Upload, []UploadPart, error) {
	upload, err := scanUpload(s.db.QueryRowContext(ctx, `
SELECT id, owner_id, parent_id, name_plain, mime_type, plaintext_size, part_size, status,
       idempotency_key, checksum_algorithm, checksum, checksum_state, plaintext_uploaded_size,
       next_part_number, created_at, updated_at, expires_at
FROM uploads
WHERE id = $1
  AND owner_id = $2`,
		uploadID,
		ownerID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return Upload{}, nil, ErrUploadNotFound
	}
	if err != nil {
		return Upload{}, nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, upload_id, part_number, plaintext_size, ciphertext_size, checksum, telegram_peer, telegram_message_id,
       status, storage_backend, storage_key, available_at, leased_until, attempts, last_error, worker_id, created_at, updated_at
FROM upload_parts
WHERE upload_id = $1
ORDER BY part_number ASC`,
		upload.ID,
	)
	if err != nil {
		return Upload{}, nil, err
	}
	defer rows.Close()

	var parts []UploadPart
	for rows.Next() {
		part, err := scanUploadPart(rows)
		if err != nil {
			return Upload{}, nil, err
		}
		parts = append(parts, part)
	}
	if err := rows.Err(); err != nil {
		return Upload{}, nil, err
	}
	return upload, parts, nil
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

func (s *Store) UploadIntegrityState(ctx context.Context, ownerID string, uploadID string, partNumber int, now time.Time) (Upload, error) {
	upload, err := scanUpload(s.db.QueryRowContext(ctx, `
SELECT id, owner_id, parent_id, name_plain, mime_type, plaintext_size, part_size, status,
       idempotency_key, checksum_algorithm, checksum, checksum_state, plaintext_uploaded_size,
       next_part_number, created_at, updated_at, expires_at
FROM uploads
WHERE id = $1
  AND owner_id = $2
  AND status IN ('pending', 'uploading')`,
		uploadID,
		ownerID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return Upload{}, ErrUploadNotFound
	}
	if err != nil {
		return Upload{}, err
	}
	if !upload.ExpiresAt.After(now) {
		return Upload{}, ErrUploadExpired
	}
	if upload.NextPartNumber != partNumber {
		return Upload{}, ErrUploadPartOutOfOrder
	}
	return upload, nil
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
          idempotency_key, checksum_algorithm, checksum, checksum_state, plaintext_uploaded_size,
          next_part_number, created_at, updated_at, expires_at`,
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
		&upload.ChecksumState,
		&upload.UploadedSize,
		&upload.NextPartNumber,
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
		&part.StorageBackend,
		&part.StorageKey,
		&part.AvailableAt,
		&part.LeasedUntil,
		&part.Attempts,
		&part.LastError,
		&part.WorkerID,
		&part.CreatedAt,
		&part.UpdatedAt,
	)
	if err != nil {
		return UploadPart{}, err
	}
	return part, nil
}

func completeParts(ctx context.Context, q queryer, uploadID string) ([]UploadPart, error) {
	rows, err := q.QueryContext(ctx, `
SELECT id, upload_id, part_number, plaintext_size, ciphertext_size, checksum, telegram_peer, telegram_message_id,
       status, storage_backend, storage_key, available_at, leased_until, attempts, last_error, worker_id, created_at, updated_at
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

func truncateError(message string) string {
	if len(message) > 1000 {
		return message[:1000]
	}
	return message
}

func checksumFromState(state []byte) ([]byte, error) {
	hash := sha256.New()
	if len(state) > 0 {
		unmarshaler, ok := hash.(encoding.BinaryUnmarshaler)
		if !ok {
			return nil, errors.New("sha256 state cannot be unmarshaled")
		}
		if err := unmarshaler.UnmarshalBinary(state); err != nil {
			return nil, err
		}
	}
	return hash.Sum(nil), nil
}

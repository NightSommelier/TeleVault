package files

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrNotFound = errors.New("file not found")

const (
	TypeFile    = "file"
	TypeFolder  = "folder"
	StatusReady = "ready"
)

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

type FilePart struct {
	PartNumber        int
	TelegramPeer      string
	TelegramMessageID int64
	CiphertextSize    int64
	Checksum          []byte
}

type TelegramSession struct {
	EncryptedSession []byte
	StoragePeer      sql.NullString
}

type Share struct {
	ID                string
	FileID            string
	OwnerID           string
	GranteeUserID     string
	GranteeTelegramID int64
	GranteeUsername   sql.NullString
	GranteeName       sql.NullString
	Permission        string
	ExpiresAt         sql.NullTime
	RevokedAt         sql.NullTime
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) ListChildren(ctx context.Context, ownerID string, parentID string) ([]File, error) {
	var rows *sql.Rows
	var err error
	if parentID == "" {
		rows, err = s.db.QueryContext(ctx, `
SELECT id, owner_id, parent_id, name_plain, mime_type, plaintext_size, ciphertext_size, type, status, created_at, updated_at
FROM files
WHERE owner_id = $1
  AND parent_id IS NULL
  AND deleted_at IS NULL
ORDER BY type DESC, name_plain ASC, created_at ASC`,
			ownerID,
		)
	} else {
		rows, err = s.db.QueryContext(ctx, `
SELECT id, owner_id, parent_id, name_plain, mime_type, plaintext_size, ciphertext_size, type, status, created_at, updated_at
FROM files
WHERE owner_id = $1
  AND parent_id = $2
  AND deleted_at IS NULL
ORDER BY type DESC, name_plain ASC, created_at ASC`,
			ownerID,
			parentID,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		file, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return files, nil
}

func (s *Store) GetByID(ctx context.Context, ownerID string, id string) (File, error) {
	file, err := scanFile(s.db.QueryRowContext(ctx, `
SELECT id, owner_id, parent_id, name_plain, mime_type, plaintext_size, ciphertext_size, type, status, created_at, updated_at
FROM files
WHERE owner_id = $1
  AND id = $2
  AND deleted_at IS NULL`,
		ownerID,
		id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, ErrNotFound
	}
	if err != nil {
		return File{}, err
	}

	return file, nil
}

func (s *Store) GetAccessibleByID(ctx context.Context, requesterID string, id string) (File, error) {
	file, err := scanFile(s.db.QueryRowContext(ctx, `
SELECT f.id, f.owner_id, f.parent_id, f.name_plain, f.mime_type, f.plaintext_size, f.ciphertext_size, f.type, f.status, f.created_at, f.updated_at
FROM files f
WHERE f.id = $2
  AND f.deleted_at IS NULL
  AND (
      f.owner_id = $1
      OR EXISTS (
          SELECT 1
          FROM file_shares s
          WHERE s.file_id = f.id
            AND s.grantee_user_id = $1
            AND s.revoked_at IS NULL
            AND (s.expires_at IS NULL OR s.expires_at > now())
      )
  )`,
		requesterID,
		id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, ErrNotFound
	}
	if err != nil {
		return File{}, err
	}

	return file, nil
}

func (s *Store) SoftDelete(ctx context.Context, ownerID string, id string, now time.Time) error {
	var count int
	err := s.db.QueryRowContext(ctx, `
WITH RECURSIVE target AS (
    SELECT id
    FROM files
    WHERE owner_id = $1
      AND id = $2
      AND deleted_at IS NULL
    UNION ALL
    SELECT child.id
    FROM files child
    INNER JOIN target parent ON child.parent_id = parent.id
    WHERE child.owner_id = $1
      AND child.deleted_at IS NULL
),
updated AS (
    UPDATE files
    SET status = 'deleted',
        deleted_at = $3,
        updated_at = $3
    WHERE id IN (SELECT id FROM target)
    RETURNING id
)
SELECT COUNT(*) FROM updated`,
		ownerID,
		id,
		now,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}

	return nil
}

func (s *Store) DownloadData(ctx context.Context, ownerID string, id string) (File, []FilePart, TelegramSession, error) {
	file, err := s.GetAccessibleByID(ctx, ownerID, id)
	if err != nil {
		return File{}, nil, TelegramSession{}, err
	}
	if file.Type != TypeFile || file.Status != StatusReady {
		return File{}, nil, TelegramSession{}, ErrNotFound
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT part_number, telegram_peer, telegram_message_id, ciphertext_size, checksum
FROM file_parts
WHERE file_id = $1
ORDER BY part_number ASC`,
		id,
	)
	if err != nil {
		return File{}, nil, TelegramSession{}, err
	}
	defer rows.Close()

	var parts []FilePart
	for rows.Next() {
		var part FilePart
		if err := rows.Scan(&part.PartNumber, &part.TelegramPeer, &part.TelegramMessageID, &part.CiphertextSize, &part.Checksum); err != nil {
			return File{}, nil, TelegramSession{}, err
		}
		parts = append(parts, part)
	}
	if err := rows.Err(); err != nil {
		return File{}, nil, TelegramSession{}, err
	}
	if len(parts) == 0 {
		return File{}, nil, TelegramSession{}, ErrNotFound
	}

	var session TelegramSession
	err = s.db.QueryRowContext(ctx, `
SELECT encrypted_session, storage_peer
FROM telegram_sessions
WHERE user_id = $1`,
		file.OwnerID,
	).Scan(&session.EncryptedSession, &session.StoragePeer)
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, nil, TelegramSession{}, ErrNotFound
	}
	if err != nil {
		return File{}, nil, TelegramSession{}, err
	}

	return file, parts, session, nil
}

func (s *Store) CreateShare(ctx context.Context, ownerID string, fileID string, granteeTelegramID int64, expiresAt sql.NullTime) (Share, error) {
	file, err := s.GetByID(ctx, ownerID, fileID)
	if err != nil {
		return Share{}, err
	}
	if file.Type != TypeFile || file.Status != StatusReady {
		return Share{}, ErrNotFound
	}

	var granteeID string
	err = s.db.QueryRowContext(ctx, `
SELECT id
FROM users
WHERE telegram_id = $1
  AND id <> $2`,
		granteeTelegramID,
		ownerID,
	).Scan(&granteeID)
	if errors.Is(err, sql.ErrNoRows) {
		return Share{}, ErrNotFound
	}
	if err != nil {
		return Share{}, err
	}

	var shareID string
	err = s.db.QueryRowContext(ctx, `
WITH updated AS (
    UPDATE file_shares
    SET expires_at = $4,
        updated_at = now()
    WHERE file_id = $1
      AND owner_id = $2
      AND grantee_user_id = $3
      AND revoked_at IS NULL
    RETURNING id
),
inserted AS (
    INSERT INTO file_shares (file_id, owner_id, grantee_user_id, permission, expires_at)
    SELECT $1, $2, $3, 'read', $4
    WHERE NOT EXISTS (SELECT 1 FROM updated)
    RETURNING id
)
SELECT id FROM updated
UNION ALL
SELECT id FROM inserted`,
		fileID,
		ownerID,
		granteeID,
		nullableTime(expiresAt),
	).Scan(&shareID)
	if err != nil {
		return Share{}, err
	}

	return s.GetShare(ctx, ownerID, fileID, shareID)
}

func (s *Store) GetShare(ctx context.Context, ownerID string, fileID string, shareID string) (Share, error) {
	share, err := scanShare(s.db.QueryRowContext(ctx, `
SELECT s.id, s.file_id, s.owner_id, s.grantee_user_id, u.telegram_id, u.username, u.display_name,
       s.permission, s.expires_at, s.revoked_at, s.created_at, s.updated_at
FROM file_shares s
JOIN users u ON u.id = s.grantee_user_id
WHERE s.owner_id = $1
  AND s.file_id = $2
  AND s.id = $3`,
		ownerID,
		fileID,
		shareID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return Share{}, ErrNotFound
	}
	if err != nil {
		return Share{}, err
	}

	return share, nil
}

func (s *Store) ListShares(ctx context.Context, ownerID string, fileID string) ([]Share, error) {
	if _, err := s.GetByID(ctx, ownerID, fileID); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT s.id, s.file_id, s.owner_id, s.grantee_user_id, u.telegram_id, u.username, u.display_name,
       s.permission, s.expires_at, s.revoked_at, s.created_at, s.updated_at
FROM file_shares s
JOIN users u ON u.id = s.grantee_user_id
WHERE s.owner_id = $1
  AND s.file_id = $2
  AND s.revoked_at IS NULL
ORDER BY s.created_at DESC`,
		ownerID,
		fileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []Share
	for rows.Next() {
		share, err := scanShare(rows)
		if err != nil {
			return nil, err
		}
		shares = append(shares, share)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return shares, nil
}

func (s *Store) RevokeShare(ctx context.Context, ownerID string, fileID string, shareID string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE file_shares
SET revoked_at = $4,
    updated_at = $4
WHERE owner_id = $1
  AND file_id = $2
  AND id = $3
  AND revoked_at IS NULL`,
		ownerID,
		fileID,
		shareID,
		now,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

func (s *Store) ListSharedWithMe(ctx context.Context, requesterID string) ([]File, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT f.id, f.owner_id, f.parent_id, f.name_plain, f.mime_type, f.plaintext_size, f.ciphertext_size, f.type, f.status, f.created_at, f.updated_at
FROM file_shares s
JOIN files f ON f.id = s.file_id
WHERE s.grantee_user_id = $1
  AND s.revoked_at IS NULL
  AND (s.expires_at IS NULL OR s.expires_at > now())
  AND f.deleted_at IS NULL
ORDER BY s.created_at DESC`,
		requesterID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		file, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return files, nil
}

func (s *Store) CreateFolder(ctx context.Context, ownerID string, parentID string, name string) (File, error) {
	if parentID != "" {
		if err := s.ensureParentFolder(ctx, ownerID, parentID); err != nil {
			return File{}, err
		}
	}

	file, err := scanFile(s.db.QueryRowContext(ctx, `
INSERT INTO files (owner_id, parent_id, name_plain, type, status)
VALUES ($1, NULLIF($2, '')::uuid, $3, 'folder', 'ready')
RETURNING id, owner_id, parent_id, name_plain, mime_type, plaintext_size, ciphertext_size, type, status, created_at, updated_at`,
		ownerID,
		parentID,
		name,
	))
	if err != nil {
		return File{}, err
	}

	return file, nil
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
		return ErrNotFound
	}

	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
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

func scanShare(row rowScanner) (Share, error) {
	var share Share
	err := row.Scan(
		&share.ID,
		&share.FileID,
		&share.OwnerID,
		&share.GranteeUserID,
		&share.GranteeTelegramID,
		&share.GranteeUsername,
		&share.GranteeName,
		&share.Permission,
		&share.ExpiresAt,
		&share.RevokedAt,
		&share.CreatedAt,
		&share.UpdatedAt,
	)
	if err != nil {
		return Share{}, err
	}
	return share, nil
}

func nullableTime(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time
}

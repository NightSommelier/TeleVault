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

func (s *Store) DownloadData(ctx context.Context, ownerID string, id string) (File, []FilePart, TelegramSession, error) {
	file, err := s.GetByID(ctx, ownerID, id)
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
		ownerID,
	).Scan(&session.EncryptedSession, &session.StoragePeer)
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, nil, TelegramSession{}, ErrNotFound
	}
	if err != nil {
		return File{}, nil, TelegramSession{}, err
	}

	return file, parts, session, nil
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

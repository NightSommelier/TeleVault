package files

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"
)

var (
	ErrNotFound          = errors.New("file not found")
	ErrForbidden         = errors.New("forbidden")
	ErrPasswordRequired  = errors.New("public link password required")
	ErrInvalidMove       = errors.New("invalid file move")
	ErrInvalidName       = errors.New("invalid file name")
	ErrInvalidPermission = errors.New("invalid permission")
)

const (
	TypeFile    = "file"
	TypeFolder  = "folder"
	StatusReady = "ready"

	SharePermissionRead       = "read"
	SharePermissionReadDelete = "read_delete"

	FileAccessOwner            = "owner"
	FileAccessSharedRead       = "shared_read"
	FileAccessSharedReadDelete = "shared_read_delete"

	PublicDownloadLimitModeHard = "hard"
	PublicDownloadLimitModeSoft = "soft"
)

type File struct {
	ID             string
	OwnerID        string
	ParentID       sql.NullString
	NamePlain      sql.NullString
	MimeType       sql.NullString
	PlaintextSize  sql.NullInt64
	CiphertextSize sql.NullInt64
	Checksum       []byte
	Type           string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type FilePart struct {
	PartNumber        int
	PlaintextStart    sql.NullInt64
	PlaintextEnd      sql.NullInt64
	PlaintextSize     sql.NullInt64
	TelegramPeer      string
	TelegramMessageID int64
	CiphertextSize    int64
	Checksum          []byte
}

type TelegramSession struct {
	EncryptedSession []byte
	StoragePeer      sql.NullString
	OwnerTelegramID  int64
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

type ShareRecipient struct {
	UserID      string
	TelegramID  int64
	Username    sql.NullString
	DisplayName sql.NullString
}

type FileAccessContext struct {
	FileID           string
	OwnerTelegramID  int64
	OwnerUsername    sql.NullString
	OwnerDisplayName sql.NullString
	Access           string
	CanDelete        bool
}

type PublicLink struct {
	ID                     string
	FileID                 string
	OwnerID                string
	Permission             string
	ExpiresAt              sql.NullTime
	RevokedAt              sql.NullTime
	MaxDownloads           sql.NullInt64
	DownloadCount          int64
	ActiveDownloadCount    int64
	DownloadLimitMode      string
	ShowChecksum           bool
	PasswordRequired       bool
	PasswordKDF            sql.NullString
	PasswordSalt           []byte
	PasswordHash           []byte
	PasswordArgonTime      sql.NullInt64
	PasswordArgonMemoryKiB sql.NullInt64
	PasswordArgonThreads   sql.NullInt64
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type PublicLinkPassword struct {
	KDF            string
	Salt           []byte
	Hash           []byte
	ArgonTime      int
	ArgonMemoryKiB int
	ArgonThreads   int
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
SELECT id, owner_id, parent_id, name_plain, mime_type, plaintext_size, ciphertext_size, type, status, checksum, created_at, updated_at
FROM files
WHERE owner_id = $1
  AND parent_id IS NULL
  AND deleted_at IS NULL
ORDER BY type DESC, name_plain ASC, created_at ASC`,
			ownerID,
		)
	} else {
		rows, err = s.db.QueryContext(ctx, `
SELECT id, owner_id, parent_id, name_plain, mime_type, plaintext_size, ciphertext_size, type, status, checksum, created_at, updated_at
FROM files
WHERE parent_id = $2
  AND deleted_at IS NULL
  AND (
      owner_id = $1
      OR EXISTS (
          WITH RECURSIVE ancestors AS (
              SELECT id, parent_id
              FROM files
              WHERE id = $2
                AND deleted_at IS NULL
              UNION ALL
              SELECT f.id, f.parent_id
              FROM files f
              JOIN ancestors a ON a.parent_id = f.id
              WHERE f.deleted_at IS NULL
          )
          SELECT 1
          FROM file_shares s
          WHERE s.grantee_user_id = $1
            AND s.revoked_at IS NULL
            AND (s.expires_at IS NULL OR s.expires_at > now())
            AND s.file_id IN (SELECT id FROM ancestors)
      )
  )
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
SELECT id, owner_id, parent_id, name_plain, mime_type, plaintext_size, ciphertext_size, type, status, checksum, created_at, updated_at
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
SELECT f.id, f.owner_id, f.parent_id, f.name_plain, f.mime_type, f.plaintext_size, f.ciphertext_size, f.type, f.status, f.checksum, f.created_at, f.updated_at
FROM files f
WHERE f.id = $2
  AND f.deleted_at IS NULL
  AND (
      f.owner_id = $1
      OR EXISTS (
          WITH RECURSIVE ancestors AS (
              SELECT id, parent_id
              FROM files
              WHERE id = f.id
                AND deleted_at IS NULL
              UNION ALL
              SELECT parent.id, parent.parent_id
              FROM files parent
              JOIN ancestors child ON child.parent_id = parent.id
              WHERE parent.deleted_at IS NULL
          )
          SELECT 1
          FROM file_shares s
          WHERE s.file_id IN (SELECT id FROM ancestors)
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

func (s *Store) CountFileParts(ctx context.Context, fileID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM file_parts
WHERE file_id = $1`,
		fileID,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) CountActivePublicLinks(ctx context.Context, ownerID string, fileID string) (int, int, error) {
	var total int
	var passwordProtected int
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*),
       COALESCE(SUM(CASE WHEN password_hash IS NOT NULL THEN 1 ELSE 0 END), 0)
FROM public_links
WHERE owner_id = $1
  AND file_id = $2
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > now())`,
		ownerID,
		fileID,
	).Scan(&total, &passwordProtected)
	if err != nil {
		return 0, 0, err
	}
	return total, passwordProtected, nil
}

func (s *Store) SoftDelete(ctx context.Context, ownerID string, id string, now time.Time) error {
	return s.softDeleteMany(ctx, ownerID, []string{id}, now)
}

func (s *Store) SoftDeleteAccessible(ctx context.Context, requesterID string, id string, now time.Time) error {
	file, err := s.GetAccessibleByID(ctx, requesterID, id)
	if err != nil {
		return err
	}
	if file.OwnerID == requesterID {
		return s.softDeleteMany(ctx, requesterID, []string{id}, now)
	}

	var count int
	err = s.db.QueryRowContext(ctx, `
WITH RECURSIVE ancestors AS (
    SELECT id, parent_id
    FROM files
    WHERE id = $3
      AND deleted_at IS NULL
    UNION ALL
    SELECT parent.id, parent.parent_id
    FROM files parent
    JOIN ancestors child ON child.parent_id = parent.id
    WHERE parent.deleted_at IS NULL
),
authorized AS (
    SELECT EXISTS (
        SELECT 1
        FROM file_shares s
        WHERE s.file_id IN (SELECT id FROM ancestors)
          AND s.grantee_user_id = $1
          AND s.permission = $4
          AND s.revoked_at IS NULL
          AND (s.expires_at IS NULL OR s.expires_at > now())
    ) AS can_delete
),
target AS (
    SELECT id
    FROM files
    WHERE owner_id = $2
      AND id = $3
      AND deleted_at IS NULL
      AND (SELECT can_delete FROM authorized)
    UNION ALL
    SELECT child.id
    FROM files child
    INNER JOIN target parent ON child.parent_id = parent.id
    WHERE child.owner_id = $2
      AND child.deleted_at IS NULL
),
updated AS (
    UPDATE files
    SET status = 'deleted',
        deleted_at = $5,
        updated_at = $5
    WHERE id IN (SELECT id FROM target)
    RETURNING id
),
queued_file_parts AS (
    SELECT p.id, row_number() OVER (ORDER BY p.created_at ASC, p.part_number ASC) AS queue_position
    FROM file_parts p
    JOIN updated f ON f.id = p.file_id
    WHERE p.telegram_deleted_at IS NULL
),
queued_cleanup AS (
    UPDATE file_parts p
    SET telegram_delete_available_at = $5 + ((q.queue_position - 1) * interval '15 seconds'),
        telegram_delete_error = NULL
    FROM queued_file_parts q
    WHERE p.id = q.id
    RETURNING p.id
)
SELECT COUNT(*) FROM updated`,
		requesterID,
		file.OwnerID,
		id,
		SharePermissionReadDelete,
		now,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrForbidden
	}
	return nil
}

func (s *Store) SoftDeleteMany(ctx context.Context, ownerID string, ids []string, now time.Time) error {
	return s.softDeleteMany(ctx, ownerID, ids, now)
}

func (s *Store) FileAccessContexts(ctx context.Context, requesterID string, fileIDs []string) (map[string]FileAccessContext, error) {
	normalized := normalizeFileIDs(fileIDs)
	if len(normalized) == 0 {
		return map[string]FileAccessContext{}, nil
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT f.id,
       u.telegram_id,
       u.username,
       u.display_name,
       CASE
         WHEN f.owner_id = $1 THEN $3
         WHEN sa.can_delete_share THEN $4
         WHEN sa.has_share THEN $5
         ELSE ''
       END AS access,
       CASE
         WHEN f.owner_id = $1 THEN TRUE
         ELSE sa.can_delete_share
       END AS can_delete
FROM files f
JOIN users u ON u.id = f.owner_id
LEFT JOIN LATERAL (
    WITH RECURSIVE ancestors AS (
        SELECT id, parent_id
        FROM files
        WHERE id = f.id
          AND deleted_at IS NULL
        UNION ALL
        SELECT parent.id, parent.parent_id
        FROM files parent
        JOIN ancestors child ON child.parent_id = parent.id
        WHERE parent.deleted_at IS NULL
    )
    SELECT COALESCE(COUNT(*) > 0, FALSE) AS has_share,
           COALESCE(BOOL_OR(s.permission = $2), FALSE) AS can_delete_share
    FROM file_shares s
    WHERE s.file_id IN (SELECT id FROM ancestors)
      AND s.grantee_user_id = $1
      AND s.revoked_at IS NULL
      AND (s.expires_at IS NULL OR s.expires_at > now())
) sa ON TRUE
WHERE f.id = ANY($6::uuid[])
  AND f.deleted_at IS NULL`,
		requesterID,
		SharePermissionReadDelete,
		FileAccessOwner,
		FileAccessSharedReadDelete,
		FileAccessSharedRead,
		normalized,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]FileAccessContext, len(normalized))
	for rows.Next() {
		var item FileAccessContext
		if err := rows.Scan(
			&item.FileID,
			&item.OwnerTelegramID,
			&item.OwnerUsername,
			&item.OwnerDisplayName,
			&item.Access,
			&item.CanDelete,
		); err != nil {
			return nil, err
		}
		result[item.FileID] = item
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Store) softDeleteMany(ctx context.Context, ownerID string, ids []string, now time.Time) error {
	if len(ids) == 0 {
		return ErrNotFound
	}
	var count int
	err := s.db.QueryRowContext(ctx, `
WITH RECURSIVE requested AS (
    SELECT DISTINCT ids.id
    FROM unnest($2::uuid[]) AS ids(id)
),
target AS (
    SELECT id
    FROM files
    WHERE owner_id = $1
      AND id IN (SELECT id FROM requested)
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
),
queued_file_parts AS (
    SELECT p.id, row_number() OVER (ORDER BY p.created_at ASC, p.part_number ASC) AS queue_position
    FROM file_parts p
    JOIN updated f ON f.id = p.file_id
    WHERE p.telegram_deleted_at IS NULL
),
queued_cleanup AS (
    UPDATE file_parts p
    SET telegram_delete_available_at = $3 + ((q.queue_position - 1) * interval '15 seconds'),
        telegram_delete_error = NULL
    FROM queued_file_parts q
    WHERE p.id = q.id
    RETURNING p.id
)
SELECT COUNT(*) FROM updated`,
		ownerID,
		ids,
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

type metadataUpdate struct {
	SetParent bool
	ParentID  string
	SetName   bool
	Name      string
}

func (s *Store) Move(ctx context.Context, ownerID string, id string, parentID string) (File, error) {
	return s.updateMetadata(ctx, ownerID, []string{id}, metadataUpdate{SetParent: true, ParentID: parentID})
}

func (s *Store) MoveMany(ctx context.Context, ownerID string, ids []string, parentID string) error {
	_, err := s.updateMetadata(ctx, ownerID, ids, metadataUpdate{SetParent: true, ParentID: parentID})
	return err
}

func (s *Store) Rename(ctx context.Context, ownerID string, id string, name string) (File, error) {
	return s.updateMetadata(ctx, ownerID, []string{id}, metadataUpdate{SetName: true, Name: name})
}

func (s *Store) updateMetadata(ctx context.Context, ownerID string, ids []string, update metadataUpdate) (File, error) {
	if len(ids) == 0 {
		return File{}, ErrNotFound
	}

	normalizedName := ""
	if update.SetName {
		normalizedName = normalizeName(update.Name)
		if normalizedName == "" || len(normalizedName) > 255 {
			return File{}, ErrInvalidName
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return File{}, err
	}
	defer tx.Rollback()

	files, err := s.filesByIDsForUpdate(ctx, tx, ownerID, ids)
	if err != nil {
		return File{}, err
	}
	if len(files) == 0 {
		return File{}, ErrNotFound
	}

	if update.SetParent && update.ParentID != "" {
		if err := ensureParentFolderTx(ctx, tx, ownerID, update.ParentID); err != nil {
			return File{}, err
		}
		for _, file := range files {
			if file.Type != TypeFolder {
				continue
			}
			if update.ParentID == file.ID {
				return File{}, ErrInvalidMove
			}
			descendant, err := isDescendantFolder(ctx, tx, ownerID, file.ID, update.ParentID)
			if err != nil {
				return File{}, err
			}
			if descendant {
				return File{}, ErrInvalidMove
			}
		}
	}

	updated, err := scanFile(tx.QueryRowContext(ctx, `
	WITH requested AS (
    SELECT DISTINCT ids.id
    FROM unnest($2::uuid[]) AS ids(id)
)
UPDATE files
SET parent_id = CASE WHEN $3 THEN NULLIF($4, '')::uuid ELSE parent_id END,
    name_plain = CASE WHEN $5 THEN $6 ELSE name_plain END,
    updated_at = now()
WHERE owner_id = $1
  AND id IN (SELECT id FROM requested)
  AND deleted_at IS NULL
RETURNING id, owner_id, parent_id, name_plain, mime_type, plaintext_size, ciphertext_size, type, status, checksum, created_at, updated_at`,
		ownerID,
		ids,
		update.SetParent,
		update.ParentID,
		update.SetName,
		normalizedName,
	))
	if err != nil {
		return File{}, err
	}

	if err := tx.Commit(); err != nil {
		return File{}, err
	}

	return updated, nil
}

func (s *Store) filesByIDsForUpdate(ctx context.Context, tx *sql.Tx, ownerID string, ids []string) ([]File, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT id, owner_id, parent_id, name_plain, mime_type, plaintext_size, ciphertext_size, type, status, checksum, created_at, updated_at
FROM files
WHERE owner_id = $1
  AND id = ANY($2::uuid[])
  AND deleted_at IS NULL
FOR UPDATE`,
		ownerID,
		ids,
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

func (s *Store) DownloadData(ctx context.Context, requesterID string, id string) (File, []FilePart, TelegramSession, error) {
	file, err := s.GetAccessibleByID(ctx, requesterID, id)
	if err != nil {
		return File{}, nil, TelegramSession{}, err
	}
	return s.downloadDataForFile(ctx, file)
}

func (s *Store) DownloadDataByPublicTokenHash(ctx context.Context, tokenHash []byte) (File, []FilePart, TelegramSession, error) {
	file, link, err := s.PublicFileByTokenHash(ctx, tokenHash)
	if err != nil {
		return File{}, nil, TelegramSession{}, err
	}
	if link.PasswordRequired {
		return File{}, nil, TelegramSession{}, ErrPasswordRequired
	}
	return s.downloadDataForFile(ctx, file)
}

func (s *Store) DownloadDataForPublicFile(ctx context.Context, file File) (File, []FilePart, TelegramSession, error) {
	return s.downloadDataForFile(ctx, file)
}

func (s *Store) PublicFileByTokenHash(ctx context.Context, tokenHash []byte) (File, PublicLink, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT f.id, f.owner_id, f.parent_id, f.name_plain, f.mime_type, f.plaintext_size, f.ciphertext_size, f.type, f.status, f.checksum, f.created_at, f.updated_at,
       l.id, l.file_id, l.owner_id, l.permission, l.expires_at, l.revoked_at, l.max_downloads, l.download_count, l.active_download_count, l.download_limit_mode, l.show_checksum, (l.password_hash IS NOT NULL),
       l.password_kdf, l.password_salt, l.password_hash, l.password_argon_time, l.password_argon_memory_kib, l.password_argon_threads,
       l.created_at, l.updated_at
FROM public_links l
JOIN files f ON f.id = l.file_id
WHERE l.token_hash = $1
  AND l.revoked_at IS NULL
  AND (l.expires_at IS NULL OR l.expires_at > now())
  AND (
    l.max_downloads IS NULL
    OR (l.download_limit_mode = 'hard' AND (l.download_count + l.active_download_count) < l.max_downloads)
    OR (l.download_limit_mode = 'soft' AND l.download_count < l.max_downloads)
  )
  AND f.deleted_at IS NULL`,
		tokenHash,
	)
	file, link, err := scanFileAndPublicLink(row)
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, PublicLink{}, ErrNotFound
	}
	if err != nil {
		return File{}, PublicLink{}, err
	}
	return file, link, nil
}

func (s *Store) downloadDataForFile(ctx context.Context, file File) (File, []FilePart, TelegramSession, error) {
	if file.Type != TypeFile || file.Status != StatusReady {
		return File{}, nil, TelegramSession{}, ErrNotFound
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT part_number, plaintext_start, plaintext_end, plaintext_size, telegram_peer, telegram_message_id, ciphertext_size, checksum
FROM file_parts
WHERE file_id = $1
ORDER BY part_number ASC`,
		file.ID,
	)
	if err != nil {
		return File{}, nil, TelegramSession{}, err
	}
	defer rows.Close()

	var parts []FilePart
	for rows.Next() {
		var part FilePart
		if err := rows.Scan(&part.PartNumber, &part.PlaintextStart, &part.PlaintextEnd, &part.PlaintextSize, &part.TelegramPeer, &part.TelegramMessageID, &part.CiphertextSize, &part.Checksum); err != nil {
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
SELECT ts.encrypted_session, ts.storage_peer, u.telegram_id
FROM telegram_sessions ts
JOIN users u ON u.id = ts.user_id
WHERE ts.user_id = $1`,
		file.OwnerID,
	).Scan(&session.EncryptedSession, &session.StoragePeer, &session.OwnerTelegramID)
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, nil, TelegramSession{}, ErrNotFound
	}
	if err != nil {
		return File{}, nil, TelegramSession{}, err
	}

	return file, parts, session, nil
}

func (s *Store) TelegramSession(ctx context.Context, ownerID string) (TelegramSession, error) {
	var session TelegramSession
	err := s.db.QueryRowContext(ctx, `
SELECT ts.encrypted_session, ts.storage_peer, u.telegram_id
FROM telegram_sessions ts
JOIN users u ON u.id = ts.user_id
WHERE ts.user_id = $1`,
		ownerID,
	).Scan(&session.EncryptedSession, &session.StoragePeer, &session.OwnerTelegramID)
	if errors.Is(err, sql.ErrNoRows) {
		return TelegramSession{}, ErrNotFound
	}
	if err != nil {
		return TelegramSession{}, err
	}
	return session, nil
}

func (s *Store) PublicLinkPasswordMinLength(ctx context.Context) (int, error) {
	const fallback = 8
	var value int
	err := s.db.QueryRowContext(ctx, `
SELECT public_link_password_min_length
FROM admin_settings
WHERE id = TRUE`,
	).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return fallback, nil
	}
	if err != nil {
		return 0, err
	}
	if value < 1 {
		return fallback, nil
	}
	if value > 1024 {
		return 1024, nil
	}
	return value, nil
}

func (s *Store) CreatePublicLink(ctx context.Context, ownerID string, fileID string, tokenHash []byte, expiresAt sql.NullTime, maxDownloads sql.NullInt64, downloadLimitMode string, showChecksum bool, password PublicLinkPassword) (PublicLink, error) {
	file, err := s.GetByID(ctx, ownerID, fileID)
	if err != nil {
		return PublicLink{}, err
	}
	if file.Type != TypeFile || file.Status != StatusReady {
		return PublicLink{}, ErrNotFound
	}

	var linkID string
	err = s.db.QueryRowContext(ctx, `
INSERT INTO public_links (
    file_id,
    owner_id,
    token_hash,
    permission,
    expires_at,
    max_downloads,
    download_limit_mode,
    show_checksum,
    password_kdf,
    password_salt,
    password_hash,
    password_argon_time,
    password_argon_memory_kib,
    password_argon_threads
)
VALUES ($1, $2, $3, 'read', $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id`,
		fileID,
		ownerID,
		tokenHash,
		nullableTime(expiresAt),
		nullableInt64(maxDownloads),
		nullableDownloadLimitMode(downloadLimitMode),
		showChecksum,
		nullablePasswordString(password.KDF),
		nullablePasswordBytes(password.Salt),
		nullablePasswordBytes(password.Hash),
		nullablePasswordInt(password.ArgonTime),
		nullablePasswordInt(password.ArgonMemoryKiB),
		nullablePasswordInt(password.ArgonThreads),
	).Scan(&linkID)
	if err != nil {
		return PublicLink{}, err
	}

	return s.GetPublicLink(ctx, ownerID, fileID, linkID)
}

func (s *Store) GetPublicLink(ctx context.Context, ownerID string, fileID string, linkID string) (PublicLink, error) {
	link, err := scanPublicLink(s.db.QueryRowContext(ctx, `
SELECT id, file_id, owner_id, permission, expires_at, revoked_at, max_downloads, download_count, active_download_count, download_limit_mode, show_checksum, (password_hash IS NOT NULL),
       password_kdf, password_salt, password_hash, password_argon_time, password_argon_memory_kib, password_argon_threads,
       created_at, updated_at
FROM public_links
WHERE owner_id = $1
  AND file_id = $2
  AND id = $3`,
		ownerID,
		fileID,
		linkID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return PublicLink{}, ErrNotFound
	}
	if err != nil {
		return PublicLink{}, err
	}
	return link, nil
}

func (s *Store) ListPublicLinks(ctx context.Context, ownerID string, fileID string) ([]PublicLink, error) {
	if _, err := s.GetByID(ctx, ownerID, fileID); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, file_id, owner_id, permission, expires_at, revoked_at, max_downloads, download_count, active_download_count, download_limit_mode, show_checksum, (password_hash IS NOT NULL),
       password_kdf, password_salt, password_hash, password_argon_time, password_argon_memory_kib, password_argon_threads,
       created_at, updated_at
FROM public_links
WHERE owner_id = $1
  AND file_id = $2
  AND revoked_at IS NULL
ORDER BY created_at DESC`,
		ownerID,
		fileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []PublicLink
	for rows.Next() {
		link, err := scanPublicLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return links, nil
}

func (s *Store) RevokePublicLink(ctx context.Context, ownerID string, fileID string, linkID string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE public_links
SET revoked_at = $4,
    updated_at = $4
WHERE owner_id = $1
  AND file_id = $2
  AND id = $3
  AND revoked_at IS NULL`,
		ownerID,
		fileID,
		linkID,
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

func (s *Store) ReservePublicLinkDownloadSlot(ctx context.Context, tokenHash []byte) (File, PublicLink, bool, error) {
	row := s.db.QueryRowContext(ctx, `
WITH claimed AS (
	UPDATE public_links
	SET active_download_count = CASE
		WHEN download_limit_mode = 'hard' THEN active_download_count + 1
		ELSE active_download_count
	END,
	    updated_at = now()
	WHERE token_hash = $1
	  AND revoked_at IS NULL
	  AND (expires_at IS NULL OR expires_at > now())
	  AND (
		max_downloads IS NULL
		OR (download_limit_mode = 'hard' AND (download_count + active_download_count) < max_downloads)
		OR (download_limit_mode = 'soft' AND download_count < max_downloads)
	  )
	RETURNING id, file_id, owner_id, permission, expires_at, revoked_at, max_downloads, download_count,
	          active_download_count,
	          download_limit_mode,
	          show_checksum,
	          (password_hash IS NOT NULL) AS password_required, password_kdf, password_salt, password_hash, password_argon_time, password_argon_memory_kib,
	          password_argon_threads, created_at, updated_at
)
SELECT f.id, f.owner_id, f.parent_id, f.name_plain, f.mime_type, f.plaintext_size, f.ciphertext_size, f.type, f.status, f.checksum, f.created_at, f.updated_at,
       c.id, c.file_id, c.owner_id, c.permission, c.expires_at, c.revoked_at, c.max_downloads, c.download_count, c.active_download_count, c.download_limit_mode,
       c.show_checksum, c.password_required, c.password_kdf, c.password_salt, c.password_hash, c.password_argon_time, c.password_argon_memory_kib, c.password_argon_threads, c.created_at, c.updated_at
FROM claimed c
JOIN files f ON f.id = c.file_id
WHERE f.deleted_at IS NULL`,
		tokenHash,
	)
	file, link, err := scanFileAndPublicLink(row)
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, PublicLink{}, false, nil
	}
	if err != nil {
		return File{}, PublicLink{}, false, err
	}
	return file, link, true, nil
}

func (s *Store) FinishPublicLinkDownload(ctx context.Context, linkID string, completed bool) error {
	downloadInc := 0
	if completed {
		downloadInc = 1
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE public_links
SET download_count = download_count + $2,
    active_download_count = CASE
      WHEN download_limit_mode = 'hard' AND active_download_count > 0 THEN active_download_count - 1
      ELSE active_download_count
    END,
    updated_at = now()
WHERE id = $1`,
		linkID,
		downloadInc,
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

func (s *Store) CreateShare(ctx context.Context, ownerID string, fileID string, granteeTelegramID int64, permission string, expiresAt sql.NullTime) (Share, error) {
	permission = normalizeSharePermission(permission)
	if permission == "" {
		return Share{}, ErrInvalidPermission
	}

	file, err := s.GetByID(ctx, ownerID, fileID)
	if err != nil {
		return Share{}, err
	}
	if file.Type != TypeFolder && (file.Type != TypeFile || file.Status != StatusReady) {
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
    SET permission = $4,
        expires_at = $5,
        updated_at = now()
    WHERE file_id = $1
      AND owner_id = $2
      AND grantee_user_id = $3
      AND revoked_at IS NULL
    RETURNING id
),
inserted AS (
    INSERT INTO file_shares (file_id, owner_id, grantee_user_id, permission, expires_at)
    SELECT $1, $2, $3, $4, $5
    WHERE NOT EXISTS (SELECT 1 FROM updated)
    RETURNING id
)
SELECT id FROM updated
UNION ALL
SELECT id FROM inserted`,
		fileID,
		ownerID,
		granteeID,
		permission,
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

func (s *Store) ListShareRecipients(ctx context.Context, ownerID string, candidateTelegramIDs []int64) ([]ShareRecipient, error) {
	normalized := normalizeTelegramIDs(candidateTelegramIDs)
	if len(normalized) == 0 {
		return []ShareRecipient{}, nil
	}
	allowed := make(map[int64]struct{}, len(normalized))
	for _, telegramID := range normalized {
		allowed[telegramID] = struct{}{}
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, telegram_id, username, display_name
FROM users
WHERE id <> $1
ORDER BY COALESCE(NULLIF(display_name, ''), NULLIF(username, ''), telegram_id::text) ASC,
         telegram_id ASC`,
		ownerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	recipients := make([]ShareRecipient, 0, len(normalized))
	for rows.Next() {
		var recipient ShareRecipient
		if err := rows.Scan(&recipient.UserID, &recipient.TelegramID, &recipient.Username, &recipient.DisplayName); err != nil {
			return nil, err
		}
		if _, ok := allowed[recipient.TelegramID]; !ok {
			continue
		}
		recipients = append(recipients, recipient)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(recipients, func(i, j int) bool {
		left := recipients[i]
		right := recipients[j]
		leftName := left.DisplayName.String
		if !left.DisplayName.Valid || leftName == "" {
			leftName = left.Username.String
		}
		rightName := right.DisplayName.String
		if !right.DisplayName.Valid || rightName == "" {
			rightName = right.Username.String
		}
		if leftName == rightName {
			return left.TelegramID < right.TelegramID
		}
		return leftName < rightName
	})

	return recipients, nil
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
SELECT f.id, f.owner_id, f.parent_id, f.name_plain, f.mime_type, f.plaintext_size, f.ciphertext_size, f.type, f.status, f.checksum, f.created_at, f.updated_at
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
RETURNING id, owner_id, parent_id, name_plain, mime_type, plaintext_size, ciphertext_size, type, status, checksum, created_at, updated_at`,
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
	return ensureParentFolderTx(ctx, s.db, ownerID, parentID)
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func ensureParentFolderTx(ctx context.Context, q queryer, ownerID string, parentID string) error {
	var exists bool
	err := q.QueryRowContext(ctx, `
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

func isDescendantFolder(ctx context.Context, tx *sql.Tx, ownerID string, rootID string, candidateID string) (bool, error) {
	var descendant bool
	err := tx.QueryRowContext(ctx, `
WITH RECURSIVE descendants AS (
    SELECT id
    FROM files
    WHERE owner_id = $1
      AND parent_id = $2
      AND deleted_at IS NULL
    UNION ALL
    SELECT child.id
    FROM files child
    JOIN descendants parent ON child.parent_id = parent.id
    WHERE child.owner_id = $1
      AND child.deleted_at IS NULL
)
SELECT EXISTS (SELECT 1 FROM descendants WHERE id = $3)`,
		ownerID,
		rootID,
		candidateID,
	).Scan(&descendant)
	return descendant, err
}

func normalizeTelegramIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func normalizeSharePermission(permission string) string {
	switch permission {
	case "":
		return SharePermissionRead
	case SharePermissionRead, SharePermissionReadDelete:
		return permission
	default:
		return ""
	}
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
		&file.Checksum,
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

func scanPublicLink(row rowScanner) (PublicLink, error) {
	var link PublicLink
	err := row.Scan(
		&link.ID,
		&link.FileID,
		&link.OwnerID,
		&link.Permission,
		&link.ExpiresAt,
		&link.RevokedAt,
		&link.MaxDownloads,
		&link.DownloadCount,
		&link.ActiveDownloadCount,
		&link.DownloadLimitMode,
		&link.ShowChecksum,
		&link.PasswordRequired,
		&link.PasswordKDF,
		&link.PasswordSalt,
		&link.PasswordHash,
		&link.PasswordArgonTime,
		&link.PasswordArgonMemoryKiB,
		&link.PasswordArgonThreads,
		&link.CreatedAt,
		&link.UpdatedAt,
	)
	if err != nil {
		return PublicLink{}, err
	}
	return link, nil
}

func scanFileAndPublicLink(row rowScanner) (File, PublicLink, error) {
	var file File
	var link PublicLink
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
		&file.Checksum,
		&file.CreatedAt,
		&file.UpdatedAt,
		&link.ID,
		&link.FileID,
		&link.OwnerID,
		&link.Permission,
		&link.ExpiresAt,
		&link.RevokedAt,
		&link.MaxDownloads,
		&link.DownloadCount,
		&link.ActiveDownloadCount,
		&link.DownloadLimitMode,
		&link.ShowChecksum,
		&link.PasswordRequired,
		&link.PasswordKDF,
		&link.PasswordSalt,
		&link.PasswordHash,
		&link.PasswordArgonTime,
		&link.PasswordArgonMemoryKiB,
		&link.PasswordArgonThreads,
		&link.CreatedAt,
		&link.UpdatedAt,
	)
	if err != nil {
		return File{}, PublicLink{}, err
	}
	return file, link, nil
}

func nullableTime(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time
}

func nullablePasswordString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullablePasswordBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func nullablePasswordInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func nullableInt64(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

func nullableDownloadLimitMode(value string) any {
	if value == "" {
		return PublicDownloadLimitModeHard
	}
	return value
}

package recovery

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"filippo.io/age"

	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/crypto/secrets"
)

var ErrNotFound = errors.New("recovery resource not found")

type Store struct {
	db  *sql.DB
	box *secrets.Box
}

func NewStore(db *sql.DB, box *secrets.Box) *Store {
	return &Store{db: db, box: box}
}

func (s *Store) ExportManifest(ctx context.Context, userID string, exportedAt time.Time) (Manifest, error) {
	if exportedAt.IsZero() {
		exportedAt = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Manifest{}, err
	}
	defer tx.Rollback()

	user, err := loadUser(ctx, tx, userID)
	if err != nil {
		return Manifest{}, err
	}

	key, err := s.ensureUserKey(ctx, tx, userID)
	if err != nil {
		return Manifest{}, err
	}
	privateIdentity, err := s.decryptPrivateIdentity(userID, key.EncryptedPrivateIdentity)
	if err != nil {
		return Manifest{}, err
	}

	snapshotID, err := newUUID()
	if err != nil {
		return Manifest{}, err
	}
	snapshotVersion, err := nextSnapshotVersion(ctx, tx, userID)
	if err != nil {
		return Manifest{}, err
	}

	files, err := loadFiles(ctx, tx, userID)
	if err != nil {
		return Manifest{}, err
	}

	manifest := Manifest{
		Schema:          ManifestSchema,
		SnapshotID:      snapshotID,
		SnapshotVersion: snapshotVersion,
		ExportedAt:      exportedAt.UTC(),
		User: UserEntry{
			ID:                 user.ID,
			TelegramID:         user.TelegramID,
			Username:           user.Username.String,
			DisplayName:        user.DisplayName.String,
			AgePublicRecipient: key.PublicRecipient,
			AgePrivateIdentity: privateIdentity,
		},
		Files: files,
	}

	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return Manifest{}, err
	}
	manifestHash := sha256.Sum256(manifestBytes)
	if err := insertSnapshot(ctx, tx, snapshotID, userID, snapshotVersion, manifestHash[:]); err != nil {
		return Manifest{}, err
	}

	if err := tx.Commit(); err != nil {
		return Manifest{}, err
	}

	return manifest, nil
}

type userRow struct {
	ID          string
	TelegramID  int64
	Username    sql.NullString
	DisplayName sql.NullString
}

type userKey struct {
	PublicRecipient          string
	EncryptedPrivateIdentity []byte
}

func loadUser(ctx context.Context, tx *sql.Tx, userID string) (userRow, error) {
	var user userRow
	err := tx.QueryRowContext(ctx, `
SELECT id, telegram_id, username, display_name
FROM users
WHERE id = $1
FOR UPDATE`,
		userID,
	).Scan(&user.ID, &user.TelegramID, &user.Username, &user.DisplayName)
	if errors.Is(err, sql.ErrNoRows) {
		return userRow{}, ErrNotFound
	}
	if err != nil {
		return userRow{}, err
	}
	return user, nil
}

func (s *Store) ensureUserKey(ctx context.Context, tx *sql.Tx, userID string) (userKey, error) {
	key, err := loadUserKey(ctx, tx, userID)
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return userKey{}, err
	}

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return userKey{}, err
	}
	encryptedIdentity, err := s.box.Encrypt([]byte(identity.String()), recoveryKeyAAD(userID))
	if err != nil {
		return userKey{}, err
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO user_recovery_keys (user_id, public_recipient, encrypted_private_identity)
VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO NOTHING`,
		userID,
		identity.Recipient().String(),
		encryptedIdentity,
	); err != nil {
		return userKey{}, err
	}

	return loadUserKey(ctx, tx, userID)
}

func loadUserKey(ctx context.Context, tx *sql.Tx, userID string) (userKey, error) {
	var key userKey
	err := tx.QueryRowContext(ctx, `
SELECT public_recipient, encrypted_private_identity
FROM user_recovery_keys
WHERE user_id = $1`,
		userID,
	).Scan(&key.PublicRecipient, &key.EncryptedPrivateIdentity)
	return key, err
}

func (s *Store) decryptPrivateIdentity(userID string, ciphertext []byte) (string, error) {
	plaintext, err := s.box.Decrypt(ciphertext, recoveryKeyAAD(userID))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func recoveryKeyAAD(userID string) []byte {
	return []byte("recovery-key:" + userID)
}

func nextSnapshotVersion(ctx context.Context, tx *sql.Tx, userID string) (int, error) {
	var version int
	err := tx.QueryRowContext(ctx, `
SELECT COALESCE(MAX(snapshot_version), 0) + 1
FROM recovery_snapshots
WHERE user_id = $1`,
		userID,
	).Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

func loadFiles(ctx context.Context, tx *sql.Tx, userID string) ([]FileEntry, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT id, owner_id, parent_id, name_plain, mime_type, plaintext_size, ciphertext_size, type, status, checksum, created_at, updated_at, deleted_at
FROM files
WHERE owner_id = $1
ORDER BY created_at ASC, id ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make([]FileEntry, 0)
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

	for i := range files {
		if files[i].Type != "file" {
			continue
		}
		parts, err := loadParts(ctx, tx, files[i].ID)
		if err != nil {
			return nil, err
		}
		files[i].Parts = parts
	}

	return files, nil
}

func scanFile(scanner interface {
	Scan(dest ...any) error
}) (FileEntry, error) {
	var file FileEntry
	var parentID sql.NullString
	var namePlain sql.NullString
	var mimeType sql.NullString
	var plaintextSize sql.NullInt64
	var ciphertextSize sql.NullInt64
	var checksum []byte
	var deletedAt sql.NullTime

	if err := scanner.Scan(
		&file.ID,
		&file.OwnerID,
		&parentID,
		&namePlain,
		&mimeType,
		&plaintextSize,
		&ciphertextSize,
		&file.Type,
		&file.Status,
		&checksum,
		&file.CreatedAt,
		&file.UpdatedAt,
		&deletedAt,
	); err != nil {
		return FileEntry{}, err
	}

	file.ParentID = parentID.String
	file.NamePlain = namePlain.String
	file.MimeType = mimeType.String
	if plaintextSize.Valid {
		value := plaintextSize.Int64
		file.PlaintextSize = &value
	}
	if ciphertextSize.Valid {
		value := ciphertextSize.Int64
		file.CiphertextSize = &value
	}
	if len(checksum) > 0 {
		file.Checksum = checksum
	}
	if deletedAt.Valid {
		value := deletedAt.Time
		file.DeletedAt = &value
	}

	return file, nil
}

func loadParts(ctx context.Context, tx *sql.Tx, fileID string) ([]PartEntry, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT id, part_number, telegram_peer, telegram_message_id, ciphertext_size, checksum, created_at
FROM file_parts
WHERE file_id = $1
ORDER BY part_number ASC`,
		fileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	parts := make([]PartEntry, 0)
	for rows.Next() {
		var part PartEntry
		if err := rows.Scan(
			&part.ID,
			&part.PartNumber,
			&part.TelegramPeer,
			&part.TelegramMessageID,
			&part.CiphertextSize,
			&part.Checksum,
			&part.CreatedAt,
		); err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	return parts, rows.Err()
}

func insertSnapshot(ctx context.Context, tx *sql.Tx, snapshotID string, userID string, snapshotVersion int, manifestHash []byte) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO recovery_snapshots (id, user_id, snapshot_version, manifest_schema, manifest_sha256)
VALUES ($1, $2, $3, $4, $5)`,
		snapshotID,
		userID,
		snapshotVersion,
		ManifestSchema,
		manifestHash,
	)
	return err
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

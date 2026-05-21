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

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/crypto/secrets"
)

var ErrNotFound = errors.New("recovery resource not found")
var ErrInvalidManifest = errors.New("invalid recovery manifest")
var ErrConflict = errors.New("recovery import conflict")

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

type ImportSummary struct {
	SnapshotID                string `json:"snapshot_id"`
	SnapshotVersion           int    `json:"snapshot_version"`
	FilesImported             int    `json:"files_imported"`
	PartsImported             int    `json:"parts_imported"`
	UsedExistingRecoveryKey   bool   `json:"used_existing_recovery_key"`
	ImportedPrivateKeyFromMap bool   `json:"imported_private_key_from_map"`
}

func (s *Store) ImportManifest(ctx context.Context, userID string, manifest Manifest) (ImportSummary, error) {
	if manifest.Schema != ManifestSchema {
		return ImportSummary{}, fmt.Errorf("%w: unsupported schema", ErrInvalidManifest)
	}
	if err := validateManifest(manifest); err != nil {
		return ImportSummary{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ImportSummary{}, err
	}
	defer tx.Rollback()

	user, err := loadUser(ctx, tx, userID)
	if err != nil {
		return ImportSummary{}, err
	}
	if user.TelegramID != manifest.User.TelegramID {
		return ImportSummary{}, fmt.Errorf("%w: telegram id mismatch", ErrInvalidManifest)
	}

	var existingKey *userKey
	if key, err := loadUserKey(ctx, tx, userID); err == nil {
		existingKey = &key
	} else if !errors.Is(err, sql.ErrNoRows) {
		return ImportSummary{}, err
	}
	identity, shouldImportKey, err := resolveImportIdentity(manifest.User, existingKey)
	if err != nil {
		return ImportSummary{}, err
	}
	usedExistingRecoveryKey := !shouldImportKey
	if shouldImportKey {
		if err := s.importUserKey(ctx, tx, userID, identity); err != nil {
			return ImportSummary{}, err
		}
	}
	if err := ensureNoFileConflicts(ctx, tx, manifest.Files); err != nil {
		return ImportSummary{}, err
	}
	if err := ensureParentsExist(manifest.Files); err != nil {
		return ImportSummary{}, err
	}

	filesImported, partsImported, err := importFiles(ctx, tx, userID, manifest.Files)
	if err != nil {
		return ImportSummary{}, err
	}

	localSnapshotID, err := newUUID()
	if err != nil {
		return ImportSummary{}, err
	}
	localSnapshotVersion, err := nextSnapshotVersion(ctx, tx, userID)
	if err != nil {
		return ImportSummary{}, err
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return ImportSummary{}, err
	}
	manifestHash := sha256.Sum256(manifestBytes)
	if err := insertSnapshot(ctx, tx, localSnapshotID, userID, localSnapshotVersion, manifestHash[:]); err != nil {
		return ImportSummary{}, err
	}

	if err := tx.Commit(); err != nil {
		return ImportSummary{}, err
	}

	return ImportSummary{
		SnapshotID:                localSnapshotID,
		SnapshotVersion:           localSnapshotVersion,
		FilesImported:             filesImported,
		PartsImported:             partsImported,
		UsedExistingRecoveryKey:   usedExistingRecoveryKey,
		ImportedPrivateKeyFromMap: shouldImportKey,
	}, nil
}

func resolveImportIdentity(user UserEntry, existing *userKey) (*age.X25519Identity, bool, error) {
	if user.AgePublicRecipient == "" {
		return nil, false, fmt.Errorf("%w: missing recovery public recipient", ErrInvalidManifest)
	}
	if user.AgePrivateIdentity == "" {
		if existing == nil {
			return nil, false, fmt.Errorf("%w: missing recovery private key material", ErrInvalidManifest)
		}
		if existing.PublicRecipient != user.AgePublicRecipient {
			return nil, false, fmt.Errorf("%w: existing recovery key differs", ErrConflict)
		}
		return nil, false, nil
	}
	identity, err := age.ParseX25519Identity(user.AgePrivateIdentity)
	if err != nil {
		return nil, false, fmt.Errorf("%w: invalid age identity", ErrInvalidManifest)
	}
	if identity.Recipient().String() != user.AgePublicRecipient {
		return nil, false, fmt.Errorf("%w: age recipient mismatch", ErrInvalidManifest)
	}
	return identity, true, nil
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

func (s *Store) importUserKey(ctx context.Context, tx *sql.Tx, userID string, identity *age.X25519Identity) error {
	existing, err := loadUserKey(ctx, tx, userID)
	if err == nil {
		if existing.PublicRecipient != identity.Recipient().String() {
			return fmt.Errorf("%w: existing recovery key differs", ErrConflict)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	encryptedIdentity, err := s.box.Encrypt([]byte(identity.String()), recoveryKeyAAD(userID))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO user_recovery_keys (user_id, public_recipient, encrypted_private_identity)
VALUES ($1, $2, $3)`,
		userID,
		identity.Recipient().String(),
		encryptedIdentity,
	)
	return err
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
		parts, err := loadParts(ctx, tx, files[i].ID, files[i].OwnerID)
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

func loadParts(ctx context.Context, tx *sql.Tx, fileID string, ownerID string) ([]PartEntry, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT id, part_number, plaintext_start, plaintext_end, plaintext_size, telegram_peer, telegram_message_id, ciphertext_size, checksum, created_at
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
		var plaintextStart sql.NullInt64
		var plaintextEnd sql.NullInt64
		var plaintextSize sql.NullInt64
		if err := rows.Scan(
			&part.ID,
			&part.PartNumber,
			&plaintextStart,
			&plaintextEnd,
			&plaintextSize,
			&part.TelegramPeer,
			&part.TelegramMessageID,
			&part.CiphertextSize,
			&part.Checksum,
			&part.CreatedAt,
		); err != nil {
			return nil, err
		}
		if plaintextStart.Valid {
			value := plaintextStart.Int64
			part.PlaintextStart = &value
		}
		if plaintextEnd.Valid {
			value := plaintextEnd.Int64
			part.PlaintextEnd = &value
		}
		if plaintextSize.Valid {
			value := plaintextSize.Int64
			part.PlaintextSize = &value
		}
		part.StorageBackend = "telegram"
		part.StorageLocator = part.TelegramPeer
		part.StorageOwnerUser = ownerID
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

func ensureNoFileConflicts(ctx context.Context, tx *sql.Tx, files []FileEntry) error {
	for _, file := range files {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM files WHERE id = $1)`, file.ID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("%w: file %s already exists", ErrConflict, file.ID)
		}
		for _, part := range file.Parts {
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM file_parts WHERE id = $1)`, part.ID).Scan(&exists); err != nil {
				return err
			}
			if exists {
				return fmt.Errorf("%w: file part %s already exists", ErrConflict, part.ID)
			}
		}
	}
	return nil
}

func ensureParentsExist(files []FileEntry) error {
	ids := make(map[string]struct{}, len(files))
	for _, file := range files {
		if file.ID == "" {
			return fmt.Errorf("%w: missing file id", ErrInvalidManifest)
		}
		ids[file.ID] = struct{}{}
	}
	for _, file := range files {
		if file.ParentID == "" {
			continue
		}
		if _, ok := ids[file.ParentID]; !ok {
			return fmt.Errorf("%w: missing parent %s", ErrInvalidManifest, file.ParentID)
		}
	}
	return nil
}

func validateManifest(manifest Manifest) error {
	if manifest.SnapshotID == "" {
		return fmt.Errorf("%w: missing snapshot id", ErrInvalidManifest)
	}
	if manifest.SnapshotVersion <= 0 {
		return fmt.Errorf("%w: invalid snapshot version", ErrInvalidManifest)
	}
	if manifest.User.ID == "" {
		return fmt.Errorf("%w: missing user id", ErrInvalidManifest)
	}
	if manifest.User.TelegramID == 0 {
		return fmt.Errorf("%w: missing telegram id", ErrInvalidManifest)
	}

	fileIDs := make(map[string]struct{}, len(manifest.Files))
	partIDs := make(map[string]struct{})
	for _, file := range manifest.Files {
		if file.ID == "" {
			return fmt.Errorf("%w: missing file id", ErrInvalidManifest)
		}
		if _, exists := fileIDs[file.ID]; exists {
			return fmt.Errorf("%w: duplicate file id %s", ErrInvalidManifest, file.ID)
		}
		fileIDs[file.ID] = struct{}{}
		if file.OwnerID != "" && file.OwnerID != manifest.User.ID {
			return fmt.Errorf("%w: file %s owner mismatch", ErrInvalidManifest, file.ID)
		}
		if file.Type != "file" && file.Type != "folder" {
			return fmt.Errorf("%w: file %s has invalid type", ErrInvalidManifest, file.ID)
		}
		if file.Status != "pending" && file.Status != "ready" && file.Status != "deleted" && file.Status != "failed" {
			return fmt.Errorf("%w: file %s has invalid status", ErrInvalidManifest, file.ID)
		}
		if file.Type == "folder" && len(file.Parts) > 0 {
			return fmt.Errorf("%w: folder %s cannot contain parts", ErrInvalidManifest, file.ID)
		}

		partNumbers := make(map[int]struct{}, len(file.Parts))
		for _, part := range file.Parts {
			if part.ID == "" {
				return fmt.Errorf("%w: missing part id", ErrInvalidManifest)
			}
			if _, exists := partIDs[part.ID]; exists {
				return fmt.Errorf("%w: duplicate part id %s", ErrInvalidManifest, part.ID)
			}
			partIDs[part.ID] = struct{}{}
			if part.PartNumber <= 0 {
				return fmt.Errorf("%w: part %s has invalid number", ErrInvalidManifest, part.ID)
			}
			if _, exists := partNumbers[part.PartNumber]; exists {
				return fmt.Errorf("%w: duplicate part number %d", ErrInvalidManifest, part.PartNumber)
			}
			partNumbers[part.PartNumber] = struct{}{}
			if part.TelegramPeer == "" {
				return fmt.Errorf("%w: part %s missing telegram peer", ErrInvalidManifest, part.ID)
			}
			if part.TelegramMessageID <= 0 {
				return fmt.Errorf("%w: part %s missing telegram message id", ErrInvalidManifest, part.ID)
			}
			if part.CiphertextSize <= 0 {
				return fmt.Errorf("%w: part %s invalid ciphertext size", ErrInvalidManifest, part.ID)
			}
			if err := validatePartRange(part); err != nil {
				return err
			}
		}
	}
	if err := ensureParentsExist(manifest.Files); err != nil {
		return err
	}
	if err := ensureNoParentCycles(manifest.Files); err != nil {
		return err
	}
	return nil
}

func validatePartRange(part PartEntry) error {
	if part.PlaintextStart == nil && part.PlaintextEnd == nil && part.PlaintextSize == nil {
		return nil
	}
	if part.PlaintextStart == nil || part.PlaintextEnd == nil || part.PlaintextSize == nil {
		return fmt.Errorf("%w: part %s has incomplete plaintext range", ErrInvalidManifest, part.ID)
	}
	if *part.PlaintextStart < 0 || *part.PlaintextEnd < *part.PlaintextStart || *part.PlaintextSize < 0 {
		return fmt.Errorf("%w: part %s has invalid plaintext range", ErrInvalidManifest, part.ID)
	}
	if *part.PlaintextEnd-*part.PlaintextStart != *part.PlaintextSize {
		return fmt.Errorf("%w: part %s plaintext range size mismatch", ErrInvalidManifest, part.ID)
	}
	return nil
}

func ensureNoParentCycles(files []FileEntry) error {
	parents := make(map[string]string, len(files))
	for _, file := range files {
		parents[file.ID] = file.ParentID
	}
	for _, file := range files {
		seen := map[string]struct{}{file.ID: {}}
		for parentID := file.ParentID; parentID != ""; parentID = parents[parentID] {
			if _, exists := seen[parentID]; exists {
				return fmt.Errorf("%w: parent cycle at %s", ErrInvalidManifest, file.ID)
			}
			seen[parentID] = struct{}{}
		}
	}
	return nil
}

func importFiles(ctx context.Context, tx *sql.Tx, userID string, files []FileEntry) (int, int, error) {
	for _, file := range files {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO files (
    id, owner_id, parent_id, name_plain, mime_type, plaintext_size, ciphertext_size,
    type, status, checksum, created_at, updated_at, deleted_at
)
VALUES ($1, $2, NULL, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			file.ID,
			userID,
			nullableString(file.NamePlain),
			nullableString(file.MimeType),
			nullableInt64(file.PlaintextSize),
			nullableInt64(file.CiphertextSize),
			file.Type,
			file.Status,
			nullableBytes(file.Checksum),
			file.CreatedAt,
			file.UpdatedAt,
			nullableTimePtr(file.DeletedAt),
		); err != nil {
			return 0, 0, err
		}
	}

	for _, file := range files {
		if file.ParentID == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE files SET parent_id = $1 WHERE id = $2`, file.ParentID, file.ID); err != nil {
			return 0, 0, err
		}
	}

	partsImported := 0
	for _, file := range files {
		for _, part := range file.Parts {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO file_parts (
    id, file_id, part_number, plaintext_start, plaintext_end, plaintext_size,
    telegram_peer, telegram_message_id, ciphertext_size, checksum, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
				part.ID,
				file.ID,
				part.PartNumber,
				nullableInt64(part.PlaintextStart),
				nullableInt64(part.PlaintextEnd),
				nullableInt64(part.PlaintextSize),
				part.TelegramPeer,
				part.TelegramMessageID,
				part.CiphertextSize,
				nullableBytes(part.Checksum),
				part.CreatedAt,
			); err != nil {
				return 0, 0, err
			}
			partsImported++
		}
	}

	return len(files), partsImported, nil
}

func nullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func nullableBytes(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	return value
}

func nullableInt64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

func nullableTimePtr(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *value, Valid: true}
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

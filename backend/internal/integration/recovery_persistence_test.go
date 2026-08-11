package integration_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/NightSommelier/TeleVault/backend/internal/auth"
	"github.com/NightSommelier/TeleVault/backend/internal/crypto/secrets"
	"github.com/NightSommelier/TeleVault/backend/internal/recovery"
)

func TestRecoveryPersistenceExportImportRoundTrip(t *testing.T) {
	database := openIntegrationDB(t)
	sessionStore := auth.NewSessionStore(database)
	ctx := context.Background()

	user, cleanupUser := createUserThroughLogin(t, database, sessionStore, 972_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupUser()

	folderID := insertRecoveryTestFile(t, database, recoveryTestFile{
		OwnerID: user.ID,
		Type:    "folder",
		Name:    "recovery-folder",
		Status:  "ready",
	})
	fileID := insertRecoveryTestFile(t, database, recoveryTestFile{
		OwnerID:        user.ID,
		ParentID:       sql.NullString{String: folderID, Valid: true},
		Type:           "file",
		Name:           "recovery.bin",
		MimeType:       sql.NullString{String: "application/octet-stream", Valid: true},
		PlaintextSize:  sql.NullInt64{Int64: 11, Valid: true},
		CiphertextSize: sql.NullInt64{Int64: 29, Valid: true},
		Status:         "ready",
		Checksum:       []byte("file-checksum"),
	})
	partID := insertRecoveryTestPart(t, database, fileID)

	box, err := secrets.NewBox([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("secrets.NewBox() error = %v", err)
	}
	store := recovery.NewStore(database, box)

	exported, err := store.ExportManifest(ctx, user.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("ExportManifest() error = %v", err)
	}
	if len(exported.Files) != 2 {
		t.Fatalf("ExportManifest() files = %d, want 2", len(exported.Files))
	}

	if _, err := database.ExecContext(ctx, `DELETE FROM file_parts WHERE file_id = $1`, fileID); err != nil {
		t.Fatalf("delete exported file parts: %v", err)
	}
	if _, err := database.ExecContext(ctx, `DELETE FROM files WHERE owner_id = $1`, user.ID); err != nil {
		t.Fatalf("delete exported files: %v", err)
	}

	summary, err := store.ImportManifest(ctx, user.ID, exported, recovery.ImportOptions{
		Mode:           recovery.ImportModeReplace,
		ConfirmReplace: true,
	})
	if err != nil {
		t.Fatalf("ImportManifest() error = %v", err)
	}
	if summary.FilesImported != 2 || summary.PartsImported != 1 {
		t.Fatalf("ImportManifest() summary = %+v, want 2 files and 1 part", summary)
	}

	var restoredParent sql.NullString
	var restoredName string
	var restoredPlaintextSize int64
	err = database.QueryRowContext(ctx, `
SELECT parent_id, name_plain, plaintext_size
FROM files
WHERE id = $1
  AND owner_id = $2`,
		fileID,
		user.ID,
	).Scan(&restoredParent, &restoredName, &restoredPlaintextSize)
	if err != nil {
		t.Fatalf("restored file query error = %v", err)
	}
	if !restoredParent.Valid || restoredParent.String != folderID || restoredName != "recovery.bin" || restoredPlaintextSize != 11 {
		t.Fatalf("restored file parent=%+v name=%q size=%d, want exported graph", restoredParent, restoredName, restoredPlaintextSize)
	}

	var restoredPartNumber int
	var restoredPeer string
	var restoredMessageID int64
	err = database.QueryRowContext(ctx, `
SELECT part_number, telegram_peer, telegram_message_id
FROM file_parts
WHERE id = $1
  AND file_id = $2`,
		partID,
		fileID,
	).Scan(&restoredPartNumber, &restoredPeer, &restoredMessageID)
	if err != nil {
		t.Fatalf("restored part query error = %v", err)
	}
	if restoredPartNumber != 1 || restoredPeer != "self" || restoredMessageID != 707 {
		t.Fatalf("restored part number=%d peer=%q message=%d, want exported part", restoredPartNumber, restoredPeer, restoredMessageID)
	}
}

type recoveryTestFile struct {
	OwnerID        string
	ParentID       sql.NullString
	Type           string
	Name           string
	MimeType       sql.NullString
	PlaintextSize  sql.NullInt64
	CiphertextSize sql.NullInt64
	Status         string
	Checksum       []byte
}

func insertRecoveryTestFile(t *testing.T, database *sql.DB, file recoveryTestFile) string {
	t.Helper()

	var id string
	err := database.QueryRowContext(context.Background(), `
INSERT INTO files (
    owner_id, parent_id, name_plain, mime_type, plaintext_size, ciphertext_size,
    type, status, checksum
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id`,
		file.OwnerID,
		file.ParentID,
		sql.NullString{String: file.Name, Valid: file.Name != ""},
		file.MimeType,
		file.PlaintextSize,
		file.CiphertextSize,
		file.Type,
		file.Status,
		file.Checksum,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert recovery test file: %v", err)
	}
	return id
}

func insertRecoveryTestPart(t *testing.T, database *sql.DB, fileID string) string {
	t.Helper()

	var id string
	err := database.QueryRowContext(context.Background(), `
INSERT INTO file_parts (
    file_id, part_number, plaintext_start, plaintext_end, plaintext_size,
    telegram_peer, telegram_message_id, ciphertext_size, checksum
)
VALUES ($1, 1, 0, 11, 11, 'self', 707, 29, $2)
RETURNING id`,
		fileID,
		[]byte("part-checksum"),
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert recovery test part: %v", err)
	}
	return id
}

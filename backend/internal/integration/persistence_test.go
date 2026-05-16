package integration_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/televault/TeleVault/backend/internal/adminsettings"
	"github.com/televault/TeleVault/backend/internal/auth"
	"github.com/televault/TeleVault/backend/internal/config"
	"github.com/televault/TeleVault/backend/internal/db"
	"github.com/televault/TeleVault/backend/internal/files"
	"github.com/televault/TeleVault/backend/internal/uploads"
)

func TestAuthPersistenceChallengeAndSessionLifecycle(t *testing.T) {
	database := openIntegrationDB(t)
	store := auth.NewSessionStore(database)
	ctx := context.Background()

	phoneHash := []byte("integration-phone-hash-" + uniqueSuffix())
	t.Cleanup(func() {
		_, _ = database.ExecContext(context.Background(), `DELETE FROM auth_challenges WHERE phone_hash = $1`, phoneHash)
	})

	err := store.CreateAuthChallenge(ctx, phoneHash, "phone-code-hash", []byte("encrypted-client-session"), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateAuthChallenge() error = %v", err)
	}

	challenge, err := store.LatestAuthChallenge(ctx, phoneHash)
	if err != nil {
		t.Fatalf("LatestAuthChallenge() error = %v", err)
	}
	if challenge.PhoneCodeHash != "phone-code-hash" || !bytes.Equal(challenge.EncryptedClientSession, []byte("encrypted-client-session")) {
		t.Fatalf("LatestAuthChallenge() = %+v, want persisted challenge", challenge)
	}

	user, cleanupUser := createUserThroughLogin(t, database, store, 910_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupUser()

	refreshHash := []byte("refresh-hash-old")
	err = store.CreateSession(ctx, user.ID, refreshHash, "integration-agent", []byte("ip-hash"), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	gotUser, err := store.UserByRefreshToken(ctx, refreshHash)
	if err != nil {
		t.Fatalf("UserByRefreshToken() error = %v", err)
	}
	if gotUser.ID != user.ID {
		t.Fatalf("UserByRefreshToken() user ID = %s, want %s", gotUser.ID, user.ID)
	}

	newHash := []byte("refresh-hash-new")
	rotatedUser, err := store.RotateRefreshToken(ctx, refreshHash, newHash, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("RotateRefreshToken() error = %v", err)
	}
	if rotatedUser.ID != user.ID {
		t.Fatalf("RotateRefreshToken() user ID = %s, want %s", rotatedUser.ID, user.ID)
	}
	if _, err := store.UserByRefreshToken(ctx, refreshHash); !errors.Is(err, auth.ErrInvalidSession) {
		t.Fatalf("old refresh token error = %v, want ErrInvalidSession", err)
	}

	if err := store.RevokeRefreshToken(ctx, newHash); err != nil {
		t.Fatalf("RevokeRefreshToken() error = %v", err)
	}
	if _, err := store.UserByRefreshToken(ctx, newHash); !errors.Is(err, auth.ErrInvalidSession) {
		t.Fatalf("revoked refresh token error = %v, want ErrInvalidSession", err)
	}
}

func TestFilesUploadsPersistenceOwnerIsolationAndCompletion(t *testing.T) {
	database := openIntegrationDB(t)
	sessionStore := auth.NewSessionStore(database)
	fileStore := files.NewStore(database)
	uploadStore := uploads.NewStore(database)
	ctx := context.Background()

	owner, cleanupOwner := createUserThroughLogin(t, database, sessionStore, 920_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupOwner()
	other, cleanupOther := createUserThroughLogin(t, database, sessionStore, 930_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupOther()

	folder, err := fileStore.CreateFolder(ctx, owner.ID, "", "integration-folder")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}
	if _, err := fileStore.GetByID(ctx, other.ID, folder.ID); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("cross-owner folder read error = %v, want files.ErrNotFound", err)
	}

	upload, err := uploadStore.Create(ctx, uploads.CreateUploadParams{
		OwnerID:       owner.ID,
		ParentID:      folder.ID,
		Name:          "integration.bin",
		MimeType:      "application/octet-stream",
		PlaintextSize: 6,
		PartSize:      3,
		ExpiresAt:     time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Create upload error = %v", err)
	}

	_, err = uploadStore.CompletePart(ctx, uploads.CompletePartParams{
		OwnerID:        owner.ID,
		UploadID:       upload.ID,
		PartNumber:     1,
		PlaintextSize:  3,
		CiphertextSize: 12,
		Checksum:       []byte("part-1"),
		UploadedSize:   3,
		TelegramPeer:   "self",
		MessageID:      101,
		Now:            time.Now(),
	})
	if err != nil {
		t.Fatalf("CompletePart(1) error = %v", err)
	}

	_, err = uploadStore.CompletePart(ctx, uploads.CompletePartParams{
		OwnerID:        owner.ID,
		UploadID:       upload.ID,
		PartNumber:     2,
		PlaintextSize:  3,
		CiphertextSize: 13,
		Checksum:       []byte("part-2"),
		UploadedSize:   6,
		TelegramPeer:   "self",
		MessageID:      102,
		Now:            time.Now(),
	})
	if err != nil {
		t.Fatalf("CompletePart(2) error = %v", err)
	}

	file, err := uploadStore.CompleteUpload(ctx, uploads.CompleteUploadParams{
		OwnerID:  owner.ID,
		UploadID: upload.ID,
		Now:      time.Now(),
	})
	if err != nil {
		t.Fatalf("CompleteUpload() error = %v", err)
	}
	if file.OwnerID != owner.ID || file.Type != files.TypeFile || file.Status != files.StatusReady {
		t.Fatalf("CompleteUpload() file = %+v, want ready owner file", file)
	}

	downloadFile, parts, session, err := fileStore.DownloadData(ctx, owner.ID, file.ID)
	if err != nil {
		t.Fatalf("DownloadData() error = %v", err)
	}
	if downloadFile.ID != file.ID || len(parts) != 2 {
		t.Fatalf("DownloadData() file ID = %s parts = %d, want file %s with 2 parts", downloadFile.ID, len(parts), file.ID)
	}
	if parts[0].TelegramMessageID != 101 || parts[1].TelegramMessageID != 102 {
		t.Fatalf("DownloadData() message IDs = %d,%d, want 101,102", parts[0].TelegramMessageID, parts[1].TelegramMessageID)
	}
	if !bytes.Equal(session.EncryptedSession, []byte("encrypted-telegram-session")) {
		t.Fatalf("DownloadData() telegram session = %q, want persisted encrypted session", string(session.EncryptedSession))
	}
	if _, _, _, err := fileStore.DownloadData(ctx, other.ID, file.ID); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("cross-owner download error = %v, want files.ErrNotFound", err)
	}
}

func TestAdminUploadSettingsVaultUploadPartSize(t *testing.T) {
	database := openIntegrationDB(t)
	sessionStore := auth.NewSessionStore(database)
	settingsStore := adminsettings.NewStore(database, config.Config{
		UploadPartSizeBytes:        64,
		TelegramDocumentLimitBytes: 1024,
		UploadSafetyMarginBytes:    64,
	})
	uploadStore := uploads.NewStore(database)
	ctx := context.Background()

	admin, cleanupAdmin := createUserThroughLogin(t, database, sessionStore, 940_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupAdmin()

	settings, err := settingsStore.UpdateUploadSettings(ctx, adminsettings.UploadSettings{
		UploadPartSizeBytes:        128,
		TelegramDocumentLimitBytes: 1024,
		UploadSafetyMarginBytes:    64,
	}, admin.ID)
	if err != nil {
		t.Fatalf("UpdateUploadSettings() error = %v", err)
	}
	if settings.UploadPartSizeBytes != 128 {
		t.Fatalf("UploadPartSizeBytes = %d, want 128", settings.UploadPartSizeBytes)
	}
	t.Cleanup(func() {
		_, _ = settingsStore.UpdateUploadSettings(context.Background(), adminsettings.UploadSettings{
			UploadPartSizeBytes:        config.DefaultUploadPartSizeBytes,
			TelegramDocumentLimitBytes: config.DefaultTelegramDocumentLimitBytes,
			UploadSafetyMarginBytes:    config.DefaultUploadSafetyMarginBytes,
		}, "")
	})

	upload, err := uploadStore.Create(ctx, uploads.CreateUploadParams{
		OwnerID:       admin.ID,
		Name:          "admin-settings.bin",
		PlaintextSize: 256,
		PartSize:      settings.UploadPartSizeBytes,
		ExpiresAt:     time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Create upload error = %v", err)
	}
	if upload.PartSize != 128 {
		t.Fatalf("upload PartSize = %d, want admin setting 128", upload.PartSize)
	}
}

func TestEffectiveUploadSettingsUseAccountLimit(t *testing.T) {
	database := openIntegrationDB(t)
	sessionStore := auth.NewSessionStore(database)
	settingsStore := adminsettings.NewStore(database, config.Config{
		UploadPartSizeBytes:        512,
		TelegramDocumentLimitBytes: 2048,
		UploadSafetyMarginBytes:    64,
	})
	ctx := context.Background()

	admin, cleanupAdmin := createUserThroughLogin(t, database, sessionStore, 950_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupAdmin()

	if _, err := settingsStore.UpdateUploadSettings(ctx, adminsettings.UploadSettings{
		UploadPartSizeBytes:        512,
		TelegramDocumentLimitBytes: 2048,
		UploadSafetyMarginBytes:    64,
	}, admin.ID); err != nil {
		t.Fatalf("UpdateUploadSettings() error = %v", err)
	}
	t.Cleanup(func() {
		_, _ = settingsStore.UpdateUploadSettings(context.Background(), adminsettings.UploadSettings{
			UploadPartSizeBytes:        config.DefaultUploadPartSizeBytes,
			TelegramDocumentLimitBytes: config.DefaultTelegramDocumentLimitBytes,
			UploadSafetyMarginBytes:    config.DefaultUploadSafetyMarginBytes,
		}, "")
	})

	if _, err := settingsStore.UpsertTelegramAccountLimit(ctx, admin.ID, adminsettings.TelegramAccountLimit{
		TelegramDocumentLimitBytes: 256,
		UploadSafetyMarginBytes:    32,
	}, admin.ID); err != nil {
		t.Fatalf("UpsertTelegramAccountLimit() error = %v", err)
	}

	effective, err := settingsStore.EffectiveUploadSettings(ctx, admin.ID)
	if err != nil {
		t.Fatalf("EffectiveUploadSettings() error = %v", err)
	}
	if effective.UploadPartSizeBytes != 224 {
		t.Fatalf("UploadPartSizeBytes = %d, want account cap minus margin 224", effective.UploadPartSizeBytes)
	}
	if effective.Source != "account_manual" {
		t.Fatalf("Source = %q, want account_manual", effective.Source)
	}
}

func openIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run Postgres integration tests")
	}

	database, err := db.Open(databaseURL)
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.Ping(ctx, database); err != nil {
		t.Fatalf("db.Ping() error = %v", err)
	}
	ensureLatestMigration(t, database)
	return database
}

func ensureLatestMigration(t *testing.T, database *sql.DB) {
	t.Helper()

	var exists bool
	err := database.QueryRowContext(context.Background(), `
SELECT EXISTS (
    SELECT 1
    FROM schema_migrations
    WHERE version = '000009'
)`).Scan(&exists)
	if err != nil {
		t.Fatalf("schema migration check failed: %v", err)
	}
	if !exists {
		t.Fatalf("TEST_DATABASE_URL database is not migrated through 000009; run go run ./cmd/migrate up first")
	}
}

func createUserThroughLogin(t *testing.T, database *sql.DB, store *auth.SessionStore, telegramID int64) (auth.User, func()) {
	t.Helper()

	user, err := store.CompleteTelegramLogin(
		context.Background(),
		auth.TelegramProfile{
			TelegramID:  telegramID,
			Username:    fmt.Sprintf("integration_%d", telegramID),
			DisplayName: "Integration Test",
		},
		[]byte("encrypted-telegram-session"),
		[]byte(fmt.Sprintf("initial-refresh-%d", telegramID)),
		"integration-test",
		nil,
		time.Now().Add(time.Hour),
	)
	if err != nil {
		t.Fatalf("CompleteTelegramLogin() error = %v", err)
	}

	cleanup := func() {
		_, _ = database.ExecContext(context.Background(), `DELETE FROM users WHERE telegram_id = $1`, telegramID)
	}
	return user, cleanup
}

func uniqueSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

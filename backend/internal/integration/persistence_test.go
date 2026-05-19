package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/adminsettings"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/adminusers"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/auth"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/config"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/db"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/files"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/telegramprobe"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/uploads"
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
	targetFolder, err := fileStore.CreateFolder(ctx, owner.ID, "", "integration-target-folder")
	if err != nil {
		t.Fatalf("CreateFolder(target) error = %v", err)
	}
	movedFolder, err := fileStore.Move(ctx, owner.ID, folder.ID, targetFolder.ID)
	if err != nil {
		t.Fatalf("Move(folder) error = %v", err)
	}
	if !movedFolder.ParentID.Valid || movedFolder.ParentID.String != targetFolder.ID {
		t.Fatalf("Move(folder) parent = %+v, want %s", movedFolder.ParentID, targetFolder.ID)
	}
	if _, err := fileStore.Move(ctx, other.ID, folder.ID, ""); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("cross-owner move error = %v, want files.ErrNotFound", err)
	}
	if _, err := fileStore.Move(ctx, owner.ID, targetFolder.ID, folder.ID); !errors.Is(err, files.ErrInvalidMove) {
		t.Fatalf("cycle move error = %v, want files.ErrInvalidMove", err)
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

	partCount, err := fileStore.CountFileParts(ctx, file.ID)
	if err != nil {
		t.Fatalf("CountFileParts() error = %v", err)
	}
	if partCount != 2 {
		t.Fatalf("CountFileParts() = %d, want 2", partCount)
	}
	plainLink, err := fileStore.CreatePublicLink(ctx, owner.ID, file.ID, []byte("plain-token"), sql.NullTime{}, files.PublicLinkPassword{})
	if err != nil {
		t.Fatalf("CreatePublicLink(plain) error = %v", err)
	}
	passwordLink, err := fileStore.CreatePublicLink(ctx, owner.ID, file.ID, []byte("password-token"), sql.NullTime{}, files.PublicLinkPassword{
		KDF:            "argon2id",
		Salt:           []byte("password-salt-1234"),
		Hash:           []byte("password-hash-abc"),
		ArgonTime:      1,
		ArgonMemoryKiB: 1,
		ArgonThreads:   1,
	})
	if err != nil {
		t.Fatalf("CreatePublicLink(password) error = %v", err)
	}
	publicLinkCount, passwordProtectedCount, err := fileStore.CountActivePublicLinks(ctx, owner.ID, file.ID)
	if err != nil {
		t.Fatalf("CountActivePublicLinks() error = %v", err)
	}
	if publicLinkCount != 2 || passwordProtectedCount != 1 {
		t.Fatalf("CountActivePublicLinks() = %d,%d, want 2,1", publicLinkCount, passwordProtectedCount)
	}
	if plainLink.ID == "" || passwordLink.ID == "" {
		t.Fatal("CreatePublicLink() returned empty id")
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
	share, err := fileStore.CreateShare(ctx, owner.ID, file.ID, other.TelegramID, sql.NullTime{})
	if err != nil {
		t.Fatalf("CreateShare() error = %v", err)
	}
	if share.GranteeUserID != other.ID || share.FileID != file.ID {
		t.Fatalf("CreateShare() = %+v, want grantee %s file %s", share, other.ID, file.ID)
	}
	shared, err := fileStore.ListSharedWithMe(ctx, other.ID)
	if err != nil {
		t.Fatalf("ListSharedWithMe() error = %v", err)
	}
	if len(shared) != 1 || shared[0].ID != file.ID {
		t.Fatalf("ListSharedWithMe() = %+v, want shared file %s", shared, file.ID)
	}
	sharedDownload, sharedParts, _, err := fileStore.DownloadData(ctx, other.ID, file.ID)
	if err != nil {
		t.Fatalf("shared DownloadData() error = %v", err)
	}
	if sharedDownload.OwnerID != owner.ID || len(sharedParts) != 2 {
		t.Fatalf("shared DownloadData() owner=%s parts=%d, want owner %s parts 2", sharedDownload.OwnerID, len(sharedParts), owner.ID)
	}
	if err := fileStore.RevokeShare(ctx, owner.ID, file.ID, share.ID, time.Now()); err != nil {
		t.Fatalf("RevokeShare() error = %v", err)
	}
	if _, _, _, err := fileStore.DownloadData(ctx, other.ID, file.ID); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("revoked shared download error = %v, want files.ErrNotFound", err)
	}
	publicTokenHash := sha256.Sum256([]byte("integration-public-token-" + uniqueSuffix()))
	publicLink, err := fileStore.CreatePublicLink(ctx, owner.ID, file.ID, publicTokenHash[:], sql.NullTime{}, files.PublicLinkPassword{})
	if err != nil {
		t.Fatalf("CreatePublicLink() error = %v", err)
	}
	publicLinks, err := fileStore.ListPublicLinks(ctx, owner.ID, file.ID)
	if err != nil {
		t.Fatalf("ListPublicLinks() error = %v", err)
	}
	if len(publicLinks) != 1 || publicLinks[0].ID != publicLink.ID {
		t.Fatalf("ListPublicLinks() = %+v, want link %s", publicLinks, publicLink.ID)
	}
	publicDownload, publicParts, publicSession, err := fileStore.DownloadDataByPublicTokenHash(ctx, publicTokenHash[:])
	if err != nil {
		t.Fatalf("DownloadDataByPublicTokenHash() error = %v", err)
	}
	if publicDownload.ID != file.ID || len(publicParts) != 2 || publicSession.OwnerTelegramID != owner.TelegramID {
		t.Fatalf("public download file=%s parts=%d owner telegram=%d, want %s parts 2 owner telegram %d", publicDownload.ID, len(publicParts), publicSession.OwnerTelegramID, file.ID, owner.TelegramID)
	}
	if err := fileStore.RevokePublicLink(ctx, owner.ID, file.ID, publicLink.ID, time.Now()); err != nil {
		t.Fatalf("RevokePublicLink() error = %v", err)
	}
	if _, _, _, err := fileStore.DownloadDataByPublicTokenHash(ctx, publicTokenHash[:]); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("revoked public download error = %v, want files.ErrNotFound", err)
	}
	protectedTokenHash := sha256.Sum256([]byte("integration-protected-token-" + uniqueSuffix()))
	protectedLink, err := fileStore.CreatePublicLink(ctx, owner.ID, file.ID, protectedTokenHash[:], sql.NullTime{}, files.PublicLinkPassword{
		KDF:            "argon2id",
		Salt:           []byte("integration-salt"),
		Hash:           []byte("integration-hash"),
		ArgonTime:      1,
		ArgonMemoryKiB: 64 * 1024,
		ArgonThreads:   4,
	})
	if err != nil {
		t.Fatalf("CreatePublicLink(protected) error = %v", err)
	}
	if !protectedLink.PasswordRequired {
		t.Fatalf("CreatePublicLink(protected) PasswordRequired = false, want true")
	}
	if _, _, _, err := fileStore.DownloadDataByPublicTokenHash(ctx, protectedTokenHash[:]); !errors.Is(err, files.ErrPasswordRequired) {
		t.Fatalf("protected public download error = %v, want files.ErrPasswordRequired", err)
	}
	if err := fileStore.SoftDelete(ctx, other.ID, file.ID, time.Now()); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("cross-owner delete error = %v, want files.ErrNotFound", err)
	}
	if _, err := fileStore.GetByID(ctx, owner.ID, file.ID); err != nil {
		t.Fatalf("owner file after cross-owner delete error = %v", err)
	}
	if err := fileStore.SoftDelete(ctx, owner.ID, folder.ID, time.Now()); err != nil {
		t.Fatalf("SoftDelete(folder) error = %v", err)
	}
	if _, err := fileStore.GetByID(ctx, owner.ID, folder.ID); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("deleted folder get error = %v, want files.ErrNotFound", err)
	}
	if _, _, _, err := fileStore.DownloadData(ctx, owner.ID, file.ID); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("deleted child download error = %v, want files.ErrNotFound", err)
	}
	children, err := fileStore.ListChildren(ctx, owner.ID, folder.ID)
	if err != nil {
		t.Fatalf("ListChildren(deleted folder) error = %v", err)
	}
	if len(children) != 0 {
		t.Fatalf("ListChildren(deleted folder) returned %d children, want 0", len(children))
	}
	cleanupArtifacts, err := uploadStore.PendingTelegramCleanupArtifacts(ctx, 1000)
	if err != nil {
		t.Fatalf("PendingTelegramCleanupArtifacts() error = %v", err)
	}
	var foundDeletedFileParts int
	for _, artifact := range cleanupArtifacts {
		if artifact.Source == "file_part" && artifact.ResourceID == file.ID {
			foundDeletedFileParts++
		}
	}
	if foundDeletedFileParts != 1 {
		t.Fatalf("immediately available deleted file cleanup artifacts = %d, want 1", foundDeletedFileParts)
	}
	var queuedParts int
	var minAvailable, maxAvailable time.Time
	err = database.QueryRowContext(ctx, `
SELECT COUNT(*), MIN(telegram_delete_available_at), MAX(telegram_delete_available_at)
FROM file_parts
WHERE file_id = $1
  AND telegram_deleted_at IS NULL`,
		file.ID,
	).Scan(&queuedParts, &minAvailable, &maxAvailable)
	if err != nil {
		t.Fatalf("deleted file cleanup queue query error = %v", err)
	}
	if queuedParts != 2 {
		t.Fatalf("deleted file queued parts = %d, want 2", queuedParts)
	}
	if maxAvailable.Sub(minAvailable) < 15*time.Second {
		t.Fatalf("deleted file cleanup delay = %s, want at least 15s", maxAvailable.Sub(minAvailable))
	}
}

func TestUploadPartQueueLeaseRetryAndFail(t *testing.T) {
	database := openIntegrationDB(t)
	sessionStore := auth.NewSessionStore(database)
	uploadStore := uploads.NewStore(database)
	ctx := context.Background()

	owner, cleanupOwner := createUserThroughLogin(t, database, sessionStore, 935_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupOwner()

	now := time.Now().UTC()
	upload, err := uploadStore.Create(ctx, uploads.CreateUploadParams{
		OwnerID:       owner.ID,
		Name:          "queue.bin",
		PlaintextSize: 10,
		PartSize:      5,
		ExpiresAt:     now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Create upload error = %v", err)
	}

	staged, err := uploadStore.StagePart(ctx, uploads.StagePartParams{
		OwnerID:        owner.ID,
		UploadID:       upload.ID,
		PartNumber:     1,
		PlaintextSize:  5,
		CiphertextSize: 9,
		Checksum:       []byte("part-1"),
		UploadedSize:   5,
		StorageBackend: "local",
		StorageKey:     "queue.bin.part-1",
		AvailableAt:    now,
		Now:            now,
	})
	if err != nil {
		t.Fatalf("StagePart(1) error = %v", err)
	}
	if staged.StorageBackend.String != "local" || staged.StorageKey.String != "queue.bin.part-1" {
		t.Fatalf("StagePart() storage = %q/%q, want local/queue.bin.part-1", staged.StorageBackend.String, staged.StorageKey.String)
	}
	if _, err := uploadStore.StagePart(ctx, uploads.StagePartParams{
		OwnerID:        owner.ID,
		UploadID:       upload.ID,
		PartNumber:     2,
		PlaintextSize:  5,
		CiphertextSize: 10,
		Checksum:       []byte("part-2"),
		UploadedSize:   10,
		StorageBackend: "local",
		StorageKey:     "queue.bin.part-2",
		AvailableAt:    now.Add(time.Minute),
		Now:            now,
	}); err != nil {
		t.Fatalf("StagePart(2) error = %v", err)
	}

	claimed, err := uploadStore.ClaimQueuedPart(ctx, uploads.ClaimQueuedPartParams{
		WorkerID:      "worker-a",
		Now:           now,
		LeaseDuration: time.Minute,
	})
	if err != nil {
		t.Fatalf("ClaimQueuedPart() error = %v", err)
	}
	if claimed.ID != staged.ID || claimed.Attempts != 1 || claimed.WorkerID.String != "worker-a" || !claimed.LeasedUntil.Valid {
		t.Fatalf("ClaimQueuedPart() = %+v, want staged part leased by worker-a with attempts=1", claimed)
	}

	if _, err := uploadStore.ClaimQueuedPart(ctx, uploads.ClaimQueuedPartParams{
		WorkerID:      "worker-b",
		Now:           now,
		LeaseDuration: time.Minute,
	}); !errors.Is(err, uploads.ErrUploadPartNotFound) {
		t.Fatalf("second ClaimQueuedPart() error = %v, want ErrUploadPartNotFound", err)
	}

	if err := uploadStore.RetryQueuedPart(ctx, uploads.RetryPartParams{
		PartID:      claimed.ID,
		LastError:   "FLOOD_WAIT_30",
		AvailableAt: now.Add(30 * time.Second),
	}); err != nil {
		t.Fatalf("RetryQueuedPart() error = %v", err)
	}
	work, err := uploadStore.ClaimQueuedPartWork(ctx, uploads.ClaimQueuedPartParams{
		WorkerID:      "worker-b",
		Now:           now.Add(31 * time.Second),
		LeaseDuration: time.Minute,
	})
	if err != nil {
		t.Fatalf("ClaimQueuedPartWork() after retry error = %v", err)
	}
	if work.Part.ID != staged.ID || work.OwnerID != owner.ID || work.OwnerTelegramID != owner.TelegramID || !bytes.Equal(work.EncryptedSession, []byte("encrypted-telegram-session")) {
		t.Fatalf("ClaimQueuedPartWork() = %+v, want staged part with owner telegram session context", work)
	}

	if err := uploadStore.FailQueuedPart(ctx, staged.ID, errors.New("permanent failure")); err != nil {
		t.Fatalf("FailQueuedPart() error = %v", err)
	}
	next, err := uploadStore.ClaimQueuedPart(ctx, uploads.ClaimQueuedPartParams{
		WorkerID:      "worker-c",
		Now:           now.Add(2 * time.Minute),
		LeaseDuration: time.Minute,
	})
	if err != nil {
		t.Fatalf("ClaimQueuedPart() after fail error = %v", err)
	}
	if next.PartNumber != 2 {
		t.Fatalf("ClaimQueuedPart() after fail part number = %d, want next queued part 2", next.PartNumber)
	}

	completed, err := uploadStore.MarkStagedPartUploaded(ctx, uploads.MarkStagedPartUploadedParams{
		PartID:       next.ID,
		TelegramPeer: "self",
		MessageID:    202,
	})
	if err != nil {
		t.Fatalf("MarkStagedPartUploaded() error = %v", err)
	}
	if completed.Status != uploads.StatusComplete || !completed.MessageID.Valid || completed.MessageID.Int64 != 202 {
		t.Fatalf("MarkStagedPartUploaded() = %+v, want complete Telegram part", completed)
	}

	statusUpload, statusParts, err := uploadStore.GetWithParts(ctx, owner.ID, upload.ID)
	if err != nil {
		t.Fatalf("GetWithParts() error = %v", err)
	}
	if statusUpload.ID != upload.ID || len(statusParts) != 2 {
		t.Fatalf("GetWithParts() upload = %+v parts = %d, want upload with 2 parts", statusUpload, len(statusParts))
	}
	if _, _, err := uploadStore.GetWithParts(ctx, "00000000-0000-0000-0000-000000000000", upload.ID); !errors.Is(err, uploads.ErrUploadNotFound) {
		t.Fatalf("cross-owner GetWithParts() error = %v, want ErrUploadNotFound", err)
	}
}

func TestLocalStagingCleanupArtifacts(t *testing.T) {
	database := openIntegrationDB(t)
	sessionStore := auth.NewSessionStore(database)
	uploadStore := uploads.NewStore(database)
	ctx := context.Background()

	owner, cleanupOwner := createUserThroughLogin(t, database, sessionStore, 938_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupOwner()

	now := time.Now().UTC()
	upload, err := uploadStore.Create(ctx, uploads.CreateUploadParams{
		OwnerID:       owner.ID,
		Name:          "cleanup.bin",
		PlaintextSize: 5,
		PartSize:      5,
		ExpiresAt:     now.Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("Create upload error = %v", err)
	}
	if _, err := database.ExecContext(ctx, `
INSERT INTO upload_parts (upload_id, part_number, plaintext_size, ciphertext_size, status, storage_backend, storage_key)
VALUES ($1, 1, 5, 10, 'pending', 'local', 'cleanup.bin.part-1')`,
		upload.ID,
	); err != nil {
		t.Fatalf("insert staged part error = %v", err)
	}
	if _, err := uploadStore.ExpireAbandoned(ctx, now); err != nil {
		t.Fatalf("ExpireAbandoned() error = %v", err)
	}

	artifacts, err := uploadStore.PendingLocalStagingCleanupArtifacts(ctx, 10)
	if err != nil {
		t.Fatalf("PendingLocalStagingCleanupArtifacts() error = %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].UploadID != upload.ID || artifacts[0].StorageKey != "cleanup.bin.part-1" {
		t.Fatalf("PendingLocalStagingCleanupArtifacts() = %+v, want staged cleanup artifact", artifacts)
	}
	if err := uploadStore.MarkLocalStagingDeleted(ctx, artifacts[0].PartID); err != nil {
		t.Fatalf("MarkLocalStagingDeleted() error = %v", err)
	}
	artifacts, err = uploadStore.PendingLocalStagingCleanupArtifacts(ctx, 10)
	if err != nil {
		t.Fatalf("PendingLocalStagingCleanupArtifacts() after mark error = %v", err)
	}
	if len(artifacts) != 0 {
		t.Fatalf("PendingLocalStagingCleanupArtifacts() after mark = %+v, want empty", artifacts)
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
		UploadPartSizeBytes:          128,
		TelegramDocumentLimitBytes:   1024,
		UploadSafetyMarginBytes:      64,
		MaxParallelUploads:           1,
		TargetUploadBytesPerSecond:   0,
		CooldownBetweenPartsMillisec: 0,
	}, admin.ID)
	if err != nil {
		t.Fatalf("UpdateUploadSettings() error = %v", err)
	}
	if settings.UploadPartSizeBytes != 128 {
		t.Fatalf("UploadPartSizeBytes = %d, want 128", settings.UploadPartSizeBytes)
	}
	t.Cleanup(func() {
		_, _ = settingsStore.UpdateUploadSettings(context.Background(), adminsettings.UploadSettings{
			UploadPartSizeBytes:          config.DefaultUploadPartSizeBytes,
			TelegramDocumentLimitBytes:   config.DefaultTelegramDocumentLimitBytes,
			UploadSafetyMarginBytes:      config.DefaultUploadSafetyMarginBytes,
			MaxParallelUploads:           1,
			TargetUploadBytesPerSecond:   0,
			CooldownBetweenPartsMillisec: 0,
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
		UploadPartSizeBytes:          512,
		TelegramDocumentLimitBytes:   2048,
		UploadSafetyMarginBytes:      64,
		MaxParallelUploads:           1,
		TargetUploadBytesPerSecond:   0,
		CooldownBetweenPartsMillisec: 0,
	}, admin.ID); err != nil {
		t.Fatalf("UpdateUploadSettings() error = %v", err)
	}
	t.Cleanup(func() {
		_, _ = settingsStore.UpdateUploadSettings(context.Background(), adminsettings.UploadSettings{
			UploadPartSizeBytes:          config.DefaultUploadPartSizeBytes,
			TelegramDocumentLimitBytes:   config.DefaultTelegramDocumentLimitBytes,
			UploadSafetyMarginBytes:      config.DefaultUploadSafetyMarginBytes,
			MaxParallelUploads:           1,
			TargetUploadBytesPerSecond:   0,
			CooldownBetweenPartsMillisec: 0,
		}, "")
	})

	if _, err := settingsStore.UpsertTelegramAccountLimit(ctx, admin.ID, adminsettings.TelegramAccountLimit{
		TelegramDocumentLimitBytes:   256,
		UploadSafetyMarginBytes:      32,
		MaxParallelUploads:           sql.NullInt64{Int64: 2, Valid: true},
		TargetUploadBytesPerSecond:   sql.NullInt64{Int64: 10_000_000, Valid: true},
		CooldownBetweenPartsMillisec: sql.NullInt64{Int64: 500, Valid: true},
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
	if effective.MaxParallelUploads != 2 || effective.TargetUploadBytesPerSecond != 10_000_000 || effective.CooldownBetweenPartsMillisec != 500 {
		t.Fatalf("effective upload policy = %+v, want account policy overrides", effective)
	}
}

func TestAdminUsersPromoteAndDemoteByTelegramID(t *testing.T) {
	database := openIntegrationDB(t)
	sessionStore := auth.NewSessionStore(database)
	adminUserStore := adminusers.NewStore(database)
	ctx := context.Background()

	user, cleanupUser := createUserThroughLogin(t, database, sessionStore, 960_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupUser()

	promoted, err := adminUserStore.PromoteByTelegramID(ctx, user.TelegramID)
	if err != nil {
		t.Fatalf("PromoteByTelegramID() error = %v", err)
	}
	if promoted.ID != user.ID || promoted.Role != "admin" {
		t.Fatalf("PromoteByTelegramID() = %+v, want same user with admin role", promoted)
	}

	demoted, err := adminUserStore.DemoteByTelegramID(ctx, user.TelegramID)
	if err != nil {
		t.Fatalf("DemoteByTelegramID() error = %v", err)
	}
	if demoted.ID != user.ID || demoted.Role != "user" {
		t.Fatalf("DemoteByTelegramID() = %+v, want same user with user role", demoted)
	}

	if _, err := adminUserStore.PromoteByTelegramID(ctx, 999_999_999_999_999); !errors.Is(err, adminusers.ErrUserNotFound) {
		t.Fatalf("missing user error = %v, want adminusers.ErrUserNotFound", err)
	}
}

func TestTelegramProbeStatePersistence(t *testing.T) {
	database := openIntegrationDB(t)
	sessionStore := auth.NewSessionStore(database)
	probeStore := telegramprobe.NewStore(database)
	ctx := context.Background()

	user, cleanupUser := createUserThroughLogin(t, database, sessionStore, 970_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupUser()

	account, err := probeStore.AccountByTelegramID(ctx, user.TelegramID)
	if err != nil {
		t.Fatalf("AccountByTelegramID() error = %v", err)
	}
	if account.UserID != user.ID || account.TelegramID != user.TelegramID {
		t.Fatalf("AccountByTelegramID() = %+v, want user %s", account, user.ID)
	}

	nextProbeAt := time.Now().Add(time.Hour)
	if err := probeStore.MarkPending(ctx, user.ID, nextProbeAt); err != nil {
		t.Fatalf("MarkPending() error = %v", err)
	}
	if err := probeStore.MarkFailed(ctx, user.ID, errors.New("temporary probe failure"), nextProbeAt); err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}
	if err := probeStore.MarkSuccess(ctx, user.ID, 128, nextProbeAt); err != nil {
		t.Fatalf("MarkSuccess() error = %v", err)
	}

	var detected int64
	var status string
	var probeError sql.NullString
	err = database.QueryRowContext(ctx, `
SELECT detected_document_limit_bytes, last_probe_status, last_probe_error
FROM telegram_account_limits
WHERE user_id = $1`,
		user.ID,
	).Scan(&detected, &status, &probeError)
	if err != nil {
		t.Fatalf("probe state query error = %v", err)
	}
	if detected != 128 || status != "success" || probeError.Valid {
		t.Fatalf("probe state detected=%d status=%q error=%q, want success without error", detected, status, probeError.String)
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
			WHERE version = '000016'
)`).Scan(&exists)
	if err != nil {
		t.Fatalf("schema migration check failed: %v", err)
	}
	if !exists {
		t.Fatalf("TEST_DATABASE_URL database is not migrated through 000016; run go run ./cmd/migrate up first")
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

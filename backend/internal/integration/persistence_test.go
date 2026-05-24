package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"net/http/httptest"
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
	renamedFolder, err := fileStore.Rename(ctx, owner.ID, folder.ID, "integration-folder-renamed")
	if err != nil {
		t.Fatalf("Rename(folder) error = %v", err)
	}
	if !renamedFolder.NamePlain.Valid || renamedFolder.NamePlain.String != "integration-folder-renamed" {
		t.Fatalf("Rename(folder) name = %+v, want integration-folder-renamed", renamedFolder.NamePlain)
	}
	if _, err := fileStore.Move(ctx, other.ID, folder.ID, ""); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("cross-owner move error = %v, want files.ErrNotFound", err)
	}
	if _, err := fileStore.Move(ctx, owner.ID, targetFolder.ID, folder.ID); !errors.Is(err, files.ErrInvalidMove) {
		t.Fatalf("cycle move error = %v, want files.ErrInvalidMove", err)
	}
	bulkMoveOne, err := fileStore.CreateFolder(ctx, owner.ID, "", "integration-bulk-move-one")
	if err != nil {
		t.Fatalf("CreateFolder(bulk one) error = %v", err)
	}
	bulkMoveTwo, err := fileStore.CreateFolder(ctx, owner.ID, "", "integration-bulk-move-two")
	if err != nil {
		t.Fatalf("CreateFolder(bulk two) error = %v", err)
	}
	if err := fileStore.MoveMany(ctx, owner.ID, []string{bulkMoveOne.ID, bulkMoveTwo.ID}, targetFolder.ID); err != nil {
		t.Fatalf("MoveMany() error = %v", err)
	}
	for _, bulkID := range []string{bulkMoveOne.ID, bulkMoveTwo.ID} {
		moved, err := fileStore.GetByID(ctx, owner.ID, bulkID)
		if err != nil {
			t.Fatalf("GetByID(%s) after MoveMany() error = %v", bulkID, err)
		}
		if !moved.ParentID.Valid || moved.ParentID.String != targetFolder.ID {
			t.Fatalf("MoveMany() parent for %s = %+v, want %s", bulkID, moved.ParentID, targetFolder.ID)
		}
	}
	if err := fileStore.MoveMany(ctx, owner.ID, []string{bulkMoveOne.ID}, ""); err != nil {
		t.Fatalf("MoveMany(root) error = %v", err)
	}
	rolledBack, err := fileStore.GetByID(ctx, owner.ID, bulkMoveOne.ID)
	if err != nil {
		t.Fatalf("GetByID(root move) error = %v", err)
	}
	if rolledBack.ParentID.Valid {
		t.Fatalf("MoveMany(root) parent = %+v, want NULL", rolledBack.ParentID)
	}
	bulkDeleteOne, err := fileStore.CreateFolder(ctx, owner.ID, "", "integration-bulk-delete-one")
	if err != nil {
		t.Fatalf("CreateFolder(delete one) error = %v", err)
	}
	bulkDeleteTwo, err := fileStore.CreateFolder(ctx, owner.ID, "", "integration-bulk-delete-two")
	if err != nil {
		t.Fatalf("CreateFolder(delete two) error = %v", err)
	}
	if err := fileStore.SoftDeleteMany(ctx, owner.ID, []string{bulkDeleteOne.ID, bulkDeleteTwo.ID}, time.Now()); err != nil {
		t.Fatalf("SoftDeleteMany() error = %v", err)
	}
	if _, err := fileStore.GetByID(ctx, owner.ID, bulkDeleteOne.ID); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("bulk delete one get error = %v, want files.ErrNotFound", err)
	}
	if _, err := fileStore.GetByID(ctx, owner.ID, bulkDeleteTwo.ID); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("bulk delete two get error = %v, want files.ErrNotFound", err)
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
	plainLink, err := fileStore.CreatePublicLink(ctx, owner.ID, file.ID, []byte("plain-token"), sql.NullTime{}, sql.NullInt64{}, files.PublicDownloadLimitModeHard, false, files.PublicLinkPassword{})
	if err != nil {
		t.Fatalf("CreatePublicLink(plain) error = %v", err)
	}
	passwordLink, err := fileStore.CreatePublicLink(ctx, owner.ID, file.ID, []byte("password-token"), sql.NullTime{}, sql.NullInt64{}, files.PublicDownloadLimitModeHard, false, files.PublicLinkPassword{
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
	if !parts[0].PlaintextStart.Valid || !parts[0].PlaintextEnd.Valid || !parts[0].PlaintextSize.Valid {
		t.Fatalf("DownloadData() part 1 ranges = %+v, want persisted plaintext range", parts[0])
	}
	if parts[0].PlaintextStart.Int64 != 0 || parts[0].PlaintextEnd.Int64 != 3 || parts[0].PlaintextSize.Int64 != 3 {
		t.Fatalf("DownloadData() part 1 ranges = [%d,%d) size=%d, want [0,3) size=3", parts[0].PlaintextStart.Int64, parts[0].PlaintextEnd.Int64, parts[0].PlaintextSize.Int64)
	}
	if !parts[1].PlaintextStart.Valid || !parts[1].PlaintextEnd.Valid || !parts[1].PlaintextSize.Valid {
		t.Fatalf("DownloadData() part 2 ranges = %+v, want persisted plaintext range", parts[1])
	}
	if parts[1].PlaintextStart.Int64 != 3 || parts[1].PlaintextEnd.Int64 != 6 || parts[1].PlaintextSize.Int64 != 3 {
		t.Fatalf("DownloadData() part 2 ranges = [%d,%d) size=%d, want [3,6) size=3", parts[1].PlaintextStart.Int64, parts[1].PlaintextEnd.Int64, parts[1].PlaintextSize.Int64)
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
	share, err := fileStore.CreateShare(ctx, owner.ID, file.ID, other.TelegramID, files.SharePermissionRead, sql.NullTime{})
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
	if err := fileStore.SoftDeleteAccessible(ctx, other.ID, file.ID, time.Now()); !errors.Is(err, files.ErrForbidden) {
		t.Fatalf("shared read-only delete error = %v, want files.ErrForbidden", err)
	}
	if _, err := fileStore.ListShares(ctx, other.ID, file.ID); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("grantee ListShares() error = %v, want files.ErrNotFound", err)
	}
	if err := fileStore.RevokeShare(ctx, other.ID, file.ID, share.ID, time.Now()); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("grantee RevokeShare() error = %v, want files.ErrNotFound", err)
	}
	if _, err := fileStore.CreateShare(ctx, other.ID, file.ID, owner.TelegramID, files.SharePermissionRead, sql.NullTime{}); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("cross-owner CreateShare() error = %v, want files.ErrNotFound", err)
	}
	if err := fileStore.RevokeShare(ctx, owner.ID, file.ID, share.ID, time.Now()); err != nil {
		t.Fatalf("RevokeShare() error = %v", err)
	}
	if _, _, _, err := fileStore.DownloadData(ctx, other.ID, file.ID); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("revoked shared download error = %v, want files.ErrNotFound", err)
	}
	publicTokenHash := sha256.Sum256([]byte("integration-public-token-" + uniqueSuffix()))
	publicLink, err := fileStore.CreatePublicLink(ctx, owner.ID, file.ID, publicTokenHash[:], sql.NullTime{}, sql.NullInt64{}, files.PublicDownloadLimitModeHard, false, files.PublicLinkPassword{})
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
	if _, err := fileStore.ListPublicLinks(ctx, other.ID, file.ID); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("cross-owner ListPublicLinks() error = %v, want files.ErrNotFound", err)
	}
	if err := fileStore.RevokePublicLink(ctx, other.ID, file.ID, publicLink.ID, time.Now()); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("cross-owner RevokePublicLink() error = %v, want files.ErrNotFound", err)
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
	expiredTokenHash := sha256.Sum256([]byte("integration-expired-token-" + uniqueSuffix()))
	if _, err := fileStore.CreatePublicLink(ctx, owner.ID, file.ID, expiredTokenHash[:], sql.NullTime{Time: time.Now().Add(-time.Hour), Valid: true}, sql.NullInt64{}, files.PublicDownloadLimitModeHard, false, files.PublicLinkPassword{}); err != nil {
		t.Fatalf("CreatePublicLink(expired) error = %v", err)
	}
	if _, _, _, err := fileStore.DownloadDataByPublicTokenHash(ctx, expiredTokenHash[:]); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("expired public download error = %v, want files.ErrNotFound", err)
	}
	limitedTokenHash := sha256.Sum256([]byte("integration-limited-token-" + uniqueSuffix()))
	limitedLink, err := fileStore.CreatePublicLink(ctx, owner.ID, file.ID, limitedTokenHash[:], sql.NullTime{}, sql.NullInt64{Int64: 1, Valid: true}, files.PublicDownloadLimitModeHard, false, files.PublicLinkPassword{})
	if err != nil {
		t.Fatalf("CreatePublicLink(limited) error = %v", err)
	}
	if _, reservedLink, claimed, err := fileStore.ReservePublicLinkDownloadSlot(ctx, limitedTokenHash[:]); err != nil || !claimed {
		t.Fatalf("ReservePublicLinkDownloadSlot(limited first active) claimed=%v error=%v", claimed, err)
	} else if reservedLink.ActiveDownloadCount != 1 {
		t.Fatalf("ReservePublicLinkDownloadSlot(limited first active) active_download_count=%d, want 1", reservedLink.ActiveDownloadCount)
	}
	if _, _, claimed, err := fileStore.ReservePublicLinkDownloadSlot(ctx, limitedTokenHash[:]); err != nil {
		t.Fatalf("ReservePublicLinkDownloadSlot(limited second active) error = %v", err)
	} else if claimed {
		t.Fatal("ReservePublicLinkDownloadSlot(limited second active) claimed = true, want false")
	}
	if err := fileStore.FinishPublicLinkDownload(ctx, limitedLink.ID, true); err != nil {
		t.Fatalf("FinishPublicLinkDownload(true) error = %v", err)
	}
	if _, _, err := fileStore.PublicFileByTokenHash(ctx, limitedTokenHash[:]); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("limited public download error = %v, want files.ErrNotFound", err)
	}
	softLimitedTokenHash := sha256.Sum256([]byte("integration-soft-limited-token-" + uniqueSuffix()))
	if _, err := fileStore.CreatePublicLink(ctx, owner.ID, file.ID, softLimitedTokenHash[:], sql.NullTime{}, sql.NullInt64{Int64: 1, Valid: true}, files.PublicDownloadLimitModeSoft, false, files.PublicLinkPassword{}); err != nil {
		t.Fatalf("CreatePublicLink(soft-limited) error = %v", err)
	}
	if _, _, claimed, err := fileStore.ReservePublicLinkDownloadSlot(ctx, softLimitedTokenHash[:]); err != nil || !claimed {
		t.Fatalf("ReservePublicLinkDownloadSlot(soft first) claimed=%v error=%v", claimed, err)
	}
	if _, _, claimed, err := fileStore.ReservePublicLinkDownloadSlot(ctx, softLimitedTokenHash[:]); err != nil || !claimed {
		t.Fatalf("ReservePublicLinkDownloadSlot(soft second) claimed=%v error=%v", claimed, err)
	}
	protectedTokenHash := sha256.Sum256([]byte("integration-protected-token-" + uniqueSuffix()))
	protectedLink, err := fileStore.CreatePublicLink(ctx, owner.ID, file.ID, protectedTokenHash[:], sql.NullTime{}, sql.NullInt64{}, files.PublicDownloadLimitModeHard, false, files.PublicLinkPassword{
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

func TestFilesAuditEventPersistence(t *testing.T) {
	database := openIntegrationDB(t)
	sessionStore := auth.NewSessionStore(database)
	fileStore := files.NewStore(database)
	ctx := context.Background()

	actor, cleanupActor := createUserThroughLogin(t, database, sessionStore, 950_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupActor()
	target, cleanupTarget := createUserThroughLogin(t, database, sessionStore, 951_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupTarget()

	request := httptest.NewRequest("POST", "/files/audit-test", nil)
	request.RemoteAddr = "198.51.100.20:51234"
	request.Header.Set("User-Agent", "integration-audit-test")

	fileStore.RecordAuditEvent(ctx, actor.ID, files.AuditPublicLinkCreate, "public_link", target.ID, request)
	fileStore.RecordAuditEvent(ctx, "", files.AuditPublicLinkDownload, "public_link", target.ID, request)

	rows, err := database.QueryContext(ctx, `
SELECT actor_user_id, action, resource_type, resource_id, ip_hash, user_agent
FROM audit_events
WHERE resource_type = 'public_link' AND resource_id = $1
ORDER BY created_at DESC
LIMIT 2`, target.ID)
	if err != nil {
		t.Fatalf("audit_events query error = %v", err)
	}
	defer rows.Close()

	type auditRow struct {
		actorID      sql.NullString
		action       string
		resourceType sql.NullString
		resourceID   sql.NullString
		ipHash       []byte
		userAgent    sql.NullString
	}
	var got []auditRow
	for rows.Next() {
		var row auditRow
		if err := rows.Scan(&row.actorID, &row.action, &row.resourceType, &row.resourceID, &row.ipHash, &row.userAgent); err != nil {
			t.Fatalf("audit_events scan error = %v", err)
		}
		got = append(got, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("audit_events rows error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("audit_events rows = %d, want 2", len(got))
	}

	if got[0].action != files.AuditPublicLinkDownload || got[0].actorID.Valid {
		t.Fatalf("latest audit row = %+v, want download with empty actor", got[0])
	}
	if !got[0].resourceType.Valid || got[0].resourceType.String != "public_link" || !got[0].resourceID.Valid || got[0].resourceID.String != target.ID {
		t.Fatalf("latest resource columns = %+v, want public_link/%s", got[0], target.ID)
	}
	if len(got[0].ipHash) == 0 || !got[0].userAgent.Valid || got[0].userAgent.String != "integration-audit-test" {
		t.Fatalf("latest audit request fields invalid: ip_hash=%d user_agent=%q", len(got[0].ipHash), got[0].userAgent.String)
	}

	if got[1].action != files.AuditPublicLinkCreate || !got[1].actorID.Valid || got[1].actorID.String != actor.ID {
		t.Fatalf("previous audit row = %+v, want create with actor %s", got[1], actor.ID)
	}
}

func TestFolderShareGrantsRecursiveAccessToDescendants(t *testing.T) {
	database := openIntegrationDB(t)
	sessionStore := auth.NewSessionStore(database)
	fileStore := files.NewStore(database)
	uploadStore := uploads.NewStore(database)
	ctx := context.Background()

	owner, cleanupOwner := createUserThroughLogin(t, database, sessionStore, 960_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupOwner()
	grantee, cleanupGrantee := createUserThroughLogin(t, database, sessionStore, 970_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupGrantee()

	root, err := fileStore.CreateFolder(ctx, owner.ID, "", "shared-root")
	if err != nil {
		t.Fatalf("CreateFolder(root) error = %v", err)
	}
	child, err := fileStore.CreateFolder(ctx, owner.ID, root.ID, "child")
	if err != nil {
		t.Fatalf("CreateFolder(child) error = %v", err)
	}
	grandchild, err := fileStore.CreateFolder(ctx, owner.ID, child.ID, "grandchild")
	if err != nil {
		t.Fatalf("CreateFolder(grandchild) error = %v", err)
	}

	upload, err := uploadStore.Create(ctx, uploads.CreateUploadParams{
		OwnerID:       owner.ID,
		ParentID:      grandchild.ID,
		Name:          "nested.txt",
		MimeType:      "text/plain",
		PlaintextSize: 5,
		PartSize:      5,
		ExpiresAt:     time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Create upload error = %v", err)
	}
	if _, err := uploadStore.CompletePart(ctx, uploads.CompletePartParams{
		OwnerID:        owner.ID,
		UploadID:       upload.ID,
		PartNumber:     1,
		PlaintextSize:  5,
		CiphertextSize: 10,
		Checksum:       []byte("nested-part"),
		UploadedSize:   5,
		TelegramPeer:   "self",
		MessageID:      201,
		Now:            time.Now(),
	}); err != nil {
		t.Fatalf("CompletePart() error = %v", err)
	}
	file, err := uploadStore.CompleteUpload(ctx, uploads.CompleteUploadParams{
		OwnerID:  owner.ID,
		UploadID: upload.ID,
		Now:      time.Now(),
	})
	if err != nil {
		t.Fatalf("CompleteUpload() error = %v", err)
	}

	if _, err := fileStore.GetAccessibleByID(ctx, grantee.ID, root.ID); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("grantee root access before share error = %v, want files.ErrNotFound", err)
	}
	if _, _, _, err := fileStore.DownloadData(ctx, grantee.ID, file.ID); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("grantee download before share error = %v, want files.ErrNotFound", err)
	}

	share, err := fileStore.CreateShare(ctx, owner.ID, root.ID, grantee.TelegramID, files.SharePermissionRead, sql.NullTime{})
	if err != nil {
		t.Fatalf("CreateShare(folder) error = %v", err)
	}

	for _, id := range []string{root.ID, child.ID, grandchild.ID, file.ID} {
		if _, err := fileStore.GetAccessibleByID(ctx, grantee.ID, id); err != nil {
			t.Fatalf("GetAccessibleByID(%s) after share error = %v", id, err)
		}
	}

	rootChildren, err := fileStore.ListChildren(ctx, grantee.ID, root.ID)
	if err != nil {
		t.Fatalf("ListChildren(root) error = %v", err)
	}
	if len(rootChildren) != 1 || rootChildren[0].ID != child.ID {
		t.Fatalf("ListChildren(root) = %+v, want child %s", rootChildren, child.ID)
	}
	childChildren, err := fileStore.ListChildren(ctx, grantee.ID, child.ID)
	if err != nil {
		t.Fatalf("ListChildren(child) error = %v", err)
	}
	if len(childChildren) != 1 || childChildren[0].ID != grandchild.ID {
		t.Fatalf("ListChildren(child) = %+v, want grandchild %s", childChildren, grandchild.ID)
	}
	grandchildChildren, err := fileStore.ListChildren(ctx, grantee.ID, grandchild.ID)
	if err != nil {
		t.Fatalf("ListChildren(grandchild) error = %v", err)
	}
	if len(grandchildChildren) != 1 || grandchildChildren[0].ID != file.ID {
		t.Fatalf("ListChildren(grandchild) = %+v, want file %s", grandchildChildren, file.ID)
	}

	if _, _, _, err := fileStore.DownloadData(ctx, grantee.ID, file.ID); err != nil {
		t.Fatalf("grantee DownloadData() after folder share error = %v", err)
	}

	if err := fileStore.RevokeShare(ctx, owner.ID, root.ID, share.ID, time.Now()); err != nil {
		t.Fatalf("RevokeShare(folder) error = %v", err)
	}
	if _, err := fileStore.GetAccessibleByID(ctx, grantee.ID, grandchild.ID); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("grantee nested folder access after revoke error = %v, want files.ErrNotFound", err)
	}
	if _, _, _, err := fileStore.DownloadData(ctx, grantee.ID, file.ID); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("grantee download after revoke error = %v, want files.ErrNotFound", err)
	}
}

func TestFolderShareReadDeleteAllowsGlobalDelete(t *testing.T) {
	database := openIntegrationDB(t)
	sessionStore := auth.NewSessionStore(database)
	fileStore := files.NewStore(database)
	uploadStore := uploads.NewStore(database)
	ctx := context.Background()

	owner, cleanupOwner := createUserThroughLogin(t, database, sessionStore, 980_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupOwner()
	grantee, cleanupGrantee := createUserThroughLogin(t, database, sessionStore, 990_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupGrantee()

	root, err := fileStore.CreateFolder(ctx, owner.ID, "", "share-delete-root")
	if err != nil {
		t.Fatalf("CreateFolder(root) error = %v", err)
	}
	child, err := fileStore.CreateFolder(ctx, owner.ID, root.ID, "share-delete-child")
	if err != nil {
		t.Fatalf("CreateFolder(child) error = %v", err)
	}
	upload, err := uploadStore.Create(ctx, uploads.CreateUploadParams{
		OwnerID:       owner.ID,
		ParentID:      child.ID,
		Name:          "share-delete.txt",
		MimeType:      "text/plain",
		PlaintextSize: 4,
		PartSize:      4,
		ExpiresAt:     time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Create upload error = %v", err)
	}
	if _, err := uploadStore.CompletePart(ctx, uploads.CompletePartParams{
		OwnerID:        owner.ID,
		UploadID:       upload.ID,
		PartNumber:     1,
		PlaintextSize:  4,
		CiphertextSize: 9,
		Checksum:       []byte("del-part"),
		UploadedSize:   4,
		TelegramPeer:   "self",
		MessageID:      301,
		Now:            time.Now(),
	}); err != nil {
		t.Fatalf("CompletePart() error = %v", err)
	}
	file, err := uploadStore.CompleteUpload(ctx, uploads.CompleteUploadParams{
		OwnerID:  owner.ID,
		UploadID: upload.ID,
		Now:      time.Now(),
	})
	if err != nil {
		t.Fatalf("CompleteUpload() error = %v", err)
	}

	share, err := fileStore.CreateShare(ctx, owner.ID, root.ID, grantee.TelegramID, files.SharePermissionReadDelete, sql.NullTime{})
	if err != nil {
		t.Fatalf("CreateShare(read_delete) error = %v", err)
	}
	if share.Permission != files.SharePermissionReadDelete {
		t.Fatalf("CreateShare(read_delete) permission = %q, want %q", share.Permission, files.SharePermissionReadDelete)
	}

	if err := fileStore.SoftDeleteAccessible(ctx, grantee.ID, root.ID, time.Now()); err != nil {
		t.Fatalf("SoftDeleteAccessible(shared root) error = %v", err)
	}
	if _, err := fileStore.GetByID(ctx, owner.ID, root.ID); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("owner root after shared delete error = %v, want files.ErrNotFound", err)
	}
	if _, err := fileStore.GetByID(ctx, owner.ID, child.ID); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("owner child after shared delete error = %v, want files.ErrNotFound", err)
	}
	if _, _, _, err := fileStore.DownloadData(ctx, owner.ID, file.ID); !errors.Is(err, files.ErrNotFound) {
		t.Fatalf("owner file after shared delete error = %v, want files.ErrNotFound", err)
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
		AvailableAt:    now,
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
		t.Fatalf("second ClaimQueuedPart() with earlier incomplete part error = %v, want ErrUploadPartNotFound", err)
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

func TestCompletedLocalStagingCleanupArtifacts(t *testing.T) {
	database := openIntegrationDB(t)
	sessionStore := auth.NewSessionStore(database)
	uploadStore := uploads.NewStore(database)
	ctx := context.Background()

	owner, cleanupOwner := createUserThroughLogin(t, database, sessionStore, 939_500_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupOwner()

	now := time.Now().UTC()
	upload, err := uploadStore.Create(ctx, uploads.CreateUploadParams{
		OwnerID:       owner.ID,
		Name:          "completed-local.bin",
		PlaintextSize: 5,
		PartSize:      5,
		ExpiresAt:     now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Create upload error = %v", err)
	}
	if _, err := database.ExecContext(ctx, `
UPDATE upload_parts
SET plaintext_size = 5,
    ciphertext_size = 10,
    telegram_peer = 'self',
    telegram_message_id = 301,
    status = 'complete',
    storage_backend = 'local',
    storage_key = 'completed-local.bin.part-1'
WHERE upload_id = $1
  AND part_number = 1`,
		upload.ID,
	); err != nil {
		t.Fatalf("update completed staged part error = %v", err)
	}

	artifacts, err := uploadStore.PendingLocalStagingCleanupArtifacts(ctx, 10)
	if err != nil {
		t.Fatalf("PendingLocalStagingCleanupArtifacts() error = %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].StorageKey != "completed-local.bin.part-1" {
		t.Fatalf("PendingLocalStagingCleanupArtifacts() = %+v, want completed local cleanup", artifacts)
	}
}

func TestCancelUploadStopsQueueAndSchedulesCleanup(t *testing.T) {
	database := openIntegrationDB(t)
	sessionStore := auth.NewSessionStore(database)
	uploadStore := uploads.NewStore(database)
	ctx := context.Background()

	owner, cleanupOwner := createUserThroughLogin(t, database, sessionStore, 939_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupOwner()

	now := time.Now().UTC()
	upload, err := uploadStore.Create(ctx, uploads.CreateUploadParams{
		OwnerID:       owner.ID,
		Name:          "cancel.bin",
		PlaintextSize: 10,
		PartSize:      5,
		ExpiresAt:     now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Create upload error = %v", err)
	}
	if _, err := uploadStore.StagePart(ctx, uploads.StagePartParams{
		OwnerID:        owner.ID,
		UploadID:       upload.ID,
		PartNumber:     1,
		PlaintextSize:  5,
		CiphertextSize: 9,
		Checksum:       []byte("part-1"),
		UploadedSize:   5,
		StorageBackend: "local",
		StorageKey:     "cancel.bin.part-1",
		AvailableAt:    now,
		Now:            now,
	}); err != nil {
		t.Fatalf("StagePart(1) error = %v", err)
	}
	part2, err := uploadStore.StagePart(ctx, uploads.StagePartParams{
		OwnerID:        owner.ID,
		UploadID:       upload.ID,
		PartNumber:     2,
		PlaintextSize:  5,
		CiphertextSize: 9,
		Checksum:       []byte("part-2"),
		UploadedSize:   10,
		StorageBackend: "local",
		StorageKey:     "cancel.bin.part-2",
		AvailableAt:    now,
		Now:            now,
	})
	if err != nil {
		t.Fatalf("StagePart(2) error = %v", err)
	}
	if _, err := uploadStore.MarkStagedPartUploaded(ctx, uploads.MarkStagedPartUploadedParams{
		PartID:       part2.ID,
		TelegramPeer: "self",
		MessageID:    302,
	}); err != nil {
		t.Fatalf("MarkStagedPartUploaded() error = %v", err)
	}
	if err := uploadStore.MarkLocalStagingDeleted(ctx, part2.ID); err != nil {
		t.Fatalf("MarkLocalStagingDeleted() error = %v", err)
	}

	if err := uploadStore.CancelUpload(ctx, uploads.CancelUploadParams{
		OwnerID:  owner.ID,
		UploadID: upload.ID,
		Now:      now,
	}); err != nil {
		t.Fatalf("CancelUpload() error = %v", err)
	}
	if _, err := uploadStore.ClaimQueuedPartWork(ctx, uploads.ClaimQueuedPartParams{
		WorkerID:      "worker-a",
		Now:           now,
		LeaseDuration: time.Minute,
	}); !errors.Is(err, uploads.ErrUploadPartNotFound) {
		t.Fatalf("ClaimQueuedPartWork() after cancel error = %v, want ErrUploadPartNotFound", err)
	}

	localArtifacts, err := uploadStore.PendingLocalStagingCleanupArtifacts(ctx, 10)
	if err != nil {
		t.Fatalf("PendingLocalStagingCleanupArtifacts() error = %v", err)
	}
	if len(localArtifacts) != 1 || localArtifacts[0].StorageKey != "cancel.bin.part-1" {
		t.Fatalf("PendingLocalStagingCleanupArtifacts() = %+v, want part 1 local cleanup", localArtifacts)
	}
	telegramArtifacts, err := uploadStore.PendingTelegramCleanupArtifacts(ctx, 10)
	if err != nil {
		t.Fatalf("PendingTelegramCleanupArtifacts() error = %v", err)
	}
	if len(telegramArtifacts) != 1 || telegramArtifacts[0].MessageID != 302 {
		t.Fatalf("PendingTelegramCleanupArtifacts() = %+v, want uploaded part cleanup", telegramArtifacts)
	}
}

func TestClaimQueuedPartWorkRespectsAccountParallelLimit(t *testing.T) {
	database := openIntegrationDB(t)
	sessionStore := auth.NewSessionStore(database)
	settingsStore := adminsettings.NewStore(database, config.Config{
		UploadPartSizeBytes:        5,
		TelegramDocumentLimitBytes: 1024,
		UploadSafetyMarginBytes:    64,
	})
	uploadStore := uploads.NewStore(database)
	ctx := context.Background()

	owner, cleanupOwner := createUserThroughLogin(t, database, sessionStore, 939_700_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupOwner()

	if _, err := settingsStore.UpdateUploadSettings(ctx, adminsettings.UploadSettings{
		UploadPartSizeBytes:          5,
		TelegramDocumentLimitBytes:   1024,
		UploadSafetyMarginBytes:      64,
		MaxParallelUploads:           4,
		TargetUploadBytesPerSecond:   0,
		CooldownBetweenPartsMillisec: 0,
		PublicLinkPasswordMinLength:  8,
	}, owner.ID); err != nil {
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
			PublicLinkPasswordMinLength:  8,
		}, "")
	})

	now := time.Now().UTC()
	upload, err := uploadStore.Create(ctx, uploads.CreateUploadParams{
		OwnerID:       owner.ID,
		Name:          "parallel-limit.bin",
		PlaintextSize: 25,
		PartSize:      5,
		ExpiresAt:     now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Create upload error = %v", err)
	}
	for part := 1; part <= 5; part++ {
		if _, err := uploadStore.StagePart(ctx, uploads.StagePartParams{
			OwnerID:        owner.ID,
			UploadID:       upload.ID,
			PartNumber:     part,
			PlaintextSize:  5,
			CiphertextSize: 9,
			Checksum:       []byte(fmt.Sprintf("part-%d", part)),
			UploadedSize:   int64(part * 5),
			StorageBackend: uploads.LocalStagingBackend,
			StorageKey:     fmt.Sprintf("parallel-limit.bin.part-%d", part),
			AvailableAt:    now,
			Now:            now,
		}); err != nil {
			t.Fatalf("StagePart(%d) error = %v", part, err)
		}
	}

	for claim := 1; claim <= 4; claim++ {
		work, err := uploadStore.ClaimQueuedPartWork(ctx, uploads.ClaimQueuedPartParams{
			WorkerID:      fmt.Sprintf("worker-%d", claim),
			Now:           now,
			LeaseDuration: time.Minute,
		})
		if err != nil {
			t.Fatalf("ClaimQueuedPartWork(%d) error = %v", claim, err)
		}
		if work.MaxParallelUploads != 4 {
			t.Fatalf("ClaimQueuedPartWork(%d) MaxParallelUploads = %d, want 4", claim, work.MaxParallelUploads)
		}
	}

	if _, err := uploadStore.ClaimQueuedPartWork(ctx, uploads.ClaimQueuedPartParams{
		WorkerID:      "worker-over-limit",
		Now:           now,
		LeaseDuration: time.Minute,
	}); !errors.Is(err, uploads.ErrUploadPartNotFound) {
		t.Fatalf("ClaimQueuedPartWork() over limit error = %v, want ErrUploadPartNotFound", err)
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
		PublicLinkPasswordMinLength:  8,
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
			PublicLinkPasswordMinLength:  8,
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
		PublicLinkPasswordMinLength:  8,
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
			PublicLinkPasswordMinLength:  8,
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

func TestEffectiveUploadSettingsUsesGlobalPartSizeWithoutAccountOverride(t *testing.T) {
	database := openIntegrationDB(t)
	sessionStore := auth.NewSessionStore(database)
	settingsStore := adminsettings.NewStore(database, config.Config{
		UploadPartSizeBytes:        384 * 1024 * 1024,
		TelegramDocumentLimitBytes: 2 * 1024 * 1024 * 1024,
		UploadSafetyMarginBytes:    64 * 1024 * 1024,
	})
	ctx := context.Background()

	admin, cleanupAdmin := createUserThroughLogin(t, database, sessionStore, 951_000_000_000+time.Now().UnixNano()%1_000_000_000)
	defer cleanupAdmin()

	if _, err := settingsStore.UpdateUploadSettings(ctx, adminsettings.UploadSettings{
		UploadPartSizeBytes:          384 * 1024 * 1024,
		TelegramDocumentLimitBytes:   2 * 1024 * 1024 * 1024,
		UploadSafetyMarginBytes:      64 * 1024 * 1024,
		MaxParallelUploads:           1,
		TargetUploadBytesPerSecond:   0,
		CooldownBetweenPartsMillisec: 0,
		PublicLinkPasswordMinLength:  8,
	}, admin.ID); err != nil {
		t.Fatalf("UpdateUploadSettings() error = %v", err)
	}

	effective, err := settingsStore.EffectiveUploadSettings(ctx, admin.ID)
	if err != nil {
		t.Fatalf("EffectiveUploadSettings() error = %v", err)
	}
	if effective.UploadPartSizeBytes != 384*1024*1024 {
		t.Fatalf("UploadPartSizeBytes = %d, want global part size 384MiB", effective.UploadPartSizeBytes)
	}
	if effective.Source != "global" {
		t.Fatalf("Source = %q, want global", effective.Source)
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
			WHERE version = '000021'
)`).Scan(&exists)
	if err != nil {
		t.Fatalf("schema migration check failed: %v", err)
	}
	if !exists {
		t.Fatalf("TEST_DATABASE_URL database is not migrated through 000021; run go run ./cmd/migrate up first")
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

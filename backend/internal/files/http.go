package files

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"filippo.io/age"
	"golang.org/x/crypto/argon2"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/auth"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/crypto/agefile"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/telegramartifact"
)

const (
	publicLinkTokenBytes        = 32
	publicLinkPasswordSaltBytes = 16
	publicLinkPasswordMinLength = 8
	publicLinkPasswordMaxLength = 1024
	publicLinkArgonTime         = 1
	publicLinkArgonMemoryKiB    = 64 * 1024
	publicLinkArgonThreads      = 4
	publicLinkPasswordHashBytes = 32
)

var publicLinkPageTemplate = template.Must(template.New("public-link").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Name}} - TeleVault</title>
  <style>
    body { margin: 0; font: 14px/1.45 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #1b1f24; background: #f7f8fa; }
    main { min-height: 100vh; display: grid; place-items: center; padding: 24px; }
    .panel { width: min(460px, 100%); background: #fff; border: 1px solid #d9dee7; border-radius: 8px; padding: 22px; box-shadow: 0 1px 2px rgba(16, 24, 40, .06); text-align: center; }
    h1 { font-size: 18px; margin: 0 0 6px; overflow-wrap: anywhere; }
    .muted { color: #68707c; margin: 0 0 14px; }
    .hash { color: #475467; font: 12px/1.4 ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace; margin: -6px 0 14px; overflow-wrap: anywhere; }
    .hash-row { display: flex; gap: 8px; justify-content: center; align-items: center; flex-wrap: wrap; }
    .copy-hash { min-height: 28px; width: auto; padding: 0 10px; border-color: #98a2b3; background: #fff; color: #344054; font-size: 12px; }
    .error { color: #b42318; background: #fef3f2; border: 1px solid #fecdca; border-radius: 6px; padding: 8px 10px; margin: 0 0 12px; text-align: left; }
    form { display: grid; gap: 10px; width: 100%; }
    input { min-height: 40px; width: 100%; box-sizing: border-box; padding: 0 12px; border: 1px solid #d9dee7; border-radius: 6px; font: inherit; }
    button, a.button { min-height: 40px; width: 100%; box-sizing: border-box; display: inline-grid; place-items: center; border: 1px solid #0f766e; border-radius: 6px; background: #0f766e; color: #fff; text-decoration: none; font: inherit; padding: 0 14px; cursor: pointer; }
  </style>
</head>
<body>
  <main>
    <div class="panel">
      <h1>{{.Name}}</h1>
      <div class="muted">{{.Size}}</div>
      {{if .ChecksumShort}}
      <div class="hash">
        <div class="hash-row">
          <span title="{{.ChecksumFull}}">SHA-256: {{.ChecksumShort}}</span>
          <button id="copyHashBtn" class="copy-hash" type="button" data-full-hash="{{.ChecksumFull}}">Copy</button>
        </div>
      </div>
      {{end}}
      {{if .PasswordError}}
      <div class="error">{{.PasswordError}}</div>
      {{end}}
      {{if .PasswordRequired}}
      <form method="post" action="/public/{{.Token}}/download">
        <input name="password" type="password" autocomplete="current-password" placeholder="Password" required autofocus>
        <button type="submit">Download</button>
      </form>
      {{else}}
      <a class="button" href="/public/{{.Token}}/download">Download</a>
      {{end}}
    </div>
  </main>
  <script>
    (function () {
      const button = document.getElementById('copyHashBtn');
      if (!button) return;
      button.addEventListener('click', async function () {
        const full = button.getAttribute('data-full-hash') || '';
        if (!full) return;
        try {
          await navigator.clipboard.writeText(full);
          button.textContent = 'Copied';
        } catch (_) {
          button.textContent = full;
        }
        setTimeout(function () { button.textContent = 'Copy'; }, 1200);
      });
    }());
  </script>
</body>
</html>`))

var publicLinkUnavailableTemplate = template.Must(template.New("public-link-unavailable").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Link unavailable - TeleVault</title>
  <style>
    body { margin: 0; font: 14px/1.45 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #1b1f24; background: #f7f8fa; }
    main { min-height: 100vh; display: grid; place-items: center; padding: 24px; }
    .panel { width: min(460px, 100%); background: #fff; border: 1px solid #d9dee7; border-radius: 8px; padding: 22px; box-shadow: 0 1px 2px rgba(16, 24, 40, .06); text-align: center; }
    h1 { font-size: 18px; margin: 0 0 8px; }
    p { color: #68707c; margin: 0; }
  </style>
</head>
<body>
  <main>
    <div class="panel">
      <h1>Link unavailable</h1>
      <p>This file is unavailable or the link is no longer active.</p>
    </div>
  </main>
</body>
</html>`))

var fileUnavailableTemplate = template.Must(template.New("file-unavailable").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>File unavailable - TeleVault</title>
  <style>
    body { margin: 0; font: 14px/1.45 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #1b1f24; background: #f7f8fa; }
    main { min-height: 100vh; display: grid; place-items: center; padding: 24px; }
    .panel { width: min(460px, 100%); background: #fff; border: 1px solid #d9dee7; border-radius: 8px; padding: 22px; box-shadow: 0 1px 2px rgba(16, 24, 40, .06); text-align: center; }
    h1 { font-size: 18px; margin: 0 0 8px; }
    p { color: #68707c; margin: 0; }
  </style>
</head>
<body>
  <main>
    <div class="panel">
      <h1>File unavailable</h1>
      <p>This file is unavailable or access is no longer active.</p>
    </div>
  </main>
</body>
</html>`))

type Handler struct {
	store         *Store
	logger        *slog.Logger
	downloads     *DownloadTracker
	publicLimiter *PublicDownloadRateLimiter
	sessionCrypto auth.TelegramSessionCrypto
	ageIdentity   age.Identity
	telegram      auth.TelegramStorageClient
}

type shareRecipientDiscovery interface {
	ListKnownUserIDs(ctx context.Context, session string, storagePeer string) ([]int64, error)
}

func NewHandler(db *sql.DB, logger *slog.Logger, downloads *DownloadTracker, publicLimiter *PublicDownloadRateLimiter, sessionCrypto auth.TelegramSessionCrypto, ageIdentity age.Identity, telegram auth.TelegramStorageClient) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if downloads == nil {
		downloads = NewDownloadTracker()
	}
	return &Handler{
		store:         NewStore(db),
		logger:        logger,
		downloads:     downloads,
		publicLimiter: publicLimiter,
		sessionCrypto: sessionCrypto,
		ageIdentity:   ageIdentity,
		telegram:      telegram,
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	parentID := strings.TrimSpace(r.URL.Query().Get("parent_id"))
	items, err := h.store.ListChildren(r.Context(), user.ID, parentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "file_list_failed")
		return
	}
	responses, err := h.filesResponseForUser(r.Context(), user.ID, items)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "file_list_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"files": responses,
	})
}

func (h *Handler) ListSharedWithMe(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	items, err := h.store.ListSharedWithMe(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "shared_file_list_failed")
		return
	}
	responses, err := h.filesResponseForUser(r.Context(), user.ID, items)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "shared_file_list_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"files": responses,
	})
}

func (h *Handler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	var request createFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	name := normalizeName(request.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "folder_name_required")
		return
	}
	if len(name) > 255 {
		writeError(w, http.StatusBadRequest, "folder_name_too_long")
		return
	}

	parentID := strings.TrimSpace(request.ParentID)
	file, err := h.store.CreateFolder(r.Context(), user.ID, parentID, name)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "parent_folder_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "folder_create_failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"file": fileResponse(file),
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "file_id_required")
		return
	}

	file, err := h.store.GetAccessibleByID(r.Context(), user.ID, id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "file_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "file_get_failed")
		return
	}

	response, err := h.fileResponseForUser(r.Context(), user.ID, file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "file_details_failed")
		return
	}
	if file.Type == TypeFile {
		partCount, err := h.store.CountFileParts(r.Context(), file.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "file_details_failed")
			return
		}
		response["part_count"] = partCount
		if file.OwnerID == user.ID {
			publicLinkCount, passwordProtectedCount, err := h.store.CountActivePublicLinks(r.Context(), file.OwnerID, file.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "file_details_failed")
				return
			}
			response["public_link_count"] = publicLinkCount
			response["public_link_password_count"] = passwordProtectedCount
		} else {
			response["public_link_count"] = nil
			response["public_link_password_count"] = nil
		}
	} else {
		response["part_count"] = nil
		response["public_link_count"] = nil
		response["public_link_password_count"] = nil
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"file": response,
	})
}

func (h *Handler) DownloadActivity(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "file_id_required")
		return
	}

	_, err := h.store.GetAccessibleByID(r.Context(), user.ID, id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "file_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "file_get_failed")
		return
	}

	activity := h.downloads.Snapshot(id)
	writeJSON(w, http.StatusOK, map[string]any{
		"download_activity": map[string]any{
			"total":  activity.Total,
			"auth":   activity.Auth,
			"public": activity.Public,
		},
	})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "file_id_required")
		return
	}

	err := h.store.SoftDeleteAccessible(r.Context(), user.ID, id, time.Now().UTC())
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "file_not_found")
		return
	}
	if errors.Is(err, ErrForbidden) {
		writeError(w, http.StatusForbidden, "file_delete_forbidden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "file_delete_failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "file_id_required")
		return
	}

	var request patchFileRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	update := metadataUpdate{}
	if request.ParentID != nil {
		update.SetParent = true
		update.ParentID = strings.TrimSpace(*request.ParentID)
	}
	if request.Name != nil {
		update.SetName = true
		update.Name = strings.TrimSpace(*request.Name)
	}
	if !update.SetParent && !update.SetName {
		writeError(w, http.StatusBadRequest, "file_update_required")
		return
	}
	file, err := h.store.updateMetadata(r.Context(), user.ID, []string{id}, update)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "file_not_found")
		return
	}
	if errors.Is(err, ErrInvalidMove) {
		writeError(w, http.StatusBadRequest, "invalid_file_move")
		return
	}
	if errors.Is(err, ErrInvalidName) {
		writeError(w, http.StatusBadRequest, "invalid_file_name")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "file_patch_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"file": fileResponse(file),
	})
}

func (h *Handler) BulkMove(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	var request bulkFilesRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	ids := normalizeFileIDs(request.IDs)
	if len(ids) == 0 {
		writeError(w, http.StatusBadRequest, "file_ids_required")
		return
	}

	parentID := ""
	if request.ParentID != nil {
		parentID = strings.TrimSpace(*request.ParentID)
	}
	if err := h.store.MoveMany(r.Context(), user.ID, ids, parentID); errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "file_not_found")
		return
	} else if errors.Is(err, ErrInvalidMove) {
		writeError(w, http.StatusBadRequest, "invalid_file_move")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "file_move_failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) BulkDelete(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	var request bulkFilesRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	ids := normalizeFileIDs(request.IDs)
	if len(ids) == 0 {
		writeError(w, http.StatusBadRequest, "file_ids_required")
		return
	}

	if err := h.store.SoftDeleteMany(r.Context(), user.ID, ids, time.Now().UTC()); errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "file_not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "file_delete_failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListShares(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	fileID := strings.TrimSpace(r.PathValue("id"))
	if fileID == "" {
		writeError(w, http.StatusBadRequest, "file_id_required")
		return
	}

	shares, err := h.store.ListShares(r.Context(), user.ID, fileID)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "file_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "share_list_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"shares": sharesResponse(shares),
	})
}

func (h *Handler) ListShareRecipients(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	discovery, ok := h.telegram.(shareRecipientDiscovery)
	if !ok {
		writeError(w, http.StatusInternalServerError, "share_recipients_list_failed")
		return
	}

	telegramSession, err := h.store.TelegramSession(r.Context(), user.ID)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "telegram_session_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "share_recipients_list_failed")
		return
	}

	session, err := h.sessionCrypto.DecryptForTelegramID(telegramSession.OwnerTelegramID, telegramSession.EncryptedSession)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "share_recipients_list_failed")
		return
	}

	telegramIDs, err := discovery.ListKnownUserIDs(r.Context(), session, nullableString(telegramSession.StoragePeer))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "share_recipients_list_failed")
		return
	}

	recipients, err := h.store.ListShareRecipients(r.Context(), user.ID, telegramIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "share_recipients_list_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"recipients": shareRecipientsResponse(recipients),
	})
}

func (h *Handler) CreateShare(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	fileID := strings.TrimSpace(r.PathValue("id"))
	if fileID == "" {
		writeError(w, http.StatusBadRequest, "file_id_required")
		return
	}

	var request createShareRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if request.TelegramID <= 0 {
		writeError(w, http.StatusBadRequest, "telegram_id_required")
		return
	}

	expiresAt, ok := parseOptionalExpiry(request.ExpiresAt)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_expires_at")
		return
	}
	permission := normalizeSharePermission(strings.TrimSpace(request.Permission))
	if permission == "" {
		writeError(w, http.StatusBadRequest, "invalid_share_permission")
		return
	}

	share, err := h.store.CreateShare(r.Context(), user.ID, fileID, request.TelegramID, permission, expiresAt)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "file_or_user_not_found")
		return
	}
	if errors.Is(err, ErrInvalidPermission) {
		writeError(w, http.StatusBadRequest, "invalid_share_permission")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "share_create_failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"share": shareResponse(share),
	})
	h.store.RecordAuditEvent(r.Context(), user.ID, AuditFileShareCreate, auditResourceTypeShare, share.ID, r)
}

func (h *Handler) RevokeShare(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	fileID := strings.TrimSpace(r.PathValue("id"))
	shareID := strings.TrimSpace(r.PathValue("share_id"))
	if fileID == "" || shareID == "" {
		writeError(w, http.StatusBadRequest, "share_id_required")
		return
	}

	err := h.store.RevokeShare(r.Context(), user.ID, fileID, shareID, time.Now().UTC())
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "share_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "share_revoke_failed")
		return
	}

	h.store.RecordAuditEvent(r.Context(), user.ID, AuditFileShareRevoke, auditResourceTypeShare, shareID, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListPublicLinks(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	fileID := strings.TrimSpace(r.PathValue("id"))
	if fileID == "" {
		writeError(w, http.StatusBadRequest, "file_id_required")
		return
	}

	links, err := h.store.ListPublicLinks(r.Context(), user.ID, fileID)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "file_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "public_link_list_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"public_links": publicLinksResponse(links),
	})
}

func (h *Handler) CreatePublicLink(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	fileID := strings.TrimSpace(r.PathValue("id"))
	if fileID == "" {
		writeError(w, http.StatusBadRequest, "file_id_required")
		return
	}

	var request createPublicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	expiresAt, ok := parseOptionalExpiry(request.ExpiresAt)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_expires_at")
		return
	}

	token, tokenHash, err := newPublicLinkToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "public_link_token_failed")
		return
	}

	minPasswordLength, err := h.store.PublicLinkPasswordMinLength(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "public_link_policy_load_failed")
		return
	}

	password, ok := derivePublicLinkPassword(request.Password, minPasswordLength)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_public_link_password")
		return
	}
	maxDownloads, ok := parseOptionalMaxDownloads(request.MaxDownloads)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_max_downloads")
		return
	}
	downloadLimitMode, ok := parseDownloadLimitMode(request.DownloadLimitMode)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_download_limit_mode")
		return
	}

	link, err := h.store.CreatePublicLink(r.Context(), user.ID, fileID, tokenHash, expiresAt, maxDownloads, downloadLimitMode, request.ShowChecksum, password)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "file_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "public_link_create_failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"public_link": publicLinkResponse(link),
		"token":       token,
		"url":         publicLinkURL(r, token),
	})
	h.store.RecordAuditEvent(r.Context(), user.ID, AuditPublicLinkCreate, auditResourceTypePubLink, link.ID, r)
}

func (h *Handler) RevokePublicLink(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	fileID := strings.TrimSpace(r.PathValue("id"))
	linkID := strings.TrimSpace(r.PathValue("link_id"))
	if fileID == "" || linkID == "" {
		writeError(w, http.StatusBadRequest, "public_link_id_required")
		return
	}

	err := h.store.RevokePublicLink(r.Context(), user.ID, fileID, linkID, time.Now().UTC())
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "public_link_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "public_link_revoke_failed")
		return
	}

	h.store.RecordAuditEvent(r.Context(), user.ID, AuditPublicLinkRevoke, auditResourceTypePubLink, linkID, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "file_id_required")
		return
	}

	file, parts, telegramSession, err := h.store.DownloadData(r.Context(), user.ID, id)
	if errors.Is(err, ErrNotFound) {
		if acceptsHTML(r) {
			h.writeFileUnavailablePage(w)
			return
		}
		writeError(w, http.StatusNotFound, "file_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "file_download_load_failed")
		return
	}

	session, err := h.sessionCrypto.DecryptForTelegramID(telegramSession.OwnerTelegramID, telegramSession.EncryptedSession)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "telegram_session_decrypt_failed")
		return
	}

	completed := h.streamDownload(w, r, file, parts, session, downloadStreamMeta{
		Source: "auth",
		FileID: file.ID,
		UserID: user.ID,
	})
	if completed && file.OwnerID != user.ID {
		h.store.RecordAuditEvent(r.Context(), user.ID, AuditFileShareDownload, auditResourceTypeFile, file.ID, r)
	}
}

func (h *Handler) PublicMetadata(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.PathValue("token"))
	if token == "" {
		writeError(w, http.StatusBadRequest, "public_token_required")
		return
	}

	file, link, err := h.store.PublicFileByTokenHash(r.Context(), publicLinkTokenHash(token))
	if errors.Is(err, ErrNotFound) {
		if acceptsHTML(r) {
			h.writePublicLinkUnavailablePage(w)
			return
		}
		writeError(w, http.StatusNotFound, "public_link_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "public_link_load_failed")
		return
	}

	if acceptsHTML(r) {
		h.writePublicLinkPage(w, r, token, file, link)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"file":              publicFileResponse(file, link.ShowChecksum),
		"password_required": link.PasswordRequired,
	})
}

func (h *Handler) PublicDownload(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.PathValue("token"))
	if token == "" {
		writeError(w, http.StatusBadRequest, "public_token_required")
		return
	}
	if h.publicLimiter != nil {
		decision := h.publicLimiter.Allow(r, token)
		if decision.Err != nil {
			h.logger.Warn("public download rate limit store failed", "error", decision.Err)
		}
		if !decision.Allowed {
			applyRateLimitResponse(w, decision)
			return
		}
	}

	file, link, claimed, err := h.store.ReservePublicLinkDownloadSlot(r.Context(), publicLinkTokenHash(token))
	if !claimed {
		if acceptsHTML(r) {
			h.writePublicLinkUnavailablePage(w)
			return
		}
		writeError(w, http.StatusNotFound, "public_link_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "public_download_load_failed")
		return
	}
	completed := false
	defer func() {
		if err := h.store.FinishPublicLinkDownload(r.Context(), link.ID, completed); err != nil {
			h.logger.Warn("public download counter update failed", "public_link_id", link.ID, "completed", completed, "error", err)
		}
	}()
	if link.PasswordRequired && !verifyPublicLinkPassword(link, publicLinkPasswordFromRequest(r)) {
		if acceptsHTML(r) {
			h.writePublicLinkPageWithError(w, r, token, file, link, http.StatusUnauthorized, "Incorrect password. Try again.")
			return
		}
		writeError(w, http.StatusUnauthorized, "public_link_password_required")
		return
	}

	file, parts, telegramSession, err := h.store.DownloadDataForPublicFile(r.Context(), file)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "public_link_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "public_download_load_failed")
		return
	}

	session, err := h.sessionCrypto.DecryptForTelegramID(telegramSession.OwnerTelegramID, telegramSession.EncryptedSession)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "telegram_session_decrypt_failed")
		return
	}

	completed = h.streamDownload(w, r, file, parts, session, downloadStreamMeta{
		Source:       "public",
		FileID:       file.ID,
		PublicLinkID: link.ID,
	})
	if completed {
		h.store.RecordAuditEvent(r.Context(), "", AuditPublicLinkDownload, auditResourceTypePubLink, link.ID, r)
	}
}

type downloadStreamMeta struct {
	Source       string
	FileID       string
	UserID       string
	PublicLinkID string
}

type countingWriter struct {
	dst io.Writer
	n   int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.dst.Write(p)
	w.n += int64(n)
	return n, err
}

func (h *Handler) streamDownload(w http.ResponseWriter, r *http.Request, file File, parts []FilePart, session string, meta downloadStreamMeta) bool {
	name := nullableString(file.NamePlain)
	if name == "" {
		name = "download"
	}
	mimeType := nullableString(file.MimeType)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(name))
	if file.PlaintextSize.Valid {
		w.Header().Set("Content-Length", strconv.FormatInt(file.PlaintextSize.Int64, 10))
	}

	totalWritten := int64(0)
	clientIP := requestClientIP(r)
	startedAt := time.Now()
	h.downloads.Increment(meta.FileID, meta.Source)
	defer h.downloads.Decrement(meta.FileID, meta.Source)

	for _, part := range parts {
		reader, writer := io.Pipe()
		errCh := make(chan error, 1)
		go func(part FilePart) {
			err := h.telegram.DownloadEncryptedPart(r.Context(), session, part.TelegramPeer, part.TelegramMessageID, writer)
			_ = writer.CloseWithError(err)
			errCh <- err
		}(part)

		unwrapReader, unwrapErr := telegramartifact.UnwrapReader(reader)
		if unwrapErr != nil {
			_ = reader.CloseWithError(unwrapErr)
			<-errCh
			return false
		}

		partWriter := &countingWriter{dst: w}
		decryptErr := agefile.DecryptStream(partWriter, unwrapReader, h.ageIdentity)
		downloadErr := <-errCh
		if decryptErr != nil || downloadErr != nil {
			h.logger.Warn("download stream failed",
				"source", meta.Source,
				"file_id", meta.FileID,
				"user_id", meta.UserID,
				"public_link_id", meta.PublicLinkID,
				"client_ip", clientIP,
				"part_number", part.PartNumber,
				"telegram_message_id", part.TelegramMessageID,
				"part_plaintext_size", part.PlaintextSize,
				"part_written_bytes", partWriter.n,
				"total_written_bytes", totalWritten+partWriter.n,
				"decrypt_error", errorString(decryptErr),
				"telegram_download_error", errorString(downloadErr),
				"context_error", errorString(r.Context().Err()),
				"duration_ms", time.Since(startedAt).Milliseconds(),
			)
			return false
		}
		totalWritten += partWriter.n
	}

	h.logger.Info("download stream completed",
		"source", meta.Source,
		"file_id", meta.FileID,
		"user_id", meta.UserID,
		"public_link_id", meta.PublicLinkID,
		"client_ip", clientIP,
		"parts", len(parts),
		"written_bytes", totalWritten,
		"duration_ms", time.Since(startedAt).Milliseconds(),
	)
	return true
}

func requestClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

type createFolderRequest struct {
	Name     string `json:"name"`
	ParentID string `json:"parent_id"`
}

type patchFileRequest struct {
	ParentID *string `json:"parent_id"`
	Name     *string `json:"name"`
}

type bulkFilesRequest struct {
	IDs      []string `json:"ids"`
	ParentID *string  `json:"parent_id"`
}

type createShareRequest struct {
	TelegramID int64  `json:"telegram_id"`
	Permission string `json:"permission"`
	ExpiresAt  string `json:"expires_at"`
}

type createPublicLinkRequest struct {
	ExpiresAt         string `json:"expires_at"`
	Password          string `json:"password"`
	MaxDownloads      *int64 `json:"max_downloads"`
	DownloadLimitMode string `json:"download_limit_mode"`
	ShowChecksum      bool   `json:"show_checksum"`
}

func normalizeName(name string) string {
	return strings.TrimSpace(strings.ReplaceAll(name, "/", ""))
}

func normalizeFileIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
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

func (h *Handler) filesResponseForUser(ctx context.Context, requesterID string, files []File) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(files))
	if len(files) == 0 {
		return out, nil
	}
	fileIDs := make([]string, 0, len(files))
	for _, file := range files {
		fileIDs = append(fileIDs, file.ID)
	}
	accessByID, err := h.store.FileAccessContexts(ctx, requesterID, fileIDs)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		access, ok := accessByID[file.ID]
		out = append(out, fileResponseWithContext(file, requesterID, access, ok))
	}
	return out, nil
}

func (h *Handler) fileResponseForUser(ctx context.Context, requesterID string, file File) (map[string]any, error) {
	accessByID, err := h.store.FileAccessContexts(ctx, requesterID, []string{file.ID})
	if err != nil {
		return nil, err
	}
	access, ok := accessByID[file.ID]
	return fileResponseWithContext(file, requesterID, access, ok), nil
}

func filesResponse(files []File) []map[string]any {
	out := make([]map[string]any, 0, len(files))
	for _, file := range files {
		out = append(out, fileResponse(file))
	}
	return out
}

func fileResponse(file File) map[string]any {
	return map[string]any{
		"id":                 file.ID,
		"owner_id":           file.OwnerID,
		"owner_telegram_id":  nil,
		"owner_username":     nil,
		"owner_display_name": nil,
		"parent_id":          nullableStringValue(file.ParentID),
		"name":               nullableStringValue(file.NamePlain),
		"mime_type":          nullableStringValue(file.MimeType),
		"plaintext_size":     nullableInt64Value(file.PlaintextSize),
		"ciphertext_size":    nullableInt64Value(file.CiphertextSize),
		"type":               file.Type,
		"status":             file.Status,
		"access":             nil,
		"can_delete":         nil,
		"created_at":         file.CreatedAt,
		"updated_at":         file.UpdatedAt,
	}
}

func fileResponseWithContext(file File, requesterID string, access FileAccessContext, hasAccess bool) map[string]any {
	response := fileResponse(file)
	if hasAccess {
		response["owner_telegram_id"] = access.OwnerTelegramID
		response["owner_username"] = nullableStringValue(access.OwnerUsername)
		response["owner_display_name"] = nullableStringValue(access.OwnerDisplayName)
		response["access"] = access.Access
		response["can_delete"] = access.CanDelete
		return response
	}
	if file.OwnerID == requesterID {
		response["access"] = FileAccessOwner
		response["can_delete"] = true
	} else {
		response["access"] = FileAccessSharedRead
		response["can_delete"] = false
	}
	return response
}

func publicFileResponse(file File, showChecksum bool) map[string]any {
	response := fileResponse(file)
	if showChecksum && len(file.Checksum) > 0 {
		response["checksum"] = hex.EncodeToString(file.Checksum)
	} else {
		response["checksum"] = nil
	}
	return response
}

func sharesResponse(shares []Share) []map[string]any {
	out := make([]map[string]any, 0, len(shares))
	for _, share := range shares {
		out = append(out, shareResponse(share))
	}
	return out
}

func shareRecipientsResponse(recipients []ShareRecipient) []map[string]any {
	out := make([]map[string]any, 0, len(recipients))
	for _, recipient := range recipients {
		out = append(out, map[string]any{
			"user_id":      recipient.UserID,
			"telegram_id":  recipient.TelegramID,
			"username":     nullableStringValue(recipient.Username),
			"display_name": nullableStringValue(recipient.DisplayName),
		})
	}
	return out
}

func shareResponse(share Share) map[string]any {
	return map[string]any{
		"id":                  share.ID,
		"file_id":             share.FileID,
		"owner_id":            share.OwnerID,
		"grantee_user_id":     share.GranteeUserID,
		"grantee_telegram_id": share.GranteeTelegramID,
		"grantee_username":    nullableStringValue(share.GranteeUsername),
		"grantee_name":        nullableStringValue(share.GranteeName),
		"permission":          share.Permission,
		"expires_at":          nullableTimeValue(share.ExpiresAt),
		"revoked_at":          nullableTimeValue(share.RevokedAt),
		"created_at":          share.CreatedAt,
		"updated_at":          share.UpdatedAt,
	}
}

func publicLinksResponse(links []PublicLink) []map[string]any {
	out := make([]map[string]any, 0, len(links))
	for _, link := range links {
		out = append(out, publicLinkResponse(link))
	}
	return out
}

func publicLinkResponse(link PublicLink) map[string]any {
	return map[string]any{
		"id":                    link.ID,
		"file_id":               link.FileID,
		"owner_id":              link.OwnerID,
		"permission":            link.Permission,
		"expires_at":            nullableTimeValue(link.ExpiresAt),
		"revoked_at":            nullableTimeValue(link.RevokedAt),
		"max_downloads":         nullableInt64Value(link.MaxDownloads),
		"download_count":        link.DownloadCount,
		"active_download_count": link.ActiveDownloadCount,
		"download_limit_mode":   link.DownloadLimitMode,
		"show_checksum":         link.ShowChecksum,
		"password_required":     link.PasswordRequired,
		"created_at":            link.CreatedAt,
		"updated_at":            link.UpdatedAt,
	}
}

func newPublicLinkToken() (string, []byte, error) {
	raw := make([]byte, publicLinkTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, publicLinkTokenHash(token), nil
}

func publicLinkTokenHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func publicLinkURL(r *http.Request, token string) string {
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	if forwardedProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = strings.Split(forwardedProto, ",")[0]
	}
	host := r.Host
	if forwardedHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
		host = strings.Split(forwardedHost, ",")[0]
	}
	return scheme + "://" + strings.TrimSpace(host) + "/public/" + token
}

func derivePublicLinkPassword(password string, minLength int) (PublicLinkPassword, bool) {
	password = strings.TrimSpace(password)
	if password == "" {
		return PublicLinkPassword{}, true
	}
	if minLength <= 0 {
		minLength = publicLinkPasswordMinLength
	}
	if len(password) < minLength || len(password) > publicLinkPasswordMaxLength {
		return PublicLinkPassword{}, false
	}
	salt := make([]byte, publicLinkPasswordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return PublicLinkPassword{}, false
	}
	hash := argon2.IDKey(
		[]byte(password),
		salt,
		publicLinkArgonTime,
		publicLinkArgonMemoryKiB,
		publicLinkArgonThreads,
		publicLinkPasswordHashBytes,
	)
	return PublicLinkPassword{
		KDF:            "argon2id",
		Salt:           salt,
		Hash:           hash,
		ArgonTime:      publicLinkArgonTime,
		ArgonMemoryKiB: publicLinkArgonMemoryKiB,
		ArgonThreads:   publicLinkArgonThreads,
	}, true
}

func verifyPublicLinkPassword(link PublicLink, password string) bool {
	if !link.PasswordRequired {
		return true
	}
	if strings.TrimSpace(password) == "" ||
		!link.PasswordKDF.Valid ||
		link.PasswordKDF.String != "argon2id" ||
		len(link.PasswordSalt) == 0 ||
		len(link.PasswordHash) == 0 ||
		!link.PasswordArgonTime.Valid ||
		!link.PasswordArgonMemoryKiB.Valid ||
		!link.PasswordArgonThreads.Valid {
		return false
	}
	hash := argon2.IDKey(
		[]byte(password),
		link.PasswordSalt,
		uint32(link.PasswordArgonTime.Int64),
		uint32(link.PasswordArgonMemoryKiB.Int64),
		uint8(link.PasswordArgonThreads.Int64),
		uint32(len(link.PasswordHash)),
	)
	return subtle.ConstantTimeCompare(hash, link.PasswordHash) == 1
}

func publicLinkPasswordFromRequest(r *http.Request) string {
	if header := strings.TrimSpace(r.Header.Get("X-Public-Link-Password")); header != "" {
		return header
	}
	if r.Method == http.MethodPost {
		contentType := r.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "application/json") {
			var body struct {
				Password string `json:"password"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				return body.Password
			}
			return ""
		}
		if err := r.ParseForm(); err == nil {
			return r.Form.Get("password")
		}
	}
	return ""
}

func acceptsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return accept == "" || strings.Contains(accept, "text/html")
}

func (h *Handler) writePublicLinkPage(w http.ResponseWriter, r *http.Request, token string, file File, link PublicLink) {
	h.writePublicLinkPageWithError(w, r, token, file, link, http.StatusOK, "")
}

func (h *Handler) writePublicLinkPageWithError(w http.ResponseWriter, r *http.Request, token string, file File, link PublicLink, status int, passwordError string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	name := nullableString(file.NamePlain)
	if name == "" {
		name = "download"
	}
	w.WriteHeader(status)
	_ = publicLinkPageTemplate.Execute(w, map[string]any{
		"Token":            token,
		"Name":             name,
		"Size":             formatPublicFileSize(file.PlaintextSize),
		"ChecksumShort":    publicChecksumShort(file, link),
		"ChecksumFull":     publicChecksumFull(file, link),
		"PasswordRequired": link.PasswordRequired,
		"PasswordError":    passwordError,
	})
}

func (h *Handler) writePublicLinkUnavailablePage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_ = publicLinkUnavailableTemplate.Execute(w, nil)
}

func (h *Handler) writeFileUnavailablePage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_ = fileUnavailableTemplate.Execute(w, nil)
}

func formatPublicFileSize(size sql.NullInt64) string {
	if !size.Valid {
		return "Unknown size"
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	value := float64(size.Int64)
	unit := units[0]
	for i := 1; i < len(units) && value >= 1024; i++ {
		value /= 1024
		unit = units[i]
	}
	if unit == "B" {
		return strconv.FormatInt(size.Int64, 10) + " B"
	}
	return strconv.FormatFloat(value, 'f', 1, 64) + " " + unit
}

func publicChecksumShort(file File, link PublicLink) string {
	full := publicChecksumFull(file, link)
	if full == "" {
		return ""
	}
	if len(full) <= 12 {
		return full
	}
	return full[:12] + "..."
}

func publicChecksumFull(file File, link PublicLink) string {
	if !link.ShowChecksum || len(file.Checksum) == 0 {
		return ""
	}
	return hex.EncodeToString(file.Checksum)
}

func nullableStringValue(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}

func nullableString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func nullableInt64Value(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

func nullableTimeValue(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time
}

func parseOptionalExpiry(value string) (sql.NullTime, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullTime{}, true
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil || !parsed.After(time.Now()) {
		return sql.NullTime{}, false
	}
	return sql.NullTime{Time: parsed.UTC(), Valid: true}, true
}

func parseOptionalMaxDownloads(value *int64) (sql.NullInt64, bool) {
	if value == nil {
		return sql.NullInt64{}, true
	}
	if *value <= 0 {
		return sql.NullInt64{}, false
	}
	return sql.NullInt64{Int64: *value, Valid: true}, true
}

func parseDownloadLimitMode(value string) (string, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return PublicDownloadLimitModeHard, true
	}
	if value != PublicDownloadLimitModeHard && value != PublicDownloadLimitModeSoft {
		return "", false
	}
	return value, true
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{
		"error": code,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

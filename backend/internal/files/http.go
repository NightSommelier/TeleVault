package files

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"filippo.io/age"
	"golang.org/x/crypto/argon2"

	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/auth"
	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/crypto/agefile"
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
    .panel { width: min(460px, 100%); background: #fff; border: 1px solid #d9dee7; border-radius: 8px; padding: 18px; box-shadow: 0 1px 2px rgba(16, 24, 40, .06); }
    h1 { font-size: 18px; margin: 0 0 6px; overflow-wrap: anywhere; }
    .muted { color: #68707c; margin-bottom: 14px; }
    form { display: grid; gap: 10px; }
    input { min-height: 38px; padding: 0 10px; border: 1px solid #d9dee7; border-radius: 6px; font: inherit; }
    button, a.button { min-height: 38px; display: inline-grid; place-items: center; border: 1px solid #0f766e; border-radius: 6px; background: #0f766e; color: #fff; text-decoration: none; font: inherit; padding: 0 14px; cursor: pointer; }
  </style>
</head>
<body>
  <main>
    <div class="panel">
      <h1>{{.Name}}</h1>
      <div class="muted">{{.Size}}</div>
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
</body>
</html>`))

type Handler struct {
	store         *Store
	sessionCrypto auth.TelegramSessionCrypto
	ageIdentity   age.Identity
	telegram      auth.TelegramStorageClient
}

func NewHandler(db *sql.DB, sessionCrypto auth.TelegramSessionCrypto, ageIdentity age.Identity, telegram auth.TelegramStorageClient) *Handler {
	return &Handler{
		store:         NewStore(db),
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

	writeJSON(w, http.StatusOK, map[string]any{
		"files": filesResponse(items),
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

	writeJSON(w, http.StatusOK, map[string]any{
		"files": filesResponse(items),
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

	writeJSON(w, http.StatusOK, map[string]any{
		"file": fileResponse(file),
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

	err := h.store.SoftDelete(r.Context(), user.ID, id, time.Now().UTC())
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "file_not_found")
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

	file, err := h.store.Move(r.Context(), user.ID, id, strings.TrimSpace(request.ParentID))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "file_not_found")
		return
	}
	if errors.Is(err, ErrInvalidMove) {
		writeError(w, http.StatusBadRequest, "invalid_file_move")
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

	share, err := h.store.CreateShare(r.Context(), user.ID, fileID, request.TelegramID, expiresAt)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "file_or_user_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "share_create_failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"share": shareResponse(share),
	})
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

	password, ok := derivePublicLinkPassword(request.Password)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_public_link_password")
		return
	}

	link, err := h.store.CreatePublicLink(r.Context(), user.ID, fileID, tokenHash, expiresAt, password)
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

	h.streamDownload(w, r, file, parts, session)
}

func (h *Handler) PublicMetadata(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.PathValue("token"))
	if token == "" {
		writeError(w, http.StatusBadRequest, "public_token_required")
		return
	}

	file, link, err := h.store.PublicFileByTokenHash(r.Context(), publicLinkTokenHash(token))
	if errors.Is(err, ErrNotFound) {
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
		"file":              fileResponse(file),
		"password_required": link.PasswordRequired,
	})
}

func (h *Handler) PublicDownload(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.PathValue("token"))
	if token == "" {
		writeError(w, http.StatusBadRequest, "public_token_required")
		return
	}

	file, link, err := h.store.PublicFileByTokenHash(r.Context(), publicLinkTokenHash(token))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "public_link_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "public_download_load_failed")
		return
	}
	if link.PasswordRequired && !verifyPublicLinkPassword(link, publicLinkPasswordFromRequest(r)) {
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

	h.streamDownload(w, r, file, parts, session)
}

func (h *Handler) streamDownload(w http.ResponseWriter, r *http.Request, file File, parts []FilePart, session string) {
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

	for _, part := range parts {
		reader, writer := io.Pipe()
		errCh := make(chan error, 1)
		go func(part FilePart) {
			err := h.telegram.DownloadEncryptedPart(r.Context(), session, part.TelegramPeer, part.TelegramMessageID, writer)
			_ = writer.CloseWithError(err)
			errCh <- err
		}(part)

		decryptErr := agefile.DecryptStream(w, reader, h.ageIdentity)
		downloadErr := <-errCh
		if decryptErr != nil || downloadErr != nil {
			return
		}
	}
}

type createFolderRequest struct {
	Name     string `json:"name"`
	ParentID string `json:"parent_id"`
}

type patchFileRequest struct {
	ParentID string `json:"parent_id"`
}

type createShareRequest struct {
	TelegramID int64  `json:"telegram_id"`
	ExpiresAt  string `json:"expires_at"`
}

type createPublicLinkRequest struct {
	ExpiresAt string `json:"expires_at"`
	Password  string `json:"password"`
}

func normalizeName(name string) string {
	return strings.TrimSpace(strings.ReplaceAll(name, "/", ""))
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
		"id":              file.ID,
		"owner_id":        file.OwnerID,
		"parent_id":       nullableStringValue(file.ParentID),
		"name":            nullableStringValue(file.NamePlain),
		"mime_type":       nullableStringValue(file.MimeType),
		"plaintext_size":  nullableInt64Value(file.PlaintextSize),
		"ciphertext_size": nullableInt64Value(file.CiphertextSize),
		"type":            file.Type,
		"status":          file.Status,
		"created_at":      file.CreatedAt,
		"updated_at":      file.UpdatedAt,
	}
}

func sharesResponse(shares []Share) []map[string]any {
	out := make([]map[string]any, 0, len(shares))
	for _, share := range shares {
		out = append(out, shareResponse(share))
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
		"id":                link.ID,
		"file_id":           link.FileID,
		"owner_id":          link.OwnerID,
		"permission":        link.Permission,
		"expires_at":        nullableTimeValue(link.ExpiresAt),
		"revoked_at":        nullableTimeValue(link.RevokedAt),
		"password_required": link.PasswordRequired,
		"created_at":        link.CreatedAt,
		"updated_at":        link.UpdatedAt,
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

func derivePublicLinkPassword(password string) (PublicLinkPassword, bool) {
	password = strings.TrimSpace(password)
	if password == "" {
		return PublicLinkPassword{}, true
	}
	if len(password) < publicLinkPasswordMinLength || len(password) > publicLinkPasswordMaxLength {
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	name := nullableString(file.NamePlain)
	if name == "" {
		name = "download"
	}
	_ = publicLinkPageTemplate.Execute(w, map[string]any{
		"Token":            token,
		"Name":             name,
		"Size":             formatPublicFileSize(file.PlaintextSize),
		"PasswordRequired": link.PasswordRequired,
	})
}

func formatPublicFileSize(size sql.NullInt64) string {
	if !size.Valid {
		return "Unknown size"
	}
	return strconv.FormatInt(size.Int64, 10) + " bytes"
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

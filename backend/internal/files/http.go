package files

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"filippo.io/age"

	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/auth"
	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/crypto/agefile"
)

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

	session, err := h.sessionCrypto.DecryptForTelegramID(user.TelegramID, telegramSession.EncryptedSession)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "telegram_session_decrypt_failed")
		return
	}

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

type createShareRequest struct {
	TelegramID int64  `json:"telegram_id"`
	ExpiresAt  string `json:"expires_at"`
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

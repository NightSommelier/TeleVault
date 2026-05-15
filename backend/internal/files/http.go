package files

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/televault/TeleVault/backend/internal/auth"
)

type Handler struct {
	store *Store
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{store: NewStore(db)}
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

	file, err := h.store.GetByID(r.Context(), user.ID, id)
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

type createFolderRequest struct {
	Name     string `json:"name"`
	ParentID string `json:"parent_id"`
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

func nullableStringValue(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}

func nullableInt64Value(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
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

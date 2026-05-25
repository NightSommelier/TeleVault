package recovery

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/auth"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/crypto/secrets"
)

type Handler struct {
	store *Store
}

func NewHandler(db *sql.DB, box *secrets.Box) *Handler {
	return &Handler{store: NewStore(db, box)}
}

func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	manifest, err := h.store.ExportManifest(r.Context(), user.ID, time.Now().UTC())
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "user_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "recovery_export_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="televault-recovery.json"`)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(manifest); err != nil {
		http.Error(w, "failed to write response", http.StatusInternalServerError)
	}
}

func (h *Handler) Import(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	var manifest Manifest
	if err := json.NewDecoder(r.Body).Decode(&manifest); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	options := parseImportOptions(r)

	summary, err := h.store.ImportManifest(r.Context(), user.ID, manifest, options)
	if errors.Is(err, ErrInvalidManifest) {
		writeError(w, http.StatusBadRequest, "invalid_recovery_manifest")
		return
	}
	if errors.Is(err, ErrReplaceConfirmationRequired) {
		writeError(w, http.StatusBadRequest, "recovery_replace_confirmation_required")
		return
	}
	if errors.Is(err, ErrSnapshotOlder) {
		writeError(w, http.StatusConflict, "recovery_snapshot_is_older")
		return
	}
	if errors.Is(err, ErrConflict) {
		writeError(w, http.StatusConflict, "recovery_import_conflict")
		return
	}
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "user_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "recovery_import_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]any{"import": summary}); err != nil {
		http.Error(w, "failed to write response", http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

func parseImportOptions(r *http.Request) ImportOptions {
	mode := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("mode")))
	options := ImportOptions{Mode: mode}
	confirmRaw := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("confirm_replace")))
	options.ConfirmReplace = confirmRaw == "1" || confirmRaw == "true" || confirmRaw == "yes"
	return options
}

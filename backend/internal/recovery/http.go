package recovery

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/auth"
	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/crypto/secrets"
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

func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

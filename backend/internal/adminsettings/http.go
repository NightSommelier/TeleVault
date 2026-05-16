package adminsettings

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/televault/TeleVault/backend/internal/auth"
	"github.com/televault/TeleVault/backend/internal/config"
)

type Handler struct {
	store *Store
}

func NewHandler(db *sql.DB, cfg config.Config) *Handler {
	return &Handler{store: NewStore(db, cfg)}
}

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.UploadSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "admin_settings_load_failed")
		return
	}
	limits, err := h.store.ListTelegramAccountLimits(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "telegram_account_limits_load_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"upload_settings":         uploadSettingsResponse(settings),
		"telegram_account_limits": telegramAccountLimitsResponse(limits),
	})
}

func (h *Handler) PatchUploadSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	var request uploadSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	settings, err := h.store.UpdateUploadSettings(r.Context(), UploadSettings{
		UploadPartSizeBytes:        request.UploadPartSizeBytes,
		TelegramDocumentLimitBytes: request.TelegramDocumentLimitBytes,
		UploadSafetyMarginBytes:    request.UploadSafetyMarginBytes,
	}, user.ID)
	if errors.Is(err, ErrInvalidSettings) {
		writeError(w, http.StatusBadRequest, "admin_settings_invalid")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "admin_settings_update_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"upload_settings": uploadSettingsResponse(settings),
	})
}

func (h *Handler) PatchTelegramAccountLimit(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	targetUserID := strings.TrimSpace(r.PathValue("user_id"))
	if targetUserID == "" {
		writeError(w, http.StatusBadRequest, "user_id_required")
		return
	}

	var request telegramAccountLimitRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	limit, err := h.store.UpsertTelegramAccountLimit(r.Context(), targetUserID, TelegramAccountLimit{
		TelegramDocumentLimitBytes: request.TelegramDocumentLimitBytes,
		UploadSafetyMarginBytes:    request.UploadSafetyMarginBytes,
		IsPremium:                  request.IsPremium,
	}, user.ID)
	if errors.Is(err, ErrInvalidSettings) {
		writeError(w, http.StatusBadRequest, "telegram_account_limit_invalid")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "telegram_account_limit_update_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"telegram_account_limit": telegramAccountLimitResponse(limit),
	})
}

type uploadSettingsRequest struct {
	UploadPartSizeBytes        int64 `json:"upload_part_size_bytes"`
	TelegramDocumentLimitBytes int64 `json:"telegram_document_limit_bytes"`
	UploadSafetyMarginBytes    int64 `json:"upload_safety_margin_bytes"`
}

type telegramAccountLimitRequest struct {
	TelegramDocumentLimitBytes int64 `json:"telegram_document_limit_bytes"`
	UploadSafetyMarginBytes    int64 `json:"upload_safety_margin_bytes"`
	IsPremium                  bool  `json:"is_premium"`
}

func uploadSettingsResponse(settings UploadSettings) map[string]any {
	return map[string]any{
		"upload_part_size_bytes":        settings.UploadPartSizeBytes,
		"telegram_document_limit_bytes": settings.TelegramDocumentLimitBytes,
		"upload_safety_margin_bytes":    settings.UploadSafetyMarginBytes,
		"updated_at":                    settings.UpdatedAt,
	}
}

func telegramAccountLimitsResponse(limits []TelegramAccountLimit) []map[string]any {
	out := make([]map[string]any, 0, len(limits))
	for _, limit := range limits {
		out = append(out, telegramAccountLimitResponse(limit))
	}
	return out
}

func telegramAccountLimitResponse(limit TelegramAccountLimit) map[string]any {
	return map[string]any{
		"user_id":                       limit.UserID,
		"telegram_id":                   limit.TelegramID,
		"username":                      nullableStringValue(limit.Username),
		"display_name":                  nullableStringValue(limit.DisplayName),
		"telegram_document_limit_bytes": limit.TelegramDocumentLimitBytes,
		"upload_safety_margin_bytes":    limit.UploadSafetyMarginBytes,
		"detected_document_limit_bytes": nullableInt64Value(limit.DetectedDocumentLimitBytes),
		"is_premium":                    limit.IsPremium,
		"last_probe_status":             nullableStringValue(limit.LastProbeStatus),
		"last_probe_error":              nullableStringValue(limit.LastProbeError),
		"last_probed_at":                nullableTimeValue(limit.LastProbedAt),
		"next_probe_at":                 nullableTimeValue(limit.NextProbeAt),
		"updated_at":                    limit.UpdatedAt,
	}
}

func nullableStringValue(value sql.NullString) any {
	if value.Valid {
		return value.String
	}
	return nil
}

func nullableInt64Value(value sql.NullInt64) any {
	if value.Valid {
		return value.Int64
	}
	return nil
}

func nullableTimeValue(value sql.NullTime) any {
	if value.Valid {
		return value.Time
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

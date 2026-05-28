package adminsettings

import (
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/NightSommelier/TeleVault/backend/internal/auth"
	"github.com/NightSommelier/TeleVault/backend/internal/config"
	"github.com/NightSommelier/TeleVault/backend/internal/licensing"
)

type Handler struct {
	store        *Store
	authStore    *auth.SessionStore
	licenseStore *licensing.Store
	cfg          config.Config
	publicKeys   map[string]ed25519.PublicKey
}

func NewHandler(db *sql.DB, cfg config.Config) *Handler {
	return &Handler{
		store:        NewStore(db, cfg),
		authStore:    auth.NewSessionStore(db),
		licenseStore: licensing.NewStore(db),
		cfg:          cfg,
		publicKeys:   licensing.DefaultPublicKeys(),
	}
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
	instanceID, err := h.store.InstanceID(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "admin_settings_load_failed")
		return
	}
	forceLocalMFA, err := h.store.ForceLocalMFA(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "admin_settings_load_failed")
		return
	}
	licenseState, err := h.licenseStore.Current(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "license_state_load_failed")
		return
	}
	entitlement := licensing.EffectiveEntitlement(licenseState)

	writeJSON(w, http.StatusOK, map[string]any{
		"upload_settings":         uploadSettingsResponse(settings),
		"telegram_account_limits": telegramAccountLimitsResponse(limits),
		"instance_id":             instanceID,
		"security": map[string]any{
			"force_local_mfa": forceLocalMFA || h.cfg.AuthForceMFA,
		},
		"license": licenseStateResponse(licenseState, entitlement, instanceID),
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
		UploadPartSizeBytes:          request.UploadPartSizeBytes,
		TelegramDocumentLimitBytes:   request.TelegramDocumentLimitBytes,
		UploadSafetyMarginBytes:      request.UploadSafetyMarginBytes,
		MaxParallelUploads:           request.MaxParallelUploads,
		TargetUploadBytesPerSecond:   request.TargetUploadBytesPerSecond,
		CooldownBetweenPartsMillisec: request.CooldownBetweenPartsMillisec,
		PublicLinkPasswordMinLength:  request.PublicLinkPasswordMinLength,
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
		TelegramDocumentLimitBytes:   request.TelegramDocumentLimitBytes,
		UploadSafetyMarginBytes:      request.UploadSafetyMarginBytes,
		IsPremium:                    request.IsPremium,
		MaxParallelUploads:           nullableInt64FromPointer(request.MaxParallelUploads),
		TargetUploadBytesPerSecond:   nullableInt64FromPointer(request.TargetUploadBytesPerSecond),
		CooldownBetweenPartsMillisec: nullableInt64FromPointer(request.CooldownBetweenPartsMillisec),
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

func (h *Handler) PatchLicense(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	var request licenseInstallRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	raw := strings.TrimSpace(request.RawLicenseJSON)
	if raw == "" {
		writeError(w, http.StatusBadRequest, "license_required")
		return
	}

	instanceID, err := h.store.InstanceID(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "admin_settings_load_failed")
		return
	}

	state, err := licensing.VerifyInstalledLicense(licensing.VerifyInput{
		RawJSON:         []byte(raw),
		LocalInstanceID: instanceID,
		Now:             time.Now().UTC(),
		PublicKeys:      h.publicKeys,
	})
	if errors.Is(err, licensing.ErrMissingPublicKeys) {
		writeError(w, http.StatusInternalServerError, "license_keys_not_configured")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "license_verify_failed")
		return
	}

	now := time.Now().UTC()
	state.InstalledAt = &now
	state.ValidatedAt = &now
	if _, err := h.licenseStore.Upsert(r.Context(), state, user.ID); err != nil {
		if errors.Is(err, licensing.ErrInvalidState) {
			writeError(w, http.StatusBadRequest, "license_invalid")
			return
		}
		writeError(w, http.StatusInternalServerError, "license_install_failed")
		return
	}

	saved, err := h.licenseStore.Current(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "license_state_load_failed")
		return
	}
	entitlement := licensing.EffectiveEntitlement(saved)
	writeJSON(w, http.StatusOK, map[string]any{
		"instance_id": instanceID,
		"license":     licenseStateResponse(saved, entitlement, instanceID),
	})
}

func (h *Handler) DeleteLicense(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	instanceID, err := h.store.InstanceID(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "admin_settings_load_failed")
		return
	}

	saved, err := h.licenseStore.Clear(r.Context(), user.ID)
	if err != nil {
		if errors.Is(err, licensing.ErrInvalidState) {
			writeError(w, http.StatusBadRequest, "license_invalid")
			return
		}
		writeError(w, http.StatusInternalServerError, "license_install_failed")
		return
	}

	entitlement := licensing.EffectiveEntitlement(saved)
	writeJSON(w, http.StatusOK, map[string]any{
		"instance_id": instanceID,
		"license":     licenseStateResponse(saved, entitlement, instanceID),
	})
}

func (h *Handler) ListInvites(w http.ResponseWriter, r *http.Request) {
	invites, err := h.authStore.ListInstanceInvites(r.Context(), 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invites_load_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"invites": instanceInvitesResponse(invites),
	})
}

func (h *Handler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	var request createInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	maxUses := request.MaxUses
	if maxUses <= 0 {
		maxUses = 1
	}
	expiresAt := request.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = time.Now().UTC().Add(7 * 24 * time.Hour)
	}
	invitedTelegramID := sql.NullInt64{}
	if request.InvitedTelegramID != nil {
		if *request.InvitedTelegramID <= 0 {
			writeError(w, http.StatusBadRequest, "invite_invalid")
			return
		}
		invitedTelegramID = sql.NullInt64{Int64: *request.InvitedTelegramID, Valid: true}
	}

	licenseState, err := h.licenseStore.Current(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "license_state_load_failed")
		return
	}
	entitlement := licensing.EffectiveEntitlement(licenseState)

	token, err := auth.NewRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invite_create_failed")
		return
	}
	tokenHash, err := auth.HashRefreshToken(token, h.cfg.RefreshTokenPepper)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invite_create_failed")
		return
	}

	invite, err := h.authStore.CreateInstanceInvite(
		r.Context(),
		user.ID,
		tokenHash,
		invitedTelegramID,
		expiresAt,
		maxUses,
		entitlement.MaxConnectedTelegramAccounts,
	)
	if errors.Is(err, auth.ErrInviteCapacityReached) {
		writeError(w, http.StatusForbidden, "invite_capacity_reached")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invite_create_failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"invite":       instanceInviteResponse(invite),
		"invite_token": token,
	})
}

func (h *Handler) RevokeInvite(w http.ResponseWriter, r *http.Request) {
	inviteID := strings.TrimSpace(r.PathValue("invite_id"))
	if inviteID == "" {
		writeError(w, http.StatusBadRequest, "invite_id_required")
		return
	}

	if err := h.authStore.RevokeInstanceInvite(r.Context(), inviteID); err != nil {
		if errors.Is(err, auth.ErrInviteInvalid) {
			writeError(w, http.StatusNotFound, "invite_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "invite_revoke_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "revoked"})
}

func (h *Handler) PatchSecurity(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	var request securitySettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	saved, err := h.store.UpdateForceLocalMFA(r.Context(), request.ForceLocalMFA, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "admin_settings_update_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"security": map[string]any{
			"force_local_mfa": saved || h.cfg.AuthForceMFA,
		},
	})
}

type uploadSettingsRequest struct {
	UploadPartSizeBytes          int64 `json:"upload_part_size_bytes"`
	TelegramDocumentLimitBytes   int64 `json:"telegram_document_limit_bytes"`
	UploadSafetyMarginBytes      int64 `json:"upload_safety_margin_bytes"`
	MaxParallelUploads           int   `json:"max_parallel_uploads"`
	TargetUploadBytesPerSecond   int64 `json:"target_upload_bytes_per_second"`
	CooldownBetweenPartsMillisec int   `json:"cooldown_between_parts_ms"`
	PublicLinkPasswordMinLength  int   `json:"public_link_password_min_length"`
}

type telegramAccountLimitRequest struct {
	TelegramDocumentLimitBytes   int64  `json:"telegram_document_limit_bytes"`
	UploadSafetyMarginBytes      int64  `json:"upload_safety_margin_bytes"`
	IsPremium                    bool   `json:"is_premium"`
	MaxParallelUploads           *int64 `json:"max_parallel_uploads"`
	TargetUploadBytesPerSecond   *int64 `json:"target_upload_bytes_per_second"`
	CooldownBetweenPartsMillisec *int64 `json:"cooldown_between_parts_ms"`
}

type licenseInstallRequest struct {
	RawLicenseJSON string `json:"raw_license_json"`
}

type createInviteRequest struct {
	InvitedTelegramID *int64    `json:"invited_telegram_id"`
	ExpiresAt         time.Time `json:"expires_at"`
	MaxUses           int       `json:"max_uses"`
}

type securitySettingsRequest struct {
	ForceLocalMFA bool `json:"force_local_mfa"`
}

func uploadSettingsResponse(settings UploadSettings) map[string]any {
	return map[string]any{
		"upload_part_size_bytes":          settings.UploadPartSizeBytes,
		"telegram_document_limit_bytes":   settings.TelegramDocumentLimitBytes,
		"upload_safety_margin_bytes":      settings.UploadSafetyMarginBytes,
		"max_parallel_uploads":            settings.MaxParallelUploads,
		"target_upload_bytes_per_second":  settings.TargetUploadBytesPerSecond,
		"cooldown_between_parts_ms":       settings.CooldownBetweenPartsMillisec,
		"public_link_password_min_length": settings.PublicLinkPasswordMinLength,
		"updated_at":                      settings.UpdatedAt,
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
		"user_id":                        limit.UserID,
		"telegram_id":                    limit.TelegramID,
		"username":                       nullableStringValue(limit.Username),
		"display_name":                   nullableStringValue(limit.DisplayName),
		"telegram_document_limit_bytes":  limit.TelegramDocumentLimitBytes,
		"upload_safety_margin_bytes":     limit.UploadSafetyMarginBytes,
		"detected_document_limit_bytes":  nullableInt64Value(limit.DetectedDocumentLimitBytes),
		"is_premium":                     limit.IsPremium,
		"max_parallel_uploads":           nullableInt64Value(limit.MaxParallelUploads),
		"target_upload_bytes_per_second": nullableInt64Value(limit.TargetUploadBytesPerSecond),
		"cooldown_between_parts_ms":      nullableInt64Value(limit.CooldownBetweenPartsMillisec),
		"last_probe_status":              nullableStringValue(limit.LastProbeStatus),
		"last_probe_error":               nullableStringValue(limit.LastProbeError),
		"last_probed_at":                 nullableTimeValue(limit.LastProbedAt),
		"next_probe_at":                  nullableTimeValue(limit.NextProbeAt),
		"updated_at":                     limit.UpdatedAt,
	}
}

func nullableInt64FromPointer(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
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

func licenseStateResponse(state licensing.State, entitlement licensing.Entitlement, localInstanceID string) map[string]any {
	return map[string]any{
		"status":                          state.Status,
		"tier":                            state.Tier,
		"effective_edition":               entitlement.Edition,
		"max_connected_telegram_accounts": entitlement.MaxConnectedTelegramAccounts,
		"max_workspaces":                  entitlement.MaxWorkspaces,
		"license_id":                      derefString(state.LicenseID),
		"schema_version":                  derefInt(state.SchemaVersion),
		"key_id":                          derefString(state.KeyID),
		"instance_id":                     derefString(state.InstanceID),
		"local_instance_id":               localInstanceID,
		"instance_match":                  localInstanceID != "" && localInstanceID == derefString(state.InstanceID),
		"issued_at":                       derefTime(state.IssuedAt),
		"expires_at":                      derefTime(state.ExpiresAt),
		"grace_days":                      derefInt(state.GraceDays),
		"validation_error":                derefString(state.ValidationError),
		"validated_at":                    derefTime(state.ValidatedAt),
		"installed_at":                    derefTime(state.InstalledAt),
	}
}

func instanceInvitesResponse(invites []auth.InstanceInvite) []map[string]any {
	out := make([]map[string]any, 0, len(invites))
	for _, invite := range invites {
		out = append(out, instanceInviteResponse(invite))
	}
	return out
}

func instanceInviteResponse(invite auth.InstanceInvite) map[string]any {
	return map[string]any{
		"id":                  invite.ID,
		"invited_telegram_id": nullableInt64Value(invite.InvitedTelegramID),
		"max_uses":            invite.MaxUses,
		"used_count":          invite.UsedCount,
		"status":              invite.Status,
		"expires_at":          invite.ExpiresAt,
		"creator_user_id":     invite.CreatorUserID,
		"consumed_at":         nullableTimeValue(invite.ConsumedAt),
		"revoked_at":          nullableTimeValue(invite.RevokedAt),
		"created_at":          invite.CreatedAt,
		"updated_at":          invite.UpdatedAt,
	}
}

func derefString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func derefInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func derefTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

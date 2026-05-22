package auth

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/config"
	"rsc.io/qr"
)

const refreshTokenTTL = 30 * 24 * time.Hour

type Handler struct {
	cfg           config.Config
	logger        *slog.Logger
	store         *SessionStore
	sessionCrypto TelegramSessionCrypto
	telegram      TelegramAuthClient
	rateLimiter   *RateLimiter
	qrLogins      *qrLoginSessions
}

func NewHandler(cfg config.Config, logger *slog.Logger, database *sql.DB, sessionCrypto TelegramSessionCrypto, telegram TelegramAuthClient) *Handler {
	return NewHandlerWithRateLimiter(cfg, logger, database, sessionCrypto, telegram, nil, nil)
}

func NewHandlerWithRateLimiter(cfg config.Config, logger *slog.Logger, database *sql.DB, sessionCrypto TelegramSessionCrypto, telegram TelegramAuthClient, rateLimitStore RateLimitStore, clientIP func(*http.Request) string) *Handler {
	var rateLimiter *RateLimiter
	if cfg.AuthRateLimitEnabled {
		rateLimiter = NewRateLimiterWithStore(RateLimitSettings{
			IPLimitPerMinute:       cfg.TelegramAuthIPLimitPerMinute,
			SendCodePhoneLimitHour: cfg.TelegramSendCodePhoneLimitPerHour,
			LoginPhoneLimitHour:    cfg.TelegramLoginPhoneLimitPerHour,
			ClientIP:               clientIP,
		}, rateLimitStore)
	}

	return &Handler{
		cfg:           cfg,
		logger:        logger,
		store:         NewSessionStore(database),
		sessionCrypto: sessionCrypto,
		telegram:      telegram,
		rateLimiter:   rateLimiter,
		qrLogins:      newQRLoginSessions(),
	}
}

func (h *Handler) StartTelegramQRLogin(w http.ResponseWriter, r *http.Request) {
	if h.telegram == nil {
		writeError(w, http.StatusNotImplemented, "telegram_auth_not_connected")
		return
	}
	if !h.allowRateLimited(w, h.rateLimiter.CheckQRStart(r)) {
		return
	}

	attempt, err := h.telegram.StartQRLogin(r.Context())
	if err != nil {
		h.logger.Warn("telegram qr login start failed", "error", err)
		h.store.RecordAuditEvent(r.Context(), "", AuditAuthCodeSendFailure, r)
		writeError(w, http.StatusBadGateway, "telegram_qr_login_start_failed")
		return
	}

	id, err := NewRefreshToken()
	if err != nil {
		if attempt.Cancel != nil {
			attempt.Cancel()
		}
		writeError(w, http.StatusInternalServerError, "qr_login_id_generation_failed")
		return
	}
	h.qrLogins.add(id, attempt)
	h.store.RecordAuditEvent(r.Context(), "", AuditAuthCodeSendSuccess, r)
	writeJSON(w, http.StatusCreated, qrLoginResponse(id, attempt.Token))
}

func (h *Handler) CompleteTelegramQRLogin(w http.ResponseWriter, r *http.Request) {
	if h.telegram == nil {
		writeError(w, http.StatusNotImplemented, "telegram_auth_not_connected")
		return
	}

	var request telegramQRCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	id := strings.TrimSpace(request.QRLoginID)
	if id == "" {
		writeError(w, http.StatusBadRequest, "qr_login_id_required")
		return
	}

	session, err := h.qrLogins.get(id)
	if errors.Is(err, ErrQRLoginNotFound) {
		writeError(w, http.StatusNotFound, "qr_login_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "qr_login_lookup_failed")
		return
	}

	select {
	case result, ok := <-session.results:
		h.qrLogins.remove(id)
		if !ok {
			writeError(w, http.StatusUnauthorized, "telegram_qr_login_failed")
			return
		}
		if result.Err != nil {
			h.logger.Warn("telegram qr login failed", "error", result.Err)
			h.store.RecordAuditEvent(r.Context(), "", AuditAuthLoginFailure, r)
			writeError(w, http.StatusUnauthorized, "telegram_qr_login_failed")
			return
		}
		h.completeTelegramLogin(w, r, result.Session, result.Profile)
	default:
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":   "pending",
			"qr_login": qrLoginResponse(id, session.token)["qr_login"],
		})
	}
}

func (h *Handler) SendTelegramCode(w http.ResponseWriter, r *http.Request) {
	if h.telegram == nil {
		writeError(w, http.StatusNotImplemented, "telegram_auth_not_connected")
		return
	}

	var request telegramCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	request.Phone = strings.TrimSpace(request.Phone)
	if request.Phone == "" {
		writeError(w, http.StatusBadRequest, "phone_required")
		return
	}

	phoneHash, err := HashPhone(request.Phone, h.cfg.RefreshTokenPepper)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "phone_hash_failed")
		return
	}
	if !h.allowRateLimited(w, h.rateLimiter.CheckSendCode(r, phoneHash)) {
		return
	}

	challenge, err := h.telegram.SendCode(r.Context(), request.Phone)
	if err != nil {
		h.logger.Warn("telegram send code failed", "error", err)
		h.store.RecordAuditEvent(r.Context(), "", AuditAuthCodeSendFailure, r)
		writeError(w, http.StatusBadGateway, "telegram_send_code_failed")
		return
	}

	encryptedClientSession, err := h.sessionCrypto.Encrypt(telegramChallengeAAD(phoneHash), challenge.Session)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "telegram_challenge_encrypt_failed")
		return
	}

	if err := h.store.CreateAuthChallenge(r.Context(), phoneHash, challenge.PhoneCodeHash, encryptedClientSession, time.Now().Add(telegramChallengeTTL)); err != nil {
		writeError(w, http.StatusInternalServerError, "auth_challenge_create_failed")
		return
	}

	h.store.RecordAuditEvent(r.Context(), "", AuditAuthCodeSendSuccess, r)
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "code_sent",
	})
}

func (h *Handler) LoginWithTelegram(w http.ResponseWriter, r *http.Request) {
	if h.telegram == nil {
		writeError(w, http.StatusNotImplemented, "telegram_auth_not_connected")
		return
	}

	var request telegramLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	request.Phone = strings.TrimSpace(request.Phone)
	request.Code = strings.TrimSpace(request.Code)
	request.Password = strings.TrimSpace(request.Password)
	if request.Phone == "" || request.Code == "" {
		writeError(w, http.StatusBadRequest, "phone_and_code_required")
		return
	}

	phoneHash, err := HashPhone(request.Phone, h.cfg.RefreshTokenPepper)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "phone_hash_failed")
		return
	}
	if !h.allowRateLimited(w, h.rateLimiter.CheckLogin(r, phoneHash)) {
		return
	}

	storedChallenge, err := h.store.LatestAuthChallenge(r.Context(), phoneHash)
	if errors.Is(err, ErrInvalidChallenge) {
		writeError(w, http.StatusUnauthorized, "invalid_auth_challenge")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth_challenge_consume_failed")
		return
	}

	clientSession, err := h.sessionCrypto.Decrypt(telegramChallengeAAD(phoneHash), storedChallenge.EncryptedClientSession)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "telegram_challenge_decrypt_failed")
		return
	}

	challenge := TelegramCodeChallenge{
		PhoneCodeHash: storedChallenge.PhoneCodeHash,
		Session:       clientSession,
	}

	telegramSession, profile, err := h.telegram.SignIn(r.Context(), TelegramLoginRequest{
		Phone:     request.Phone,
		Code:      request.Code,
		Password:  request.Password,
		Challenge: challenge,
	})
	if err != nil {
		if errors.Is(err, ErrTelegramMFARequired) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error":        "telegram_mfa_required",
				"mfa_required": true,
			})
			return
		}
		if errors.Is(err, ErrTelegramMFAInvalid) {
			writeError(w, http.StatusUnauthorized, "telegram_mfa_invalid")
			return
		}
		h.logger.Warn("telegram login failed", "error", err)
		h.store.RecordAuditEvent(r.Context(), "", AuditAuthLoginFailure, r)
		writeError(w, http.StatusUnauthorized, "telegram_login_failed")
		return
	}

	if err := h.store.ConsumeAuthChallenge(r.Context(), phoneHash); err != nil {
		writeError(w, http.StatusInternalServerError, "auth_challenge_consume_failed")
		return
	}

	h.completeTelegramLogin(w, r, telegramSession, profile)
}

type telegramCodeRequest struct {
	Phone string `json:"phone"`
}

type telegramLoginRequest struct {
	Phone    string `json:"phone"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

type telegramQRCompleteRequest struct {
	QRLoginID string `json:"qr_login_id"`
}

func (h *Handler) completeTelegramLogin(w http.ResponseWriter, r *http.Request, telegramSession string, profile TelegramProfile) {
	refreshToken, err := NewRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "refresh_token_generation_failed")
		return
	}

	refreshHash, err := HashRefreshToken(refreshToken, h.cfg.RefreshTokenPepper)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "refresh_token_hash_failed")
		return
	}

	encryptedTelegramSession, err := h.sessionCrypto.Encrypt(telegramSessionAADForProfile(profile), telegramSession)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "telegram_session_encrypt_failed")
		return
	}

	expiresAt := time.Now().Add(refreshTokenTTL)
	user, err := h.store.CompleteTelegramLogin(
		r.Context(),
		profile,
		encryptedTelegramSession,
		refreshHash,
		r.UserAgent(),
		nil,
		expiresAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "login_persist_failed")
		return
	}

	SetRefreshCookie(w, h.cfg, refreshToken, expiresAt)
	SetCSRFCookie(w, h.cfg, refreshToken, expiresAt)
	h.store.RecordAuditEvent(r.Context(), user.ID, AuditAuthLoginSuccess, r)
	writeJSON(w, http.StatusOK, map[string]any{
		"user": userResponse(user),
	})
}

func qrLoginResponse(id string, token TelegramQRLoginToken) map[string]any {
	qrImage := ""
	if code, err := qr.Encode(token.URL, qr.M); err == nil {
		qrImage = "data:image/png;base64," + base64.StdEncoding.EncodeToString(code.PNG())
	}
	return map[string]any{
		"qr_login": map[string]any{
			"id":           id,
			"login_url":    token.URL,
			"qr_image_url": qrImage,
			"expires_at":   token.ExpiresAt,
		},
	}
}

func telegramSessionAADForProfile(profile TelegramProfile) string {
	return telegramUserAAD(profile.TelegramID)
}

func (h *Handler) allowRateLimited(w http.ResponseWriter, decision RateLimitDecision) bool {
	if decision.Err != nil {
		h.logger.Warn("auth rate limiter backend failed; allowing request", "error", decision.Err)
	}
	if decision.Allowed {
		return true
	}
	w.Header().Set("Retry-After", retryAfterSeconds(decision.RetryAfter))
	writeError(w, http.StatusTooManyRequests, "auth_rate_limited")
	return false
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(RefreshCookieName)
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "missing_refresh_token")
		return
	}

	oldHash, err := HashRefreshToken(cookie.Value, h.cfg.RefreshTokenPepper)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_refresh_token")
		return
	}

	newToken, err := NewRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "refresh_token_generation_failed")
		return
	}

	newHash, err := HashRefreshToken(newToken, h.cfg.RefreshTokenPepper)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "refresh_token_hash_failed")
		return
	}

	expiresAt := time.Now().Add(refreshTokenTTL)
	user, err := h.store.RotateRefreshToken(r.Context(), oldHash, newHash, expiresAt)
	if errors.Is(err, ErrInvalidSession) {
		ClearRefreshCookie(w, h.cfg)
		ClearCSRFCookie(w, h.cfg)
		h.store.RecordAuditEvent(r.Context(), "", AuditAuthRefreshFailure, r)
		writeError(w, http.StatusUnauthorized, "invalid_refresh_token")
		return
	}
	if err != nil {
		h.store.RecordAuditEvent(r.Context(), "", AuditAuthRefreshFailure, r)
		writeError(w, http.StatusInternalServerError, "refresh_failed")
		return
	}

	SetRefreshCookie(w, h.cfg, newToken, expiresAt)
	SetCSRFCookie(w, h.cfg, newToken, expiresAt)
	h.store.RecordAuditEvent(r.Context(), user.ID, AuditAuthRefreshSuccess, r)
	writeJSON(w, http.StatusOK, map[string]any{
		"user": userResponse(user),
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(RefreshCookieName)
	if err != nil || cookie.Value == "" {
		ClearRefreshCookie(w, h.cfg)
		ClearCSRFCookie(w, h.cfg)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	tokenHash, err := HashRefreshToken(cookie.Value, h.cfg.RefreshTokenPepper)
	if err == nil {
		_ = h.store.RevokeRefreshToken(r.Context(), tokenHash)
	}

	ClearRefreshCookie(w, h.cfg)
	ClearCSRFCookie(w, h.cfg)
	h.store.RecordAuditEvent(r.Context(), "", AuditAuthLogout, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": userResponse(user),
	})
}

func userResponse(user User) map[string]any {
	return map[string]any{
		"id":          user.ID,
		"telegram_id": user.TelegramID,
		"username":    nullableStringValue(user.Username),
		"displayName": nullableStringValue(user.DisplayName),
		"role":        user.Role,
	}
}

func nullableStringValue(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
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

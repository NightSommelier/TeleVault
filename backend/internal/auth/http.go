package auth

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/NightSommelier/TeleVault/backend/internal/config"
	"github.com/NightSommelier/TeleVault/backend/internal/licensing"
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
	probeMu       sync.Mutex
	probeCache    map[string]telegramSessionProbeCache
	probeTTL      time.Duration
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
		probeCache:    make(map[string]telegramSessionProbeCache),
		probeTTL:      60 * time.Second,
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
	request.Password = strings.TrimSpace(request.Password)
	request.InviteToken = strings.TrimSpace(request.InviteToken)
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

	if request.Password != "" {
		result, err := h.qrLogins.submitPassword(id, request.Password)
		if errors.Is(err, ErrQRLoginNotFound) {
			writeError(w, http.StatusNotFound, "qr_login_not_found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "telegram_qr_password_not_expected")
			return
		}
		if result.Err != nil {
			if errors.Is(result.Err, ErrTelegramMFARequired) {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error":        "telegram_mfa_required",
					"mfa_required": true,
				})
				return
			}
			if errors.Is(result.Err, ErrTelegramMFAInvalid) {
				writeError(w, http.StatusUnauthorized, "telegram_mfa_invalid")
				return
			}
			h.qrLogins.remove(id)
			h.logger.Warn("telegram qr login password failed", "error", result.Err)
			h.store.RecordAuditEvent(r.Context(), "", AuditAuthLoginFailure, r)
			writeError(w, http.StatusUnauthorized, "telegram_qr_login_failed")
			return
		}
		h.qrLogins.remove(id)
		h.completeTelegramLogin(w, r, result.Session, result.Profile, request.InviteToken)
		return
	}

	select {
	case result, ok := <-session.results:
		if !ok {
			h.qrLogins.remove(id)
			writeError(w, http.StatusUnauthorized, "telegram_qr_login_failed")
			return
		}
		if result.Err != nil {
			if errors.Is(result.Err, ErrTelegramMFARequired) {
				_ = h.qrLogins.markMFARequired(id)
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error":        "telegram_mfa_required",
					"mfa_required": true,
				})
				return
			}
			h.qrLogins.remove(id)
			h.logger.Warn("telegram qr login failed", "error", result.Err)
			h.store.RecordAuditEvent(r.Context(), "", AuditAuthLoginFailure, r)
			writeError(w, http.StatusUnauthorized, "telegram_qr_login_failed")
			return
		}
		h.qrLogins.remove(id)
		h.completeTelegramLogin(w, r, result.Session, result.Profile, request.InviteToken)
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
	phone, phoneCode := normalizeTelegramPhone(request.Phone)
	if phoneCode != "" {
		writeError(w, http.StatusBadRequest, phoneCode)
		return
	}
	request.Phone = phone

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
		status, code, retryAfter := classifyTelegramCodeSendError(err, "telegram_send_code_failed")
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		digitsCount, hasPlus := phoneLogShape(request.Phone)
		h.logger.Warn("telegram send code failed",
			"action", "send",
			"phone_hash_prefix", phoneHashLogValue(phoneHash),
			"phone_digits_count", digitsCount,
			"phone_has_plus", hasPlus,
			"error_kind", logErrorKind(err),
		)
		h.store.RecordAuditEvent(r.Context(), "", AuditAuthCodeSendFailure, r)
		writeError(w, status, code)
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

	h.logger.Info("telegram auth code delivery",
		"action", "send",
		"phone_hash_prefix", phoneHashLogValue(phoneHash),
		"phone_digits_count", len(onlyPhoneDigits(request.Phone)),
		"phone_has_plus", strings.HasPrefix(request.Phone, "+"),
		"delivery_type", challenge.CodeType,
		"delivery_length", challenge.CodeLength,
		"delivery_next_type", challenge.NextCodeType,
		"delivery_timeout_seconds", challenge.TimeoutSecs,
	)
	h.store.RecordAuditEvent(r.Context(), "", AuditAuthCodeSendSuccess, r)
	writeTelegramCodeDelivery(w, challenge)
}

func (h *Handler) ResendTelegramCode(w http.ResponseWriter, r *http.Request) {
	if h.telegram == nil {
		writeError(w, http.StatusNotImplemented, "telegram_auth_not_connected")
		return
	}

	var request telegramCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	phone, phoneCode := normalizeTelegramPhone(request.Phone)
	if phoneCode != "" {
		writeError(w, http.StatusBadRequest, phoneCode)
		return
	}
	request.Phone = phone

	phoneHash, err := HashPhone(request.Phone, h.cfg.RefreshTokenPepper)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "phone_hash_failed")
		return
	}
	if !h.allowRateLimited(w, h.rateLimiter.CheckSendCode(r, phoneHash)) {
		return
	}

	storedChallenge, err := h.store.LatestAuthChallenge(r.Context(), phoneHash)
	if errors.Is(err, ErrInvalidChallenge) {
		h.logger.Info("telegram resend code challenge missing",
			"action", "resend",
			"phone_hash_prefix", phoneHashLogValue(phoneHash),
		)
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

	challenge, err := h.telegram.ResendCode(r.Context(), request.Phone, TelegramCodeChallenge{
		PhoneCodeHash: storedChallenge.PhoneCodeHash,
		Session:       clientSession,
	})
	if err != nil {
		status, code, retryAfter := classifyTelegramCodeSendError(err, "telegram_resend_code_failed")
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		digitsCount, hasPlus := phoneLogShape(request.Phone)
		h.logger.Warn("telegram resend code failed",
			"action", "resend",
			"phone_hash_prefix", phoneHashLogValue(phoneHash),
			"phone_digits_count", digitsCount,
			"phone_has_plus", hasPlus,
			"error_kind", logErrorKind(err),
		)
		h.store.RecordAuditEvent(r.Context(), "", AuditAuthCodeSendFailure, r)
		writeError(w, status, code)
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

	h.logger.Info("telegram auth code delivery",
		"action", "resend",
		"phone_hash_prefix", phoneHashLogValue(phoneHash),
		"phone_digits_count", len(onlyPhoneDigits(request.Phone)),
		"phone_has_plus", strings.HasPrefix(request.Phone, "+"),
		"delivery_type", challenge.CodeType,
		"delivery_length", challenge.CodeLength,
		"delivery_next_type", challenge.NextCodeType,
		"delivery_timeout_seconds", challenge.TimeoutSecs,
	)
	h.store.RecordAuditEvent(r.Context(), "", AuditAuthCodeSendSuccess, r)
	writeTelegramCodeDelivery(w, challenge)
}

func writeTelegramCodeDelivery(w http.ResponseWriter, challenge TelegramCodeChallenge) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "code_sent",
		"delivery": map[string]any{
			"type":            challenge.CodeType,
			"length":          challenge.CodeLength,
			"next_type":       challenge.NextCodeType,
			"timeout_seconds": challenge.TimeoutSecs,
		},
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
	var phoneCode string
	request.Phone, phoneCode = normalizeTelegramPhone(request.Phone)
	request.Code = strings.TrimSpace(request.Code)
	request.Password = strings.TrimSpace(request.Password)
	request.InviteToken = strings.TrimSpace(request.InviteToken)
	if phoneCode != "" {
		writeError(w, http.StatusBadRequest, phoneCode)
		return
	}
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
		if errors.Is(err, ErrTelegramCodeInvalid) {
			writeError(w, http.StatusUnauthorized, "telegram_code_invalid")
			return
		}
		if errors.Is(err, ErrTelegramCodeExpired) {
			writeError(w, http.StatusUnauthorized, "telegram_code_expired")
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

	h.completeTelegramLogin(w, r, telegramSession, profile, request.InviteToken)
}

func (h *Handler) ReconnectTelegramSession(w http.ResponseWriter, r *http.Request) {
	if h.telegram == nil {
		writeError(w, http.StatusNotImplemented, "telegram_auth_not_connected")
		return
	}

	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	var request telegramLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	var phoneCode string
	request.Phone, phoneCode = normalizeTelegramPhone(request.Phone)
	request.Code = strings.TrimSpace(request.Code)
	request.Password = strings.TrimSpace(request.Password)
	if phoneCode != "" {
		writeError(w, http.StatusBadRequest, phoneCode)
		return
	}
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
		if errors.Is(err, ErrTelegramCodeInvalid) {
			writeError(w, http.StatusUnauthorized, "telegram_code_invalid")
			return
		}
		if errors.Is(err, ErrTelegramCodeExpired) {
			writeError(w, http.StatusUnauthorized, "telegram_code_expired")
			return
		}
		h.logger.Warn("telegram reconnect failed", "error", err)
		h.store.RecordAuditEvent(r.Context(), user.ID, AuditAuthLoginFailure, r)
		writeError(w, http.StatusUnauthorized, "telegram_login_failed")
		return
	}

	if err := h.store.ConsumeAuthChallenge(r.Context(), phoneHash); err != nil {
		writeError(w, http.StatusInternalServerError, "auth_challenge_consume_failed")
		return
	}

	if profile.TelegramID != user.TelegramID {
		h.store.RecordAuditEvent(r.Context(), user.ID, AuditAuthLoginFailure, r)
		writeError(w, http.StatusForbidden, "telegram_account_mismatch")
		return
	}

	encryptedTelegramSession, err := h.sessionCrypto.Encrypt(telegramSessionAADForProfile(profile), telegramSession)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "telegram_session_encrypt_failed")
		return
	}

	if err := h.store.UpsertTelegramSession(r.Context(), user.ID, encryptedTelegramSession, ""); err != nil {
		writeError(w, http.StatusInternalServerError, "telegram_session_update_failed")
		return
	}
	h.setTelegramSessionProbeStatus(user.ID, "ok", "")

	h.store.RecordAuditEvent(r.Context(), user.ID, AuditAuthLoginSuccess, r)
	writeJSON(w, http.StatusOK, map[string]any{
		"user": userResponse(user),
		"session": map[string]any{
			"telegram_session_status": "ok",
			"read_only_map_mode":      false,
		},
	})
}

type telegramCodeRequest struct {
	Phone string `json:"phone"`
}

type telegramLoginRequest struct {
	Phone       string `json:"phone"`
	Code        string `json:"code"`
	Password    string `json:"password"`
	InviteToken string `json:"invite_token"`
}

type telegramQRCompleteRequest struct {
	QRLoginID   string `json:"qr_login_id"`
	Password    string `json:"password"`
	InviteToken string `json:"invite_token"`
}

type logoutRequest struct {
	ForgetDevice bool `json:"forget_device"`
}

func (h *Handler) completeTelegramLogin(w http.ResponseWriter, r *http.Request, telegramSession string, profile TelegramProfile, inviteToken string) {
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

	var loginAccess LoginAccess
	if inviteToken != "" {
		inviteHash, err := HashRefreshToken(inviteToken, h.cfg.RefreshTokenPepper)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "invite_token_hash_failed")
			return
		}
		loginAccess.InviteTokenHash = inviteHash
	}

	expiresAt := time.Now().Add(refreshTokenTTL)
	loginPolicy := h.resolveLoginPolicy(r.Context())
	user, err := h.store.CompleteTelegramLoginWithAccessPolicyAndAccess(
		r.Context(),
		profile,
		encryptedTelegramSession,
		refreshHash,
		r.UserAgent(),
		nil,
		expiresAt,
		loginPolicy,
		loginAccess,
	)
	if errors.Is(err, ErrCommunityUserLimitReached) || errors.Is(err, ErrAccountLimitReached) {
		h.store.RecordAuditEvent(r.Context(), "", AuditAuthLoginFailure, r)
		if loginPolicy.BindCommunityOwner {
			writeError(w, http.StatusForbidden, "community_user_limit_reached")
		} else {
			writeError(w, http.StatusForbidden, "account_limit_reached")
		}
		return
	}
	if errors.Is(err, ErrInviteRequired) || errors.Is(err, ErrInviteInvalid) {
		h.store.RecordAuditEvent(r.Context(), "", AuditAuthLoginFailure, r)
		writeError(w, http.StatusForbidden, "invite_required")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "login_persist_failed")
		return
	}
	h.setTelegramSessionProbeStatus(user.ID, "ok", "")

	localRequired, setupRequired, forceEnabled, err := h.resolveLocalMFARequirement(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_state_failed")
		return
	}
	if localRequired {
		methods, methodsErr := h.resolveReauthMethods(r.Context(), user.ID)
		if methodsErr != nil {
			writeError(w, http.StatusInternalServerError, "local_mfa_state_failed")
			return
		}
		if err := h.store.MarkSessionMFARequiredByToken(r.Context(), refreshHash, true); err != nil {
			writeError(w, http.StatusInternalServerError, "local_mfa_state_failed")
			return
		}
		remembered, rememberErr := h.issueRememberedDevice(r.Context(), w, user.ID, r.UserAgent())
		if rememberErr != nil {
			writeError(w, http.StatusInternalServerError, "remember_device_persist_failed")
			return
		}
		SetRefreshCookie(w, h.cfg, refreshToken, expiresAt)
		SetCSRFCookie(w, h.cfg, refreshToken, expiresAt)
		h.store.RecordAuditEvent(r.Context(), user.ID, AuditAuthLoginSuccess, r)
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error":          "local_mfa_required",
			"mfa_required":   true,
			"setup_required": setupRequired,
			"force_enabled":  forceEnabled,
			"methods":        methods,
			"remembered":     remembered,
		})
		return
	}

	remembered, err := h.issueRememberedDevice(r.Context(), w, user.ID, r.UserAgent())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "remember_device_persist_failed")
		return
	}
	SetRefreshCookie(w, h.cfg, refreshToken, expiresAt)
	SetCSRFCookie(w, h.cfg, refreshToken, expiresAt)
	h.store.RecordAuditEvent(r.Context(), user.ID, AuditAuthLoginSuccess, r)
	writeJSON(w, http.StatusOK, map[string]any{
		"user":       userResponse(user),
		"remembered": remembered,
	})
}

func (h *Handler) RememberedAccount(w http.ResponseWriter, r *http.Request) {
	remembered, device, err := h.verifyRememberedDevice(r.Context(), r)
	if errors.Is(err, ErrRememberedDeviceInvalid) {
		ClearRememberCookie(w, h.cfg)
		writeJSON(w, http.StatusOK, map[string]any{
			"available": false,
		})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "remember_device_lookup_failed")
		return
	}

	methods, err := h.resolveReauthMethods(r.Context(), device.User.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_state_failed")
		return
	}

	telegramStatus := h.telegramSessionStatus(r.Context(), device.User.ID)

	writeJSON(w, http.StatusOK, map[string]any{
		"available": true,
		"remembered": map[string]any{
			"enabled": remembered,
		},
		"user":                    userResponse(device.User),
		"methods":                 methods,
		"telegram_session_status": telegramStatus,
		"read_only_map_mode":      telegramStatus != "ok",
	})
}

func (h *Handler) LoginWithRememberedDevice(w http.ResponseWriter, r *http.Request) {
	_, device, err := h.verifyRememberedDevice(r.Context(), r)
	if errors.Is(err, ErrRememberedDeviceInvalid) {
		ClearRememberCookie(w, h.cfg)
		writeError(w, http.StatusUnauthorized, "remember_device_invalid")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "remember_device_lookup_failed")
		return
	}

	methods, err := h.resolveReauthMethods(r.Context(), device.User.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_state_failed")
		return
	}
	if len(methods) == 0 {
		writeError(w, http.StatusUnauthorized, "telegram_login_required")
		return
	}

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
	expiresAt := time.Now().Add(refreshTokenTTL)
	if err := h.store.CreateSession(r.Context(), device.User.ID, refreshHash, r.UserAgent(), nil, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "session_create_failed")
		return
	}
	if err := h.store.MarkSessionMFARequiredByToken(r.Context(), refreshHash, true); err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_state_failed")
		return
	}

	remembered, rememberErr := h.rotateRememberedDevice(r.Context(), w, device.ID, r.UserAgent())
	if rememberErr != nil {
		writeError(w, http.StatusInternalServerError, "remember_device_persist_failed")
		return
	}

	SetRefreshCookie(w, h.cfg, refreshToken, expiresAt)
	SetCSRFCookie(w, h.cfg, refreshToken, expiresAt)
	writeJSON(w, http.StatusUnauthorized, map[string]any{
		"error":        "local_mfa_required",
		"mfa_required": true,
		"methods":      methods,
		"remembered":   remembered,
	})
}

func (h *Handler) ForgetRememberedDevice(w http.ResponseWriter, r *http.Request) {
	_, device, err := h.verifyRememberedDevice(r.Context(), r)
	if errors.Is(err, ErrRememberedDeviceInvalid) {
		ClearRememberCookie(w, h.cfg)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "remember_device_lookup_failed")
		return
	}
	_ = h.store.RevokeRememberedDevice(r.Context(), device.ID)
	ClearRememberCookie(w, h.cfg)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) resolveLoginPolicy(ctx context.Context) LoginPolicy {
	if h == nil || h.store == nil || h.store.db == nil {
		if h != nil && h.logger != nil {
			h.logger.Warn("license state store unavailable; using community login policy fallback")
		}
		return CommunityLoginPolicy()
	}

	state, err := licensing.NewStore(h.store.db).Current(ctx)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("license state lookup failed; using community login policy fallback", "error", err)
		}
		return CommunityLoginPolicy()
	}

	entitlement := licensing.EffectiveEntitlement(state)
	if entitlement.Edition == licensing.TierCommunity {
		return CommunityLoginPolicy()
	}
	return LoginPolicy{
		MaxConnectedTelegramAccounts: entitlement.MaxConnectedTelegramAccounts,
		BindCommunityOwner:           false,
	}
}

func (h *Handler) resolveLocalMFARequirement(ctx context.Context, userID string) (required bool, setupRequired bool, forceEnabled bool, err error) {
	forceEnabled, err = h.store.IsLocalMFAForced(ctx, h.cfg.AuthForceMFA)
	if err != nil {
		return false, false, false, err
	}
	totpEnabled := false
	totpState, totpErr := h.store.LocalTOTP(ctx, userID)
	if totpErr == nil {
		totpEnabled = totpState.Enabled
	} else if !errors.Is(totpErr, ErrLocalMFANotConfigured) {
		return false, false, false, totpErr
	}
	webAuthnConfigured := false
	webAuthnCredentials, credErr := h.store.WebAuthnCredentials(ctx, userID)
	if credErr != nil {
		return false, false, false, credErr
	}
	if len(webAuthnCredentials) > 0 {
		webAuthnConfigured = true
	}
	passwordConfigured, passErr := h.store.IsLocalPasswordConfigured(ctx, userID)
	if passErr != nil {
		return false, false, false, passErr
	}

	required = forceEnabled || totpEnabled || webAuthnConfigured || passwordConfigured
	setupRequired = required && !(totpEnabled || webAuthnConfigured || passwordConfigured)
	return required, setupRequired, forceEnabled, nil
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

func phoneHashLogValue(phoneHash []byte) string {
	if len(phoneHash) == 0 {
		return ""
	}
	value := hex.EncodeToString(phoneHash)
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

func logErrorKind(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%T", err)
}

func classifyTelegramCodeSendError(err error, fallbackCode string) (status int, code string, retryAfter string) {
	switch {
	case errors.Is(err, ErrTelegramPhoneInvalid):
		return http.StatusBadRequest, "phone_invalid_format", ""
	case errors.Is(err, ErrInvalidChallenge):
		return http.StatusUnauthorized, "invalid_auth_challenge", ""
	case errors.Is(err, ErrTelegramSendCodeRateLimited):
		var rateLimited TelegramRateLimitError
		if errors.As(err, &rateLimited) {
			return http.StatusTooManyRequests, "auth_rate_limited", retryAfterSeconds(rateLimited.RetryAfter)
		}
		return http.StatusTooManyRequests, "auth_rate_limited", ""
	default:
		return http.StatusBadGateway, fallbackCode, ""
	}
}

func normalizeTelegramPhone(raw string) (normalized string, errorCode string) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", "phone_required"
	}

	digits := onlyPhoneDigits(value)
	if len(digits) < 8 || len(digits) > 15 {
		return "", "phone_invalid_format"
	}
	return "+" + digits, ""
}

func onlyPhoneDigits(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func phoneLogShape(phone string) (digitsCount int, hasPlus bool) {
	digitsCount = len(onlyPhoneDigits(phone))
	hasPlus = strings.HasPrefix(strings.TrimSpace(phone), "+")
	return digitsCount, hasPlus
}

func rateLimitScopeFromKey(key string) string {
	switch {
	case strings.HasPrefix(key, "telegram_auth_ip:"):
		return "auth_ip"
	case strings.HasPrefix(key, "telegram_send_code_phone:"):
		return "send_code_phone"
	case strings.HasPrefix(key, "telegram_login_phone:"):
		return "login_phone"
	default:
		return "unknown"
	}
}

func (h *Handler) allowRateLimited(w http.ResponseWriter, decision RateLimitDecision) bool {
	if decision.Err != nil {
		h.logger.Warn("auth rate limiter backend failed; allowing request", "error", decision.Err)
	}
	if decision.Allowed {
		return true
	}
	retryAfter := retryAfterSeconds(decision.RetryAfter)
	h.logger.Info("auth rate limited",
		"scope", rateLimitScopeFromKey(decision.Key),
		"retry_after_seconds", retryAfter,
	)
	w.Header().Set("Retry-After", retryAfter)
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
	var request logoutRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}

	cookie, err := r.Cookie(RefreshCookieName)
	if err != nil || cookie.Value == "" {
		ClearRefreshCookie(w, h.cfg)
		ClearCSRFCookie(w, h.cfg)
		if request.ForgetDevice {
			_ = h.revokeRememberedDeviceFromRequest(r.Context(), r)
			ClearRememberCookie(w, h.cfg)
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	tokenHash, err := HashRefreshToken(cookie.Value, h.cfg.RefreshTokenPepper)
	if err == nil {
		_ = h.store.RevokeRefreshToken(r.Context(), tokenHash)
	}

	ClearRefreshCookie(w, h.cfg)
	ClearCSRFCookie(w, h.cfg)
	if request.ForgetDevice {
		_ = h.revokeRememberedDeviceFromRequest(r.Context(), r)
		ClearRememberCookie(w, h.cfg)
	}
	h.store.RecordAuditEvent(r.Context(), "", AuditAuthLogout, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}
	telegramStatus := h.telegramSessionStatus(r.Context(), user.ID)

	writeJSON(w, http.StatusOK, map[string]any{
		"user": userResponse(user),
		"session": map[string]any{
			"telegram_session_status": telegramStatus,
			"read_only_map_mode":      telegramStatus != "ok",
		},
	})
}

func (h *Handler) issueRememberedDevice(ctx context.Context, w http.ResponseWriter, userID string, userAgent string) (bool, error) {
	token, err := NewRememberToken()
	if err != nil {
		return false, err
	}
	selectorHash, err := HashRefreshToken(token.Selector, h.cfg.RefreshTokenPepper)
	if err != nil {
		return false, err
	}
	verifierHash, err := HashRefreshToken(token.Verifier, h.cfg.RefreshTokenPepper)
	if err != nil {
		return false, err
	}
	expiresAt := time.Now().Add(rememberTokenTTL)
	if err := h.store.CreateRememberedDevice(ctx, userID, selectorHash, verifierHash, userAgent, expiresAt); err != nil {
		return false, err
	}
	SetRememberCookie(w, h.cfg, token.String(), expiresAt)
	return true, nil
}

func (h *Handler) rotateRememberedDevice(ctx context.Context, w http.ResponseWriter, deviceID string, userAgent string) (bool, error) {
	token, err := NewRememberToken()
	if err != nil {
		return false, err
	}
	selectorHash, err := HashRefreshToken(token.Selector, h.cfg.RefreshTokenPepper)
	if err != nil {
		return false, err
	}
	verifierHash, err := HashRefreshToken(token.Verifier, h.cfg.RefreshTokenPepper)
	if err != nil {
		return false, err
	}
	expiresAt := time.Now().Add(rememberTokenTTL)
	if err := h.store.RotateRememberedDevice(ctx, deviceID, selectorHash, verifierHash, userAgent, expiresAt); err != nil {
		return false, err
	}
	SetRememberCookie(w, h.cfg, token.String(), expiresAt)
	return true, nil
}

func (h *Handler) verifyRememberedDevice(ctx context.Context, r *http.Request) (bool, RememberedDevice, error) {
	cookie, err := r.Cookie(RememberCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return false, RememberedDevice{}, ErrRememberedDeviceInvalid
	}
	token, err := ParseRememberToken(cookie.Value)
	if err != nil {
		return false, RememberedDevice{}, ErrRememberedDeviceInvalid
	}
	selectorHash, err := HashRefreshToken(token.Selector, h.cfg.RefreshTokenPepper)
	if err != nil {
		return false, RememberedDevice{}, err
	}
	verifierHash, err := HashRefreshToken(token.Verifier, h.cfg.RefreshTokenPepper)
	if err != nil {
		return false, RememberedDevice{}, err
	}
	device, err := h.store.RememberedDeviceBySelectorHash(ctx, selectorHash)
	if err != nil {
		return false, RememberedDevice{}, err
	}
	if !verifyRememberVerifier(device.VerifierHash, verifierHash) {
		return false, RememberedDevice{}, ErrRememberedDeviceInvalid
	}
	return true, device, nil
}

func (h *Handler) revokeRememberedDeviceFromRequest(ctx context.Context, r *http.Request) error {
	_, device, err := h.verifyRememberedDevice(ctx, r)
	if errors.Is(err, ErrRememberedDeviceInvalid) {
		return nil
	}
	if err != nil {
		return err
	}
	return h.store.RevokeRememberedDevice(ctx, device.ID)
}

func (h *Handler) resolveReauthMethods(ctx context.Context, userID string) ([]string, error) {
	methods := make([]string, 0, 4)
	credentials, err := h.store.WebAuthnCredentials(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(credentials) > 0 {
		methods = append(methods, "webauthn")
	}
	totpEnabled := false
	if state, err := h.store.LocalTOTP(ctx, userID); err == nil {
		totpEnabled = state.Enabled
	} else if !errors.Is(err, ErrLocalMFANotConfigured) {
		return nil, err
	}
	if totpEnabled {
		methods = append(methods, "totp", "recovery")
	}
	passwordConfigured, err := h.store.IsLocalPasswordConfigured(ctx, userID)
	if err != nil {
		return nil, err
	}
	if passwordConfigured {
		methods = append(methods, "password")
	}
	seen := map[string]struct{}{}
	ordered := make([]string, 0, len(methods))
	for _, method := range methods {
		if _, ok := seen[method]; ok {
			continue
		}
		seen[method] = struct{}{}
		ordered = append(ordered, method)
	}
	return ordered, nil
}

func (h *Handler) telegramSessionStatus(ctx context.Context, userID string) string {
	status, _ := h.resolveTelegramSessionStatus(ctx, userID)
	return status
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

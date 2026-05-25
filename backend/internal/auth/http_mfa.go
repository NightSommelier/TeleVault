package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"rsc.io/qr"
)

const defaultMFACodeCount = 10
const webAuthnChallengeTTL = 10 * time.Minute

type totpConfirmRequest struct {
	Code string `json:"code"`
}

type recoveryVerifyRequest struct {
	Code string `json:"code"`
}

func (h *Handler) LocalMFAStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}
	session, _ := SessionFromContext(r.Context())

	forced, err := h.store.IsLocalMFAForced(r.Context(), h.cfg.AuthForceMFA)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_status_failed")
		return
	}

	totpConfigured := false
	totpEnabled := false
	if state, err := h.store.LocalTOTP(r.Context(), user.ID); err == nil {
		totpConfigured = len(state.EncryptedSecret) > 0
		totpEnabled = state.Enabled
	} else if !errors.Is(err, ErrLocalMFANotConfigured) {
		writeError(w, http.StatusInternalServerError, "local_mfa_status_failed")
		return
	}

	recoveryRemaining, err := h.store.RecoveryCodesRemaining(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_status_failed")
		return
	}
	webAuthnCredentials, err := h.store.WebAuthnCredentials(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_status_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"force_enabled":             forced,
		"totp_configured":           totpConfigured,
		"totp_enabled":              totpEnabled,
		"webauthn_configured":       len(webAuthnCredentials) > 0,
		"recovery_codes_remaining":  recoveryRemaining,
		"session_mfa_required":      session.MFARequired,
		"session_mfa_verified":      session.MFAVerifiedAt,
		"session_requires_complete": session.MFARequired && !session.MFAVerifiedAt,
	})
}

func (h *Handler) StartWebAuthnRegistration(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	credentials, err := h.store.WebAuthnCredentials(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "webauthn_registration_start_failed")
		return
	}
	wa, err := webAuthnFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "webauthn_configuration_invalid")
		return
	}
	waUser := webAuthnUser{
		id:          []byte(user.ID),
		name:        mfaUserLabel(user),
		displayName: mfaUserLabel(user),
		credentials: credentials,
	}
	creation, session, err := wa.BeginRegistration(waUser)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "webauthn_registration_start_failed")
		return
	}
	challengeID, err := h.store.CreateWebAuthnChallenge(r.Context(), user.ID, "registration", *session, webAuthnChallengeTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "webauthn_registration_start_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"challenge_id": challengeID,
		"public_key":   creation.Response,
	})
}

func (h *Handler) FinishWebAuthnRegistration(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}
	sessionCtx, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_session")
		return
	}

	challengeID := strings.TrimSpace(r.URL.Query().Get("challenge_id"))
	if challengeID == "" {
		challengeID = strings.TrimSpace(r.Header.Get("X-WebAuthn-Challenge-ID"))
	}
	if challengeID == "" {
		writeError(w, http.StatusBadRequest, "webauthn_challenge_required")
		return
	}

	session, err := h.store.ConsumeWebAuthnChallenge(r.Context(), user.ID, challengeID, "registration")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "webauthn_challenge_invalid")
		return
	}
	wa, err := webAuthnFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "webauthn_configuration_invalid")
		return
	}
	credentials, err := h.store.WebAuthnCredentials(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "webauthn_registration_finish_failed")
		return
	}
	waUser := webAuthnUser{
		id:          []byte(user.ID),
		name:        mfaUserLabel(user),
		displayName: mfaUserLabel(user),
		credentials: credentials,
	}
	credential, err := wa.FinishRegistration(waUser, session, r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "webauthn_registration_finish_failed")
		return
	}
	if err := h.store.UpsertWebAuthnCredential(r.Context(), user.ID, *credential); err != nil {
		writeError(w, http.StatusInternalServerError, "webauthn_registration_finish_failed")
		return
	}
	if err := h.store.MarkSessionMFAVerified(r.Context(), sessionCtx.ID, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "webauthn_registration_finish_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "enabled",
		"user":   userResponse(user),
	})
}

func (h *Handler) StartWebAuthnVerify(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	credentials, err := h.store.WebAuthnCredentials(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "webauthn_verify_start_failed")
		return
	}
	if len(credentials) == 0 {
		writeError(w, http.StatusBadRequest, "webauthn_not_configured")
		return
	}
	wa, err := webAuthnFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "webauthn_configuration_invalid")
		return
	}
	waUser := webAuthnUser{
		id:          []byte(user.ID),
		name:        mfaUserLabel(user),
		displayName: mfaUserLabel(user),
		credentials: credentials,
	}
	assertion, session, err := wa.BeginLogin(waUser)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "webauthn_verify_start_failed")
		return
	}
	challengeID, err := h.store.CreateWebAuthnChallenge(r.Context(), user.ID, "authentication", *session, webAuthnChallengeTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "webauthn_verify_start_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"challenge_id": challengeID,
		"public_key":   assertion.Response,
	})
}

func (h *Handler) FinishWebAuthnVerify(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}
	sessionCtx, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_session")
		return
	}

	challengeID := strings.TrimSpace(r.URL.Query().Get("challenge_id"))
	if challengeID == "" {
		challengeID = strings.TrimSpace(r.Header.Get("X-WebAuthn-Challenge-ID"))
	}
	if challengeID == "" {
		writeError(w, http.StatusBadRequest, "webauthn_challenge_required")
		return
	}

	session, err := h.store.ConsumeWebAuthnChallenge(r.Context(), user.ID, challengeID, "authentication")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "webauthn_challenge_invalid")
		return
	}
	credentials, err := h.store.WebAuthnCredentials(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "webauthn_verify_failed")
		return
	}
	if len(credentials) == 0 {
		writeError(w, http.StatusBadRequest, "webauthn_not_configured")
		return
	}
	wa, err := webAuthnFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "webauthn_configuration_invalid")
		return
	}
	waUser := webAuthnUser{
		id:          []byte(user.ID),
		name:        mfaUserLabel(user),
		displayName: mfaUserLabel(user),
		credentials: credentials,
	}
	credential, err := wa.FinishLogin(waUser, session, r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "webauthn_verify_failed")
		return
	}
	if err := h.store.UpsertWebAuthnCredential(r.Context(), user.ID, *credential); err != nil {
		writeError(w, http.StatusInternalServerError, "webauthn_verify_failed")
		return
	}
	if err := h.store.MarkSessionMFAVerified(r.Context(), sessionCtx.ID, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "webauthn_verify_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "verified",
		"user":   userResponse(user),
	})
}

func (h *Handler) StartTOTPEnrollment(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	secret, err := NewTOTPSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_enroll_start_failed")
		return
	}

	encryptedSecret, err := h.sessionCrypto.Encrypt(mfaSecretAAD(user.ID), secret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_enroll_start_failed")
		return
	}
	if err := h.store.UpsertLocalTOTPSecret(r.Context(), user.ID, encryptedSecret); err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_enroll_start_failed")
		return
	}

	uri := TOTPURI("TeleVault", mfaUserLabel(user), secret)
	qrImage := ""
	if code, err := qr.Encode(uri, qr.M); err == nil {
		qrImage = "data:image/png;base64," + base64.StdEncoding.EncodeToString(code.PNG())
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"totp": map[string]any{
			"secret":       secret,
			"otpauth_uri":  uri,
			"qr_image_url": qrImage,
		},
	})
}

func (h *Handler) ConfirmTOTPEnrollment(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}
	session, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_session")
		return
	}

	var request totpConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	request.Code = strings.TrimSpace(request.Code)
	if request.Code == "" {
		writeError(w, http.StatusBadRequest, "local_mfa_code_required")
		return
	}

	state, err := h.store.LocalTOTP(r.Context(), user.ID)
	if errors.Is(err, ErrLocalMFANotConfigured) {
		writeError(w, http.StatusBadRequest, "local_mfa_not_configured")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_enroll_confirm_failed")
		return
	}

	secret, err := h.sessionCrypto.Decrypt(mfaSecretAAD(user.ID), state.EncryptedSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_enroll_confirm_failed")
		return
	}
	if !VerifyTOTPCode(secret, request.Code, time.Now().UTC()) {
		writeError(w, http.StatusUnauthorized, "local_mfa_code_invalid")
		return
	}

	if err := h.store.EnableLocalTOTP(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_enroll_confirm_failed")
		return
	}
	recoveryCodes, err := GenerateRecoveryCodes(defaultMFACodeCount)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_enroll_confirm_failed")
		return
	}
	recoveryHashes := make([][]byte, 0, len(recoveryCodes))
	for _, code := range recoveryCodes {
		hash, hashErr := HashRefreshToken(code, h.cfg.RefreshTokenPepper)
		if hashErr != nil {
			writeError(w, http.StatusInternalServerError, "local_mfa_enroll_confirm_failed")
			return
		}
		recoveryHashes = append(recoveryHashes, hash)
	}
	if err := h.store.ReplaceRecoveryCodes(r.Context(), user.ID, recoveryHashes); err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_enroll_confirm_failed")
		return
	}
	if err := h.store.MarkSessionMFAVerified(r.Context(), session.ID, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_enroll_confirm_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "enabled",
		"recovery_codes": recoveryCodes,
		"user":           userResponse(user),
	})
}

func (h *Handler) VerifyLocalTOTP(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}
	session, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_session")
		return
	}

	var request totpConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	request.Code = strings.TrimSpace(request.Code)
	if request.Code == "" {
		writeError(w, http.StatusBadRequest, "local_mfa_code_required")
		return
	}

	state, err := h.store.LocalTOTP(r.Context(), user.ID)
	if errors.Is(err, ErrLocalMFANotConfigured) || !state.Enabled {
		writeError(w, http.StatusBadRequest, "local_mfa_not_configured")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_verify_failed")
		return
	}

	secret, err := h.sessionCrypto.Decrypt(mfaSecretAAD(user.ID), state.EncryptedSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_verify_failed")
		return
	}
	if !VerifyTOTPCode(secret, request.Code, time.Now().UTC()) {
		writeError(w, http.StatusUnauthorized, "local_mfa_code_invalid")
		return
	}

	if err := h.store.MarkSessionMFAVerified(r.Context(), session.ID, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "local_mfa_verify_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "verified",
		"user":   userResponse(user),
	})
}

func (h *Handler) VerifyRecoveryCode(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}
	session, ok := SessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_session")
		return
	}

	var request recoveryVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	request.Code = strings.TrimSpace(request.Code)
	if request.Code == "" {
		writeError(w, http.StatusBadRequest, "recovery_code_required")
		return
	}

	hash, err := HashRefreshToken(request.Code, h.cfg.RefreshTokenPepper)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "recovery_code_verify_failed")
		return
	}
	consumed, err := h.store.ConsumeRecoveryCode(r.Context(), user.ID, hash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "recovery_code_verify_failed")
		return
	}
	if !consumed {
		writeError(w, http.StatusUnauthorized, "recovery_code_invalid")
		return
	}
	if err := h.store.MarkSessionMFAVerified(r.Context(), session.ID, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "recovery_code_verify_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "verified",
		"user":   userResponse(user),
	})
}

func webAuthnFromRequest(r *http.Request) (*webauthn.WebAuthn, error) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		scheme = strings.ToLower(strings.Split(forwarded, ",")[0])
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return nil, errors.New("host is required")
	}
	rpID := host
	if h, _, err := net.SplitHostPort(host); err == nil && strings.TrimSpace(h) != "" {
		rpID = h
	}
	origin := fmt.Sprintf("%s://%s", scheme, host)

	return webauthn.New(&webauthn.Config{
		RPDisplayName: "TeleVault",
		RPID:          rpID,
		RPOrigins:     []string{origin},
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			UserVerification: protocol.VerificationPreferred,
		},
	})
}

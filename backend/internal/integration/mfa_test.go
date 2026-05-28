package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NightSommelier/TeleVault/backend/internal/auth"
	"github.com/NightSommelier/TeleVault/backend/internal/config"
)

func TestAuthMFARequireAuthBoundary(t *testing.T) {
	database := openIntegrationDB(t)
	store := auth.NewSessionStore(database)
	ctx := context.Background()

	telegramID := int64(970_000_000_000 + time.Now().UnixNano()%1_000_000_000)
	user, cleanupUser := createUserThroughLogin(t, database, store, telegramID)
	defer cleanupUser()

	const pepper = "integration-refresh-pepper-with-at-least-32-bytes"
	refreshToken := "integration-mfa-boundary-token"
	refreshHash, err := auth.HashRefreshToken(refreshToken, pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken() error = %v", err)
	}
	if err := store.CreateSession(ctx, user.ID, refreshHash, "integration-test", nil, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := store.MarkSessionMFARequiredByToken(ctx, refreshHash, true); err != nil {
		t.Fatalf("MarkSessionMFARequiredByToken(true) error = %v", err)
	}

	handler := auth.NewHandler(
		config.Config{RefreshTokenPepper: pepper},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		database,
		auth.TelegramSessionCrypto{},
		nil,
	)

	protected := handler.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/files", nil)
	req.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: refreshToken})
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("protected status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode protected response: %v", err)
	}
	if body["error"] != "local_mfa_required" {
		t.Fatalf("protected error = %v, want local_mfa_required", body["error"])
	}

	allowed := handler.RequireAuthAllowUnverifiedMFA(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	reqAllowed := httptest.NewRequest(http.MethodGet, "/auth/mfa/status", nil)
	reqAllowed.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: refreshToken})
	recAllowed := httptest.NewRecorder()
	allowed.ServeHTTP(recAllowed, reqAllowed)
	if recAllowed.Code != http.StatusNoContent {
		t.Fatalf("allow-unverified status = %d, want %d", recAllowed.Code, http.StatusNoContent)
	}

	session, err := store.SessionByRefreshToken(ctx, refreshHash)
	if err != nil {
		t.Fatalf("SessionByRefreshToken() error = %v", err)
	}
	if err := store.MarkSessionMFAVerified(ctx, session.ID, user.ID); err != nil {
		t.Fatalf("MarkSessionMFAVerified() error = %v", err)
	}

	reqVerified := httptest.NewRequest(http.MethodGet, "/files", nil)
	reqVerified.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: refreshToken})
	recVerified := httptest.NewRecorder()
	protected.ServeHTTP(recVerified, reqVerified)
	if recVerified.Code != http.StatusNoContent {
		t.Fatalf("verified protected status = %d, want %d", recVerified.Code, http.StatusNoContent)
	}
}

func TestAuthMFARecoveryCodeOneTimeUse(t *testing.T) {
	database := openIntegrationDB(t)
	store := auth.NewSessionStore(database)
	ctx := context.Background()

	telegramID := int64(971_000_000_000 + time.Now().UnixNano()%1_000_000_000)
	user, cleanupUser := createUserThroughLogin(t, database, store, telegramID)
	defer cleanupUser()

	const pepper = "integration-refresh-pepper-with-at-least-32-bytes"
	recoveryCode := "ABCD-EFGH-IJKL"
	recoveryHash, err := auth.HashRefreshToken(recoveryCode, pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken(recovery) error = %v", err)
	}

	if err := store.ReplaceRecoveryCodes(ctx, user.ID, [][]byte{recoveryHash}); err != nil {
		t.Fatalf("ReplaceRecoveryCodes() error = %v", err)
	}
	remainingBefore, err := store.RecoveryCodesRemaining(ctx, user.ID)
	if err != nil {
		t.Fatalf("RecoveryCodesRemaining(before) error = %v", err)
	}
	if remainingBefore != 1 {
		t.Fatalf("RecoveryCodesRemaining(before) = %d, want 1", remainingBefore)
	}

	consumedFirst, err := store.ConsumeRecoveryCode(ctx, user.ID, recoveryHash)
	if err != nil {
		t.Fatalf("ConsumeRecoveryCode(first) error = %v", err)
	}
	if !consumedFirst {
		t.Fatal("ConsumeRecoveryCode(first) = false, want true")
	}

	consumedSecond, err := store.ConsumeRecoveryCode(ctx, user.ID, recoveryHash)
	if err != nil {
		t.Fatalf("ConsumeRecoveryCode(second) error = %v", err)
	}
	if consumedSecond {
		t.Fatal("ConsumeRecoveryCode(second) = true, want false")
	}

	remainingAfter, err := store.RecoveryCodesRemaining(ctx, user.ID)
	if err != nil {
		t.Fatalf("RecoveryCodesRemaining(after) error = %v", err)
	}
	if remainingAfter != 0 {
		t.Fatalf("RecoveryCodesRemaining(after) = %d, want 0", remainingAfter)
	}
}

func TestAuthMFARecoveryCodesRegenerateEndpoint(t *testing.T) {
	database := openIntegrationDB(t)
	store := auth.NewSessionStore(database)
	ctx := context.Background()

	telegramID := int64(972_000_000_000 + time.Now().UnixNano()%1_000_000_000)
	user, cleanupUser := createUserThroughLogin(t, database, store, telegramID)
	defer cleanupUser()

	const pepper = "integration-refresh-pepper-with-at-least-32-bytes"
	const refreshToken = "integration-mfa-regenerate-token"
	refreshHash, err := auth.HashRefreshToken(refreshToken, pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken(refresh) error = %v", err)
	}
	if err := store.CreateSession(ctx, user.ID, refreshHash, "integration-test", nil, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := store.UpsertLocalTOTPSecret(ctx, user.ID, []byte("integration-encrypted-secret")); err != nil {
		t.Fatalf("UpsertLocalTOTPSecret() error = %v", err)
	}
	if err := store.EnableLocalTOTP(ctx, user.ID); err != nil {
		t.Fatalf("EnableLocalTOTP() error = %v", err)
	}

	oldCodeHash, err := auth.HashRefreshToken("ABCD-EFGH-IJKL", pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken(old recovery) error = %v", err)
	}
	if err := store.ReplaceRecoveryCodes(ctx, user.ID, [][]byte{oldCodeHash}); err != nil {
		t.Fatalf("ReplaceRecoveryCodes(initial) error = %v", err)
	}

	handler := auth.NewHandler(
		config.Config{RefreshTokenPepper: pepper},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		database,
		auth.TelegramSessionCrypto{},
		nil,
	)
	endpoint := handler.RequireAuth(handler.RequireCSRF(http.HandlerFunc(handler.RegenerateRecoveryCodes)))

	req := httptest.NewRequest(http.MethodPost, "/auth/mfa/recovery/regenerate", bytes.NewBufferString("{}"))
	req.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: refreshToken})
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "csrf-token"})
	req.Header.Set(auth.CSRFHeaderName, "csrf-token")
	rec := httptest.NewRecorder()
	endpoint.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("regenerate status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode regenerate response: %v", err)
	}
	rawCodes, ok := body["recovery_codes"].([]any)
	if !ok {
		t.Fatalf("recovery_codes type = %T, want []any", body["recovery_codes"])
	}
	if len(rawCodes) != 10 {
		t.Fatalf("len(recovery_codes) = %d, want 10", len(rawCodes))
	}

	remaining, err := store.RecoveryCodesRemaining(ctx, user.ID)
	if err != nil {
		t.Fatalf("RecoveryCodesRemaining(after regenerate) error = %v", err)
	}
	if remaining != len(rawCodes) {
		t.Fatalf("RecoveryCodesRemaining(after regenerate) = %d, want %d", remaining, len(rawCodes))
	}

	stillValidOld, err := store.ConsumeRecoveryCode(ctx, user.ID, oldCodeHash)
	if err != nil {
		t.Fatalf("ConsumeRecoveryCode(old hash) error = %v", err)
	}
	if stillValidOld {
		t.Fatal("old recovery hash is still valid after regeneration")
	}
}

func TestAuthMFADisableLocalPasswordEndpoint(t *testing.T) {
	database := openIntegrationDB(t)
	store := auth.NewSessionStore(database)
	ctx := context.Background()

	telegramID := int64(973_000_000_000 + time.Now().UnixNano()%1_000_000_000)
	user, cleanupUser := createUserThroughLogin(t, database, store, telegramID)
	defer cleanupUser()

	const pepper = "integration-refresh-pepper-with-at-least-32-bytes"
	const refreshToken = "integration-mfa-disable-password-token"
	refreshHash, err := auth.HashRefreshToken(refreshToken, pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken(refresh) error = %v", err)
	}
	if err := store.CreateSession(ctx, user.ID, refreshHash, "integration-test", nil, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	passwordHash, err := auth.HashLocalPassword("abcde")
	if err != nil {
		t.Fatalf("HashLocalPassword() error = %v", err)
	}
	if err := store.UpsertLocalPasswordHash(ctx, user.ID, passwordHash); err != nil {
		t.Fatalf("UpsertLocalPasswordHash() error = %v", err)
	}

	handler := auth.NewHandler(
		config.Config{RefreshTokenPepper: pepper},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		database,
		auth.TelegramSessionCrypto{},
		nil,
	)
	endpoint := handler.RequireAuth(handler.RequireCSRF(http.HandlerFunc(handler.DisableLocalPassword)))

	req := httptest.NewRequest(http.MethodDelete, "/auth/local-password", bytes.NewBufferString("{}"))
	req.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: refreshToken})
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "csrf-token"})
	req.Header.Set(auth.CSRFHeaderName, "csrf-token")
	rec := httptest.NewRecorder()
	endpoint.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("disable local password status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	configured, err := store.IsLocalPasswordConfigured(ctx, user.ID)
	if err != nil {
		t.Fatalf("IsLocalPasswordConfigured() error = %v", err)
	}
	if configured {
		t.Fatal("local password is still configured after disable endpoint")
	}
}

func TestAuthMFADisableTOTPAlsoClearsRecoveryCodesWhenNoWebAuthn(t *testing.T) {
	database := openIntegrationDB(t)
	store := auth.NewSessionStore(database)
	ctx := context.Background()

	telegramID := int64(974_000_000_000 + time.Now().UnixNano()%1_000_000_000)
	user, cleanupUser := createUserThroughLogin(t, database, store, telegramID)
	defer cleanupUser()

	const pepper = "integration-refresh-pepper-with-at-least-32-bytes"
	const refreshToken = "integration-mfa-disable-totp-token"
	refreshHash, err := auth.HashRefreshToken(refreshToken, pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken(refresh) error = %v", err)
	}
	if err := store.CreateSession(ctx, user.ID, refreshHash, "integration-test", nil, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := store.UpsertLocalTOTPSecret(ctx, user.ID, []byte("integration-encrypted-secret")); err != nil {
		t.Fatalf("UpsertLocalTOTPSecret() error = %v", err)
	}
	if err := store.EnableLocalTOTP(ctx, user.ID); err != nil {
		t.Fatalf("EnableLocalTOTP() error = %v", err)
	}
	codeHash, err := auth.HashRefreshToken("ABCD-EFGH-IJKL", pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken(recovery) error = %v", err)
	}
	if err := store.ReplaceRecoveryCodes(ctx, user.ID, [][]byte{codeHash}); err != nil {
		t.Fatalf("ReplaceRecoveryCodes() error = %v", err)
	}

	handler := auth.NewHandler(
		config.Config{RefreshTokenPepper: pepper},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		database,
		auth.TelegramSessionCrypto{},
		nil,
	)
	endpoint := handler.RequireAuth(handler.RequireCSRF(http.HandlerFunc(handler.DisableLocalTOTP)))

	req := httptest.NewRequest(http.MethodDelete, "/auth/mfa/totp", bytes.NewBufferString("{}"))
	req.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: refreshToken})
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "csrf-token"})
	req.Header.Set(auth.CSRFHeaderName, "csrf-token")
	rec := httptest.NewRecorder()
	endpoint.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("disable totp status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if _, err := store.LocalTOTP(ctx, user.ID); err == nil {
		t.Fatal("LocalTOTP() still configured after disable endpoint")
	}
	remaining, err := store.RecoveryCodesRemaining(ctx, user.ID)
	if err != nil {
		t.Fatalf("RecoveryCodesRemaining() error = %v", err)
	}
	if remaining != 0 {
		t.Fatalf("RecoveryCodesRemaining() = %d, want 0 after disabling last local MFA method", remaining)
	}
}

func TestAuthMFADisableRecoveryCodesRejectedWhenTOTPEnabled(t *testing.T) {
	database := openIntegrationDB(t)
	store := auth.NewSessionStore(database)
	ctx := context.Background()

	telegramID := int64(975_000_000_000 + time.Now().UnixNano()%1_000_000_000)
	user, cleanupUser := createUserThroughLogin(t, database, store, telegramID)
	defer cleanupUser()

	const pepper = "integration-refresh-pepper-with-at-least-32-bytes"
	const refreshToken = "integration-mfa-disable-recovery-token"
	refreshHash, err := auth.HashRefreshToken(refreshToken, pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken(refresh) error = %v", err)
	}
	if err := store.CreateSession(ctx, user.ID, refreshHash, "integration-test", nil, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := store.UpsertLocalTOTPSecret(ctx, user.ID, []byte("integration-encrypted-secret")); err != nil {
		t.Fatalf("UpsertLocalTOTPSecret() error = %v", err)
	}
	if err := store.EnableLocalTOTP(ctx, user.ID); err != nil {
		t.Fatalf("EnableLocalTOTP() error = %v", err)
	}
	codeHash, err := auth.HashRefreshToken("ABCD-EFGH-IJKL", pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken(recovery) error = %v", err)
	}
	if err := store.ReplaceRecoveryCodes(ctx, user.ID, [][]byte{codeHash}); err != nil {
		t.Fatalf("ReplaceRecoveryCodes() error = %v", err)
	}

	handler := auth.NewHandler(
		config.Config{RefreshTokenPepper: pepper},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		database,
		auth.TelegramSessionCrypto{},
		nil,
	)
	endpoint := handler.RequireAuth(handler.RequireCSRF(http.HandlerFunc(handler.DisableRecoveryCodes)))

	req := httptest.NewRequest(http.MethodDelete, "/auth/mfa/recovery", bytes.NewBufferString("{}"))
	req.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: refreshToken})
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "csrf-token"})
	req.Header.Set(auth.CSRFHeaderName, "csrf-token")
	rec := httptest.NewRecorder()
	endpoint.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("disable recovery status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode disable recovery response: %v", err)
	}
	if body["error"] != "recovery_codes_required_for_totp" {
		t.Fatalf("disable recovery error = %v, want recovery_codes_required_for_totp", body["error"])
	}
	remaining, err := store.RecoveryCodesRemaining(ctx, user.ID)
	if err != nil {
		t.Fatalf("RecoveryCodesRemaining() error = %v", err)
	}
	if remaining != 1 {
		t.Fatalf("RecoveryCodesRemaining() = %d, want 1", remaining)
	}
}

func TestAuthMFAWebAuthnCredentialRenameAndDeleteEndpoint(t *testing.T) {
	database := openIntegrationDB(t)
	store := auth.NewSessionStore(database)
	ctx := context.Background()

	telegramID := int64(976_000_000_000 + time.Now().UnixNano()%1_000_000_000)
	user, cleanupUser := createUserThroughLogin(t, database, store, telegramID)
	defer cleanupUser()

	const pepper = "integration-refresh-pepper-with-at-least-32-bytes"
	const refreshToken = "integration-mfa-webauthn-credential-token"
	refreshHash, err := auth.HashRefreshToken(refreshToken, pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken(refresh) error = %v", err)
	}
	if err := store.CreateSession(ctx, user.ID, refreshHash, "integration-test", nil, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	recoveryHash, err := auth.HashRefreshToken("WXYZ-QRST-UVAB", pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken(recovery) error = %v", err)
	}
	if err := store.ReplaceRecoveryCodes(ctx, user.ID, [][]byte{recoveryHash}); err != nil {
		t.Fatalf("ReplaceRecoveryCodes() error = %v", err)
	}

	var credentialRowID string
	credentialBytes := []byte("credential-id-" + time.Now().UTC().Format("150405.000000000"))
	if err := database.QueryRowContext(ctx, `
INSERT INTO user_webauthn_credentials (user_id, credential_id, credential_json, display_name, created_at, updated_at)
VALUES ($1, $2, $3, $4, now(), now())
RETURNING id`,
		user.ID,
		credentialBytes,
		[]byte(`{}`),
		"Old key",
	).Scan(&credentialRowID); err != nil {
		t.Fatalf("insert webauthn credential row: %v", err)
	}

	handler := auth.NewHandler(
		config.Config{RefreshTokenPepper: pepper},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		database,
		auth.TelegramSessionCrypto{},
		nil,
	)

	renameEndpoint := handler.RequireAuth(handler.RequireCSRF(http.HandlerFunc(handler.RenameWebAuthnCredential)))
	renameReq := httptest.NewRequest(http.MethodPatch, "/auth/mfa/webauthn/"+credentialRowID, bytes.NewBufferString(`{"display_name":"Work key"}`))
	renameReq.SetPathValue("credential_id", credentialRowID)
	renameReq.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: refreshToken})
	renameReq.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "csrf-token"})
	renameReq.Header.Set(auth.CSRFHeaderName, "csrf-token")
	renameRec := httptest.NewRecorder()
	renameEndpoint.ServeHTTP(renameRec, renameReq)
	if renameRec.Code != http.StatusOK {
		t.Fatalf("rename passkey status = %d, want %d body=%s", renameRec.Code, http.StatusOK, renameRec.Body.String())
	}

	var displayName string
	if err := database.QueryRowContext(ctx, `
SELECT display_name
FROM user_webauthn_credentials
WHERE id = $1
  AND user_id = $2`,
		credentialRowID,
		user.ID,
	).Scan(&displayName); err != nil {
		t.Fatalf("query renamed passkey: %v", err)
	}
	if displayName != "Work key" {
		t.Fatalf("display_name = %q, want %q", displayName, "Work key")
	}

	statusEndpoint := handler.RequireAuthAllowUnverifiedMFA(http.HandlerFunc(handler.LocalMFAStatus))
	statusReq := httptest.NewRequest(http.MethodGet, "/auth/mfa/status", nil)
	statusReq.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: refreshToken})
	statusRec := httptest.NewRecorder()
	statusEndpoint.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("mfa status code = %d, want %d body=%s", statusRec.Code, http.StatusOK, statusRec.Body.String())
	}
	var statusBody map[string]any
	if err := json.Unmarshal(statusRec.Body.Bytes(), &statusBody); err != nil {
		t.Fatalf("decode mfa status response: %v", err)
	}
	credentials, ok := statusBody["webauthn_credentials"].([]any)
	if !ok || len(credentials) != 1 {
		t.Fatalf("webauthn_credentials = %T len=%d, want one entry", statusBody["webauthn_credentials"], len(credentials))
	}
	credential, ok := credentials[0].(map[string]any)
	if !ok {
		t.Fatalf("credential item type = %T, want map[string]any", credentials[0])
	}
	if credential["display_name"] != "Work key" {
		t.Fatalf("status display_name = %v, want Work key", credential["display_name"])
	}

	deleteEndpoint := handler.RequireAuth(handler.RequireCSRF(http.HandlerFunc(handler.DeleteWebAuthnCredential)))
	deleteReq := httptest.NewRequest(http.MethodDelete, "/auth/mfa/webauthn/"+credentialRowID, bytes.NewBufferString("{}"))
	deleteReq.SetPathValue("credential_id", credentialRowID)
	deleteReq.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: refreshToken})
	deleteReq.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "csrf-token"})
	deleteReq.Header.Set(auth.CSRFHeaderName, "csrf-token")
	deleteRec := httptest.NewRecorder()
	deleteEndpoint.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete passkey status = %d, want %d body=%s", deleteRec.Code, http.StatusOK, deleteRec.Body.String())
	}

	var rowCount int
	if err := database.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM user_webauthn_credentials
WHERE user_id = $1`, user.ID).Scan(&rowCount); err != nil {
		t.Fatalf("count passkeys after delete: %v", err)
	}
	if rowCount != 0 {
		t.Fatalf("passkey rows after delete = %d, want 0", rowCount)
	}
	remaining, err := store.RecoveryCodesRemaining(ctx, user.ID)
	if err != nil {
		t.Fatalf("RecoveryCodesRemaining() error = %v", err)
	}
	if remaining != 0 {
		t.Fatalf("RecoveryCodesRemaining() = %d, want 0 after deleting last passkey without TOTP", remaining)
	}
}

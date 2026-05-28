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

func TestRememberedAccountAndLoginFlow(t *testing.T) {
	database := openIntegrationDB(t)
	store := auth.NewSessionStore(database)
	ctx := context.Background()

	telegramID := int64(977_000_000_000 + time.Now().UnixNano()%1_000_000_000)
	user, cleanupUser := createUserThroughLogin(t, database, store, telegramID)
	defer cleanupUser()

	const pepper = "integration-refresh-pepper-with-at-least-32-bytes"
	passwordHash, err := auth.HashLocalPassword("abcde")
	if err != nil {
		t.Fatalf("HashLocalPassword() error = %v", err)
	}
	if err := store.UpsertLocalPasswordHash(ctx, user.ID, passwordHash); err != nil {
		t.Fatalf("UpsertLocalPasswordHash() error = %v", err)
	}

	rememberToken, err := auth.NewRememberToken()
	if err != nil {
		t.Fatalf("NewRememberToken() error = %v", err)
	}
	selectorHash, err := auth.HashRefreshToken(rememberToken.Selector, pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken(selector) error = %v", err)
	}
	verifierHash, err := auth.HashRefreshToken(rememberToken.Verifier, pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken(verifier) error = %v", err)
	}
	if err := store.CreateRememberedDevice(ctx, user.ID, selectorHash, verifierHash, "integration-test", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateRememberedDevice() error = %v", err)
	}

	handler := auth.NewHandler(
		config.Config{RefreshTokenPepper: pepper},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		database,
		auth.TelegramSessionCrypto{},
		nil,
	)

	accountReq := httptest.NewRequest(http.MethodGet, "/auth/remembered-account", nil)
	accountReq.AddCookie(&http.Cookie{Name: auth.RememberCookieName, Value: rememberToken.String()})
	accountRec := httptest.NewRecorder()
	handler.RememberedAccount(accountRec, accountReq)
	if accountRec.Code != http.StatusOK {
		t.Fatalf("RememberedAccount() status = %d, want %d body=%s", accountRec.Code, http.StatusOK, accountRec.Body.String())
	}
	var accountBody map[string]any
	if err := json.Unmarshal(accountRec.Body.Bytes(), &accountBody); err != nil {
		t.Fatalf("decode RememberedAccount() response: %v", err)
	}
	if accountBody["available"] != true {
		t.Fatalf("RememberedAccount() available = %v, want true", accountBody["available"])
	}
	if accountBody["telegram_session_status"] != "ok" {
		t.Fatalf("RememberedAccount() telegram_session_status = %v, want ok", accountBody["telegram_session_status"])
	}
	if !methodListContains(accountBody["methods"], "password") {
		t.Fatalf("RememberedAccount() methods = %v, want password", accountBody["methods"])
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/auth/remembered-login", nil)
	loginReq.AddCookie(&http.Cookie{Name: auth.RememberCookieName, Value: rememberToken.String()})
	loginRec := httptest.NewRecorder()
	handler.LoginWithRememberedDevice(loginRec, loginReq)
	if loginRec.Code != http.StatusUnauthorized {
		t.Fatalf("LoginWithRememberedDevice() status = %d, want %d body=%s", loginRec.Code, http.StatusUnauthorized, loginRec.Body.String())
	}
	var loginBody map[string]any
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginBody); err != nil {
		t.Fatalf("decode LoginWithRememberedDevice() response: %v", err)
	}
	if loginBody["error"] != "local_mfa_required" {
		t.Fatalf("LoginWithRememberedDevice() error = %v, want local_mfa_required", loginBody["error"])
	}
	if !methodListContains(loginBody["methods"], "password") {
		t.Fatalf("LoginWithRememberedDevice() methods = %v, want password", loginBody["methods"])
	}

	setCookies := loginRec.Result().Cookies()
	if !cookieListContains(setCookies, auth.RefreshCookieName) {
		t.Fatalf("LoginWithRememberedDevice() did not set %s cookie", auth.RefreshCookieName)
	}
	if !cookieListContains(setCookies, auth.CSRFCookieName) {
		t.Fatalf("LoginWithRememberedDevice() did not set %s cookie", auth.CSRFCookieName)
	}
	if !cookieListContains(setCookies, auth.RememberCookieName) {
		t.Fatalf("LoginWithRememberedDevice() did not rotate %s cookie", auth.RememberCookieName)
	}
}

func TestReadOnlyMapModeWhenTelegramSessionMissing(t *testing.T) {
	database := openIntegrationDB(t)
	store := auth.NewSessionStore(database)
	ctx := context.Background()

	telegramID := int64(978_000_000_000 + time.Now().UnixNano()%1_000_000_000)
	user, cleanupUser := createUserThroughLogin(t, database, store, telegramID)
	defer cleanupUser()

	if _, err := database.ExecContext(ctx, `DELETE FROM telegram_sessions WHERE user_id = $1`, user.ID); err != nil {
		t.Fatalf("delete telegram session: %v", err)
	}

	const pepper = "integration-refresh-pepper-with-at-least-32-bytes"
	refreshToken := "integration-read-only-map-token"
	refreshHash, err := auth.HashRefreshToken(refreshToken, pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken() error = %v", err)
	}
	if err := store.CreateSession(ctx, user.ID, refreshHash, "integration-test", nil, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	handler := auth.NewHandler(
		config.Config{RefreshTokenPepper: pepper},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		database,
		auth.TelegramSessionCrypto{},
		nil,
	)

	protected := handler.RequireAuth(handler.RequireTelegramSessionWritable(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	protectedReq := httptest.NewRequest(http.MethodPost, "/files", nil)
	protectedReq.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: refreshToken})
	protectedRec := httptest.NewRecorder()
	protected.ServeHTTP(protectedRec, protectedReq)
	if protectedRec.Code != http.StatusForbidden {
		t.Fatalf("RequireTelegramSessionWritable() status = %d, want %d body=%s", protectedRec.Code, http.StatusForbidden, protectedRec.Body.String())
	}
	var protectedBody map[string]any
	if err := json.Unmarshal(protectedRec.Body.Bytes(), &protectedBody); err != nil {
		t.Fatalf("decode RequireTelegramSessionWritable() response: %v", err)
	}
	if protectedBody["error"] != "telegram_session_missing_read_only" {
		t.Fatalf("RequireTelegramSessionWritable() error = %v, want telegram_session_missing_read_only", protectedBody["error"])
	}
	if protectedBody["read_only_map_mode"] != true {
		t.Fatalf("RequireTelegramSessionWritable() read_only_map_mode = %v, want true", protectedBody["read_only_map_mode"])
	}

	me := handler.RequireAuth(http.HandlerFunc(handler.Me))
	meReq := httptest.NewRequest(http.MethodGet, "/me", nil)
	meReq.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: refreshToken})
	meRec := httptest.NewRecorder()
	me.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("Me() status = %d, want %d body=%s", meRec.Code, http.StatusOK, meRec.Body.String())
	}
	var meBody map[string]any
	if err := json.Unmarshal(meRec.Body.Bytes(), &meBody); err != nil {
		t.Fatalf("decode Me() response: %v", err)
	}
	session, ok := meBody["session"].(map[string]any)
	if !ok {
		t.Fatalf("Me() session type = %T, want map[string]any", meBody["session"])
	}
	if session["telegram_session_status"] != "missing" {
		t.Fatalf("Me() telegram_session_status = %v, want missing", session["telegram_session_status"])
	}
	if session["read_only_map_mode"] != true {
		t.Fatalf("Me() read_only_map_mode = %v, want true", session["read_only_map_mode"])
	}
}

func TestRememberedLoginRequiresLocalReauthMethod(t *testing.T) {
	database := openIntegrationDB(t)
	store := auth.NewSessionStore(database)
	ctx := context.Background()

	telegramID := int64(979_000_000_000 + time.Now().UnixNano()%1_000_000_000)
	user, cleanupUser := createUserThroughLogin(t, database, store, telegramID)
	defer cleanupUser()

	const pepper = "integration-refresh-pepper-with-at-least-32-bytes"
	rememberToken, err := auth.NewRememberToken()
	if err != nil {
		t.Fatalf("NewRememberToken() error = %v", err)
	}
	selectorHash, err := auth.HashRefreshToken(rememberToken.Selector, pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken(selector) error = %v", err)
	}
	verifierHash, err := auth.HashRefreshToken(rememberToken.Verifier, pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken(verifier) error = %v", err)
	}
	if err := store.CreateRememberedDevice(ctx, user.ID, selectorHash, verifierHash, "integration-test", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateRememberedDevice() error = %v", err)
	}

	handler := auth.NewHandler(
		config.Config{RefreshTokenPepper: pepper},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		database,
		auth.TelegramSessionCrypto{},
		nil,
	)

	req := httptest.NewRequest(http.MethodPost, "/auth/remembered-login", nil)
	req.AddCookie(&http.Cookie{Name: auth.RememberCookieName, Value: rememberToken.String()})
	rec := httptest.NewRecorder()
	handler.LoginWithRememberedDevice(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("LoginWithRememberedDevice() status = %d, want %d body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode LoginWithRememberedDevice() response: %v", err)
	}
	if body["error"] != "telegram_login_required" {
		t.Fatalf("LoginWithRememberedDevice() error = %v, want telegram_login_required", body["error"])
	}
}

func TestLogoutForgetDeviceRevokesRememberedDevice(t *testing.T) {
	database := openIntegrationDB(t)
	store := auth.NewSessionStore(database)
	ctx := context.Background()

	telegramID := int64(980_000_000_000 + time.Now().UnixNano()%1_000_000_000)
	user, cleanupUser := createUserThroughLogin(t, database, store, telegramID)
	defer cleanupUser()

	const pepper = "integration-refresh-pepper-with-at-least-32-bytes"
	const refreshToken = "integration-logout-forget-device-token"
	refreshHash, err := auth.HashRefreshToken(refreshToken, pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken(refresh) error = %v", err)
	}
	if err := store.CreateSession(ctx, user.ID, refreshHash, "integration-test", nil, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	rememberToken, err := auth.NewRememberToken()
	if err != nil {
		t.Fatalf("NewRememberToken() error = %v", err)
	}
	selectorHash, err := auth.HashRefreshToken(rememberToken.Selector, pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken(selector) error = %v", err)
	}
	verifierHash, err := auth.HashRefreshToken(rememberToken.Verifier, pepper)
	if err != nil {
		t.Fatalf("HashRefreshToken(verifier) error = %v", err)
	}
	if err := store.CreateRememberedDevice(ctx, user.ID, selectorHash, verifierHash, "integration-test", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateRememberedDevice() error = %v", err)
	}

	handler := auth.NewHandler(
		config.Config{RefreshTokenPepper: pepper},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		database,
		auth.TelegramSessionCrypto{},
		nil,
	)

	logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout", bytes.NewBufferString(`{"forget_device":true}`))
	logoutReq.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: refreshToken})
	logoutReq.AddCookie(&http.Cookie{Name: auth.RememberCookieName, Value: rememberToken.String()})
	logoutRec := httptest.NewRecorder()
	handler.Logout(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("Logout() status = %d, want %d body=%s", logoutRec.Code, http.StatusNoContent, logoutRec.Body.String())
	}

	accountReq := httptest.NewRequest(http.MethodGet, "/auth/remembered-account", nil)
	accountReq.AddCookie(&http.Cookie{Name: auth.RememberCookieName, Value: rememberToken.String()})
	accountRec := httptest.NewRecorder()
	handler.RememberedAccount(accountRec, accountReq)
	if accountRec.Code != http.StatusOK {
		t.Fatalf("RememberedAccount() status after forget = %d, want %d body=%s", accountRec.Code, http.StatusOK, accountRec.Body.String())
	}
	var accountBody map[string]any
	if err := json.Unmarshal(accountRec.Body.Bytes(), &accountBody); err != nil {
		t.Fatalf("decode RememberedAccount() response after forget: %v", err)
	}
	if accountBody["available"] != false {
		t.Fatalf("RememberedAccount() available after forget = %v, want false", accountBody["available"])
	}
}

func methodListContains(raw any, method string) bool {
	methods, ok := raw.([]any)
	if !ok {
		return false
	}
	for _, item := range methods {
		value, ok := item.(string)
		if ok && value == method {
			return true
		}
	}
	return false
}

func cookieListContains(cookies []*http.Cookie, name string) bool {
	for _, cookie := range cookies {
		if cookie != nil && cookie.Name == name {
			return true
		}
	}
	return false
}

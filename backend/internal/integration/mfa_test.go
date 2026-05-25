package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/auth"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/config"
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

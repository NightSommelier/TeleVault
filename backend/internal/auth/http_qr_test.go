package auth

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
)

func TestCompleteTelegramQRLoginPending(t *testing.T) {
	handler := &Handler{
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		telegram: stubTelegramAuthClient{},
		qrLogins: newQRLoginSessions(),
	}
	handler.qrLogins.add("qr-1", TelegramQRLoginAttempt{
		Token: TelegramQRLoginToken{
			URL:       "tg://login?token=a",
			ExpiresAt: time.Now().Add(time.Minute),
		},
		Tokens:    closedTokenChannel(),
		Results:   make(chan TelegramQRLoginResult),
		Passwords: make(chan TelegramQRLoginPasswordAttempt),
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/telegram/qr/complete", bytes.NewBufferString(`{"qr_login_id":"qr-1"}`))
	rec := httptest.NewRecorder()
	handler.CompleteTelegramQRLogin(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "pending" {
		t.Fatalf("status body = %v, want pending", body["status"])
	}
}

func TestCompleteTelegramQRLoginMFARequiredKeepsSession(t *testing.T) {
	results := make(chan TelegramQRLoginResult, 1)
	results <- TelegramQRLoginResult{Err: ErrTelegramMFARequired}

	handler := &Handler{
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		telegram: stubTelegramAuthClient{},
		qrLogins: newQRLoginSessions(),
	}
	handler.qrLogins.add("qr-1", TelegramQRLoginAttempt{
		Token: TelegramQRLoginToken{
			URL:       "tg://login?token=a",
			ExpiresAt: time.Now().Add(time.Minute),
		},
		Tokens:    closedTokenChannel(),
		Results:   results,
		Passwords: make(chan TelegramQRLoginPasswordAttempt),
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/telegram/qr/complete", bytes.NewBufferString(`{"qr_login_id":"qr-1"}`))
	rec := httptest.NewRecorder()
	handler.CompleteTelegramQRLogin(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "telegram_mfa_required" {
		t.Fatalf("error = %v, want telegram_mfa_required", body["error"])
	}
	session, err := handler.qrLogins.get("qr-1")
	if err != nil {
		t.Fatalf("session lookup failed: %v", err)
	}
	if !session.mfaNeeded {
		t.Fatal("mfa flag was not kept on qr session")
	}
}

func TestCompleteTelegramQRLoginMFAInvalid(t *testing.T) {
	passwords := make(chan TelegramQRLoginPasswordAttempt, 1)
	go func() {
		attempt := <-passwords
		attempt.Result <- TelegramQRLoginResult{Err: ErrTelegramMFAInvalid}
	}()

	handler := &Handler{
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		telegram: stubTelegramAuthClient{},
		qrLogins: newQRLoginSessions(),
	}
	handler.qrLogins.add("qr-1", TelegramQRLoginAttempt{
		Token: TelegramQRLoginToken{
			URL:       "tg://login?token=a",
			ExpiresAt: time.Now().Add(time.Minute),
		},
		Tokens:    closedTokenChannel(),
		Results:   make(chan TelegramQRLoginResult),
		Passwords: passwords,
	})
	if err := handler.qrLogins.markMFARequired("qr-1"); err != nil {
		t.Fatalf("markMFARequired() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/telegram/qr/complete", bytes.NewBufferString(`{"qr_login_id":"qr-1","password":"bad-password"}`))
	rec := httptest.NewRecorder()
	handler.CompleteTelegramQRLogin(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "telegram_mfa_invalid" {
		t.Fatalf("error = %v, want telegram_mfa_invalid", body["error"])
	}
}

type stubTelegramAuthClient struct{}

func (stubTelegramAuthClient) SendCode(context.Context, string) (TelegramCodeChallenge, error) {
	return TelegramCodeChallenge{}, nil
}

func (stubTelegramAuthClient) SignIn(context.Context, TelegramLoginRequest) (string, TelegramProfile, error) {
	return "", TelegramProfile{}, nil
}

func (stubTelegramAuthClient) StartQRLogin(context.Context) (TelegramQRLoginAttempt, error) {
	return TelegramQRLoginAttempt{}, nil
}

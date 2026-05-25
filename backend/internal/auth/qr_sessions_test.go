package auth

import (
	"errors"
	"testing"
	"time"
)

func TestQRLoginSessionsTracksTokenUpdates(t *testing.T) {
	sessions := newQRLoginSessions()
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	sessions.now = func() time.Time { return now }
	tokens := make(chan TelegramQRLoginToken, 1)
	results := make(chan TelegramQRLoginResult)
	expiresAt := now.Add(time.Minute)

	sessions.add("qr-1", TelegramQRLoginAttempt{
		Token: TelegramQRLoginToken{
			URL:       "tg://login?token=first",
			ExpiresAt: expiresAt,
		},
		Tokens:  tokens,
		Results: results,
	})

	tokens <- TelegramQRLoginToken{URL: "tg://login?token=second", ExpiresAt: expiresAt.Add(time.Minute)}
	close(tokens)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		session, err := sessions.get("qr-1")
		if err != nil {
			t.Fatalf("get() error = %v", err)
		}
		if session.token.URL == "tg://login?token=second" {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("token update was not stored")
}

func TestQRLoginSessionsExpiresAndCancels(t *testing.T) {
	sessions := newQRLoginSessions()
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	sessions.now = func() time.Time { return now }
	cancelled := false

	sessions.add("qr-1", TelegramQRLoginAttempt{
		Token: TelegramQRLoginToken{
			URL:       "tg://login?token=first",
			ExpiresAt: now.Add(-time.Second),
		},
		Tokens:  closedTokenChannel(),
		Results: make(chan TelegramQRLoginResult),
		Cancel:  func() { cancelled = true },
	})

	if _, err := sessions.get("qr-1"); !errors.Is(err, ErrQRLoginNotFound) {
		t.Fatalf("get() error = %v, want ErrQRLoginNotFound", err)
	}
	if !cancelled {
		t.Fatal("expired session did not call Cancel")
	}
}

func TestQRLoginSessionsMFAWindowKeepsSessionAfterQRExpiry(t *testing.T) {
	sessions := newQRLoginSessions()
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	sessions.now = func() time.Time { return now }

	sessions.add("qr-1", TelegramQRLoginAttempt{
		Token: TelegramQRLoginToken{
			URL:       "tg://login?token=first",
			ExpiresAt: now.Add(-time.Second),
		},
		Tokens:  closedTokenChannel(),
		Results: make(chan TelegramQRLoginResult),
	})
	if err := sessions.markMFARequired("qr-1"); err != nil {
		t.Fatalf("markMFARequired() error = %v", err)
	}

	if _, err := sessions.get("qr-1"); err != nil {
		t.Fatalf("get() error = %v, want active session during MFA window", err)
	}
}

func TestQRLoginSessionsMFAWindowExpiresAndCancels(t *testing.T) {
	sessions := newQRLoginSessions()
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	sessions.now = func() time.Time { return now }
	cancelled := false

	sessions.add("qr-1", TelegramQRLoginAttempt{
		Token: TelegramQRLoginToken{
			URL:       "tg://login?token=first",
			ExpiresAt: now.Add(time.Minute),
		},
		Tokens:  closedTokenChannel(),
		Results: make(chan TelegramQRLoginResult),
		Cancel:  func() { cancelled = true },
	})
	if err := sessions.markMFARequired("qr-1"); err != nil {
		t.Fatalf("markMFARequired() error = %v", err)
	}

	now = now.Add(qrLoginMFAWindow + time.Second)
	if _, err := sessions.get("qr-1"); !errors.Is(err, ErrQRLoginNotFound) {
		t.Fatalf("get() error = %v, want ErrQRLoginNotFound after MFA window", err)
	}
	if !cancelled {
		t.Fatal("expired MFA session did not call Cancel")
	}
}

func closedTokenChannel() <-chan TelegramQRLoginToken {
	ch := make(chan TelegramQRLoginToken)
	close(ch)
	return ch
}

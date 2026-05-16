package auth

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterLimitsSendCodeByPhone(t *testing.T) {
	limiter := NewRateLimiter(RateLimitSettings{
		IPLimitPerMinute:       10,
		SendCodePhoneLimitHour: 1,
		LoginPhoneLimitHour:    10,
	})
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	request := httptest.NewRequest("POST", "/auth/telegram/send-code", nil)
	request.RemoteAddr = "203.0.113.10:12345"
	phoneHash := []byte("phone-hash")

	if decision := limiter.CheckSendCode(request, phoneHash); !decision.Allowed {
		t.Fatalf("first CheckSendCode() allowed = false, retry_after = %s", decision.RetryAfter)
	}
	if decision := limiter.CheckSendCode(request, phoneHash); decision.Allowed {
		t.Fatal("second CheckSendCode() allowed = true, want rate limited")
	}

	now = now.Add(time.Hour)
	if decision := limiter.CheckSendCode(request, phoneHash); !decision.Allowed {
		t.Fatalf("CheckSendCode() after reset allowed = false, retry_after = %s", decision.RetryAfter)
	}
}

func TestRateLimiterLimitsSharedAuthIP(t *testing.T) {
	limiter := NewRateLimiter(RateLimitSettings{
		IPLimitPerMinute:       1,
		SendCodePhoneLimitHour: 10,
		LoginPhoneLimitHour:    10,
	})
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	request := httptest.NewRequest("POST", "/auth/telegram/send-code", nil)
	request.RemoteAddr = "203.0.113.20:12345"

	if decision := limiter.CheckSendCode(request, []byte("phone-a")); !decision.Allowed {
		t.Fatalf("first CheckSendCode() allowed = false, retry_after = %s", decision.RetryAfter)
	}
	if decision := limiter.CheckLogin(request, []byte("phone-b")); decision.Allowed {
		t.Fatal("CheckLogin() allowed = true, want shared IP rate limited")
	}
}

func TestRetryAfterSecondsRoundsUp(t *testing.T) {
	if got := retryAfterSeconds(1500 * time.Millisecond); got != "2" {
		t.Fatalf("retryAfterSeconds() = %q, want 2", got)
	}
	if got := retryAfterSeconds(0); got != "1" {
		t.Fatalf("retryAfterSeconds(0) = %q, want 1", got)
	}
}

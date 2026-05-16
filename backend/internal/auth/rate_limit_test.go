package auth

import (
	"context"
	"errors"
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
	limiter.store.(*MemoryRateLimitStore).now = func() time.Time { return now }

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
	limiter.store.(*MemoryRateLimitStore).now = func() time.Time { return now }

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

func TestValkeyRateLimitStoreUsesSharedFixedWindow(t *testing.T) {
	client := &fakeValkeyCounter{}
	store := NewValkeyRateLimitStore(client, "test")
	now := time.Date(2026, 5, 16, 12, 0, 30, 0, time.UTC)

	first, err := store.Allow(context.Background(), "phone", 1, time.Minute, now)
	if err != nil {
		t.Fatalf("first Allow() error = %v", err)
	}
	if !first.Allowed {
		t.Fatal("first Allow() allowed = false, want true")
	}

	second, err := store.Allow(context.Background(), "phone", 1, time.Minute, now)
	if err != nil {
		t.Fatalf("second Allow() error = %v", err)
	}
	if second.Allowed {
		t.Fatal("second Allow() allowed = true, want false")
	}
	if second.RetryAfter != 30*time.Second {
		t.Fatalf("RetryAfter = %s, want 30s", second.RetryAfter)
	}
	if len(client.expiredKeys) != 1 {
		t.Fatalf("Expire calls = %d, want 1", len(client.expiredKeys))
	}
}

func TestRateLimiterFailsOpenWhenBackendFails(t *testing.T) {
	limiter := NewRateLimiterWithStore(RateLimitSettings{
		IPLimitPerMinute:       1,
		SendCodePhoneLimitHour: 1,
		LoginPhoneLimitHour:    1,
	}, failingRateLimitStore{})

	request := httptest.NewRequest("POST", "/auth/telegram/send-code", nil)
	decision := limiter.CheckSendCode(request, []byte("phone-hash"))
	if !decision.Allowed {
		t.Fatal("CheckSendCode() allowed = false, want fail-open")
	}
	if !errors.Is(decision.Err, errFakeRateLimitStore) {
		t.Fatalf("CheckSendCode() Err = %v, want errFakeRateLimitStore", decision.Err)
	}
}

type fakeValkeyCounter struct {
	counts      map[string]int64
	expiredKeys []string
}

func (c *fakeValkeyCounter) Incr(ctx context.Context, key string) (int64, error) {
	if c.counts == nil {
		c.counts = make(map[string]int64)
	}
	c.counts[key]++
	return c.counts[key], nil
}

func (c *fakeValkeyCounter) Expire(ctx context.Context, key string, ttl time.Duration) error {
	c.expiredKeys = append(c.expiredKeys, key)
	return nil
}

var errFakeRateLimitStore = errors.New("rate limit store failed")

type failingRateLimitStore struct{}

func (failingRateLimitStore) Allow(ctx context.Context, key string, limit int, window time.Duration, now time.Time) (RateLimitDecision, error) {
	return RateLimitDecision{}, errFakeRateLimitStore
}

package auth

import (
	"context"
	"encoding/hex"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type RateLimitSettings struct {
	IPLimitPerMinute       int
	SendCodePhoneLimitHour int
	LoginPhoneLimitHour    int
	ClientIP               func(*http.Request) string
}

type RateLimitDecision struct {
	Allowed    bool
	RetryAfter time.Duration
	Err        error
}

type RateLimitStore interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration, now time.Time) (RateLimitDecision, error)
}

type rateWindow struct {
	count     int
	resetAt   time.Time
	updatedAt time.Time
}

type RateLimiter struct {
	settings RateLimitSettings
	store    RateLimitStore
}

func NewRateLimiter(settings RateLimitSettings) *RateLimiter {
	return NewRateLimiterWithStore(settings, NewMemoryRateLimitStore())
}

func NewRateLimiterWithStore(settings RateLimitSettings, store RateLimitStore) *RateLimiter {
	if store == nil {
		store = NewMemoryRateLimitStore()
	}
	return &RateLimiter{
		settings: settings,
		store:    store,
	}
}

func (l *RateLimiter) CheckSendCode(r *http.Request, phoneHash []byte) RateLimitDecision {
	if l == nil {
		return allowDecision()
	}
	if decision := l.allow(r.Context(), "telegram_auth_ip:"+l.clientIP(r), l.settings.IPLimitPerMinute, time.Minute); !decision.Allowed || decision.Err != nil {
		return decision
	}
	return l.allow(r.Context(), "telegram_send_code_phone:"+hex.EncodeToString(phoneHash), l.settings.SendCodePhoneLimitHour, time.Hour)
}

func (l *RateLimiter) CheckLogin(r *http.Request, phoneHash []byte) RateLimitDecision {
	if l == nil {
		return allowDecision()
	}
	if decision := l.allow(r.Context(), "telegram_auth_ip:"+l.clientIP(r), l.settings.IPLimitPerMinute, time.Minute); !decision.Allowed || decision.Err != nil {
		return decision
	}
	return l.allow(r.Context(), "telegram_login_phone:"+hex.EncodeToString(phoneHash), l.settings.LoginPhoneLimitHour, time.Hour)
}

func (l *RateLimiter) CheckQRStart(r *http.Request) RateLimitDecision {
	if l == nil {
		return allowDecision()
	}
	return l.allow(r.Context(), "telegram_auth_ip:"+l.clientIP(r), l.settings.IPLimitPerMinute, time.Minute)
}

func (l *RateLimiter) allow(ctx context.Context, key string, limit int, window time.Duration) RateLimitDecision {
	if limit <= 0 {
		return allowDecision()
	}

	decision, err := l.store.Allow(ctx, key, limit, window, time.Now())
	if err != nil {
		decision = allowDecision()
		decision.Err = err
		return decision
	}
	return decision
}

type MemoryRateLimitStore struct {
	mu      sync.Mutex
	windows map[string]rateWindow
	now     func() time.Time
}

func NewMemoryRateLimitStore() *MemoryRateLimitStore {
	return &MemoryRateLimitStore{
		windows: make(map[string]rateWindow),
		now:     time.Now,
	}
}

func (s *MemoryRateLimitStore) Allow(ctx context.Context, key string, limit int, window time.Duration, now time.Time) (RateLimitDecision, error) {
	if s.now != nil {
		now = s.now()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked(now)
	current := s.windows[key]
	if current.resetAt.IsZero() || !current.resetAt.After(now) {
		current = rateWindow{resetAt: now.Add(window)}
	}
	current.updatedAt = now
	if current.count >= limit {
		retryAfter := current.resetAt.Sub(now)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		s.windows[key] = current
		return RateLimitDecision{Allowed: false, RetryAfter: retryAfter}, nil
	}

	current.count++
	s.windows[key] = current
	return allowDecision(), nil
}

func (s *MemoryRateLimitStore) pruneLocked(now time.Time) {
	for key, window := range s.windows {
		if window.resetAt.Before(now) {
			delete(s.windows, key)
		}
	}
}

type ValkeyCounter interface {
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
}

type ValkeyRateLimitStore struct {
	client ValkeyCounter
	prefix string
}

func NewValkeyRateLimitStore(client ValkeyCounter, prefix string) *ValkeyRateLimitStore {
	if prefix == "" {
		prefix = "t2d:rate_limit"
	}
	return &ValkeyRateLimitStore{
		client: client,
		prefix: prefix,
	}
}

func (s *ValkeyRateLimitStore) Allow(ctx context.Context, key string, limit int, window time.Duration, now time.Time) (RateLimitDecision, error) {
	if s.client == nil {
		return allowDecision(), nil
	}
	if limit <= 0 {
		return allowDecision(), nil
	}

	windowSeconds := int64((window + time.Second - 1) / time.Second)
	if windowSeconds < 1 {
		windowSeconds = 1
	}
	bucket := now.Unix() / windowSeconds
	resetAt := time.Unix((bucket+1)*windowSeconds, 0)
	storageKey := s.prefix + ":" + key + ":" + strconv.FormatInt(bucket, 10)

	count, err := s.client.Incr(ctx, storageKey)
	if err != nil {
		return RateLimitDecision{}, err
	}
	if count == 1 {
		if err := s.client.Expire(ctx, storageKey, window+time.Minute); err != nil {
			return RateLimitDecision{}, err
		}
	}
	if count > int64(limit) {
		retryAfter := resetAt.Sub(now)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return RateLimitDecision{Allowed: false, RetryAfter: retryAfter}, nil
	}

	return allowDecision(), nil
}

func allowDecision() RateLimitDecision {
	return RateLimitDecision{Allowed: true}
}

func (l *RateLimiter) clientIP(r *http.Request) string {
	if l != nil && l.settings.ClientIP != nil {
		return l.settings.ClientIP(r)
	}
	return clientIP(r)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}

func retryAfterSeconds(duration time.Duration) string {
	seconds := int64((duration + time.Second - 1) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	return strconv.FormatInt(seconds, 10)
}

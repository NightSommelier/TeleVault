package auth

import (
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
}

type RateLimitDecision struct {
	Allowed    bool
	RetryAfter time.Duration
}

type rateWindow struct {
	count     int
	resetAt   time.Time
	updatedAt time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	windows  map[string]rateWindow
	settings RateLimitSettings
	now      func() time.Time
}

func NewRateLimiter(settings RateLimitSettings) *RateLimiter {
	return &RateLimiter{
		windows:  make(map[string]rateWindow),
		settings: settings,
		now:      time.Now,
	}
}

func (l *RateLimiter) CheckSendCode(r *http.Request, phoneHash []byte) RateLimitDecision {
	if l == nil {
		return allowDecision()
	}
	if decision := l.allow("telegram_auth_ip:"+clientIP(r), l.settings.IPLimitPerMinute, time.Minute); !decision.Allowed {
		return decision
	}
	return l.allow("telegram_send_code_phone:"+hex.EncodeToString(phoneHash), l.settings.SendCodePhoneLimitHour, time.Hour)
}

func (l *RateLimiter) CheckLogin(r *http.Request, phoneHash []byte) RateLimitDecision {
	if l == nil {
		return allowDecision()
	}
	if decision := l.allow("telegram_auth_ip:"+clientIP(r), l.settings.IPLimitPerMinute, time.Minute); !decision.Allowed {
		return decision
	}
	return l.allow("telegram_login_phone:"+hex.EncodeToString(phoneHash), l.settings.LoginPhoneLimitHour, time.Hour)
}

func (l *RateLimiter) allow(key string, limit int, window time.Duration) RateLimitDecision {
	if limit <= 0 {
		return allowDecision()
	}

	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	l.pruneLocked(now)
	current := l.windows[key]
	if current.resetAt.IsZero() || !current.resetAt.After(now) {
		current = rateWindow{resetAt: now.Add(window)}
	}
	current.updatedAt = now
	if current.count >= limit {
		retryAfter := current.resetAt.Sub(now)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		l.windows[key] = current
		return RateLimitDecision{Allowed: false, RetryAfter: retryAfter}
	}

	current.count++
	l.windows[key] = current
	return allowDecision()
}

func (l *RateLimiter) pruneLocked(now time.Time) {
	for key, window := range l.windows {
		if window.resetAt.Before(now) {
			delete(l.windows, key)
		}
	}
}

func allowDecision() RateLimitDecision {
	return RateLimitDecision{Allowed: true}
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

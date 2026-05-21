package files

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/auth"
)

type PublicRateLimitSettings struct {
	Enabled             bool
	IPLimitPerMinute    int
	TokenLimitPerMinute int
	ClientIP            func(*http.Request) string
}

type PublicDownloadRateLimiter struct {
	settings PublicRateLimitSettings
	store    auth.RateLimitStore
}

func NewPublicDownloadRateLimiter(settings PublicRateLimitSettings, store auth.RateLimitStore) *PublicDownloadRateLimiter {
	if store == nil || !settings.Enabled {
		return nil
	}
	return &PublicDownloadRateLimiter{settings: settings, store: store}
}

func (l *PublicDownloadRateLimiter) Allow(r *http.Request, token string) auth.RateLimitDecision {
	if l == nil {
		return auth.RateLimitDecision{Allowed: true}
	}
	clientIP := fallbackClientIP(r, l.settings.ClientIP)
	if decision, err := l.store.Allow(r.Context(), "public_download_ip:"+clientIP, l.settings.IPLimitPerMinute, time.Minute, time.Now()); err != nil {
		return auth.RateLimitDecision{Allowed: true, Err: err}
	} else if !decision.Allowed {
		return decision
	}

	tokenHash := sha256.Sum256([]byte(token))
	key := "public_download_token:" + hex.EncodeToString(tokenHash[:8])
	decision, err := l.store.Allow(r.Context(), key, l.settings.TokenLimitPerMinute, time.Minute, time.Now())
	if err != nil {
		return auth.RateLimitDecision{Allowed: true, Err: err}
	}
	return decision
}

func applyRateLimitResponse(w http.ResponseWriter, decision auth.RateLimitDecision) {
	if decision.Allowed {
		return
	}
	w.Header().Set("Retry-After", retryAfterSeconds(decision.RetryAfter))
	writeError(w, http.StatusTooManyRequests, "public_download_rate_limited")
}

func retryAfterSeconds(duration time.Duration) string {
	seconds := int64((duration + time.Second - 1) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	return strconv.FormatInt(seconds, 10)
}

func fallbackClientIP(r *http.Request, resolve func(*http.Request) string) string {
	if resolve != nil {
		return resolve(r)
	}
	return requestClientIP(r)
}

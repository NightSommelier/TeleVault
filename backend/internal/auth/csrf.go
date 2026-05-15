package auth

import (
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/televault/TeleVault/backend/internal/config"
)

const (
	CSRFCookieName = "td_csrf"
	CSRFHeaderName = "X-CSRF-Token"
)

func SetCSRFCookie(w http.ResponseWriter, cfg config.Config, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: false,
		Secure:   cfg.SecureCookie,
		SameSite: sameSite(cfg.CookieSameSite),
	})
}

func ClearCSRFCookie(w http.ResponseWriter, cfg config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: false,
		Secure:   cfg.SecureCookie,
		SameSite: sameSite(cfg.CookieSameSite),
	})
}

func (h *Handler) RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !usesCookieAuth(r) {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(CSRFCookieName)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusForbidden, "missing_csrf_token")
			return
		}

		header := r.Header.Get(CSRFHeaderName)
		if header == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
			writeError(w, http.StatusForbidden, "invalid_csrf_token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func usesCookieAuth(r *http.Request) bool {
	_, err := r.Cookie(RefreshCookieName)
	return err == nil
}

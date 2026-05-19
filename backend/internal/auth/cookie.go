package auth

import (
	"net/http"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/config"
)

const RefreshCookieName = "td_refresh"

func SetRefreshCookie(w http.ResponseWriter, cfg config.Config, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   cfg.SecureCookie,
		SameSite: sameSite(cfg.CookieSameSite),
	})
}

func ClearRefreshCookie(w http.ResponseWriter, cfg config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.SecureCookie,
		SameSite: sameSite(cfg.CookieSameSite),
	})
}

func sameSite(value string) http.SameSite {
	switch value {
	case "Strict":
		return http.SameSiteStrictMode
	default:
		return http.SameSiteLaxMode
	}
}

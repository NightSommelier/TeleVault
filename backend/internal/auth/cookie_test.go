package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/config"
)

func TestSetRefreshCookieUsesSecureDefaults(t *testing.T) {
	recorder := httptest.NewRecorder()
	cfg := config.Config{
		SecureCookie:   true,
		CookieSameSite: "Strict",
	}
	expiresAt := time.Now().Add(time.Hour)

	SetRefreshCookie(recorder, cfg, "token", expiresAt)

	cookie := firstCookie(t, recorder.Result())
	if cookie.Name != RefreshCookieName {
		t.Fatalf("cookie name = %q, want %q", cookie.Name, RefreshCookieName)
	}
	if cookie.Value != "token" {
		t.Fatalf("cookie value = %q, want token", cookie.Value)
	}
	if !cookie.HttpOnly {
		t.Fatal("cookie HttpOnly = false, want true")
	}
	if !cookie.Secure {
		t.Fatal("cookie Secure = false, want true")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie SameSite = %v, want Strict", cookie.SameSite)
	}
}

func testCookieExpiry() time.Time {
	return time.Now().Add(time.Hour)
}

func TestClearRefreshCookieExpiresCookie(t *testing.T) {
	recorder := httptest.NewRecorder()
	cfg := config.Config{CookieSameSite: "Lax"}

	ClearRefreshCookie(recorder, cfg)

	cookie := firstCookie(t, recorder.Result())
	if cookie.Value != "" {
		t.Fatalf("cookie value = %q, want empty", cookie.Value)
	}
	if cookie.MaxAge != -1 {
		t.Fatalf("cookie MaxAge = %d, want -1", cookie.MaxAge)
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie SameSite = %v, want Lax", cookie.SameSite)
	}
}

func firstCookie(t *testing.T, response *http.Response) *http.Cookie {
	t.Helper()

	cookies := response.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies length = %d, want 1", len(cookies))
	}
	return cookies[0]
}

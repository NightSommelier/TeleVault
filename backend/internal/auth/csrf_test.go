package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/config"
)

func TestSetCSRFCookieIsReadableByClient(t *testing.T) {
	recorder := httptest.NewRecorder()
	cfg := config.Config{CookieSameSite: "Lax"}

	SetCSRFCookie(recorder, cfg, "csrf-token", testCookieExpiry())

	cookie := firstCookie(t, recorder.Result())
	if cookie.Name != CSRFCookieName {
		t.Fatalf("cookie name = %q, want %q", cookie.Name, CSRFCookieName)
	}
	if cookie.HttpOnly {
		t.Fatal("csrf cookie HttpOnly = true, want false")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie SameSite = %v, want Lax", cookie.SameSite)
	}
}

func TestRequireCSRFSkipsRequestsWithoutRefreshCookie(t *testing.T) {
	handler := (&Handler{}).RequireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/auth/telegram/login", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestRequireCSRFRejectsMissingTokenForCookieAuth(t *testing.T) {
	handler := (&Handler{}).RequireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: "refresh"})

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestRequireCSRFAcceptsMatchingDoubleSubmitToken(t *testing.T) {
	handler := (&Handler{}).RequireCSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: "refresh"})
	request.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "csrf"})
	request.Header.Set(CSRFHeaderName, "csrf")

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

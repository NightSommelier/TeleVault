package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserContextRoundTrip(t *testing.T) {
	user := User{
		ID:         "user-1",
		TelegramID: 123,
		Role:       "user",
	}

	ctx := withUser(context.Background(), user)
	got, ok := UserFromContext(ctx)
	if !ok {
		t.Fatal("UserFromContext() ok = false, want true")
	}
	if got.ID != user.ID || got.TelegramID != user.TelegramID || got.Role != user.Role {
		t.Fatalf("UserFromContext() = %#v, want %#v", got, user)
	}
}

func TestUserFromContextMissing(t *testing.T) {
	if _, ok := UserFromContext(context.Background()); ok {
		t.Fatal("UserFromContext() ok = true, want false")
	}
}

func TestRequireAdminRejectsNonAdminUser(t *testing.T) {
	handler := (&Handler{}).RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	request = request.WithContext(withUser(request.Context(), User{ID: "user-1", Role: "user"}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestRequireAdminAcceptsAdminUser(t *testing.T) {
	handler := (&Handler{}).RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	request = request.WithContext(withUser(request.Context(), User{ID: "admin-1", Role: "admin"}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

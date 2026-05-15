package auth

import (
	"context"
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

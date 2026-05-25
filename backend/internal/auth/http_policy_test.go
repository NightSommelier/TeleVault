package auth

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

func TestResolveLoginPolicyFallsBackToCommunityWhenStoreUnavailable(t *testing.T) {
	handler := &Handler{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		store:  nil,
	}

	got := handler.resolveLoginPolicy(context.Background())
	want := CommunityLoginPolicy()
	if got != want {
		t.Fatalf("resolveLoginPolicy() = %+v, want %+v", got, want)
	}
}

func TestResolveLoginPolicyFallsBackToCommunityWhenDBUnavailable(t *testing.T) {
	handler := &Handler{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		store:  &SessionStore{},
	}

	got := handler.resolveLoginPolicy(context.Background())
	want := CommunityLoginPolicy()
	if got != want {
		t.Fatalf("resolveLoginPolicy() = %+v, want %+v", got, want)
	}
}

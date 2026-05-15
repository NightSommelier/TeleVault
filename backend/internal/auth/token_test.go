package auth

import (
	"bytes"
	"testing"
)

func TestNewRefreshTokenGeneratesURLSafeToken(t *testing.T) {
	token, err := NewRefreshToken()
	if err != nil {
		t.Fatalf("NewRefreshToken() error = %v", err)
	}
	if token == "" {
		t.Fatal("NewRefreshToken() returned empty token")
	}
}

func TestHashRefreshTokenIsStableAndPeppered(t *testing.T) {
	token := "refresh-token"

	hashA, err := HashRefreshToken(token, "pepper-a")
	if err != nil {
		t.Fatalf("HashRefreshToken() error = %v", err)
	}

	hashAAgain, err := HashRefreshToken(token, "pepper-a")
	if err != nil {
		t.Fatalf("HashRefreshToken() error = %v", err)
	}

	hashB, err := HashRefreshToken(token, "pepper-b")
	if err != nil {
		t.Fatalf("HashRefreshToken() error = %v", err)
	}

	if !bytes.Equal(hashA, hashAAgain) {
		t.Fatal("HashRefreshToken() is not stable for the same token and pepper")
	}
	if bytes.Equal(hashA, hashB) {
		t.Fatal("HashRefreshToken() ignored pepper")
	}
}

func TestHashRefreshTokenRejectsMissingInputs(t *testing.T) {
	if _, err := HashRefreshToken("", "pepper"); err == nil {
		t.Fatal("HashRefreshToken() error = nil, want token error")
	}
	if _, err := HashRefreshToken("token", ""); err == nil {
		t.Fatal("HashRefreshToken() error = nil, want pepper error")
	}
}

package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

const refreshTokenBytes = 32

func NewRefreshToken() (string, error) {
	raw := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func HashRefreshToken(token string, pepper string) ([]byte, error) {
	if token == "" {
		return nil, errors.New("refresh token is required")
	}
	if pepper == "" {
		return nil, errors.New("refresh token pepper is required")
	}

	mac := hmac.New(sha256.New, []byte(pepper))
	_, _ = mac.Write([]byte(token))
	return mac.Sum(nil), nil
}

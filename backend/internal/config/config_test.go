package config

import (
	"strings"
	"testing"
)

const (
	validDatabaseURL        = "postgres://televault:televault@localhost:5432/televault2?sslmode=disable"
	validSessionSecret      = "session-secret-with-at-least-32-bytes"
	validRefreshTokenPepper = "refresh-pepper-with-at-least-32-bytes"
	validAgeIdentity        = "AGE-SECRET-KEY-1234567890ABCDEFGHIJKLMNOPQRSTUV"
	validTelegramSessionKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
)

func TestValidateAcceptsDevelopmentConfig(t *testing.T) {
	cfg := validConfig()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRequiresSecrets(t *testing.T) {
	cfg := validConfig()
	cfg.AppSessionSecret = ""
	cfg.RefreshTokenPepper = "short"
	cfg.AppAgeIdentity = ""
	cfg.TelegramSessionKey = "not-base64"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want required secret errors")
	}

	assertErrorContains(t, err, "APP_SESSION_SECRET is required")
	assertErrorContains(t, err, "REFRESH_TOKEN_PEPPER must be at least 32 characters")
	assertErrorContains(t, err, "APP_AGE_IDENTITY is required")
	assertErrorContains(t, err, "TELEGRAM_SESSION_KEY must be standard base64")
}

func TestValidateRejectsWildcardCORSWithCredentials(t *testing.T) {
	cfg := validConfig()
	cfg.CredentialsCORSMode = true
	cfg.CORSAllowedOrigins = []string{"*"}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want wildcard CORS error")
	}

	assertErrorContains(t, err, "CORS_ALLOWED_ORIGINS cannot contain * when credentials are enabled")
}

func TestValidateRequiresProductionSecureCookieAndCORS(t *testing.T) {
	cfg := validConfig()
	cfg.Env = EnvProduction
	cfg.SecureCookie = false
	cfg.CORSAllowedOrigins = nil

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want production hardening errors")
	}

	assertErrorContains(t, err, "SECURE_COOKIE must be true in production")
	assertErrorContains(t, err, "CORS_ALLOWED_ORIGINS is required in production")
}

func TestValidateRejectsInvalidAgeIdentityShape(t *testing.T) {
	cfg := validConfig()
	cfg.AppAgeIdentity = "not-an-age-identity"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want age identity error")
	}

	assertErrorContains(t, err, "APP_AGE_IDENTITY must look like an age secret identity")
}

func TestDatabaseConfigRequiresDatabaseURL(t *testing.T) {
	err := (DatabaseConfig{}).Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want DATABASE_URL error")
	}

	assertErrorContains(t, err, "DATABASE_URL is required")
}

func validConfig() Config {
	return Config{
		Env:                 EnvDevelopment,
		HTTPAddr:            ":8080",
		DatabaseURL:         validDatabaseURL,
		ValkeyAddr:          "localhost:6379",
		AppSessionSecret:    validSessionSecret,
		RefreshTokenPepper:  validRefreshTokenPepper,
		AppAgeIdentity:      validAgeIdentity,
		TelegramSessionKey:  validTelegramSessionKey,
		TelegramAPIID:       "12345",
		TelegramAPIHash:     "telegram-api-hash",
		CORSAllowedOrigins:  []string{"http://localhost:3000"},
		SecureCookie:        false,
		CookieSameSite:      "Lax",
		CredentialsCORSMode: true,
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()

	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err.Error(), want)
	}
}

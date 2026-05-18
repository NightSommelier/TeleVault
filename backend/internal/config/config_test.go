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

func TestValidateRejectsUnsafeUploadLimits(t *testing.T) {
	cfg := validConfig()
	cfg.UploadPartSizeBytes = 100
	cfg.TelegramDocumentLimitBytes = 128
	cfg.UploadSafetyMarginBytes = 64

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want upload limit error")
	}

	assertErrorContains(t, err, "UPLOAD_PART_SIZE_BYTES plus UPLOAD_SAFETY_MARGIN_BYTES must not exceed TELEGRAM_DOCUMENT_LIMIT_BYTES")
}

func TestValidateRejectsNonPositiveUploadLimits(t *testing.T) {
	cfg := validConfig()
	cfg.UploadPartSizeBytes = 0
	cfg.TelegramDocumentLimitBytes = -1
	cfg.UploadSafetyMarginBytes = -1

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want upload limit errors")
	}

	assertErrorContains(t, err, "UPLOAD_PART_SIZE_BYTES must be greater than 0")
	assertErrorContains(t, err, "TELEGRAM_DOCUMENT_LIMIT_BYTES must be greater than 0")
	assertErrorContains(t, err, "UPLOAD_SAFETY_MARGIN_BYTES must be greater than or equal to 0")
}

func TestValidateRequiresUploadStagingDir(t *testing.T) {
	cfg := validConfig()
	cfg.UploadStagingDir = " "

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want upload staging dir error")
	}

	assertErrorContains(t, err, "UPLOAD_STAGING_DIR is required")
}

func TestValidateRejectsInvalidEnabledRateLimits(t *testing.T) {
	cfg := validConfig()
	cfg.AuthRateLimitEnabled = true
	cfg.TelegramAuthIPLimitPerMinute = 0
	cfg.TelegramSendCodePhoneLimitPerHour = 0
	cfg.TelegramLoginPhoneLimitPerHour = 0

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want auth rate limit errors")
	}

	assertErrorContains(t, err, "TELEGRAM_AUTH_IP_LIMIT_PER_MINUTE must be greater than 0 when auth rate limiting is enabled")
	assertErrorContains(t, err, "TELEGRAM_SEND_CODE_PHONE_LIMIT_PER_HOUR must be greater than 0 when auth rate limiting is enabled")
	assertErrorContains(t, err, "TELEGRAM_LOGIN_PHONE_LIMIT_PER_HOUR must be greater than 0 when auth rate limiting is enabled")
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

		UploadPartSizeBytes:        DefaultUploadPartSizeBytes,
		TelegramDocumentLimitBytes: DefaultTelegramDocumentLimitBytes,
		UploadSafetyMarginBytes:    DefaultUploadSafetyMarginBytes,
		UploadStagingDir:           DefaultUploadStagingDir,

		AuthRateLimitEnabled:              true,
		TelegramAuthIPLimitPerMinute:      DefaultTelegramAuthIPLimitPerMinute,
		TelegramSendCodePhoneLimitPerHour: DefaultTelegramSendCodePhoneLimitPerHour,
		TelegramLoginPhoneLimitPerHour:    DefaultTelegramLoginPhoneLimitPerHour,
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()

	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err.Error(), want)
	}
}

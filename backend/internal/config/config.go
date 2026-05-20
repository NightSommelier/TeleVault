package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	EnvDevelopment = "development"
	EnvProduction  = "production"

	DefaultUploadPartSizeBytes        int64 = 384 * 1024 * 1024
	DefaultTelegramDocumentLimitBytes int64 = 2 * 1024 * 1024 * 1024
	DefaultUploadSafetyMarginBytes    int64 = 64 * 1024 * 1024
	DefaultUploadStagingDir                 = "var/upload-staging"

	DefaultTelegramAuthIPLimitPerMinute      = 30
	DefaultTelegramSendCodePhoneLimitPerHour = 5
	DefaultTelegramLoginPhoneLimitPerHour    = 10
)

type Config struct {
	Env                 string
	AppDebug            bool
	LogLevel            string
	HTTPAddr            string
	DatabaseURL         string
	ValkeyAddr          string
	AppSessionSecret    string
	RefreshTokenPepper  string
	AppAgeIdentity      string
	TelegramSessionKey  string
	TelegramAPIID       string
	TelegramAPIHash     string
	CORSAllowedOrigins  []string
	SecureCookie        bool
	CookieSameSite      string
	CredentialsCORSMode bool
	ContainerRuntime    bool

	UploadPartSizeBytes        int64
	TelegramDocumentLimitBytes int64
	UploadSafetyMarginBytes    int64
	UploadStagingDir           string

	AuthRateLimitEnabled              bool
	TelegramAuthIPLimitPerMinute      int
	TelegramSendCodePhoneLimitPerHour int
	TelegramLoginPhoneLimitPerHour    int
}

type DatabaseConfig struct {
	DatabaseURL string
}

func Load() (Config, error) {
	cfg := Config{
		Env:                getEnv("APP_ENV", EnvDevelopment),
		AppDebug:           parseBoolDefault(os.Getenv("APP_DEBUG"), false),
		HTTPAddr:           getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		ValkeyAddr:         getEnv("VALKEY_ADDR", getEnv("REDIS_ADDR", "localhost:6379")),
		AppSessionSecret:   os.Getenv("APP_SESSION_SECRET"),
		RefreshTokenPepper: os.Getenv("REFRESH_TOKEN_PEPPER"),
		AppAgeIdentity:     os.Getenv("APP_AGE_IDENTITY"),
		TelegramSessionKey: os.Getenv("TELEGRAM_SESSION_KEY"),
		TelegramAPIID:      os.Getenv("TELEGRAM_API_ID"),
		TelegramAPIHash:    os.Getenv("TELEGRAM_API_HASH"),
		CookieSameSite:     getEnv("COOKIE_SAME_SITE", "Lax"),
		ContainerRuntime:   parseBoolDefault(os.Getenv("TELEVAULT_CONTAINER"), false),
		UploadStagingDir:   getEnv("UPLOAD_STAGING_DIR", DefaultUploadStagingDir),
	}
	cfg.LogLevel = strings.ToLower(strings.TrimSpace(getEnv("LOG_LEVEL", defaultLogLevel(cfg.AppDebug))))

	var err error
	if cfg.UploadPartSizeBytes, err = parseInt64Default(os.Getenv("UPLOAD_PART_SIZE_BYTES"), DefaultUploadPartSizeBytes); err != nil {
		return Config{}, fmt.Errorf("UPLOAD_PART_SIZE_BYTES must be an integer: %w", err)
	}
	if cfg.TelegramDocumentLimitBytes, err = parseInt64Default(os.Getenv("TELEGRAM_DOCUMENT_LIMIT_BYTES"), DefaultTelegramDocumentLimitBytes); err != nil {
		return Config{}, fmt.Errorf("TELEGRAM_DOCUMENT_LIMIT_BYTES must be an integer: %w", err)
	}
	if cfg.UploadSafetyMarginBytes, err = parseInt64Default(os.Getenv("UPLOAD_SAFETY_MARGIN_BYTES"), DefaultUploadSafetyMarginBytes); err != nil {
		return Config{}, fmt.Errorf("UPLOAD_SAFETY_MARGIN_BYTES must be an integer: %w", err)
	}
	if cfg.TelegramAuthIPLimitPerMinute, err = parseIntDefault(os.Getenv("TELEGRAM_AUTH_IP_LIMIT_PER_MINUTE"), DefaultTelegramAuthIPLimitPerMinute); err != nil {
		return Config{}, fmt.Errorf("TELEGRAM_AUTH_IP_LIMIT_PER_MINUTE must be an integer: %w", err)
	}
	if cfg.TelegramSendCodePhoneLimitPerHour, err = parseIntDefault(os.Getenv("TELEGRAM_SEND_CODE_PHONE_LIMIT_PER_HOUR"), DefaultTelegramSendCodePhoneLimitPerHour); err != nil {
		return Config{}, fmt.Errorf("TELEGRAM_SEND_CODE_PHONE_LIMIT_PER_HOUR must be an integer: %w", err)
	}
	if cfg.TelegramLoginPhoneLimitPerHour, err = parseIntDefault(os.Getenv("TELEGRAM_LOGIN_PHONE_LIMIT_PER_HOUR"), DefaultTelegramLoginPhoneLimitPerHour); err != nil {
		return Config{}, fmt.Errorf("TELEGRAM_LOGIN_PHONE_LIMIT_PER_HOUR must be an integer: %w", err)
	}

	cfg.CORSAllowedOrigins = splitCSV(os.Getenv("CORS_ALLOWED_ORIGINS"))
	cfg.SecureCookie = parseBoolDefault(os.Getenv("SECURE_COOKIE"), cfg.Env == EnvProduction)
	cfg.CredentialsCORSMode = parseBoolDefault(os.Getenv("CORS_ALLOW_CREDENTIALS"), true)
	cfg.AuthRateLimitEnabled = parseBoolDefault(os.Getenv("AUTH_RATE_LIMIT_ENABLED"), true)

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func LoadDatabase() (DatabaseConfig, error) {
	cfg := DatabaseConfig{
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}

	if err := cfg.Validate(); err != nil {
		return DatabaseConfig{}, err
	}

	return cfg, nil
}

func (cfg DatabaseConfig) Validate() error {
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if _, err := url.Parse(cfg.DatabaseURL); err != nil {
		return errors.New("DATABASE_URL must be a valid URL")
	}
	return nil
}

func (cfg Config) Validate() error {
	var problems []string

	switch cfg.Env {
	case EnvDevelopment, EnvProduction:
	default:
		problems = append(problems, "APP_ENV must be development or production")
	}

	if cfg.HTTPAddr == "" {
		problems = append(problems, "HTTP_ADDR is required")
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "warning", "error":
	default:
		problems = append(problems, "LOG_LEVEL must be debug, info, warn, or error")
	}

	if cfg.DatabaseURL == "" {
		problems = append(problems, "DATABASE_URL is required")
	} else if _, err := url.Parse(cfg.DatabaseURL); err != nil {
		problems = append(problems, "DATABASE_URL must be a valid URL")
	}

	requireSecret(&problems, "APP_SESSION_SECRET", cfg.AppSessionSecret, 32)
	requireSecret(&problems, "REFRESH_TOKEN_PEPPER", cfg.RefreshTokenPepper, 32)
	requireBase64Secret(&problems, "TELEGRAM_SESSION_KEY", cfg.TelegramSessionKey, 32)

	if cfg.AppAgeIdentity == "" {
		problems = append(problems, "APP_AGE_IDENTITY is required")
	} else if !strings.HasPrefix(cfg.AppAgeIdentity, "AGE-SECRET-KEY-") {
		problems = append(problems, "APP_AGE_IDENTITY must look like an age secret identity")
	}

	if cfg.TelegramAPIID == "" {
		problems = append(problems, "TELEGRAM_API_ID is required")
	} else if _, err := strconv.Atoi(cfg.TelegramAPIID); err != nil {
		problems = append(problems, "TELEGRAM_API_ID must be an integer")
	}
	if cfg.TelegramAPIHash == "" {
		problems = append(problems, "TELEGRAM_API_HASH is required")
	}

	if cfg.CookieSameSite != "Lax" && cfg.CookieSameSite != "Strict" {
		problems = append(problems, "COOKIE_SAME_SITE must be Lax or Strict")
	}

	if cfg.UploadPartSizeBytes <= 0 {
		problems = append(problems, "UPLOAD_PART_SIZE_BYTES must be greater than 0")
	}
	if cfg.TelegramDocumentLimitBytes <= 0 {
		problems = append(problems, "TELEGRAM_DOCUMENT_LIMIT_BYTES must be greater than 0")
	}
	if cfg.UploadSafetyMarginBytes < 0 {
		problems = append(problems, "UPLOAD_SAFETY_MARGIN_BYTES must be greater than or equal to 0")
	}
	if strings.TrimSpace(cfg.UploadStagingDir) == "" {
		problems = append(problems, "UPLOAD_STAGING_DIR is required")
	}
	if cfg.ContainerRuntime && !filepath.IsAbs(cfg.UploadStagingDir) {
		problems = append(problems, "UPLOAD_STAGING_DIR must be an absolute shared volume path in container runtime, for example /data/upload-staging")
	}
	if cfg.UploadPartSizeBytes > 0 && cfg.TelegramDocumentLimitBytes > 0 && cfg.UploadSafetyMarginBytes >= 0 {
		if cfg.UploadPartSizeBytes > cfg.TelegramDocumentLimitBytes-cfg.UploadSafetyMarginBytes {
			problems = append(problems, "UPLOAD_PART_SIZE_BYTES plus UPLOAD_SAFETY_MARGIN_BYTES must not exceed TELEGRAM_DOCUMENT_LIMIT_BYTES")
		}
	}
	if cfg.AuthRateLimitEnabled {
		if cfg.TelegramAuthIPLimitPerMinute <= 0 {
			problems = append(problems, "TELEGRAM_AUTH_IP_LIMIT_PER_MINUTE must be greater than 0 when auth rate limiting is enabled")
		}
		if cfg.TelegramSendCodePhoneLimitPerHour <= 0 {
			problems = append(problems, "TELEGRAM_SEND_CODE_PHONE_LIMIT_PER_HOUR must be greater than 0 when auth rate limiting is enabled")
		}
		if cfg.TelegramLoginPhoneLimitPerHour <= 0 {
			problems = append(problems, "TELEGRAM_LOGIN_PHONE_LIMIT_PER_HOUR must be greater than 0 when auth rate limiting is enabled")
		}
	}

	if cfg.Env == EnvProduction {
		if !cfg.SecureCookie {
			problems = append(problems, "SECURE_COOKIE must be true in production")
		}
		if len(cfg.CORSAllowedOrigins) == 0 {
			problems = append(problems, "CORS_ALLOWED_ORIGINS is required in production")
		}
	}

	if cfg.CredentialsCORSMode {
		for _, origin := range cfg.CORSAllowedOrigins {
			if origin == "*" {
				problems = append(problems, "CORS_ALLOWED_ORIGINS cannot contain * when credentials are enabled")
				break
			}
		}
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}

	return nil
}

func (cfg Config) TelegramAppID() (int, error) {
	return strconv.Atoi(cfg.TelegramAPIID)
}

func getEnv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func defaultLogLevel(debug bool) string {
	if debug {
		return "debug"
	}
	return "info"
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseBoolDefault(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "y":
		return true
	case "false", "0", "no", "n":
		return false
	default:
		return fallback
	}
}

func parseInt64Default(value string, fallback int64) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	return strconv.ParseInt(value, 10, 64)
}

func parseIntDefault(value string, fallback int) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func requireSecret(problems *[]string, name string, value string, minLen int) {
	if value == "" {
		*problems = append(*problems, fmt.Sprintf("%s is required", name))
		return
	}
	if len(value) < minLen {
		*problems = append(*problems, fmt.Sprintf("%s must be at least %d characters", name, minLen))
	}
}

func requireBase64Secret(problems *[]string, name string, value string, decodedLen int) {
	if value == "" {
		*problems = append(*problems, fmt.Sprintf("%s is required", name))
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		*problems = append(*problems, fmt.Sprintf("%s must be standard base64", name))
		return
	}
	if len(decoded) != decodedLen {
		*problems = append(*problems, fmt.Sprintf("%s must decode to %d bytes", name, decodedLen))
	}
}

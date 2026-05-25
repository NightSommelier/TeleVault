package auth

import (
	"database/sql"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestTOTPSecretGenerateAndVerify(t *testing.T) {
	secret, err := NewTOTPSecret()
	if err != nil {
		t.Fatalf("NewTOTPSecret() error = %v", err)
	}
	if secret == "" {
		t.Fatal("NewTOTPSecret() returned empty secret")
	}

	code := totpCodeForTime(secret, time.Now().UTC())
	if !VerifyTOTPCode(secret, code, time.Now().UTC()) {
		t.Fatal("VerifyTOTPCode() = false, want true for current code")
	}
}

func TestVerifyTOTPRejectsInvalidCode(t *testing.T) {
	secret, err := NewTOTPSecret()
	if err != nil {
		t.Fatalf("NewTOTPSecret() error = %v", err)
	}
	if VerifyTOTPCode(secret, "000000", time.Now().UTC()) && totpCodeForTime(secret, time.Now().UTC()) != "000000" {
		t.Fatal("VerifyTOTPCode() accepted invalid code")
	}
}

func TestGenerateRecoveryCodes(t *testing.T) {
	codes, err := GenerateRecoveryCodes(10)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes() error = %v", err)
	}
	if len(codes) != 10 {
		t.Fatalf("len(codes) = %d, want 10", len(codes))
	}
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		if strings.Count(code, "-") != 2 {
			t.Fatalf("code %q has wrong format", code)
		}
		if _, ok := seen[code]; ok {
			t.Fatalf("duplicate recovery code: %s", code)
		}
		seen[code] = struct{}{}
	}
}

func TestVerifyTOTPRejectsOutOfWindowCode(t *testing.T) {
	secret, err := NewTOTPSecret()
	if err != nil {
		t.Fatalf("NewTOTPSecret() error = %v", err)
	}

	now := time.Unix(1_700_000_000, 0).UTC()
	oldCode := totpCodeForTime(secret, now.Add(-2*30*time.Second))
	if oldCode == "" {
		t.Fatal("totpCodeForTime() returned empty code")
	}
	if VerifyTOTPCode(secret, oldCode, now) {
		t.Fatal("VerifyTOTPCode() accepted out-of-window code")
	}
}

func TestGenerateRecoveryCodesRejectsNonPositiveCount(t *testing.T) {
	if _, err := GenerateRecoveryCodes(0); err == nil {
		t.Fatal("GenerateRecoveryCodes(0) error = nil, want error")
	}
	if _, err := GenerateRecoveryCodes(-1); err == nil {
		t.Fatal("GenerateRecoveryCodes(-1) error = nil, want error")
	}
}

func TestTOTPURIDefaults(t *testing.T) {
	uri := TOTPURI(" ", " ", "SECRET123")
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	if parsed.Scheme != "otpauth" {
		t.Fatalf("scheme = %q, want otpauth", parsed.Scheme)
	}
	if parsed.Host != "totp" {
		t.Fatalf("host = %q, want totp", parsed.Host)
	}
	if parsed.Path != "/TeleVault:user" {
		t.Fatalf("path = %q, want /TeleVault:user", parsed.Path)
	}
	query := parsed.Query()
	if query.Get("issuer") != "TeleVault" {
		t.Fatalf("issuer = %q, want TeleVault", query.Get("issuer"))
	}
	if query.Get("secret") != "SECRET123" {
		t.Fatalf("secret = %q, want SECRET123", query.Get("secret"))
	}
	if query.Get("digits") != "6" {
		t.Fatalf("digits = %q, want 6", query.Get("digits"))
	}
	if query.Get("period") != "30" {
		t.Fatalf("period = %q, want 30", query.Get("period"))
	}
}

func TestMFAUserLabelPriority(t *testing.T) {
	userWithUsername := User{
		TelegramID: 101,
		Username:   sql.NullString{String: "user_name", Valid: true},
		DisplayName: sql.NullString{
			String: "Display Name",
			Valid:  true,
		},
	}
	if got := mfaUserLabel(userWithUsername); got != "user_name" {
		t.Fatalf("mfaUserLabel() = %q, want %q", got, "user_name")
	}

	userWithDisplay := User{
		TelegramID: 202,
		Username:   sql.NullString{Valid: false},
		DisplayName: sql.NullString{
			String: "Display Name",
			Valid:  true,
		},
	}
	if got := mfaUserLabel(userWithDisplay); got != "Display Name" {
		t.Fatalf("mfaUserLabel() = %q, want %q", got, "Display Name")
	}

	userFallback := User{
		TelegramID: 303,
		Username:   sql.NullString{String: " ", Valid: true},
		DisplayName: sql.NullString{
			String: " ",
			Valid:  true,
		},
	}
	if got := mfaUserLabel(userFallback); got != "telegram-303" {
		t.Fatalf("mfaUserLabel() = %q, want %q", got, "telegram-303")
	}
}

func TestWebAuthnFromRequestRequiresHost(t *testing.T) {
	req := httptest.NewRequest("GET", "http://localhost/auth/mfa/status", nil)
	req.Host = ""

	if _, err := webAuthnFromRequest(req); err == nil {
		t.Fatal("webAuthnFromRequest() error = nil, want host error")
	}
}

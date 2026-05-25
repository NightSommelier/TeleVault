package auth

import (
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


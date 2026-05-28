package auth

import "testing"

func TestLocalPasswordHashAndVerify(t *testing.T) {
	password := "correct horse battery"
	hash, err := HashLocalPassword(password)
	if err != nil {
		t.Fatalf("HashLocalPassword() error = %v", err)
	}
	ok, err := VerifyLocalPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyLocalPassword() error = %v", err)
	}
	if !ok {
		t.Fatal("VerifyLocalPassword() = false, want true")
	}
	bad, err := VerifyLocalPassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("VerifyLocalPassword(wrong) error = %v", err)
	}
	if bad {
		t.Fatal("VerifyLocalPassword(wrong) = true, want false")
	}
}

func TestNormalizeLocalPassword(t *testing.T) {
	if _, err := NormalizeLocalPassword(" 1234 "); err == nil {
		t.Fatal("NormalizeLocalPassword(short) error = nil, want error")
	}
	value, err := NormalizeLocalPassword("  12345  ")
	if err != nil {
		t.Fatalf("NormalizeLocalPassword() error = %v", err)
	}
	if value != "12345" {
		t.Fatalf("NormalizeLocalPassword() = %q, want %q", value, "12345")
	}
}

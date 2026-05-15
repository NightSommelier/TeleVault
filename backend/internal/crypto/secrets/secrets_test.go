package secrets

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestParseBase64KeyAccepts32Bytes(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, KeyBytes))

	raw, err := ParseBase64Key(key)
	if err != nil {
		t.Fatalf("ParseBase64Key() error = %v", err)
	}
	if len(raw) != KeyBytes {
		t.Fatalf("key length = %d, want %d", len(raw), KeyBytes)
	}
}

func TestParseBase64KeyRejectsInvalidKeys(t *testing.T) {
	tests := []string{
		"",
		"not-base64",
		base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, KeyBytes-1)),
	}

	for _, tt := range tests {
		if _, err := ParseBase64Key(tt); err == nil {
			t.Fatalf("ParseBase64Key(%q) error = nil, want error", tt)
		}
	}
}

func TestBoxEncryptDecryptRoundTrip(t *testing.T) {
	box := newTestBox(t)
	plaintext := []byte("telegram-session-string")
	aad := []byte("user-id:123")

	ciphertext, err := box.Encrypt(plaintext, aad)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if bytes.Contains(ciphertext, plaintext) {
		t.Fatal("ciphertext contains plaintext")
	}

	got, err := box.Decrypt(ciphertext, aad)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("Decrypt() = %q, want %q", got, plaintext)
	}
}

func TestBoxEncryptUsesRandomNonce(t *testing.T) {
	box := newTestBox(t)
	plaintext := []byte("telegram-session-string")

	first, err := box.Encrypt(plaintext, nil)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	second, err := box.Encrypt(plaintext, nil)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	if bytes.Equal(first, second) {
		t.Fatal("Encrypt() returned identical ciphertexts for same plaintext")
	}
}

func TestBoxDecryptRejectsWrongAdditionalData(t *testing.T) {
	box := newTestBox(t)
	ciphertext, err := box.Encrypt([]byte("telegram-session-string"), []byte("user-id:123"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	if _, err := box.Decrypt(ciphertext, []byte("user-id:456")); err == nil {
		t.Fatal("Decrypt() error = nil, want authentication error")
	}
}

func newTestBox(t *testing.T) *Box {
	t.Helper()

	box, err := NewBox(bytes.Repeat([]byte{7}, KeyBytes))
	if err != nil {
		t.Fatalf("NewBox() error = %v", err)
	}
	return box
}

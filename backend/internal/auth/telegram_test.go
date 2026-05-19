package auth

import (
	"bytes"
	"testing"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/crypto/secrets"
)

func TestTelegramSessionCryptoRoundTrip(t *testing.T) {
	box, err := secrets.NewBox(bytes.Repeat([]byte{9}, secrets.KeyBytes))
	if err != nil {
		t.Fatalf("NewBox() error = %v", err)
	}

	crypto := NewTelegramSessionCrypto(box)
	encrypted, err := crypto.Encrypt("user-1", "telegram-session")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if bytes.Contains(encrypted, []byte("telegram-session")) {
		t.Fatal("encrypted session contains plaintext")
	}

	session, err := crypto.Decrypt("user-1", encrypted)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if session != "telegram-session" {
		t.Fatalf("Decrypt() = %q, want telegram-session", session)
	}
}

func TestTelegramSessionCryptoBindsCiphertextToUserID(t *testing.T) {
	box, err := secrets.NewBox(bytes.Repeat([]byte{9}, secrets.KeyBytes))
	if err != nil {
		t.Fatalf("NewBox() error = %v", err)
	}

	crypto := NewTelegramSessionCrypto(box)
	encrypted, err := crypto.Encrypt("user-1", "telegram-session")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	if _, err := crypto.Decrypt("user-2", encrypted); err == nil {
		t.Fatal("Decrypt() error = nil, want user binding authentication error")
	}
}

func TestHashPhoneIsStableAndPeppered(t *testing.T) {
	hashA, err := HashPhone("+15551234567", "pepper-a")
	if err != nil {
		t.Fatalf("HashPhone() error = %v", err)
	}
	hashAAgain, err := HashPhone("+15551234567", "pepper-a")
	if err != nil {
		t.Fatalf("HashPhone() error = %v", err)
	}
	hashB, err := HashPhone("+15551234567", "pepper-b")
	if err != nil {
		t.Fatalf("HashPhone() error = %v", err)
	}

	if !bytes.Equal(hashA, hashAAgain) {
		t.Fatal("HashPhone() is not stable for the same phone and pepper")
	}
	if bytes.Equal(hashA, hashB) {
		t.Fatal("HashPhone() ignored pepper")
	}
}

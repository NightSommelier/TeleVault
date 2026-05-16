package agefile

import (
	"bytes"
	"io"
	"testing"

	"filippo.io/age"
)

func TestEncryptStream(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("GenerateX25519Identity() error = %v", err)
	}

	plaintext := []byte("private file contents")
	var ciphertext bytes.Buffer
	result, err := EncryptStream(&ciphertext, bytes.NewReader(plaintext), identity.Recipient())
	if err != nil {
		t.Fatalf("EncryptStream() error = %v", err)
	}

	if result.PlaintextSize != int64(len(plaintext)) {
		t.Fatalf("PlaintextSize = %d, want %d", result.PlaintextSize, len(plaintext))
	}
	if result.CiphertextSize <= result.PlaintextSize {
		t.Fatalf("CiphertextSize = %d, want greater than plaintext", result.CiphertextSize)
	}
	if len(result.Checksum) != 32 {
		t.Fatalf("Checksum length = %d, want 32", len(result.Checksum))
	}

	reader, err := age.Decrypt(bytes.NewReader(ciphertext.Bytes()), identity)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	decrypted, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatal("decrypted plaintext mismatch")
	}
}

func TestRecipientFromIdentity(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("GenerateX25519Identity() error = %v", err)
	}

	recipient, err := RecipientFromIdentity(identity.String())
	if err != nil {
		t.Fatalf("RecipientFromIdentity() error = %v", err)
	}
	if recipient == nil {
		t.Fatal("recipient is nil")
	}
}

func TestDecryptStream(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("GenerateX25519Identity() error = %v", err)
	}

	plaintext := []byte("download me")
	var ciphertext bytes.Buffer
	if _, err := EncryptStream(&ciphertext, bytes.NewReader(plaintext), identity.Recipient()); err != nil {
		t.Fatalf("EncryptStream() error = %v", err)
	}

	var decrypted bytes.Buffer
	if err := DecryptStream(&decrypted, bytes.NewReader(ciphertext.Bytes()), identity); err != nil {
		t.Fatalf("DecryptStream() error = %v", err)
	}
	if !bytes.Equal(decrypted.Bytes(), plaintext) {
		t.Fatal("decrypted plaintext mismatch")
	}
}

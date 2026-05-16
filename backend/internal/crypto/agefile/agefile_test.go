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
	if len(result.HashState) == 0 {
		t.Fatal("HashState is empty")
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

func TestEncryptStreamWithHashContinuesSHA256State(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("GenerateX25519Identity() error = %v", err)
	}

	firstHash, err := NewSHA256FromState(nil)
	if err != nil {
		t.Fatalf("NewSHA256FromState(nil) error = %v", err)
	}
	var firstCiphertext bytes.Buffer
	first, err := EncryptStreamWithHash(&firstCiphertext, bytes.NewReader([]byte("hello ")), identity.Recipient(), firstHash)
	if err != nil {
		t.Fatalf("EncryptStreamWithHash(first) error = %v", err)
	}

	secondHash, err := NewSHA256FromState(first.HashState)
	if err != nil {
		t.Fatalf("NewSHA256FromState(first.HashState) error = %v", err)
	}
	var secondCiphertext bytes.Buffer
	second, err := EncryptStreamWithHash(&secondCiphertext, bytes.NewReader([]byte("world")), identity.Recipient(), secondHash)
	if err != nil {
		t.Fatalf("EncryptStreamWithHash(second) error = %v", err)
	}

	finalHash, err := NewSHA256FromState(second.HashState)
	if err != nil {
		t.Fatalf("NewSHA256FromState(second.HashState) error = %v", err)
	}
	got := finalHash.Sum(nil)

	wantHash, err := NewSHA256FromState(nil)
	if err != nil {
		t.Fatalf("NewSHA256FromState(nil) error = %v", err)
	}
	_, _ = wantHash.Write([]byte("hello world"))
	want := wantHash.Sum(nil)

	if !bytes.Equal(got, want) {
		t.Fatal("continued hash state did not match whole plaintext hash")
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

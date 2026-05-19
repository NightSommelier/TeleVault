package telegramartifact

import (
	"bytes"
	"io"
	"testing"

	"filippo.io/age"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/crypto/agefile"
)

func TestSpecForArtifactIDIsDeterministic(t *testing.T) {
	first := SpecForArtifactID("part-1")
	second := SpecForArtifactID("part-1")
	if first.ProfileIndex != second.ProfileIndex {
		t.Fatalf("ProfileIndex = %d and %d, want same", first.ProfileIndex, second.ProfileIndex)
	}
	if first.Name() != second.Name() || first.MIMEType() != second.MIMEType() {
		t.Fatalf("specs differ: %+v vs %+v", first, second)
	}
	if first.Name() == "part-1.bin" {
		t.Fatalf("Name() = %q, want decoy extension", first.Name())
	}
}

func TestWrapReaderAndUnwrapReaderRoundTrip(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("GenerateX25519Identity() error = %v", err)
	}
	plaintext := []byte("ciphertext payload")
	var ciphertext bytes.Buffer
	if _, err := agefile.EncryptStream(&ciphertext, bytes.NewReader(plaintext), identity.Recipient()); err != nil {
		t.Fatalf("EncryptStream() error = %v", err)
	}

	wrapped, err := io.ReadAll(WrapReader("part-1", bytes.NewReader(ciphertext.Bytes())))
	if err != nil {
		t.Fatalf("ReadAll(WrapReader()) error = %v", err)
	}
	if bytes.HasPrefix(wrapped, []byte(ageMagicPrefix)) {
		t.Fatal("wrapped bytes start with age header")
	}

	unwrap, err := UnwrapReader(bytes.NewReader(wrapped))
	if err != nil {
		t.Fatalf("UnwrapReader() error = %v", err)
	}
	roundTrip, err := io.ReadAll(unwrap)
	if err != nil {
		t.Fatalf("ReadAll(UnwrapReader()) error = %v", err)
	}
	if !bytes.Equal(roundTrip, ciphertext.Bytes()) {
		t.Fatal("unwrapped bytes did not match original ciphertext")
	}
}

func TestUnwrapReaderPassesThroughLegacyAgeCiphertext(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("GenerateX25519Identity() error = %v", err)
	}
	var ciphertext bytes.Buffer
	if _, err := agefile.EncryptStream(&ciphertext, bytes.NewReader([]byte("hello")), identity.Recipient()); err != nil {
		t.Fatalf("EncryptStream() error = %v", err)
	}

	unwrap, err := UnwrapReader(bytes.NewReader(ciphertext.Bytes()))
	if err != nil {
		t.Fatalf("UnwrapReader() error = %v", err)
	}
	roundTrip, err := io.ReadAll(unwrap)
	if err != nil {
		t.Fatalf("ReadAll(UnwrapReader()) error = %v", err)
	}
	if !bytes.Equal(roundTrip, ciphertext.Bytes()) {
		t.Fatal("legacy ciphertext changed during unwrap")
	}
}

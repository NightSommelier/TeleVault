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

func TestSpecForArtifactIDAndSizeUsesSizeBuckets(t *testing.T) {
	small := SpecForArtifactIDAndSize("part-small", 4*1024*1024)
	if ext := small.Profile.Extension; ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".webp" && ext != ".pdf" {
		t.Fatalf("small bucket extension = %q, want image/document decoy", ext)
	}

	medium := SpecForArtifactIDAndSize("part-medium", 32*1024*1024)
	if ext := medium.Profile.Extension; ext != ".mp3" && ext != ".m4a" && ext != ".docx" && ext != ".xlsx" && ext != ".pptx" && ext != ".zip" && ext != ".rar" {
		t.Fatalf("medium bucket extension = %q, want audio/document/archive decoy", ext)
	}

	large := SpecForArtifactIDAndSize("part-large", 256*1024*1024)
	if ext := large.Profile.Extension; ext != ".mp4" && ext != ".m4v" && ext != ".mkv" && ext != ".avi" && ext != ".3gp" && ext != ".7z" && ext != ".bin" {
		t.Fatalf("large bucket extension = %q, want video/archive/bin decoy", ext)
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
	if bytes.Contains(wrapped, []byte(ageMagicPrefix)) {
		t.Fatal("wrapped bytes contain age header")
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

func TestWrapReaderAndUnwrapReaderRoundTripAcrossSizeBuckets(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int64
	}{
		{name: "small", size: 4 * 1024 * 1024},
		{name: "medium", size: 32 * 1024 * 1024},
		{name: "large", size: 256 * 1024 * 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext := []byte("age-encrypted bytes")
			wrapped, err := io.ReadAll(WrapReaderForSize("part-"+tc.name, tc.size, bytes.NewReader(ciphertext)))
			if err != nil {
				t.Fatalf("ReadAll(WrapReaderForSize()) error = %v", err)
			}

			unwrap, err := UnwrapReader(bytes.NewReader(wrapped))
			if err != nil {
				t.Fatalf("UnwrapReader() error = %v", err)
			}
			roundTrip, err := io.ReadAll(unwrap)
			if err != nil {
				t.Fatalf("ReadAll(UnwrapReader()) error = %v", err)
			}
			if !bytes.Equal(roundTrip, ciphertext) {
				t.Fatal("unwrapped bytes did not match original ciphertext")
			}
		})
	}
}

func TestUnwrapReaderPassesThroughLegacyWrappedCiphertext(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("GenerateX25519Identity() error = %v", err)
	}
	var ciphertext bytes.Buffer
	if _, err := agefile.EncryptStream(&ciphertext, bytes.NewReader([]byte("legacy wrapped")), identity.Recipient()); err != nil {
		t.Fatalf("EncryptStream() error = %v", err)
	}

	spec := SpecForArtifactID("part-1")
	legacy := bytes.NewBuffer(nil)
	legacy.Write(spec.Profile.Prefix)
	legacy.Write(wrapperHeader(spec.ProfileIndex, wrapperVersionPlain))
	legacy.Write(ciphertext.Bytes())

	unwrap, err := UnwrapReader(bytes.NewReader(legacy.Bytes()))
	if err != nil {
		t.Fatalf("UnwrapReader() error = %v", err)
	}
	roundTrip, err := io.ReadAll(unwrap)
	if err != nil {
		t.Fatalf("ReadAll(UnwrapReader()) error = %v", err)
	}
	if !bytes.Equal(roundTrip, ciphertext.Bytes()) {
		t.Fatal("legacy wrapped ciphertext changed during unwrap")
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

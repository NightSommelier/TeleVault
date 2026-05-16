package agefile

import (
	"crypto/sha256"
	"encoding"
	"errors"
	"hash"
	"io"

	"filippo.io/age"
)

type EncryptResult struct {
	PlaintextSize  int64
	CiphertextSize int64
	Checksum       []byte
	HashState      []byte
}

func RecipientFromIdentity(identity string) (age.Recipient, error) {
	parsed, err := age.ParseX25519Identity(identity)
	if err != nil {
		return nil, err
	}
	return parsed.Recipient(), nil
}

func IdentityFromString(identity string) (age.Identity, error) {
	return age.ParseX25519Identity(identity)
}

func EncryptStream(dst io.Writer, src io.Reader, recipient age.Recipient) (EncryptResult, error) {
	return EncryptStreamWithHash(dst, src, recipient, sha256.New())
}

func EncryptStreamWithHash(dst io.Writer, src io.Reader, recipient age.Recipient, plaintextHash hash.Hash) (EncryptResult, error) {
	if recipient == nil {
		return EncryptResult{}, errors.New("age recipient is required")
	}
	if plaintextHash == nil {
		return EncryptResult{}, errors.New("plaintext hash is required")
	}

	counter := &countingWriter{writer: dst}
	encrypted, err := age.Encrypt(counter, recipient)
	if err != nil {
		return EncryptResult{}, err
	}

	partHash := sha256.New()
	hashWriter := io.MultiWriter(partHash, plaintextHash)
	plaintextCounter := &countingWriter{writer: hashWriter}
	plaintextReader := io.TeeReader(src, plaintextCounter)

	if _, err := io.Copy(encrypted, plaintextReader); err != nil {
		_ = encrypted.Close()
		return EncryptResult{}, err
	}
	if err := encrypted.Close(); err != nil {
		return EncryptResult{}, err
	}

	state, err := marshalHashState(plaintextHash)
	if err != nil {
		return EncryptResult{}, err
	}

	return EncryptResult{
		PlaintextSize:  plaintextCounter.n,
		CiphertextSize: counter.n,
		Checksum:       partHash.Sum(nil),
		HashState:      state,
	}, nil
}

func NewSHA256FromState(state []byte) (hash.Hash, error) {
	plaintextHash := sha256.New()
	if len(state) == 0 {
		return plaintextHash, nil
	}

	unmarshaler, ok := plaintextHash.(encoding.BinaryUnmarshaler)
	if !ok {
		return nil, errors.New("sha256 state cannot be unmarshaled")
	}
	if err := unmarshaler.UnmarshalBinary(state); err != nil {
		return nil, err
	}
	return plaintextHash, nil
}

func marshalHashState(plaintextHash hash.Hash) ([]byte, error) {
	marshaler, ok := plaintextHash.(encoding.BinaryMarshaler)
	if !ok {
		return nil, errors.New("sha256 state cannot be marshaled")
	}
	return marshaler.MarshalBinary()
}

func DecryptStream(dst io.Writer, src io.Reader, identity age.Identity) error {
	if identity == nil {
		return errors.New("age identity is required")
	}

	plaintext, err := age.Decrypt(src, identity)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, plaintext)
	return err
}

type countingWriter struct {
	writer io.Writer
	n      int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	w.n += int64(n)
	return n, err
}

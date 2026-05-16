package agefile

import (
	"crypto/sha256"
	"errors"
	"io"

	"filippo.io/age"
)

type EncryptResult struct {
	PlaintextSize  int64
	CiphertextSize int64
	Checksum       []byte
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
	if recipient == nil {
		return EncryptResult{}, errors.New("age recipient is required")
	}

	counter := &countingWriter{writer: dst}
	encrypted, err := age.Encrypt(counter, recipient)
	if err != nil {
		return EncryptResult{}, err
	}

	hash := sha256.New()
	plaintextCounter := &countingWriter{writer: hash}
	plaintextReader := io.TeeReader(src, plaintextCounter)

	if _, err := io.Copy(encrypted, plaintextReader); err != nil {
		_ = encrypted.Close()
		return EncryptResult{}, err
	}
	if err := encrypted.Close(); err != nil {
		return EncryptResult{}, err
	}

	return EncryptResult{
		PlaintextSize:  plaintextCounter.n,
		CiphertextSize: counter.n,
		Checksum:       hash.Sum(nil),
	}, nil
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

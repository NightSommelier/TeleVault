package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

const KeyBytes = 32

type Box struct {
	aead cipher.AEAD
}

func NewBox(rawKey []byte) (*Box, error) {
	if len(rawKey) != KeyBytes {
		return nil, fmt.Errorf("secret key must be %d bytes", KeyBytes)
	}

	block, err := aes.NewCipher(rawKey)
	if err != nil {
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &Box{aead: aead}, nil
}

func ParseBase64Key(encoded string) ([]byte, error) {
	if encoded == "" {
		return nil, errors.New("secret key is required")
	}

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	if len(raw) != KeyBytes {
		return nil, fmt.Errorf("secret key must decode to %d bytes", KeyBytes)
	}

	return raw, nil
}

func (b *Box) Encrypt(plaintext []byte, additionalData []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := b.aead.Seal(nil, nonce, plaintext, additionalData)
	out := make([]byte, 0, len(nonce)+len(ciphertext))
	out = append(out, nonce...)
	out = append(out, ciphertext...)

	return out, nil
}

func (b *Box) Decrypt(ciphertext []byte, additionalData []byte) ([]byte, error) {
	nonceSize := b.aead.NonceSize()
	if len(ciphertext) <= nonceSize {
		return nil, errors.New("ciphertext is too short")
	}

	nonce := ciphertext[:nonceSize]
	body := ciphertext[nonceSize:]

	return b.aead.Open(nil, nonce, body, additionalData)
}

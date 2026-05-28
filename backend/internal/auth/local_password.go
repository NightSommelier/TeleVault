package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	localPasswordArgonTime      uint32 = 3
	localPasswordArgonMemoryKiB uint32 = 64 * 1024
	localPasswordArgonThreads   uint8  = 1
	localPasswordSaltBytes             = 16
	localPasswordHashBytes             = 32
	localPasswordMinLength             = 5
	localPasswordMaxLength             = 256
)

func NormalizeLocalPassword(value string) (string, error) {
	clean := strings.TrimSpace(value)
	if len(clean) < localPasswordMinLength {
		return "", errors.New("password is too short")
	}
	if len(clean) > localPasswordMaxLength {
		return "", errors.New("password is too long")
	}
	return clean, nil
}

func HashLocalPassword(password string) (string, error) {
	salt := make([]byte, localPasswordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := argon2.IDKey([]byte(password), salt, localPasswordArgonTime, localPasswordArgonMemoryKiB, localPasswordArgonThreads, localPasswordHashBytes)
	return fmt.Sprintf(
		"argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		localPasswordArgonMemoryKiB,
		localPasswordArgonTime,
		localPasswordArgonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum),
	), nil
}

func VerifyLocalPassword(password string, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 {
		return false, errors.New("invalid password hash")
	}
	if parts[0] != "argon2id" || parts[1] != "v=19" {
		return false, errors.New("unsupported password hash")
	}

	var memory uint32
	var timeCost uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[2], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return false, errors.New("invalid password hash params")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, errors.New("invalid password hash salt")
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, errors.New("invalid password hash digest")
	}
	if len(want) == 0 {
		return false, errors.New("invalid password hash digest")
	}

	got := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return false, nil
	}
	return true, nil
}

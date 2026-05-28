package auth

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"strings"
	"time"
)

const (
	RememberCookieName = "td_remember"
	rememberTokenTTL   = 30 * 24 * time.Hour
)

var ErrRememberedDeviceInvalid = errors.New("remembered device invalid")

type RememberedDevice struct {
	ID           string
	User         User
	VerifierHash []byte
	ExpiresAt    time.Time
	RevokedAt    sql.NullTime
}

type RememberToken struct {
	Selector string
	Verifier string
}

func NewRememberToken() (RememberToken, error) {
	selector, err := NewRefreshToken()
	if err != nil {
		return RememberToken{}, err
	}
	verifier, err := NewRefreshToken()
	if err != nil {
		return RememberToken{}, err
	}
	return RememberToken{Selector: selector, Verifier: verifier}, nil
}

func ParseRememberToken(raw string) (RememberToken, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return RememberToken{}, ErrRememberedDeviceInvalid
	}
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return RememberToken{}, ErrRememberedDeviceInvalid
	}
	return RememberToken{Selector: parts[0], Verifier: parts[1]}, nil
}

func (t RememberToken) String() string {
	return t.Selector + "." + t.Verifier
}

func (s *SessionStore) CreateRememberedDevice(ctx context.Context, userID string, selectorHash []byte, verifierHash []byte, userAgent string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO remembered_devices (user_id, selector_hash, verifier_hash, user_agent, expires_at)
VALUES ($1, $2, $3, $4, $5)`,
		userID,
		selectorHash,
		verifierHash,
		nullableString(userAgent),
		expiresAt.UTC(),
	)
	return err
}

func (s *SessionStore) RememberedDeviceBySelectorHash(ctx context.Context, selectorHash []byte) (RememberedDevice, error) {
	var device RememberedDevice
	err := s.db.QueryRowContext(ctx, `
SELECT rd.id, rd.verifier_hash, rd.expires_at, rd.revoked_at,
       u.id, u.telegram_id, u.username, u.display_name, u.role
FROM remembered_devices rd
JOIN users u ON u.id = rd.user_id
WHERE rd.selector_hash = $1`, selectorHash).Scan(
		&device.ID,
		&device.VerifierHash,
		&device.ExpiresAt,
		&device.RevokedAt,
		&device.User.ID,
		&device.User.TelegramID,
		&device.User.Username,
		&device.User.DisplayName,
		&device.User.Role,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RememberedDevice{}, ErrRememberedDeviceInvalid
	}
	if err != nil {
		return RememberedDevice{}, err
	}
	if device.RevokedAt.Valid || !device.ExpiresAt.After(time.Now().UTC()) {
		return RememberedDevice{}, ErrRememberedDeviceInvalid
	}
	return device, nil
}

func (s *SessionStore) RotateRememberedDevice(ctx context.Context, id string, selectorHash []byte, verifierHash []byte, userAgent string, expiresAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE remembered_devices
SET selector_hash = $2,
    verifier_hash = $3,
    user_agent = $4,
    expires_at = $5,
    last_used_at = now()
WHERE id = $1
  AND revoked_at IS NULL`,
		id,
		selectorHash,
		verifierHash,
		nullableString(userAgent),
		expiresAt.UTC(),
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrRememberedDeviceInvalid
	}
	return nil
}

func (s *SessionStore) RevokeRememberedDevice(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE remembered_devices
SET revoked_at = now()
WHERE id = $1
  AND revoked_at IS NULL`, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrRememberedDeviceInvalid
	}
	return nil
}

func verifyRememberVerifier(storedHash []byte, candidateHash []byte) bool {
	return subtle.ConstantTimeCompare(storedHash, candidateHash) == 1
}

package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrInvalidSession = errors.New("invalid session")
var ErrCommunityUserLimitReached = errors.New("community user limit reached")

type User struct {
	ID          string
	TelegramID  int64
	Username    sql.NullString
	DisplayName sql.NullString
	Role        string
}

type SessionStore struct {
	db *sql.DB
}

func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{db: db}
}

func (s *SessionStore) CreateSession(ctx context.Context, userID string, refreshTokenHash []byte, userAgent string, ipHash []byte, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO sessions (user_id, refresh_token_hash, user_agent, ip_hash, expires_at)
VALUES ($1, $2, $3, $4, $5)`,
		userID,
		refreshTokenHash,
		nullableString(userAgent),
		nullableBytes(ipHash),
		expiresAt,
	)
	return err
}

func (s *SessionStore) CompleteTelegramLogin(ctx context.Context, profile TelegramProfile, encryptedSession []byte, refreshTokenHash []byte, userAgent string, ipHash []byte, expiresAt time.Time) (User, error) {
	return s.CompleteTelegramLoginWithPolicy(ctx, profile, encryptedSession, refreshTokenHash, userAgent, ipHash, expiresAt, false)
}

func (s *SessionStore) CompleteTelegramLoginWithPolicy(ctx context.Context, profile TelegramProfile, encryptedSession []byte, refreshTokenHash []byte, userAgent string, ipHash []byte, expiresAt time.Time, singleUserMode bool) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	if singleUserMode {
		if err := enforceSingleUserMode(ctx, tx, profile.TelegramID); err != nil {
			return User{}, err
		}
	}

	var user User
	err = tx.QueryRowContext(ctx, `
INSERT INTO users (telegram_id, username, display_name)
VALUES ($1, $2, $3)
ON CONFLICT (telegram_id)
DO UPDATE SET
    username = EXCLUDED.username,
    display_name = EXCLUDED.display_name,
    updated_at = now()
RETURNING id, telegram_id, username, display_name, role`,
		profile.TelegramID,
		nullableString(profile.Username),
		nullableString(profile.DisplayName),
	).Scan(&user.ID, &user.TelegramID, &user.Username, &user.DisplayName, &user.Role)
	if err != nil {
		return User{}, err
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO telegram_sessions (user_id, encrypted_session)
VALUES ($1, $2)
ON CONFLICT (user_id)
DO UPDATE SET
    encrypted_session = EXCLUDED.encrypted_session,
    updated_at = now()`,
		user.ID,
		encryptedSession,
	); err != nil {
		return User{}, err
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO sessions (user_id, refresh_token_hash, user_agent, ip_hash, expires_at)
VALUES ($1, $2, $3, $4, $5)`,
		user.ID,
		refreshTokenHash,
		nullableString(userAgent),
		nullableBytes(ipHash),
		expiresAt,
	); err != nil {
		return User{}, err
	}

	if err := tx.Commit(); err != nil {
		return User{}, err
	}

	return user, nil
}

func enforceSingleUserMode(ctx context.Context, tx *sql.Tx, telegramID int64) error {
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(6471320)); err != nil {
		return err
	}
	if telegramID <= 0 {
		return ErrCommunityUserLimitReached
	}

	var ownerTelegramID sql.NullInt64
	if err := tx.QueryRowContext(ctx, `
SELECT community_owner_telegram_id
FROM admin_settings
WHERE id = TRUE
FOR UPDATE`).Scan(&ownerTelegramID); err != nil {
		return err
	}

	if ownerTelegramID.Valid {
		if ownerTelegramID.Int64 != telegramID {
			return ErrCommunityUserLimitReached
		}
		return nil
	}

	var currentUserExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE telegram_id = $1)`, telegramID).Scan(&currentUserExists); err != nil {
		return err
	}
	if !currentUserExists {
		var anyUserExists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users)`).Scan(&anyUserExists); err != nil {
			return err
		}
		if anyUserExists {
			return ErrCommunityUserLimitReached
		}
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE admin_settings
SET community_owner_telegram_id = $1
WHERE id = TRUE
  AND community_owner_telegram_id IS NULL`, telegramID); err != nil {
		return err
	}
	return nil
}

func (s *SessionStore) RotateRefreshToken(ctx context.Context, oldHash []byte, newHash []byte, expiresAt time.Time) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	var user User
	var oldSessionID string
	err = tx.QueryRowContext(ctx, `
SELECT s.id, u.id, u.telegram_id, u.username, u.display_name, u.role
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.refresh_token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now()
FOR UPDATE`,
		oldHash,
	).Scan(&oldSessionID, &user.ID, &user.TelegramID, &user.Username, &user.DisplayName, &user.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrInvalidSession
	}
	if err != nil {
		return User{}, err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at = now() WHERE id = $1`, oldSessionID); err != nil {
		return User{}, err
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO sessions (user_id, refresh_token_hash, expires_at)
VALUES ($1, $2, $3)`,
		user.ID,
		newHash,
		expiresAt,
	); err != nil {
		return User{}, err
	}

	if err := tx.Commit(); err != nil {
		return User{}, err
	}

	return user, nil
}

func (s *SessionStore) UserByRefreshToken(ctx context.Context, refreshTokenHash []byte) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `
SELECT u.id, u.telegram_id, u.username, u.display_name, u.role
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.refresh_token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now()`,
		refreshTokenHash,
	).Scan(&user.ID, &user.TelegramID, &user.Username, &user.DisplayName, &user.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrInvalidSession
	}
	if err != nil {
		return User{}, err
	}

	return user, nil
}

func (s *SessionStore) RevokeRefreshToken(ctx context.Context, tokenHash []byte) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE sessions
SET revoked_at = now()
WHERE refresh_token_hash = $1
  AND revoked_at IS NULL`,
		tokenHash,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrInvalidSession
	}

	return nil
}

func nullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func nullableBytes(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	return value
}

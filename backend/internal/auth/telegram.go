package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
	"strconv"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/crypto/secrets"
)

const telegramChallengeTTL = 10 * time.Minute

var ErrInvalidChallenge = errors.New("invalid auth challenge")
var ErrTelegramMFARequired = errors.New("telegram mfa required")
var ErrTelegramMFAInvalid = errors.New("telegram mfa invalid")

type TelegramProfile struct {
	TelegramID  int64
	Username    string
	DisplayName string
}

type TelegramCodeChallenge struct {
	PhoneCodeHash string
	Session       string
}

type TelegramLoginRequest struct {
	Phone     string
	Code      string
	Password  string
	Challenge TelegramCodeChallenge
}

type StoredAuthChallenge struct {
	PhoneCodeHash          string
	EncryptedClientSession []byte
}

type TelegramAuthClient interface {
	SendCode(ctx context.Context, phone string) (TelegramCodeChallenge, error)
	SignIn(ctx context.Context, request TelegramLoginRequest) (session string, profile TelegramProfile, err error)
	StartQRLogin(ctx context.Context) (TelegramQRLoginAttempt, error)
}

type TelegramQRLoginToken struct {
	URL       string
	ExpiresAt time.Time
}

type TelegramQRLoginResult struct {
	Session string
	Profile TelegramProfile
	Err     error
}

type TelegramQRLoginAttempt struct {
	Token   TelegramQRLoginToken
	Tokens  <-chan TelegramQRLoginToken
	Results <-chan TelegramQRLoginResult
	Cancel  func()
}

type TelegramUploadResult struct {
	Peer      string
	MessageID int64
}

type TelegramStorageClient interface {
	UploadEncryptedPart(ctx context.Context, session string, storagePeer string, name string, mimeType string, body io.Reader) (TelegramUploadResult, error)
	DownloadEncryptedPart(ctx context.Context, session string, storagePeer string, messageID int64, dst io.Writer) error
	DeleteEncryptedPart(ctx context.Context, session string, storagePeer string, messageID int64) error
}

type TelegramSessionCrypto struct {
	box *secrets.Box
}

func NewTelegramSessionCrypto(box *secrets.Box) TelegramSessionCrypto {
	return TelegramSessionCrypto{box: box}
}

func (c TelegramSessionCrypto) Encrypt(userID string, session string) ([]byte, error) {
	return c.box.Encrypt([]byte(session), telegramSessionAAD(userID))
}

func telegramUserAAD(telegramID int64) string {
	return "telegram:" + strconv.FormatInt(telegramID, 10)
}

func telegramChallengeAAD(phoneHash []byte) string {
	return "telegram-challenge:" + base64.StdEncoding.EncodeToString(phoneHash)
}

func (c TelegramSessionCrypto) Decrypt(userID string, encryptedSession []byte) (string, error) {
	plaintext, err := c.box.Decrypt(encryptedSession, telegramSessionAAD(userID))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func (c TelegramSessionCrypto) DecryptForTelegramID(telegramID int64, encryptedSession []byte) (string, error) {
	plaintext, err := c.box.Decrypt(encryptedSession, telegramSessionAAD(telegramUserAAD(telegramID)))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func telegramSessionAAD(userID string) []byte {
	return []byte("telegram-session:" + userID)
}

func HashPhone(phone string, pepper string) ([]byte, error) {
	if phone == "" {
		return nil, errors.New("phone is required")
	}
	if pepper == "" {
		return nil, errors.New("phone pepper is required")
	}

	mac := hmac.New(sha256.New, []byte(pepper))
	_, _ = mac.Write([]byte(phone))
	return mac.Sum(nil), nil
}

func (s *SessionStore) CreateAuthChallenge(ctx context.Context, phoneHash []byte, phoneCodeHash string, encryptedClientSession []byte, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO auth_challenges (phone_hash, phone_code_hash, encrypted_client_session, expires_at)
VALUES ($1, $2, $3, $4)`,
		phoneHash,
		phoneCodeHash,
		encryptedClientSession,
		expiresAt,
	)
	return err
}

func (s *SessionStore) LatestAuthChallenge(ctx context.Context, phoneHash []byte) (StoredAuthChallenge, error) {
	var challenge StoredAuthChallenge
	err := s.db.QueryRowContext(ctx, `
SELECT phone_code_hash, encrypted_client_session
FROM auth_challenges
WHERE phone_hash = $1
  AND consumed_at IS NULL
  AND expires_at > now()
ORDER BY created_at DESC
LIMIT 1`,
		phoneHash,
	).Scan(&challenge.PhoneCodeHash, &challenge.EncryptedClientSession)
	if errors.Is(err, sql.ErrNoRows) {
		return StoredAuthChallenge{}, ErrInvalidChallenge
	}
	if err != nil {
		return StoredAuthChallenge{}, err
	}

	return challenge, nil
}

func (s *SessionStore) ConsumeAuthChallenge(ctx context.Context, phoneHash []byte) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE auth_challenges
SET consumed_at = now()
WHERE id = (
    SELECT id
    FROM auth_challenges
    WHERE phone_hash = $1
      AND consumed_at IS NULL
      AND expires_at > now()
    ORDER BY created_at DESC
    LIMIT 1
)`,
		phoneHash,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrInvalidChallenge
	}

	return nil
}

func (s *SessionStore) UpsertTelegramSession(ctx context.Context, userID string, encryptedSession []byte, storagePeer string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO telegram_sessions (user_id, encrypted_session, storage_peer)
VALUES ($1, $2, $3)
ON CONFLICT (user_id)
DO UPDATE SET
    encrypted_session = EXCLUDED.encrypted_session,
    storage_peer = EXCLUDED.storage_peer,
    updated_at = now()`,
		userID,
		encryptedSession,
		nullableString(storagePeer),
	)
	return err
}

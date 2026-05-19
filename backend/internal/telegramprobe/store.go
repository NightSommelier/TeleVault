package telegramprobe

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/config"
)

var ErrAccountNotFound = errors.New("telegram account not found")

type Account struct {
	UserID           string
	TelegramID       int64
	EncryptedSession []byte
	StoragePeer      sql.NullString
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) AccountByTelegramID(ctx context.Context, telegramID int64) (Account, error) {
	var account Account
	err := s.db.QueryRowContext(ctx, `
SELECT u.id, u.telegram_id, ts.encrypted_session, ts.storage_peer
FROM users u
JOIN telegram_sessions ts ON ts.user_id = u.id
WHERE u.telegram_id = $1`,
		telegramID,
	).Scan(&account.UserID, &account.TelegramID, &account.EncryptedSession, &account.StoragePeer)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, fmt.Errorf("%w: telegram_id=%d", ErrAccountNotFound, telegramID)
	}
	if err != nil {
		return Account{}, err
	}
	return account, nil
}

func (s *Store) MarkPending(ctx context.Context, userID string, nextProbeAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO telegram_account_limits (
    user_id,
    telegram_document_limit_bytes,
    upload_safety_margin_bytes,
    last_probe_status,
    last_probe_error,
    last_probed_at,
    next_probe_at,
    updated_at
)
VALUES ($1, $2, $3, 'pending', NULL, now(), $4, now())
ON CONFLICT (user_id)
DO UPDATE SET
    last_probe_status = 'pending',
    last_probe_error = NULL,
    last_probed_at = now(),
    next_probe_at = EXCLUDED.next_probe_at,
    updated_at = now()`,
		userID,
		config.DefaultTelegramDocumentLimitBytes,
		config.DefaultUploadSafetyMarginBytes,
		nextProbeAt,
	)
	return err
}

func (s *Store) MarkSuccess(ctx context.Context, userID string, detectedLimitBytes int64, nextProbeAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE telegram_account_limits
SET detected_document_limit_bytes = $2,
    last_probe_status = 'success',
    last_probe_error = NULL,
    last_probed_at = now(),
    next_probe_at = $3,
    updated_at = now()
WHERE user_id = $1`,
		userID,
		detectedLimitBytes,
		nextProbeAt,
	)
	return err
}

func (s *Store) MarkFailed(ctx context.Context, userID string, probeErr error, nextProbeAt time.Time) error {
	message := ""
	if probeErr != nil {
		message = probeErr.Error()
	}
	if len(message) > 512 {
		message = message[:512]
	}

	_, err := s.db.ExecContext(ctx, `
UPDATE telegram_account_limits
SET last_probe_status = 'failed',
    last_probe_error = NULLIF($2, ''),
    last_probed_at = now(),
    next_probe_at = $3,
    updated_at = now()
WHERE user_id = $1`,
		userID,
		message,
		nextProbeAt,
	)
	return err
}

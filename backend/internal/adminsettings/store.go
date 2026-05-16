package adminsettings

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/televault/TeleVault/backend/internal/config"
)

var ErrInvalidSettings = errors.New("invalid admin settings")

type UploadSettings struct {
	UploadPartSizeBytes        int64 `json:"upload_part_size_bytes"`
	TelegramDocumentLimitBytes int64 `json:"telegram_document_limit_bytes"`
	UploadSafetyMarginBytes    int64 `json:"upload_safety_margin_bytes"`
	UpdatedAt                  time.Time
}

type TelegramAccountLimit struct {
	UserID                     string
	TelegramID                 int64
	Username                   sql.NullString
	DisplayName                sql.NullString
	TelegramDocumentLimitBytes int64
	UploadSafetyMarginBytes    int64
	IsPremium                  bool
	UpdatedAt                  time.Time
}

type Store struct {
	db       *sql.DB
	fallback UploadSettings
}

func NewStore(db *sql.DB, cfg config.Config) *Store {
	return &Store{
		db: db,
		fallback: UploadSettings{
			UploadPartSizeBytes:        cfg.UploadPartSizeBytes,
			TelegramDocumentLimitBytes: cfg.TelegramDocumentLimitBytes,
			UploadSafetyMarginBytes:    cfg.UploadSafetyMarginBytes,
		},
	}
}

func (s *Store) UploadSettings(ctx context.Context) (UploadSettings, error) {
	settings := s.fallback
	err := s.db.QueryRowContext(ctx, `
SELECT upload_part_size_bytes, telegram_document_limit_bytes, upload_safety_margin_bytes, updated_at
FROM admin_settings
WHERE id = TRUE`,
	).Scan(
		&settings.UploadPartSizeBytes,
		&settings.TelegramDocumentLimitBytes,
		&settings.UploadSafetyMarginBytes,
		&settings.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return s.fallback, nil
	}
	if err != nil {
		return UploadSettings{}, err
	}
	return settings, nil
}

func (s *Store) UpdateUploadSettings(ctx context.Context, settings UploadSettings, updatedBy string) (UploadSettings, error) {
	if err := validateUploadSettings(settings); err != nil {
		return UploadSettings{}, err
	}

	return scanUploadSettings(s.db.QueryRowContext(ctx, `
INSERT INTO admin_settings (
    id,
    upload_part_size_bytes,
    telegram_document_limit_bytes,
    upload_safety_margin_bytes,
    updated_by,
    updated_at
)
VALUES (TRUE, $1, $2, $3, NULLIF($4, '')::uuid, now())
ON CONFLICT (id)
DO UPDATE SET
    upload_part_size_bytes = EXCLUDED.upload_part_size_bytes,
    telegram_document_limit_bytes = EXCLUDED.telegram_document_limit_bytes,
    upload_safety_margin_bytes = EXCLUDED.upload_safety_margin_bytes,
    updated_by = EXCLUDED.updated_by,
    updated_at = now()
RETURNING upload_part_size_bytes, telegram_document_limit_bytes, upload_safety_margin_bytes, updated_at`,
		settings.UploadPartSizeBytes,
		settings.TelegramDocumentLimitBytes,
		settings.UploadSafetyMarginBytes,
		updatedBy,
	))
}

func (s *Store) ListTelegramAccountLimits(ctx context.Context) ([]TelegramAccountLimit, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT u.id, u.telegram_id, u.username, u.display_name,
       l.telegram_document_limit_bytes, l.upload_safety_margin_bytes, l.is_premium, l.updated_at
FROM telegram_account_limits l
JOIN users u ON u.id = l.user_id
ORDER BY l.updated_at DESC, u.telegram_id ASC
LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var limits []TelegramAccountLimit
	for rows.Next() {
		var limit TelegramAccountLimit
		if err := rows.Scan(
			&limit.UserID,
			&limit.TelegramID,
			&limit.Username,
			&limit.DisplayName,
			&limit.TelegramDocumentLimitBytes,
			&limit.UploadSafetyMarginBytes,
			&limit.IsPremium,
			&limit.UpdatedAt,
		); err != nil {
			return nil, err
		}
		limits = append(limits, limit)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return limits, nil
}

func (s *Store) UpsertTelegramAccountLimit(ctx context.Context, userID string, limit TelegramAccountLimit, updatedBy string) (TelegramAccountLimit, error) {
	if userID == "" || limit.TelegramDocumentLimitBytes <= 0 || limit.UploadSafetyMarginBytes < 0 || limit.UploadSafetyMarginBytes >= limit.TelegramDocumentLimitBytes {
		return TelegramAccountLimit{}, ErrInvalidSettings
	}

	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists); err != nil {
		return TelegramAccountLimit{}, err
	}
	if !exists {
		return TelegramAccountLimit{}, ErrInvalidSettings
	}

	return scanTelegramAccountLimitWithUser(s.db.QueryRowContext(ctx, `
WITH upserted AS (
    INSERT INTO telegram_account_limits (
    user_id,
    telegram_document_limit_bytes,
    upload_safety_margin_bytes,
    is_premium,
    updated_by,
    updated_at
    )
    VALUES ($1, $2, $3, $4, NULLIF($5, '')::uuid, now())
    ON CONFLICT (user_id)
    DO UPDATE SET
        telegram_document_limit_bytes = EXCLUDED.telegram_document_limit_bytes,
        upload_safety_margin_bytes = EXCLUDED.upload_safety_margin_bytes,
        is_premium = EXCLUDED.is_premium,
        updated_by = EXCLUDED.updated_by,
        updated_at = now()
    RETURNING user_id, telegram_document_limit_bytes, upload_safety_margin_bytes, is_premium, updated_at
)
SELECT u.id, u.telegram_id, u.username, u.display_name,
       upserted.telegram_document_limit_bytes, upserted.upload_safety_margin_bytes, upserted.is_premium, upserted.updated_at
FROM upserted
JOIN users u ON u.id = upserted.user_id`,
		userID,
		limit.TelegramDocumentLimitBytes,
		limit.UploadSafetyMarginBytes,
		limit.IsPremium,
		updatedBy,
	))
}

func validateUploadSettings(settings UploadSettings) error {
	if settings.UploadPartSizeBytes <= 0 ||
		settings.TelegramDocumentLimitBytes <= 0 ||
		settings.UploadSafetyMarginBytes < 0 ||
		settings.UploadPartSizeBytes > settings.TelegramDocumentLimitBytes-settings.UploadSafetyMarginBytes {
		return ErrInvalidSettings
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUploadSettings(row rowScanner) (UploadSettings, error) {
	var settings UploadSettings
	err := row.Scan(
		&settings.UploadPartSizeBytes,
		&settings.TelegramDocumentLimitBytes,
		&settings.UploadSafetyMarginBytes,
		&settings.UpdatedAt,
	)
	if err != nil {
		return UploadSettings{}, err
	}
	return settings, nil
}

func scanTelegramAccountLimitWithUser(row rowScanner) (TelegramAccountLimit, error) {
	var limit TelegramAccountLimit
	err := row.Scan(
		&limit.UserID,
		&limit.TelegramID,
		&limit.Username,
		&limit.DisplayName,
		&limit.TelegramDocumentLimitBytes,
		&limit.UploadSafetyMarginBytes,
		&limit.IsPremium,
		&limit.UpdatedAt,
	)
	if err != nil {
		return TelegramAccountLimit{}, err
	}
	return limit, nil
}

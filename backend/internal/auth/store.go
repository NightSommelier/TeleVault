package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrInvalidSession = errors.New("invalid session")
var ErrCommunityUserLimitReached = errors.New("community user limit reached")
var ErrAccountLimitReached = errors.New("account limit reached")
var ErrInviteRequired = errors.New("invite required")
var ErrInviteInvalid = errors.New("invite invalid")
var ErrInviteCapacityReached = errors.New("invite capacity reached")

type LoginPolicy struct {
	MaxConnectedTelegramAccounts int
	BindCommunityOwner           bool
}

type LoginAccess struct {
	InviteTokenHash []byte
}

type InstanceInvite struct {
	ID                string
	InvitedTelegramID sql.NullInt64
	MaxUses           int
	UsedCount         int
	Status            string
	ExpiresAt         time.Time
	CreatorUserID     string
	ConsumedAt        sql.NullTime
	RevokedAt         sql.NullTime
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func CommunityLoginPolicy() LoginPolicy {
	return LoginPolicy{
		MaxConnectedTelegramAccounts: 1,
		BindCommunityOwner:           true,
	}
}

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
	return s.CompleteTelegramLoginWithAccessPolicy(ctx, profile, encryptedSession, refreshTokenHash, userAgent, ipHash, expiresAt, LoginPolicy{})
}

func (s *SessionStore) CompleteTelegramLoginWithPolicy(ctx context.Context, profile TelegramProfile, encryptedSession []byte, refreshTokenHash []byte, userAgent string, ipHash []byte, expiresAt time.Time, singleUserMode bool) (User, error) {
	policy := LoginPolicy{}
	if singleUserMode {
		policy = CommunityLoginPolicy()
	}
	return s.CompleteTelegramLoginWithAccessPolicy(ctx, profile, encryptedSession, refreshTokenHash, userAgent, ipHash, expiresAt, policy)
}

func (s *SessionStore) CompleteTelegramLoginWithAccessPolicy(ctx context.Context, profile TelegramProfile, encryptedSession []byte, refreshTokenHash []byte, userAgent string, ipHash []byte, expiresAt time.Time, policy LoginPolicy) (User, error) {
	return s.completeTelegramLoginWithAccessPolicy(ctx, profile, encryptedSession, refreshTokenHash, userAgent, ipHash, expiresAt, policy, LoginAccess{})
}

func (s *SessionStore) CompleteTelegramLoginWithAccessPolicyAndAccess(ctx context.Context, profile TelegramProfile, encryptedSession []byte, refreshTokenHash []byte, userAgent string, ipHash []byte, expiresAt time.Time, policy LoginPolicy, access LoginAccess) (User, error) {
	return s.completeTelegramLoginWithAccessPolicy(ctx, profile, encryptedSession, refreshTokenHash, userAgent, ipHash, expiresAt, policy, access)
}

func (s *SessionStore) completeTelegramLoginWithAccessPolicy(ctx context.Context, profile TelegramProfile, encryptedSession []byte, refreshTokenHash []byte, userAgent string, ipHash []byte, expiresAt time.Time, policy LoginPolicy, access LoginAccess) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	if policy.MaxConnectedTelegramAccounts > 0 {
		if err := enforceConnectedAccountPolicy(ctx, tx, profile.TelegramID, policy); err != nil {
			return User{}, err
		}
	}
	if err := enforceInvitePolicy(ctx, tx, profile.TelegramID, access); err != nil {
		return User{}, err
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

func enforceInvitePolicy(ctx context.Context, tx *sql.Tx, telegramID int64, access LoginAccess) error {
	if telegramID <= 0 {
		return ErrInviteRequired
	}

	var currentUserExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE telegram_id = $1)`, telegramID).Scan(&currentUserExists); err != nil {
		return err
	}
	if currentUserExists {
		return nil
	}

	var anyUserExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users)`).Scan(&anyUserExists); err != nil {
		return err
	}
	if !anyUserExists {
		// Bootstrap flow on a fresh instance.
		return nil
	}

	if len(access.InviteTokenHash) == 0 {
		return ErrInviteRequired
	}

	var inviteID string
	var invitedTelegramID sql.NullInt64
	var maxUses int
	var usedCount int
	var status string
	var expiresAt time.Time
	err := tx.QueryRowContext(ctx, `
SELECT id, invited_telegram_id, max_uses, used_count, status, expires_at
FROM instance_invites
WHERE token_hash = $1
FOR UPDATE`,
		access.InviteTokenHash,
	).Scan(&inviteID, &invitedTelegramID, &maxUses, &usedCount, &status, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInviteInvalid
	}
	if err != nil {
		return err
	}

	if status != "active" {
		return ErrInviteInvalid
	}
	if !expiresAt.After(time.Now().UTC()) {
		return ErrInviteInvalid
	}
	if usedCount >= maxUses {
		return ErrInviteInvalid
	}
	if invitedTelegramID.Valid && invitedTelegramID.Int64 != telegramID {
		return ErrInviteInvalid
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE instance_invites
SET used_count = used_count + 1,
    status = CASE WHEN used_count + 1 >= max_uses THEN 'consumed' ELSE status END,
    consumed_at = CASE WHEN used_count + 1 >= max_uses THEN now() ELSE consumed_at END,
    updated_at = now()
WHERE id = $1`,
		inviteID,
	); err != nil {
		return err
	}
	return nil
}

func enforceConnectedAccountPolicy(ctx context.Context, tx *sql.Tx, telegramID int64, policy LoginPolicy) error {
	if policy.BindCommunityOwner {
		return enforceSingleUserMode(ctx, tx, telegramID)
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(6471321)); err != nil {
		return err
	}
	if telegramID <= 0 {
		return ErrAccountLimitReached
	}

	var currentUserExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE telegram_id = $1)`, telegramID).Scan(&currentUserExists); err != nil {
		return err
	}
	if currentUserExists {
		return nil
	}

	var userCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		return err
	}
	if userCount >= policy.MaxConnectedTelegramAccounts {
		return ErrAccountLimitReached
	}
	return nil
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

func (s *SessionStore) CreateInstanceInvite(ctx context.Context, creatorUserID string, tokenHash []byte, invitedTelegramID sql.NullInt64, expiresAt time.Time, maxUses int, maxConnectedAccounts int) (InstanceInvite, error) {
	if creatorUserID == "" || len(tokenHash) == 0 || maxUses <= 0 || maxConnectedAccounts <= 1 || !expiresAt.After(time.Now().UTC()) {
		return InstanceInvite{}, ErrInviteCapacityReached
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return InstanceInvite{}, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(6471322)); err != nil {
		return InstanceInvite{}, err
	}

	var userCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		return InstanceInvite{}, err
	}

	var activeInviteCount int
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM instance_invites
WHERE status = 'active'
  AND expires_at > now()
  AND used_count < max_uses`).Scan(&activeInviteCount); err != nil {
		return InstanceInvite{}, err
	}

	if userCount+activeInviteCount >= maxConnectedAccounts {
		return InstanceInvite{}, ErrInviteCapacityReached
	}

	invite, err := scanInstanceInvite(tx.QueryRowContext(ctx, `
INSERT INTO instance_invites (
    token_hash,
    invited_telegram_id,
    max_uses,
    used_count,
    status,
    expires_at,
    creator_user_id
)
VALUES ($1, $2, $3, 0, 'active', $4, $5)
RETURNING id, invited_telegram_id, max_uses, used_count, status, expires_at, creator_user_id,
          consumed_at, revoked_at, created_at, updated_at`,
		tokenHash,
		invitedTelegramID,
		maxUses,
		expiresAt.UTC(),
		creatorUserID,
	))
	if err != nil {
		return InstanceInvite{}, err
	}

	if err := tx.Commit(); err != nil {
		return InstanceInvite{}, err
	}

	return invite, nil
}

func (s *SessionStore) ListInstanceInvites(ctx context.Context, limit int) ([]InstanceInvite, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, invited_telegram_id, max_uses, used_count, status, expires_at, creator_user_id,
       consumed_at, revoked_at, created_at, updated_at
FROM instance_invites
ORDER BY created_at DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invites := make([]InstanceInvite, 0, limit)
	for rows.Next() {
		invite, err := scanInstanceInvite(rows)
		if err != nil {
			return nil, err
		}
		invites = append(invites, invite)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return invites, nil
}

func (s *SessionStore) RevokeInstanceInvite(ctx context.Context, inviteID string) error {
	if inviteID == "" {
		return ErrInviteInvalid
	}

	result, err := s.db.ExecContext(ctx, `
UPDATE instance_invites
SET status = 'revoked',
    revoked_at = now(),
    updated_at = now()
WHERE id = $1
  AND status = 'active'`, inviteID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrInviteInvalid
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
	var oldMFARequired bool
	var oldMFAVerifiedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
SELECT s.id, s.mfa_required, s.mfa_verified_at, u.id, u.telegram_id, u.username, u.display_name, u.role
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.refresh_token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now()
FOR UPDATE`,
		oldHash,
	).Scan(&oldSessionID, &oldMFARequired, &oldMFAVerifiedAt, &user.ID, &user.TelegramID, &user.Username, &user.DisplayName, &user.Role)
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
INSERT INTO sessions (user_id, refresh_token_hash, expires_at, mfa_required, mfa_verified_at)
VALUES ($1, $2, $3, $4, $5)`,
		user.ID,
		newHash,
		expiresAt,
		oldMFARequired,
		oldMFAVerifiedAt,
	); err != nil {
		return User{}, err
	}

	if err := tx.Commit(); err != nil {
		return User{}, err
	}

	return user, nil
}

func (s *SessionStore) UserByRefreshToken(ctx context.Context, refreshTokenHash []byte) (User, error) {
	session, err := s.SessionByRefreshToken(ctx, refreshTokenHash)
	if err != nil {
		return User{}, err
	}
	return session.User, nil
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

type scanner interface {
	Scan(dest ...any) error
}

func scanInstanceInvite(row scanner) (InstanceInvite, error) {
	var invite InstanceInvite
	err := row.Scan(
		&invite.ID,
		&invite.InvitedTelegramID,
		&invite.MaxUses,
		&invite.UsedCount,
		&invite.Status,
		&invite.ExpiresAt,
		&invite.CreatorUserID,
		&invite.ConsumedAt,
		&invite.RevokedAt,
		&invite.CreatedAt,
		&invite.UpdatedAt,
	)
	if err != nil {
		return InstanceInvite{}, err
	}
	return invite, nil
}

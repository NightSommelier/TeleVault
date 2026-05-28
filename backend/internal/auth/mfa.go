package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"database/sql"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

var ErrLocalMFAInvalidCode = errors.New("invalid local mfa code")
var ErrLocalMFANotConfigured = errors.New("local mfa not configured")
var ErrLocalPasswordNotConfigured = errors.New("local password not configured")
var ErrWebAuthnCredentialNotFound = errors.New("webauthn credential not found")

type AuthSession struct {
	ID            string
	User          User
	MFARequired   bool
	MFAVerifiedAt sql.NullTime
}

type LocalMFAStatus struct {
	ForceEnabled            bool
	TOTPConfigured          bool
	TOTPEnabled             bool
	WebAuthnConfigured      bool
	RecoveryCodesRemaining  int
	SessionRequiresComplete bool
	SessionVerified         bool
}

type LocalTOTPState struct {
	EncryptedSecret []byte
	Enabled         bool
}

type WebAuthnChallenge struct {
	ID        string
	UserID    string
	Kind      string
	Session   webauthn.SessionData
	ExpiresAt time.Time
}

type WebAuthnCredentialEntry struct {
	ID          string
	DisplayName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	LastUsedAt  sql.NullTime
}

func (s *SessionStore) SessionByRefreshToken(ctx context.Context, refreshTokenHash []byte) (AuthSession, error) {
	var session AuthSession
	err := s.db.QueryRowContext(ctx, `
SELECT s.id, s.mfa_required, s.mfa_verified_at, u.id, u.telegram_id, u.username, u.display_name, u.role
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.refresh_token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now()`,
		refreshTokenHash,
	).Scan(
		&session.ID,
		&session.MFARequired,
		&session.MFAVerifiedAt,
		&session.User.ID,
		&session.User.TelegramID,
		&session.User.Username,
		&session.User.DisplayName,
		&session.User.Role,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return AuthSession{}, ErrInvalidSession
	}
	if err != nil {
		return AuthSession{}, err
	}
	return session, nil
}

func (s *SessionStore) MarkSessionMFARequiredByToken(ctx context.Context, refreshTokenHash []byte, required bool) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE sessions
SET mfa_required = $2,
    mfa_verified_at = CASE WHEN $2 THEN NULL ELSE now() END
WHERE refresh_token_hash = $1
  AND revoked_at IS NULL`,
		refreshTokenHash,
		required,
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

func (s *SessionStore) MarkSessionMFAVerified(ctx context.Context, sessionID string, userID string) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE sessions
SET mfa_verified_at = now()
WHERE id = $1
  AND user_id = $2
  AND revoked_at IS NULL`,
		sessionID,
		userID,
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

func (s *SessionStore) IsLocalMFAForced(ctx context.Context, envDefault bool) (bool, error) {
	var force bool
	err := s.db.QueryRowContext(ctx, `
SELECT force_local_mfa
FROM admin_settings
WHERE id = TRUE`).Scan(&force)
	if errors.Is(err, sql.ErrNoRows) {
		return envDefault, nil
	}
	if err != nil {
		return false, err
	}
	return force || envDefault, nil
}

func (s *SessionStore) LocalTOTP(ctx context.Context, userID string) (LocalTOTPState, error) {
	var state LocalTOTPState
	err := s.db.QueryRowContext(ctx, `
SELECT encrypted_secret, enabled
FROM user_local_totp
WHERE user_id = $1`, userID).Scan(&state.EncryptedSecret, &state.Enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return LocalTOTPState{}, ErrLocalMFANotConfigured
	}
	if err != nil {
		return LocalTOTPState{}, err
	}
	return state, nil
}

func (s *SessionStore) UpsertLocalTOTPSecret(ctx context.Context, userID string, encryptedSecret []byte) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO user_local_totp (user_id, encrypted_secret, enabled, updated_at)
VALUES ($1, $2, FALSE, now())
ON CONFLICT (user_id)
DO UPDATE SET
    encrypted_secret = EXCLUDED.encrypted_secret,
    enabled = FALSE,
    updated_at = now()`,
		userID,
		encryptedSecret,
	)
	return err
}

func (s *SessionStore) EnableLocalTOTP(ctx context.Context, userID string) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE user_local_totp
SET enabled = TRUE, updated_at = now()
WHERE user_id = $1`, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrLocalMFANotConfigured
	}
	return nil
}

func (s *SessionStore) ReplaceRecoveryCodes(ctx context.Context, userID string, hashes [][]byte) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM user_mfa_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, hash := range hashes {
		if len(hash) == 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO user_mfa_recovery_codes (user_id, code_hash)
VALUES ($1, $2)`,
			userID,
			hash,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SessionStore) WebAuthnCredentials(ctx context.Context, userID string) ([]webauthn.Credential, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT credential_json
FROM user_webauthn_credentials
WHERE user_id = $1
ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	credentials := make([]webauthn.Credential, 0, 4)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var credential webauthn.Credential
		if err := json.Unmarshal(raw, &credential); err != nil {
			return nil, err
		}
		credentials = append(credentials, credential)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return credentials, nil
}

func (s *SessionStore) ListWebAuthnCredentialEntries(ctx context.Context, userID string) ([]WebAuthnCredentialEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, display_name, created_at, updated_at, last_used_at
FROM user_webauthn_credentials
WHERE user_id = $1
ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]WebAuthnCredentialEntry, 0, 4)
	for rows.Next() {
		var entry WebAuthnCredentialEntry
		if err := rows.Scan(&entry.ID, &entry.DisplayName, &entry.CreatedAt, &entry.UpdatedAt, &entry.LastUsedAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *SessionStore) UpsertWebAuthnCredential(ctx context.Context, userID string, credential webauthn.Credential, displayName string, markUsed bool) error {
	raw, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	if len(credential.ID) == 0 {
		return errors.New("credential id is required")
	}
	cleanDisplayName := normalizeWebAuthnDisplayName(displayName)
	lastUsedAt := sql.NullTime{}
	if markUsed {
		lastUsedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	}
	result, err := s.db.ExecContext(ctx, `
INSERT INTO user_webauthn_credentials (user_id, credential_id, credential_json, display_name, last_used_at, updated_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (credential_id)
DO UPDATE SET
    credential_json = EXCLUDED.credential_json,
    updated_at = now(),
    last_used_at = CASE WHEN EXCLUDED.last_used_at IS NULL THEN user_webauthn_credentials.last_used_at ELSE EXCLUDED.last_used_at END,
    display_name = CASE WHEN btrim(EXCLUDED.display_name) = '' THEN user_webauthn_credentials.display_name ELSE EXCLUDED.display_name END
WHERE user_webauthn_credentials.user_id = EXCLUDED.user_id`,
		userID,
		credential.ID,
		raw,
		cleanDisplayName,
		lastUsedAt,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrWebAuthnCredentialNotFound
	}
	return nil
}

func (s *SessionStore) CreateWebAuthnChallenge(ctx context.Context, userID string, kind string, session webauthn.SessionData, ttl time.Duration) (string, error) {
	if userID == "" {
		return "", errors.New("user id is required")
	}
	if kind != "registration" && kind != "authentication" {
		return "", errors.New("challenge kind is invalid")
	}
	challengeID, err := NewRefreshToken()
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().UTC().Add(ttl)
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO user_webauthn_challenges (id, user_id, challenge_kind, session_data_json, expires_at)
VALUES ($1, $2, $3, $4, $5)`,
		challengeID,
		userID,
		kind,
		raw,
		expiresAt,
	); err != nil {
		return "", err
	}
	return challengeID, nil
}

func (s *SessionStore) ConsumeWebAuthnChallenge(ctx context.Context, userID string, challengeID string, kind string) (webauthn.SessionData, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return webauthn.SessionData{}, err
	}
	defer tx.Rollback()

	var raw []byte
	var expiresAt time.Time
	err = tx.QueryRowContext(ctx, `
SELECT session_data_json, expires_at
FROM user_webauthn_challenges
WHERE id = $1
  AND user_id = $2
  AND challenge_kind = $3
  AND consumed_at IS NULL
FOR UPDATE`,
		challengeID,
		userID,
		kind,
	).Scan(&raw, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return webauthn.SessionData{}, ErrLocalMFAInvalidCode
	}
	if err != nil {
		return webauthn.SessionData{}, err
	}
	if !expiresAt.After(time.Now().UTC()) {
		return webauthn.SessionData{}, ErrLocalMFAInvalidCode
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE user_webauthn_challenges
SET consumed_at = now()
WHERE id = $1`, challengeID); err != nil {
		return webauthn.SessionData{}, err
	}
	if err := tx.Commit(); err != nil {
		return webauthn.SessionData{}, err
	}

	var session webauthn.SessionData
	if err := json.Unmarshal(raw, &session); err != nil {
		return webauthn.SessionData{}, err
	}
	return session, nil
}

func (s *SessionStore) ConsumeRecoveryCode(ctx context.Context, userID string, codeHash []byte) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
UPDATE user_mfa_recovery_codes
SET used_at = now()
WHERE user_id = $1
  AND code_hash = $2
  AND used_at IS NULL`,
		userID,
		codeHash,
	)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (s *SessionStore) RecoveryCodesRemaining(ctx context.Context, userID string) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM user_mfa_recovery_codes
WHERE user_id = $1
  AND used_at IS NULL`, userID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *SessionStore) UpsertLocalPasswordHash(ctx context.Context, userID string, passwordHash string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO user_local_passwords (user_id, password_hash, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (user_id)
DO UPDATE SET
    password_hash = EXCLUDED.password_hash,
    updated_at = now()`,
		userID,
		passwordHash,
	)
	return err
}

func (s *SessionStore) LocalPasswordHash(ctx context.Context, userID string) (string, error) {
	var hash string
	if err := s.db.QueryRowContext(ctx, `
SELECT password_hash
FROM user_local_passwords
WHERE user_id = $1`, userID).Scan(&hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrLocalPasswordNotConfigured
		}
		return "", err
	}
	if strings.TrimSpace(hash) == "" {
		return "", ErrLocalPasswordNotConfigured
	}
	return hash, nil
}

func (s *SessionStore) IsLocalPasswordConfigured(ctx context.Context, userID string) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
SELECT EXISTS(
    SELECT 1
    FROM user_local_passwords
    WHERE user_id = $1
)`, userID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *SessionStore) DeleteLocalPassword(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `
DELETE FROM user_local_passwords
WHERE user_id = $1`, userID)
	return err
}

func (s *SessionStore) DeleteLocalTOTP(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `
DELETE FROM user_local_totp
WHERE user_id = $1`, userID)
	return err
}

func (s *SessionStore) DeleteWebAuthnCredentials(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `
DELETE FROM user_webauthn_credentials
WHERE user_id = $1`, userID)
	return err
}

func (s *SessionStore) DeleteWebAuthnCredential(ctx context.Context, userID string, credentialID string) error {
	result, err := s.db.ExecContext(ctx, `
DELETE FROM user_webauthn_credentials
WHERE user_id = $1
  AND id = $2`,
		userID,
		credentialID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrWebAuthnCredentialNotFound
	}
	return nil
}

func (s *SessionStore) RenameWebAuthnCredential(ctx context.Context, userID string, credentialID string, displayName string) error {
	cleanDisplayName := normalizeWebAuthnDisplayName(displayName)
	result, err := s.db.ExecContext(ctx, `
UPDATE user_webauthn_credentials
SET display_name = $3,
    updated_at = now()
WHERE user_id = $1
  AND id = $2`,
		userID,
		credentialID,
		cleanDisplayName,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrWebAuthnCredentialNotFound
	}
	return nil
}

func (s *SessionStore) DeleteRecoveryCodes(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `
DELETE FROM user_mfa_recovery_codes
WHERE user_id = $1`, userID)
	return err
}

func NewTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

func TOTPURI(issuer string, account string, secret string) string {
	cleanIssuer := strings.TrimSpace(issuer)
	if cleanIssuer == "" {
		cleanIssuer = "TeleVault"
	}
	cleanAccount := strings.TrimSpace(account)
	if cleanAccount == "" {
		cleanAccount = "user"
	}
	label := url.PathEscape(cleanIssuer + ":" + cleanAccount)
	query := url.Values{}
	query.Set("secret", secret)
	query.Set("issuer", cleanIssuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", "6")
	query.Set("period", "30")
	return "otpauth://totp/" + label + "?" + query.Encode()
}

func VerifyTOTPCode(secret string, code string, now time.Time) bool {
	cleanCode := strings.TrimSpace(code)
	if len(cleanCode) != 6 {
		return false
	}
	for i := -1; i <= 1; i++ {
		t := now.Add(time.Duration(i) * 30 * time.Second)
		if totpCodeForTime(secret, t) == cleanCode {
			return true
		}
	}
	return false
}

func GenerateRecoveryCodes(n int) ([]string, error) {
	if n <= 0 {
		return nil, errors.New("recovery code count must be positive")
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		token, err := NewRefreshToken()
		if err != nil {
			return nil, err
		}
		token = strings.ToUpper(strings.ReplaceAll(token, "_", "A"))
		token = strings.ToUpper(strings.ReplaceAll(token, "-", "B"))
		if len(token) > 12 {
			token = token[:12]
		}
		out = append(out, token[:4]+"-"+token[4:8]+"-"+token[8:12])
	}
	return out, nil
}

func totpCodeForTime(secret string, when time.Time) string {
	secretBytes, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil || len(secretBytes) == 0 {
		return ""
	}
	counter := uint64(when.Unix() / 30)
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)

	mac := hmac.New(sha1.New, secretBytes)
	_, _ = mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := int(sum[len(sum)-1] & 0x0f)
	if offset+4 > len(sum) {
		return ""
	}
	binCode := (int(sum[offset])&0x7f)<<24 |
		(int(sum[offset+1])&0xff)<<16 |
		(int(sum[offset+2])&0xff)<<8 |
		(int(sum[offset+3]) & 0xff)
	return fmt.Sprintf("%06d", binCode%1000000)
}

func mfaSecretAAD(userID string) string {
	return "local-mfa-totp:" + userID
}

func mfaUserLabel(user User) string {
	if user.Username.Valid && strings.TrimSpace(user.Username.String) != "" {
		return user.Username.String
	}
	if user.DisplayName.Valid && strings.TrimSpace(user.DisplayName.String) != "" {
		return user.DisplayName.String
	}
	return "telegram-" + strconv.FormatInt(user.TelegramID, 10)
}

func normalizeWebAuthnDisplayName(name string) string {
	const maxLen = 80
	clean := strings.TrimSpace(name)
	if clean == "" {
		return "Passkey"
	}
	if len(clean) > maxLen {
		return clean[:maxLen]
	}
	return clean
}

type webAuthnUser struct {
	id          []byte
	name        string
	displayName string
	credentials []webauthn.Credential
}

func (u webAuthnUser) WebAuthnID() []byte {
	return u.id
}

func (u webAuthnUser) WebAuthnName() string {
	return u.name
}

func (u webAuthnUser) WebAuthnDisplayName() string {
	return u.displayName
}

func (u webAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}

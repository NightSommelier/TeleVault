package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"net"
	"net/http"
)

const (
	AuditAuthCodeSendSuccess = "auth.telegram_code_send.success"
	AuditAuthCodeSendFailure = "auth.telegram_code_send.failure"
	AuditAuthLoginSuccess    = "auth.telegram_login.success"
	AuditAuthLoginFailure    = "auth.telegram_login.failure"
	AuditAuthRefreshSuccess  = "auth.refresh.success"
	AuditAuthRefreshFailure  = "auth.refresh.failure"
	AuditAuthLogout          = "auth.logout"
)

func (s *SessionStore) RecordAuditEvent(ctx context.Context, actorUserID string, action string, r *http.Request) {
	if action == "" {
		return
	}

	_, _ = s.db.ExecContext(ctx, `
INSERT INTO audit_events (actor_user_id, action, ip_hash, user_agent)
VALUES ($1, $2, $3, $4)`,
		nullableUUID(actorUserID),
		action,
		requestIPHash(r),
		nullableString(r.UserAgent()),
	)
}

func nullableUUID(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func requestIPHash(r *http.Request) []byte {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if host == "" {
		return nil
	}

	mac := hmac.New(sha256.New, []byte("audit-ip"))
	_, _ = mac.Write([]byte(host))
	return mac.Sum(nil)
}

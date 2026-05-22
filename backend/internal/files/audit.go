package files

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"net"
	"net/http"
)

const (
	AuditFileShareCreate     = "files.share.create"
	AuditFileShareRevoke     = "files.share.revoke"
	AuditFileShareDownload   = "files.share.download"
	AuditPublicLinkCreate    = "files.public_link.create"
	AuditPublicLinkRevoke    = "files.public_link.revoke"
	AuditPublicLinkDownload  = "files.public_link.download"
	auditResourceTypeFile    = "file"
	auditResourceTypeShare   = "file_share"
	auditResourceTypePubLink = "public_link"
)

func (s *Store) RecordAuditEvent(ctx context.Context, actorUserID string, action string, resourceType string, resourceID string, r *http.Request) {
	if action == "" {
		return
	}

	_, _ = s.db.ExecContext(ctx, `
INSERT INTO audit_events (actor_user_id, action, resource_type, resource_id, ip_hash, user_agent)
VALUES ($1, $2, $3, $4, $5, $6)`,
		nullableAuditUUID(actorUserID),
		action,
		nullableAuditString(resourceType),
		nullableAuditUUID(resourceID),
		requestAuditIPHash(r),
		nullableAuditString(r.UserAgent()),
	)
}

func nullableAuditUUID(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func nullableAuditString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func requestAuditIPHash(r *http.Request) []byte {
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

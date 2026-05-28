package auth

import (
	"context"
	"errors"
	"time"
)

type telegramSessionProbeCache struct {
	Status    string
	ErrorCode string
	CheckedAt time.Time
}

func (h *Handler) getTelegramSessionProbeStatus(userID string) (telegramSessionProbeCache, bool) {
	h.probeMu.Lock()
	defer h.probeMu.Unlock()
	entry, ok := h.probeCache[userID]
	return entry, ok
}

func (h *Handler) setTelegramSessionProbeStatus(userID string, status string, errorCode string) {
	h.probeMu.Lock()
	if h.probeCache == nil {
		h.probeCache = make(map[string]telegramSessionProbeCache)
	}
	h.probeCache[userID] = telegramSessionProbeCache{
		Status:    status,
		ErrorCode: errorCode,
		CheckedAt: time.Now().UTC(),
	}
	h.probeMu.Unlock()
}

func (h *Handler) resolveTelegramSessionStatus(ctx context.Context, userID string) (string, string) {
	if cached := h.cachedTelegramSessionStatus(userID); cached != "" {
		return cached, ""
	}

	hasSession, err := h.store.HasTelegramSession(ctx, userID)
	if err != nil {
		return "unknown", "telegram_session_status_failed"
	}
	if !hasSession {
		h.setTelegramSessionProbeStatus(userID, "missing", "")
		return "missing", ""
	}

	if h.telegram == nil {
		h.setTelegramSessionProbeStatus(userID, "ok", "")
		return "ok", ""
	}

	stored, err := h.store.StoredTelegramSession(ctx, userID)
	if errors.Is(err, ErrTelegramSessionNotFound) {
		h.setTelegramSessionProbeStatus(userID, "missing", "")
		return "missing", ""
	}
	if err != nil {
		return "unknown", "telegram_session_status_failed"
	}

	decodedSession, err := h.sessionCrypto.DecryptForTelegramID(stored.OwnerTelegramID, stored.EncryptedSession)
	if err != nil {
		h.setTelegramSessionProbeStatus(userID, "stale", "telegram_session_stale")
		return "stale", "telegram_session_stale"
	}

	probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := h.telegram.ValidateSession(probeCtx, decodedSession); err != nil {
		if errors.Is(err, ErrTelegramSessionInvalid) {
			h.setTelegramSessionProbeStatus(userID, "stale", "telegram_session_stale")
			return "stale", "telegram_session_stale"
		}
		h.setTelegramSessionProbeStatus(userID, "unknown", "telegram_session_check_failed")
		return "unknown", "telegram_session_check_failed"
	}

	h.setTelegramSessionProbeStatus(userID, "ok", "")
	return "ok", ""
}

func (h *Handler) cachedTelegramSessionStatus(userID string) string {
	entry, ok := h.getTelegramSessionProbeStatus(userID)
	if !ok {
		return ""
	}
	if h.probeTTL <= 0 {
		return ""
	}
	if time.Since(entry.CheckedAt) > h.probeTTL {
		return ""
	}
	return entry.Status
}

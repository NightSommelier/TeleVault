package auth

import (
	"context"
	"net/http"
)

type userContextKey struct{}
type sessionContextKey struct{}

type SessionContext struct {
	ID            string
	MFARequired   bool
	MFAVerifiedAt bool
}

func UserFromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(userContextKey{}).(User)
	return user, ok
}

func withUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

func SessionFromContext(ctx context.Context) (SessionContext, bool) {
	session, ok := ctx.Value(sessionContextKey{}).(SessionContext)
	return session, ok
}

func withSession(ctx context.Context, session SessionContext) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, session)
}

func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return h.requireAuth(next, false)
}

func (h *Handler) RequireAuthAllowUnverifiedMFA(next http.Handler) http.Handler {
	return h.requireAuth(next, true)
}

func (h *Handler) requireAuth(next http.Handler, allowUnverifiedMFA bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(RefreshCookieName)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusUnauthorized, "missing_refresh_token")
			return
		}

		tokenHash, err := HashRefreshToken(cookie.Value, h.cfg.RefreshTokenPepper)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_refresh_token")
			return
		}

		session, err := h.store.SessionByRefreshToken(r.Context(), tokenHash)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_refresh_token")
			return
		}
		if session.MFARequired && !session.MFAVerifiedAt.Valid && !allowUnverifiedMFA {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error":        "local_mfa_required",
				"mfa_required": true,
			})
			return
		}

		ctx := withUser(r.Context(), session.User)
		ctx = withSession(ctx, SessionContext{
			ID:            session.ID,
			MFARequired:   session.MFARequired,
			MFAVerifiedAt: session.MFAVerifiedAt.Valid,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
			return
		}
		if user.Role != "admin" {
			writeError(w, http.StatusForbidden, "admin_required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

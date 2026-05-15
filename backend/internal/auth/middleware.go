package auth

import (
	"context"
	"net/http"
)

type userContextKey struct{}

func UserFromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(userContextKey{}).(User)
	return user, ok
}

func withUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

func (h *Handler) RequireAuth(next http.Handler) http.Handler {
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

		user, err := h.store.UserByRefreshToken(r.Context(), tokenHash)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_refresh_token")
			return
		}

		next.ServeHTTP(w, r.WithContext(withUser(r.Context(), user)))
	})
}

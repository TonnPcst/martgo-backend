package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/TonnPcst/martgo-backend/internal/auth"
	"github.com/TonnPcst/martgo-backend/internal/httpx"
	"github.com/TonnPcst/martgo-backend/internal/session"
)

// SessionLookup returns the token state the auth service published for a user.
type SessionLookup interface {
	Get(ctx context.Context, userID int64) (session.State, error)
}

func Authenticate(tokens *auth.Verifier, sessions SessionLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			schema, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
			if !ok || !strings.EqualFold(schema, "Bearer") || token == "" {
				httpx.Error(w, http.StatusUnauthorized, "missing or malformed bearer token")
				return
			}

			claims, err := tokens.Parse(token)
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			st, err := sessions.Get(r.Context(), claims.UserID)
			if err != nil {
				if errors.Is(err, session.ErrNotFound) {
					httpx.Error(w, http.StatusUnauthorized, "session expired, please log in again")
					return
				}
				// Redis is down or the entry is corrupt: fail closed, never skip the check.
				httpx.HandleError(w, r, err)
				return
			}
			if !st.Active {
				httpx.Error(w, http.StatusUnauthorized, "account is disabled")
				return
			}
			if claims.TokenVersion != st.TokenVersion {
				httpx.Error(w, http.StatusUnauthorized, "token has been revoked")
				return
			}
			claims.Role = st.Role

			next.ServeHTTP(w, r.WithContext(auth.WithClaims(r.Context(), claims)))
		})
	}
}

func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok {
				httpx.Error(w, http.StatusUnauthorized, "authentication required")
				return
			}
			if !allowed[claims.Role] {
				httpx.Error(w, http.StatusForbidden, "you do not have permission to access this resource")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TonnPcst/martgo-backend/internal/auth"
	"github.com/TonnPcst/martgo-backend/internal/db"
	"github.com/TonnPcst/martgo-backend/internal/domain"
)

const testSecret = "b2e55d7260e460942c63e92d8096bf06ff316a0ff2d1bee656b525a6514f7eb8"

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

type fakeUsers map[int64]db.User

func (f fakeUsers) GetUserByID(_ context.Context, id int64) (db.User, error) {
	u, ok := f[id]
	if !ok {
		return db.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func newTestRouter(users fakeUsers, pinger Pinger) (http.Handler, *auth.TokenManager) {
	tokens := auth.NewTokenManager(testSecret, time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(logger, tokens, users, pinger, Services{}, RouterOptions{}), tokens
}

func send(h http.Handler, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func mustToken(t *testing.T, tokens *auth.TokenManager, userID int64, role string) string {
	t.Helper()
	token, _, err := tokens.Generate(userID, role)
	require.NoError(t, err)
	return token
}

func TestHealthz(t *testing.T) {
	h, _ := newTestRouter(nil, fakePinger{})
	assert.Equal(t, http.StatusOK, send(h, http.MethodGet, "/healthz", "").Code)

	h, _ = newTestRouter(nil, fakePinger{err: errors.New("db down")})
	assert.Equal(t, http.StatusServiceUnavailable, send(h, http.MethodGet, "/healthz", "").Code)
}

func TestNotFoundAndMethodNotAllowed(t *testing.T) {
	h, _ := newTestRouter(nil, fakePinger{})
	assert.Equal(t, http.StatusNotFound, send(h, http.MethodGet, "/nope", "").Code)
	assert.Equal(t, http.StatusMethodNotAllowed, send(h, http.MethodDelete, "/healthz", "").Code)
}

func TestAuthenticate(t *testing.T) {
	users := fakeUsers{
		1: {ID: 1, Role: domain.RoleCashier, IsActive: true},
		2: {ID: 2, Role: domain.RoleCashier, IsActive: false},
	}
	h, tokens := newTestRouter(users, fakePinger{})

	tests := []struct {
		name  string
		token string
		want  int
	}{
		{"missing token", "", http.StatusUnauthorized},
		{"garbage token", "not-a-jwt", http.StatusUnauthorized},
		{"disabled user", mustToken(t, tokens, 2, domain.RoleCashier), http.StatusUnauthorized},
		{"deleted user", mustToken(t, tokens, 99, domain.RoleCashier), http.StatusUnauthorized},
		// Passes authentication, then RequireRole rejects a cashier on /users.
		{"active cashier", mustToken(t, tokens, 1, domain.RoleCashier), http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, send(h, http.MethodGet, "/api/v1/users", tt.token).Code)
		})
	}
}

func TestRoleComesFromDatabase(t *testing.T) {
	// The token still says admin, but the user was demoted to cashier.
	users := fakeUsers{1: {ID: 1, Role: domain.RoleCashier, IsActive: true}}
	h, tokens := newTestRouter(users, fakePinger{})

	rec := send(h, http.MethodGet, "/api/v1/users", mustToken(t, tokens, 1, domain.RoleAdmin))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestLoginRateLimit(t *testing.T) {
	h, _ := newTestRouter(nil, fakePinger{})

	// An empty body fails validation (400) before reaching the auth service.
	for range 5 {
		assert.Equal(t, http.StatusBadRequest, send(h, http.MethodPost, "/api/v1/auth/login", "").Code)
	}
	rec := send(h, http.MethodPost, "/api/v1/auth/login", "")
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}

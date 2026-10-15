package handler

import (
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TonnPcst/martgo-backend/internal/auth"
	"github.com/TonnPcst/martgo-backend/internal/domain"
	mw "github.com/TonnPcst/martgo-backend/internal/middleware"
	"github.com/TonnPcst/martgo-backend/internal/session"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

type fakeSessions map[int64]session.State

func (f fakeSessions) Get(_ context.Context, id int64) (session.State, error) {
	s, ok := f[id]
	if !ok {
		return session.State{}, session.ErrNotFound
	}
	return s, nil
}

type brokenSessions struct{}

func (brokenSessions) Get(context.Context, int64) (session.State, error) {
	return session.State{}, errors.New("redis down")
}

// newTestRouter returns the router and the private key that plays the auth service.
func newTestRouter(t *testing.T, sessions mw.SessionLookup, pinger Pinger) (http.Handler, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	checks := []HealthCheck{{Name: "database", Pinger: pinger}}
	return NewRouter(logger, auth.NewVerifierFromKey(pub), sessions, checks, Services{}, RouterOptions{}), priv
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

// mustToken signs a token the way the auth service does.
func mustToken(t *testing.T, priv ed25519.PrivateKey, userID int64, role string, version int32) string {
	t.Helper()
	claims := auth.Claims{
		UserID:       userID,
		Role:         role,
		TokenVersion: version,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "martgo-auth",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(priv)
	require.NoError(t, err)
	return signed
}

func TestHealthz(t *testing.T) {
	h, _ := newTestRouter(t, fakeSessions{}, fakePinger{})
	assert.Equal(t, http.StatusOK, send(h, http.MethodGet, "/healthz", "").Code)

	h, _ = newTestRouter(t, fakeSessions{}, fakePinger{err: errors.New("db down")})
	assert.Equal(t, http.StatusServiceUnavailable, send(h, http.MethodGet, "/healthz", "").Code)
}

func TestHealthzReportsRedisDown(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	checks := []HealthCheck{
		{Name: "database", Pinger: fakePinger{}},
		{Name: "redis", Pinger: PingFunc(func(context.Context) error { return errors.New("connection refused") })},
	}
	h := NewRouter(logger, auth.NewVerifierFromKey(priv.Public().(ed25519.PublicKey)), fakeSessions{}, checks, Services{}, RouterOptions{})

	rec := send(h, http.MethodGet, "/healthz", "")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "redis unavailable")
}

func TestNotFoundAndMethodNotAllowed(t *testing.T) {
	h, _ := newTestRouter(t, fakeSessions{}, fakePinger{})
	assert.Equal(t, http.StatusNotFound, send(h, http.MethodGet, "/nope", "").Code)
	assert.Equal(t, http.StatusMethodNotAllowed, send(h, http.MethodDelete, "/healthz", "").Code)
}

func TestAuthRoutesMovedOut(t *testing.T) {
	h, _ := newTestRouter(t, fakeSessions{}, fakePinger{})
	assert.Equal(t, http.StatusNotFound, send(h, http.MethodPost, "/api/v1/auth/login", "").Code)
}

func TestAuthenticate(t *testing.T) {
	sessions := fakeSessions{
		1: {TokenVersion: 1, Role: domain.RoleCashier, Active: true},
		2: {TokenVersion: 1, Role: domain.RoleCashier, Active: false},
		3: {TokenVersion: 2, Role: domain.RoleCashier, Active: true},
	}
	h, priv := newTestRouter(t, sessions, fakePinger{})
	_, otherKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	tests := []struct {
		name  string
		token string
		want  int
	}{
		{"missing token", "", http.StatusUnauthorized},
		{"garbage token", "not-a-jwt", http.StatusUnauthorized},
		{"signed by another key", mustToken(t, otherKey, 1, domain.RoleCashier, 1), http.StatusUnauthorized},
		{"disabled user", mustToken(t, priv, 2, domain.RoleCashier, 1), http.StatusUnauthorized},
		{"no session", mustToken(t, priv, 99, domain.RoleCashier, 1), http.StatusUnauthorized},
		{"revoked token", mustToken(t, priv, 3, domain.RoleCashier, 1), http.StatusUnauthorized},
		// Passes authentication, then RequireRole rejects a cashier on /suppliers.
		{"active cashier", mustToken(t, priv, 1, domain.RoleCashier, 1), http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, send(h, http.MethodGet, "/api/v1/suppliers", tt.token).Code)
		})
	}
}

func TestAuthorizationHeaderScheme(t *testing.T) {
	sessions := fakeSessions{1: {TokenVersion: 1, Role: domain.RoleCashier, Active: true}}
	h, priv := newTestRouter(t, sessions, fakePinger{})
	token := mustToken(t, priv, 1, domain.RoleCashier, 1)

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"basic scheme", "Basic " + token, http.StatusUnauthorized},
		{"no scheme", token, http.StatusUnauthorized},
		{"empty bearer", "Bearer ", http.StatusUnauthorized},
		// The scheme is case-insensitive; a valid cashier then fails RequireRole.
		{"lowercase bearer", "bearer " + token, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/suppliers", nil)
			req.Header.Set("Authorization", tt.header)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			assert.Equal(t, tt.want, rec.Code)
		})
	}
}

func TestRoleComesFromSession(t *testing.T) {
	// The token still says admin, but the auth service has since demoted the user.
	sessions := fakeSessions{1: {TokenVersion: 1, Role: domain.RoleCashier, Active: true}}
	h, priv := newTestRouter(t, sessions, fakePinger{})

	rec := send(h, http.MethodGet, "/api/v1/suppliers", mustToken(t, priv, 1, domain.RoleAdmin, 1))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestSessionStoreDownFailsClosed(t *testing.T) {
	h, priv := newTestRouter(t, brokenSessions{}, fakePinger{})

	rec := send(h, http.MethodGet, "/api/v1/suppliers", mustToken(t, priv, 1, domain.RoleAdmin, 1))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

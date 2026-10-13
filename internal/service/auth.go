package service

import (
	"context"
	"sync"
	"time"

	"github.com/TonnPcst/martgo-backend/internal/apperr"
	"github.com/TonnPcst/martgo-backend/internal/auth"
	"github.com/TonnPcst/martgo-backend/internal/db"
	"github.com/TonnPcst/martgo-backend/internal/store"
)

type AuthService struct {
	store  store.Store
	tokens *auth.TokenManager
	guard  *loginGuard
}

func NewAuthService(s store.Store, tokens *auth.TokenManager) *AuthService {
	return &AuthService{store: s, tokens: tokens, guard: newLoginGuard(5, 15*time.Minute)}
}

type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	User      db.User
}

var (
	errInvalidCredentials = apperr.Unauthorized("invalid email or password")
	errTooManyAttempts    = apperr.TooManyRequests("too many failed login attempts, please try again later")
)

var dummyHash = sync.OnceValue(func() string {
	hash, _ := auth.HashPassword("martgo-timing-dummy")
	return hash
})

func (s *AuthService) Login(ctx context.Context, email string, password string) (LoginResult, error) {
	key := normalizeEmail(email)
	now := time.Now()
	if s.guard.locked(key, now) {
		return LoginResult{}, errTooManyAttempts
	}

	user, err := s.store.GetUserByEmail(ctx, key)
	if err != nil {
		if store.IsNotFound(err) {
			auth.CheckPassword(dummyHash(), password)
			s.guard.fail(key, now)
			return LoginResult{}, errInvalidCredentials
		}
		return LoginResult{}, err
	}
	if !auth.CheckPassword(user.PasswordHash, password) || !user.IsActive {
		s.guard.fail(key, now)
		return LoginResult{}, errInvalidCredentials
	}
	s.guard.reset(key)

	token, expiresAt, err := s.tokens.Generate(user.ID, user.Role, user.TokenVersion)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, ExpiresAt: expiresAt, User: user}, nil
}

// Logout revokes every token the user currently holds, on all devices.
func (s *AuthService) Logout(ctx context.Context, userID int64) error {
	return s.store.BumpTokenVersion(ctx, userID)
}

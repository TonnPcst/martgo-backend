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
}

func NewAuthService(s store.Store, tokens *auth.TokenManager) *AuthService {
	return &AuthService{store: s, tokens: tokens}
}

type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	User      db.User
}

var errInvalidCredentials = apperr.Unauthorized("invalid email or password")

var dummyHash = sync.OnceValue(func() string {
	hash, _ := auth.HashPassword("martgo-timing-dummy")
	return hash
})

func (s *AuthService) Login(ctx context.Context, email string, password string) (LoginResult, error) {
	user, err := s.store.GetUserByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if store.IsNotFound(err) {
			auth.CheckPassword(dummyHash(), password)
			return LoginResult{}, errInvalidCredentials
		}
		return LoginResult{}, err
	}
	if !auth.CheckPassword(user.PasswordHash, password) || !user.IsActive {
		return LoginResult{}, errInvalidCredentials
	}

	token, expiresAt, err := s.tokens.Generate(user.ID, user.Role)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, ExpiresAt: expiresAt, User: user}, nil
}

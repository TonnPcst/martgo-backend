package auth

import (
	"context"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const issuer = "martgo"

type Claims struct {
	UserID       int64  `json:"uid"`
	Role         string `json:"role"`
	TokenVersion int32  `json:"ver"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), ttl: ttl}
}

func (m *TokenManager) Generate(userID int64, role string, version int32) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(m.ttl)
	claims := Claims{
		UserID:       userID,
		Role:         role,
		TokenVersion: version,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

func (m *TokenManager) Parse(token string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(t *jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

type claimsKey struct{}

func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, c)
}

func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsKey{}).(*Claims)
	return c, ok
}

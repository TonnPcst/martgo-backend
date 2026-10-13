package auth

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// issuer must match the auth service (martgo-auth: internal/token.Issuer).
const issuer = "martgo-auth"

// Claims mirrors the auth service's token contract: keep the JSON tags identical on both sides.
type Claims struct {
	UserID       int64  `json:"uid"`
	Role         string `json:"role"`
	TokenVersion int32  `json:"ver"`
	jwt.RegisteredClaims
}

// Verifier checks tokens issued by the auth service. It only holds the public
// key, so this service cannot mint tokens.
type Verifier struct {
	pub ed25519.PublicKey
}

func NewVerifier(pubPEM []byte) (*Verifier, error) {
	key, err := jwt.ParseEdPublicKeyFromPEM(pubPEM)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	pub, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("public key is not ed25519")
	}
	return NewVerifierFromKey(pub), nil
}

func NewVerifierFromKey(pub ed25519.PublicKey) *Verifier { return &Verifier{pub: pub} }

func (v *Verifier) Parse(token string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(t *jwt.Token) (any, error) { return v.pub, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
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

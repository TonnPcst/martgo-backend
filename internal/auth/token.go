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

// NewVerifier creates a token verifier from a PEM-encoded Ed25519 public key.
//
// ENG: This service only holds the public key, so it can check tokens from the auth
// service but can never create them.
// LAO: ສ້າງຕົວກວດສອບ token ຈາກ public key (Ed25519) — ກວດໄດ້ຢ່າງດຽວ, ສ້າງ token ບໍ່ໄດ້.
//
// @param pubPEM - contents of the public key PEM file
// @returns the verifier, or an error if the key cannot be parsed or is not Ed25519
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

// NewVerifierFromKey creates a verifier from an already parsed public key.
//
// ENG: Mainly used by tests that generate a key pair in memory.
// LAO: ສ້າງຕົວກວດສອບ token ຈາກ public key ທີ່ parse ແລ້ວ (ໃຊ້ໃນ test).
//
// @param pub - Ed25519 public key
// @returns the verifier
func NewVerifierFromKey(pub ed25519.PublicKey) *Verifier { return &Verifier{pub: pub} }

// Parse verifies a JWT and returns its claims.
//
// ENG: Only accepts EdDSA-signed tokens issued by "martgo-auth" that have an expiry
// and have not expired. This only checks the signature; revocation is checked
// separately against the Redis session.
// LAO: ກວດສອບ JWT (ລາຍເຊັນ, ຜູ້ອອກ, ວັນໝົດອາຍຸ) ແລ້ວສົ່ງຄືນ claims.
//
// @param token - raw JWT string without the "Bearer " prefix
// @returns the claims, or an error if the token is invalid or expired
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

// WithClaims stores the token claims in the context.
//
// ENG: Called by the Authenticate middleware after the token is verified.
// LAO: ເກັບ claims ຂອງ token ໄວ້ໃນ context.
//
// @param ctx - parent context
// @param c - verified claims
// @returns a new context holding the claims
func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, c)
}

// ClaimsFromContext reads the token claims stored by WithClaims.
//
// ENG: ok is false for requests that did not go through Authenticate.
// LAO: ດຶງ claims ຂອງ token ອອກຈາກ context.
//
// @param ctx - request context
// @returns the claims and true, or nil and false
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsKey{}).(*Claims)
	return c, ok
}

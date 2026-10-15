package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newKeys(t *testing.T) (*Verifier, ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	return NewVerifierFromKey(pub), pub, priv
}

func validClaims() Claims {
	return Claims{
		UserID:       7,
		Role:         "manager",
		TokenVersion: 3,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
}

func sign(t *testing.T, method jwt.SigningMethod, claims Claims, key any) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, claims).SignedString(key)
	require.NoError(t, err)
	return s
}

func TestParseValidToken(t *testing.T) {
	v, _, priv := newKeys(t)

	got, err := v.Parse(sign(t, jwt.SigningMethodEdDSA, validClaims(), priv))
	require.NoError(t, err)
	assert.Equal(t, int64(7), got.UserID)
	assert.Equal(t, "manager", got.Role)
	assert.Equal(t, int32(3), got.TokenVersion)
}

func TestParseRejects(t *testing.T) {
	v, pub, priv := newKeys(t)

	expired := validClaims()
	expired.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))

	noExpiry := validClaims()
	noExpiry.ExpiresAt = nil

	wrongIssuer := validClaims()
	wrongIssuer.Issuer = "someone-else"

	tests := []struct {
		name  string
		token string
	}{
		{"expired", sign(t, jwt.SigningMethodEdDSA, expired, priv)},
		{"no expiry", sign(t, jwt.SigningMethodEdDSA, noExpiry, priv)},
		{"wrong issuer", sign(t, jwt.SigningMethodEdDSA, wrongIssuer, priv)},
		// Algorithm confusion: an attacker signs with HMAC using the public key as the secret.
		{"hs256 with public key", sign(t, jwt.SigningMethodHS256, validClaims(), []byte(pub))},
		{"alg none", sign(t, jwt.SigningMethodNone, validClaims(), jwt.UnsafeAllowNoneSignatureType)},
		{"tampered payload", sign(t, jwt.SigningMethodEdDSA, validClaims(), priv) + "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := v.Parse(tt.token)
			assert.Error(t, err)
		})
	}
}

func TestNewVerifier(t *testing.T) {
	_, pub, _ := newKeys(t)
	der, err := x509.MarshalPKIXPublicKey(pub)
	require.NoError(t, err)
	_, err = NewVerifier(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	assert.NoError(t, err)

	_, err = NewVerifier([]byte("not a pem"))
	assert.Error(t, err)

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err = x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	require.NoError(t, err)
	_, err = NewVerifier(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	assert.Error(t, err, "an RSA key must not be accepted")
}

func TestClaimsContext(t *testing.T) {
	_, ok := ClaimsFromContext(t.Context())
	assert.False(t, ok)

	c := &Claims{UserID: 1}
	got, ok := ClaimsFromContext(WithClaims(t.Context(), c))
	require.True(t, ok)
	assert.Same(t, c, got)
}

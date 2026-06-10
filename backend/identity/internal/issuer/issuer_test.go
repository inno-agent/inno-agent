package issuer_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/inno-agent/identity/internal/issuer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makePrivateKeyPEM(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func TestIssuer_IssueAndVerify(t *testing.T) {
	keyPEM := makePrivateKeyPEM(t)
	iss, err := issuer.New(keyPEM, 30*time.Minute)
	require.NoError(t, err)

	token, err := iss.Issue("user-uuid-123", "premium", 7)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := iss.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, "user-uuid-123", claims.UserID)
	assert.Equal(t, "premium", claims.Tier)
	assert.Equal(t, int32(7), claims.CtxVersion)
}

func TestIssuer_VerifyExpired(t *testing.T) {
	keyPEM := makePrivateKeyPEM(t)
	iss, err := issuer.New(keyPEM, -time.Second) // already expired
	require.NoError(t, err)

	token, err := iss.Issue("user-123", "user", 0)
	require.NoError(t, err)

	_, err = iss.Verify(token)
	require.Error(t, err)
}

func TestIssuer_InvalidPEM(t *testing.T) {
	_, err := issuer.New([]byte("not-a-pem"), 30*time.Minute)
	require.Error(t, err)
}

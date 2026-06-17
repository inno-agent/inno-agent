package transport_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net"
	"testing"
	"time"

	"github.com/inno-agent/identity/internal/issuer"
	"github.com/inno-agent/identity/internal/transport"
	identityv1 "github.com/inno-agent/identity/proto/identity/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func makeTestIssuer(t *testing.T) *issuer.Issuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	iss, err := issuer.New(pemBytes, 30*time.Minute)
	require.NoError(t, err)
	return iss
}

func startGRPCServer(t *testing.T, iss *issuer.Issuer) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	identityv1.RegisterIdentityServiceServer(srv, transport.NewGRPCServer(iss))

	go func() { srv.Serve(lis) }() //nolint:errcheck,gosec
	t.Cleanup(func() { srv.GracefulStop() })

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestGRPC_ValidateToken_Valid(t *testing.T) {
	iss := makeTestIssuer(t)
	token, err := iss.Issue("user-xyz")
	require.NoError(t, err)

	conn := startGRPCServer(t, iss)
	client := identityv1.NewIdentityServiceClient(conn)

	resp, err := client.ValidateToken(context.Background(), &identityv1.ValidateTokenRequest{Token: token})
	require.NoError(t, err)
	assert.Equal(t, "user-xyz", resp.UserId)
}

func TestGRPC_ValidateToken_Invalid(t *testing.T) {
	iss := makeTestIssuer(t)
	conn := startGRPCServer(t, iss)
	client := identityv1.NewIdentityServiceClient(conn)

	_, err := client.ValidateToken(context.Background(), &identityv1.ValidateTokenRequest{Token: "bad.token.here"})
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

package transport

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/inno-agent/identity/internal/issuer"
	identityv1 "github.com/inno-agent/identity/proto/identity/v1"
)

type grpcServer struct {
	identityv1.UnimplementedIdentityServiceServer
	iss *issuer.Issuer
}

func NewGRPCServer(iss *issuer.Issuer) identityv1.IdentityServiceServer {
	return &grpcServer{iss: iss}
}

func (s *grpcServer) ValidateToken(_ context.Context, req *identityv1.ValidateTokenRequest) (*identityv1.UserInfo, error) {
	claims, err := s.iss.Verify(req.Token)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}
	return &identityv1.UserInfo{
		UserId: claims.UserID,
	}, nil
}

package transport

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/inno-agent/identity/internal/issuer"
	"github.com/inno-agent/identity/internal/user"
	identityv1 "github.com/inno-agent/identity/proto/identity/v1"
)

// UserServicer is the subset of user.Service used by the gRPC server.
type UserServicer interface {
	GetContext(ctx context.Context, userID string) (user.UserContext, error)
	UpdateContext(ctx context.Context, userID string, data []byte) error
}

type grpcServer struct {
	identityv1.UnimplementedIdentityServiceServer
	iss     *issuer.Issuer
	userSvc UserServicer
}

func NewGRPCServer(iss *issuer.Issuer, userSvc UserServicer) identityv1.IdentityServiceServer {
	return &grpcServer{iss: iss, userSvc: userSvc}
}

func (s *grpcServer) ValidateToken(_ context.Context, req *identityv1.ValidateTokenRequest) (*identityv1.UserInfo, error) {
	claims, err := s.iss.Verify(req.Token)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}
	return &identityv1.UserInfo{
		UserId:     claims.UserID,
		Tier:       claims.Tier,
		CtxVersion: claims.CtxVersion,
	}, nil
}

func (s *grpcServer) GetUserContext(ctx context.Context, req *identityv1.GetUserContextRequest) (*identityv1.UserContext, error) {
	if s.userSvc == nil {
		return nil, status.Error(codes.Unimplemented, "user service not configured")
	}
	uctx, err := s.userSvc.GetContext(ctx, req.UserId)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "user %s not found", req.UserId)
		}
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &identityv1.UserContext{
		UserId:  req.UserId,
		Data:    uctx.Data,
		Version: uctx.Version,
	}, nil
}

func (s *grpcServer) UpdateUserContext(ctx context.Context, req *identityv1.UpdateUserContextRequest) (*emptypb.Empty, error) {
	if s.userSvc == nil {
		return nil, status.Error(codes.Unimplemented, "user service not configured")
	}
	if err := s.userSvc.UpdateContext(ctx, req.UserId, req.Data); err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "user %s not found", req.UserId)
		}
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &emptypb.Empty{}, nil
}

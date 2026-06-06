package transport

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/inno-agent/auth/internal/issuer"
	"github.com/inno-agent/auth/internal/user"
	authv1 "github.com/inno-agent/auth/proto/auth/v1"
)

// UserServicer is the subset of user.Service used by the gRPC server.
type UserServicer interface {
	GetContext(ctx context.Context, userID string) (user.UserContext, error)
	UpdateContext(ctx context.Context, userID string, data []byte) error
}

type grpcServer struct {
	authv1.UnimplementedAuthServiceServer
	iss     *issuer.Issuer
	userSvc UserServicer
}

func NewGRPCServer(iss *issuer.Issuer, userSvc UserServicer) authv1.AuthServiceServer {
	return &grpcServer{iss: iss, userSvc: userSvc}
}

func (s *grpcServer) ValidateToken(_ context.Context, req *authv1.ValidateTokenRequest) (*authv1.UserInfo, error) {
	claims, err := s.iss.Verify(req.Token)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}
	return &authv1.UserInfo{
		UserId:     claims.UserID,
		Tier:       claims.Tier,
		CtxVersion: claims.CtxVersion,
	}, nil
}

func (s *grpcServer) GetUserContext(ctx context.Context, req *authv1.GetUserContextRequest) (*authv1.UserContext, error) {
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
	return &authv1.UserContext{
		UserId:  req.UserId,
		Data:    uctx.Data,
		Version: uctx.Version,
	}, nil
}

func (s *grpcServer) UpdateUserContext(ctx context.Context, req *authv1.UpdateUserContextRequest) (*emptypb.Empty, error) {
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

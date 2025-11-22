package authClient

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/auth"

	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

type (
	AuthResponse          = auth.AuthResponse
	EmptyResponse         = auth.EmptyResponse
	LoginRequest          = auth.LoginRequest
	LogoutRequest         = auth.LogoutRequest
	RefreshRequest        = auth.RefreshRequest
	RegisterRequest       = auth.RegisterRequest
	User                  = auth.User
	ValidateTokenRequest  = auth.ValidateTokenRequest
	ValidateTokenResponse = auth.ValidateTokenResponse

	Auth interface {
		Register(ctx context.Context, in *RegisterRequest, opts ...grpc.CallOption) (*AuthResponse, error)
		Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (*AuthResponse, error)
		RefreshToken(ctx context.Context, in *RefreshRequest, opts ...grpc.CallOption) (*AuthResponse, error)
		Logout(ctx context.Context, in *LogoutRequest, opts ...grpc.CallOption) (*EmptyResponse, error)
		ValidateToken(ctx context.Context, in *ValidateTokenRequest, opts ...grpc.CallOption) (*ValidateTokenResponse, error)
	}

	defaultAuth struct {
		cli zrpc.Client
	}
)

func NewAuth(cli zrpc.Client) Auth {
	return &defaultAuth{
		cli: cli,
	}
}

func (m *defaultAuth) Register(ctx context.Context, in *RegisterRequest, opts ...grpc.CallOption) (*AuthResponse, error) {
	client := auth.NewAuthServiceClient(m.cli.Conn())
	return client.Register(ctx, in, opts...)
}

func (m *defaultAuth) Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (*AuthResponse, error) {
	client := auth.NewAuthServiceClient(m.cli.Conn())
	return client.Login(ctx, in, opts...)
}

func (m *defaultAuth) RefreshToken(ctx context.Context, in *RefreshRequest, opts ...grpc.CallOption) (*AuthResponse, error) {
	client := auth.NewAuthServiceClient(m.cli.Conn())
	return client.RefreshToken(ctx, in, opts...)
}

func (m *defaultAuth) Logout(ctx context.Context, in *LogoutRequest, opts ...grpc.CallOption) (*EmptyResponse, error) {
	client := auth.NewAuthServiceClient(m.cli.Conn())
	return client.Logout(ctx, in, opts...)
}

func (m *defaultAuth) ValidateToken(ctx context.Context, in *ValidateTokenRequest, opts ...grpc.CallOption) (*ValidateTokenResponse, error) {
	client := auth.NewAuthServiceClient(m.cli.Conn())
	return client.ValidateToken(ctx, in, opts...)
}

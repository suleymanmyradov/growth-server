package logic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/validator"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LoginLogic) Login(in *auth.LoginRequest) (*auth.AuthResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "LoginLogic.Login")
	defer span.End()

	l.Infof("Login attempt for email: %s", in.Email)

	if in == nil || in.Email == "" || in.Password == "" {
		l.Errorf("Login validation failed: email and password are required")
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	if !validator.IsValidEmail(in.Email) {
		l.Errorf("Login validation failed: invalid email format: %s", in.Email)
		return nil, status.Error(codes.InvalidArgument, "invalid email format")
	}

	user, err := l.svcCtx.Repo.Users.GetUserByEmail(ctx, in.Email)
	if err != nil {
		l.Errorf("Login failed to get user by email: %v", err)
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password))
	if err != nil {
		l.Errorf("Login password mismatch for user %s: %v", user.ID, err)
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	sessionID := uuid.New()

	accessToken, err := l.svcCtx.TokenMaker.CreateAccessToken(ctx, user.ID, user.Username, []string{"user"}, sessionID)
	if err != nil {
		l.Errorf("Login failed to create access token for user %s: %v", user.ID, err)
		return nil, status.Error(codes.Internal, "failed to generate access token")
	}

	refreshToken, err := l.svcCtx.TokenMaker.CreateRefreshToken(ctx, user.ID, user.Username, []string{"user"}, sessionID)
	if err != nil {
		l.Errorf("Login failed to create refresh token for user %s: %v", user.ID, err)
		return nil, status.Error(codes.Internal, "failed to generate refresh token")
	}

	l.Infof("Login successful for user %s", user.ID)

	return &auth.AuthResponse{
		AccessToken:  accessToken.Token,
		RefreshToken: refreshToken.Token,
		ExpiresIn:    int64(l.svcCtx.Config.JWT.AccessExpiryDuration.Seconds()),
		User:         toPbUser(user),
	}, nil
}

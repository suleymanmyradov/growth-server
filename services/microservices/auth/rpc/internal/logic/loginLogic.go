package logic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/validator"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"github.com/zeromicro/go-zero/core/logx"
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
	if in == nil || in.Email == "" || in.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	if !validator.IsValidEmail(in.Email) {
		return nil, status.Error(codes.InvalidArgument, "invalid email format")
	}

	user, err := l.svcCtx.Repo.Users.GetUserByEmail(l.ctx, in.Email)
	if err != nil {
		l.Errorf("failed to get user by email: %v", err)
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password))
	if err != nil {
		l.Errorf("password mismatch: %v", err)
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	profile, err := l.svcCtx.Repo.Profiles.GetProfileByUserID(l.ctx, user.ID)
	if err != nil {
		l.Errorf("failed to get profile: %v", err)
		return nil, status.Error(codes.Internal, "failed to get profile")
	}

	sessionID := uuid.New()

	accessToken, err := l.svcCtx.TokenMaker.CreateAccessToken(l.ctx, user.ID, user.Username, []string{"user"}, sessionID)
	if err != nil {
		l.Errorf("failed to create access token: %v", err)
		return nil, status.Error(codes.Internal, "failed to generate access token")
	}

	refreshToken, err := l.svcCtx.TokenMaker.CreateRefreshToken(l.ctx, user.ID, user.Username, sessionID)
	if err != nil {
		l.Errorf("failed to create refresh token: %v", err)
		return nil, status.Error(codes.Internal, "failed to generate refresh token")
	}

	return &auth.AuthResponse{
		AccessToken:  accessToken.Token,
		RefreshToken: refreshToken.Token,
		ExpiresIn:    int64(l.svcCtx.Config.JWT.AccessExpiryDuration.Seconds()),
		User:         toPbUser(user, profile),
	}, nil
}

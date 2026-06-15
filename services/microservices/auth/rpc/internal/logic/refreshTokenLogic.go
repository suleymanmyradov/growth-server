package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RefreshTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefreshTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshTokenLogic {
	return &RefreshTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RefreshTokenLogic) RefreshToken(in *auth.RefreshRequest) (*auth.AuthResponse, error) {
	if in == nil || in.RefreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh token is required")
	}

	refreshClaims, err := l.svcCtx.TokenMaker.VerifyRefreshToken(l.ctx, in.RefreshToken)
	if err != nil {
		l.Errorf("failed to verify refresh token: %v", err)
		return nil, status.Error(codes.Unauthenticated, "invalid or expired refresh token")
	}

	userID := refreshClaims.Subject
	user, err := l.svcCtx.Repo.Users.GetUserByID(l.ctx, userID)
	if err != nil {
		l.Errorf("failed to get user: %v", err)
		return nil, status.Error(codes.NotFound, "user not found")
	}

	sessionID := refreshClaims.SessionID
	accessToken, err := l.svcCtx.TokenMaker.CreateAccessToken(l.ctx, user.ID, user.Username, []string{"user"}, sessionID)
	if err != nil {
		l.Errorf("failed to create access token: %v", err)
		return nil, status.Error(codes.Internal, "failed to generate access token")
	}

	newRefreshToken, err := l.svcCtx.TokenMaker.RotateRefreshToken(l.ctx, in.RefreshToken)
	if err != nil {
		l.Errorf("failed to rotate refresh token: %v", err)
		return nil, status.Error(codes.Internal, "failed to generate refresh token")
	}

	return &auth.AuthResponse{
		AccessToken:  accessToken.Token,
		RefreshToken: newRefreshToken.Token,
		ExpiresIn:    int64(l.svcCtx.Config.JWT.AccessExpiryDuration.Seconds()),
		User:         toPbUser(user),
	}, nil
}

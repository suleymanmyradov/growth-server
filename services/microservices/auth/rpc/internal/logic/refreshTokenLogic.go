package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "RefreshTokenLogic.RefreshToken")
	defer span.End()

	l.Infof("RefreshToken attempt")

	if in == nil || in.RefreshToken == "" {
		l.Errorf("RefreshToken validation failed: refresh token is required")
		return nil, status.Error(codes.InvalidArgument, "refresh token is required")
	}

	refreshClaims, err := l.svcCtx.TokenMaker.VerifyRefreshToken(ctx, in.RefreshToken)
	if err != nil {
		l.Errorf("RefreshToken failed to verify refresh token: %v", err)
		return nil, status.Error(codes.Unauthenticated, "invalid or expired refresh token")
	}

	userID := refreshClaims.Subject
	user, err := l.svcCtx.Repo.Users.GetUserByID(ctx, userID)
	if err != nil {
		l.Errorf("RefreshToken failed to get user %s: %v", userID, err)
		return nil, status.Error(codes.NotFound, "user not found")
	}

	sessionID := refreshClaims.SessionID
	accessToken, err := l.svcCtx.TokenMaker.CreateAccessToken(ctx, user.ID, user.Username, []string{"user"}, sessionID)
	if err != nil {
		l.Errorf("RefreshToken failed to create access token for user %s: %v", user.ID, err)
		return nil, status.Error(codes.Internal, "failed to generate access token")
	}

	newRefreshToken, err := l.svcCtx.TokenMaker.RotateRefreshToken(ctx, in.RefreshToken)
	if err != nil {
		l.Errorf("RefreshToken failed to rotate refresh token for user %s: %v", user.ID, err)
		return nil, status.Error(codes.Internal, "failed to generate refresh token")
	}

	l.Infof("RefreshToken successful for user %s", user.ID)

	return &auth.AuthResponse{
		AccessToken:  accessToken.Token,
		RefreshToken: newRefreshToken.Token,
		ExpiresIn:    int64(l.svcCtx.Config.JWT.AccessExpiryDuration.Seconds()),
		User:         toPbUser(user),
	}, nil
}

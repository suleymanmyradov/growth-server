// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/suleymanmyradov/growth-server/backend/services/gateway/internal/model"
	"github.com/suleymanmyradov/growth-server/backend/services/gateway/internal/svc"
	"github.com/suleymanmyradov/growth-server/backend/services/gateway/internal/types"
)

type RefreshTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefreshTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshTokenLogic {
	return &RefreshTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RefreshTokenLogic) RefreshToken(req *types.RefreshRequest) (resp *types.AuthResponse, err error) {
	// Validate input
	if req.RefreshToken == "" {
		return nil, errors.New("refresh token is required")
	}

	// Validate refresh token
	claims, err := l.svcCtx.JWTMiddleware.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	// Get user by ID
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, errors.New("invalid user ID in token")
	}

	user, err := l.svcCtx.UserRepo.GetUserByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Generate new JWT tokens
	accessToken, err := l.svcCtx.JWTMiddleware.GenerateAccessToken(
		user.ID.String(),
		user.Username,
		user.Email,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	newRefreshToken, err := l.svcCtx.JWTMiddleware.GenerateRefreshToken(user.ID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Get user profile
	profile, err := l.svcCtx.ProfileRepo.GetProfileByUserID(user.ID)
	if err != nil {
		logx.Errorf("Failed to get user profile: %v", err)
		// Continue without profile if it fails
		profile = &model.Profile{UserID: user.ID}
	}

	// Build response
	resp = &types.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    l.svcCtx.Config.Auth.AccessExpire,
		User: types.Profile{
			Id:        user.ID.String(),
			FullName:  user.FullName,
			Username:  user.Username,
			Email:     user.Email,
			Bio:       stringPtrToString(profile.Bio),
			Location:  stringPtrToString(profile.Location),
			Website:   stringPtrToString(profile.Website),
			Interests: []string(profile.Interests),
			AvatarUrl: stringPtrToString(profile.AvatarUrl),
			CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: user.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	}

	return resp, nil
}

func stringPtrToString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

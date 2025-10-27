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

type RegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegisterLogic) Register(req *types.RegisterRequest) (resp *types.AuthResponse, err error) {
	// Validate input
	if req.Username == "" || req.Email == "" || req.Password == "" || req.FullName == "" {
		return nil, errors.New("username, email, password, and full name are required")
	}

	// Check if email already exists
	emailExists, err := l.svcCtx.UserRepo.CheckEmailExists(req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check email existence: %w", err)
	}
	if emailExists {
		return nil, errors.New("email already exists")
	}

	// Check if username already exists
	usernameExists, err := l.svcCtx.UserRepo.CheckUsernameExists(req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to check username existence: %w", err)
	}
	if usernameExists {
		return nil, errors.New("username already exists")
	}

	// Hash password
	hashedPassword, err := model.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &model.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hashedPassword,
		FullName:     req.FullName,
	}

	err = l.svcCtx.UserRepo.CreateUser(user)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create profile for the user
	err = l.svcCtx.ProfileRepo.EnsureProfileExists(user.ID)
	if err != nil {
		logx.Errorf("Failed to create profile for user %s: %v", user.ID, err)
		// Don't fail the registration if profile creation fails, just log it
	}

	// Generate JWT tokens
	accessToken, err := l.svcCtx.JWTMiddleware.GenerateAccessToken(
		user.ID.String(),
		user.Username,
		user.Email,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := l.svcCtx.JWTMiddleware.GenerateRefreshToken(user.ID.String())
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
		RefreshToken: refreshToken,
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

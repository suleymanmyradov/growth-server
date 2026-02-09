package logic

import (
	"context"
	"errors"

	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Core authentication
func (l *RegisterLogic) Register(in *auth.RegisterRequest) (*auth.AuthResponse, error) {
	if in == nil {
		return nil, errors.New("request is nil")
	}

	user, err := l.svcCtx.Repo.Users.CreateUser(l.ctx, db.CreateUserParams{
		Username:     in.Username,
		Email:        in.Email,
		PasswordHash: in.Password,
		FullName:     in.FullName,
	})
	if err != nil {
		l.Logger.Errorf("failed to create user: %v", err)
		return nil, err
	}

	profile, err := l.svcCtx.Repo.Profiles.CreateProfile(l.ctx, db.CreateProfileParams{
		UserID:    user.ID,
		Bio:       toNullString(""),
		Location:  toNullString(""),
		Website:   toNullString(""),
		Interests: []string{},
		AvatarUrl: toNullString(""),
	})
	if err != nil {
		l.Logger.Errorf("failed to create profile: %v", err)
		return nil, err
	}

	return &auth.AuthResponse{
		AccessToken:  "", // token generation not implemented yet
		RefreshToken: "",
		ExpiresIn:    0,
		User:         toPbUser(user, profile),
	}, nil
}

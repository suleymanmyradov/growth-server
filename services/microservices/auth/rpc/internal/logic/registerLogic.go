package logic

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/suleymanmyradov/growth-server/pkg/validator"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func (l *RegisterLogic) Register(in *auth.RegisterRequest) (*auth.AuthResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}

	if in.Username == "" || in.Email == "" || in.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "username, email and password are required")
	}

	if !validator.IsValidEmail(in.Email) {
		return nil, status.Error(codes.InvalidArgument, "invalid email format")
	}

	if !validator.IsStrongPassword(in.Password) {
		return nil, status.Error(codes.InvalidArgument, "password must be at least 8 characters and contain uppercase, lowercase, number, and special character")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		l.Errorf("failed to hash password: %v", err)
		return nil, status.Error(codes.Internal, "failed to process password")
	}

	user, err := l.svcCtx.Repo.Users.CreateUser(l.ctx, db.CreateUserParams{
		Username:     in.Username,
		Email:        in.Email,
		PasswordHash: string(hashedPassword),
		FullName:     in.FullName,
	})
	if err != nil {
		l.Errorf("failed to create user: %v", err)
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			// Unique constraint violation - email or username already exists
			return nil, status.Error(codes.AlreadyExists, "user already exists")
		}
		return nil, status.Error(codes.Internal, "failed to create user")
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
		l.Errorf("failed to create profile: %v", err)
		return nil, status.Error(codes.Internal, "failed to create profile")
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

package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/auth"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/shared/models"
	"github.com/suleymanmyradov/growth-server/shared/repository"

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

func (l *RegisterLogic) Register(in *auth.RegisterRequest) (*auth.AuthResponse, error) {
	// 1. Check if user already exists
	var existingUser models.User
	err := l.svcCtx.Repo.GetByID(l.ctx, &existingUser, repository.SelectUserByEmailQuery, in.Email)
	if err == nil {
		return nil, fmt.Errorf("user with email %s already exists", in.Email)
	}

	// 2. Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// 3. Create user
	userId := uuid.New()
	now := time.Now()
	user := models.User{
		BaseModel: models.BaseModel{
			ID:        userId,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Username:     in.Username,
		Email:        in.Email,
		PasswordHash: string(hashedPassword),
		FullName:     in.FullName,
	}

	// 4. Create profile
	profile := models.Profile{
		BaseModel: models.BaseModel{
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
		},
		UserID: userId,
	}

	// 5. Transaction to save user and profile
	err = l.svcCtx.Repo.Transaction(l.ctx, func(tx *sqlx.Tx) error {
		_, err := tx.NamedExecContext(l.ctx, repository.InsertUserQuery, user)
		if err != nil {
			return err
		}
		_, err = tx.NamedExecContext(l.ctx, repository.InsertProfileQuery, profile)
		return err
	})
	if err != nil {
		return nil, err
	}

	// 6. Generate JWT
	nowUnix := now.Unix()
	accessExpire := l.svcCtx.Config.Auth.AccessExpire
	jwtToken, err := l.getJwtToken(l.svcCtx.Config.Auth.AccessSecret, nowUnix, accessExpire, userId.String())
	if err != nil {
		return nil, err
	}

	return &auth.AuthResponse{
		AccessToken:  jwtToken,
		ExpiresIn:    accessExpire,
		RefreshToken: "", // TODO: Implement refresh token
	}, nil
}

func (l *RegisterLogic) getJwtToken(secretKey string, iat, seconds int64, userId string) (string, error) {
	claims := make(jwt.MapClaims)
	claims["exp"] = iat + seconds
	claims["iat"] = iat
	claims["userId"] = userId
	token := jwt.New(jwt.SigningMethodHS256)
	token.Claims = claims
	return token.SignedString([]byte(secretKey))
}

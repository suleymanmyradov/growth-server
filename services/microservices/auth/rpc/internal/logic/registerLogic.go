package logic

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/suleymanmyradov/growth-server/pkg/validator"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "RegisterLogic.Register")
	defer span.End()

	l.Infof("Register attempt for email: %s, username: %s", in.Email, in.Username)

	if in == nil {
		l.Errorf("Register validation failed: request is nil")
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}

	if in.Username == "" || in.Email == "" || in.Password == "" {
		l.Errorf("Register validation failed: username, email and password are required")
		return nil, status.Error(codes.InvalidArgument, "username, email and password are required")
	}

	if !validator.IsValidEmail(in.Email) {
		l.Errorf("Register validation failed: invalid email format: %s", in.Email)
		return nil, status.Error(codes.InvalidArgument, "invalid email format")
	}

	if !validator.IsStrongPassword(in.Password) {
		l.Errorf("Register validation failed: weak password for email: %s", in.Email)
		return nil, status.Error(codes.InvalidArgument, "password must be at least 8 characters and contain uppercase, lowercase, number, and special character")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		l.Errorf("Register failed to hash password: %v", err)
		return nil, status.Error(codes.Internal, "failed to process password")
	}

	var user db.User
	err = l.svcCtx.TxRunner.Run(ctx, "", func(tx pgx.Tx) error {
		q := db.New(tx)
		var err error
		user, err = q.CreateUser(ctx, in.Username, in.Email, string(hashedPassword), in.FullName)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				l.Errorf("Register failed: user already exists for email: %s", in.Email)
				return status.Error(codes.AlreadyExists, "user already exists")
			}
			l.Errorf("Register failed to create user: %v", err)
			return status.Error(codes.Internal, "failed to create user")
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	sessionID := uuid.New()

	accessToken, err := l.svcCtx.TokenMaker.CreateAccessToken(ctx, user.ID, user.Username, []string{"user"}, sessionID)
	if err != nil {
		l.Errorf("Register failed to create access token for user %s: %v", user.ID, err)
		return nil, status.Error(codes.Internal, "failed to generate access token")
	}

	refreshToken, err := l.svcCtx.TokenMaker.CreateRefreshToken(ctx, user.ID, user.Username, sessionID)
	if err != nil {
		l.Errorf("Register failed to create refresh token for user %s: %v", user.ID, err)
		return nil, status.Error(codes.Internal, "failed to generate refresh token")
	}

	l.Infof("Register successful for user %s", user.ID)

	return &auth.AuthResponse{
		AccessToken:  accessToken.Token,
		RefreshToken: refreshToken.Token,
		ExpiresIn:    int64(l.svcCtx.Config.JWT.AccessExpiryDuration.Seconds()),
		User:         toPbUser(user),
	}, nil
}

package logic

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/suleymanmyradov/growth-server/pkg/email"
	"github.com/suleymanmyradov/growth-server/pkg/validator"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const verificationTokenTTL = 1 * time.Hour

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

func (l *RegisterLogic) Register(in *auth.RegisterRequest) (*auth.RegisterResponse, error) {
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

	hashStr := string(hashedPassword)
	var user db.User
	err = l.svcCtx.TxRunner.Run(ctx, "", func(tx pgx.Tx) error {
		q := db.New(tx)
		row, err := q.CreateUser(ctx, db.CreateUserParams{
			Username:      in.Username,
			Email:         in.Email,
			PasswordHash:  &hashStr,
			FullName:      in.FullName,
			EmailVerified: false,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				l.Errorf("Register failed: user already exists for email: %s", in.Email)
				return status.Error(codes.AlreadyExists, "user already exists")
			}
			l.Errorf("Register failed to create user: %v", err)
			return status.Error(codes.Internal, "failed to create user")
		}
		user = db.User(row)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Generate and store an email verification token (Redis, 1h TTL).
	token := generateRandomToken(32)
	verificationRepo := repository.NewVerificationRepo(l.svcCtx.RedisClient)
	if err := verificationRepo.Store(ctx, token, user.ID.String(), user.Email, verificationTokenTTL); err != nil {
		l.Errorf("Register failed to store verification token for user %s: %v", user.ID, err)
		return nil, status.Error(codes.Internal, "failed to generate verification token")
	}
	if err := verificationRepo.SetThrottle(ctx, user.Email, 60*time.Second); err != nil {
		l.Errorf("Register failed to set verification throttle: %v", err)
	}

	verificationURL := l.svcCtx.Config.Email.FrontendBaseURL + "/verify-email?token=" + token
	if err := l.svcCtx.EmailSender.Send(ctx, email.Email{
		To:      []string{user.Email},
		Subject: "Verify your email",
		HTML:    emailVerificationHTML(user.FullName, verificationURL),
	}); err != nil {
		l.Errorf("Register failed to send verification email to %s: %v", user.Email, err)
		// Don't fail registration if email delivery fails — user can resend.
	}

	l.Infof("Register successful for user %s (pending email verification)", user.ID)

	return &auth.RegisterResponse{
		RequiresVerification: true,
		Message:              "Account created. Check your email for a verification link to activate your account.",
	}, nil
}

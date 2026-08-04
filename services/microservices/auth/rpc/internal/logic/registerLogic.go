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

	if in == nil {
		l.Errorf("Register validation failed: request is nil")
		return nil, errInvalidArgument(MsgRequestIsNil)
	}

	l.Infof("Register attempt for email: %s, username: %s", in.Email, in.Username)

	if in.Username == "" || in.Email == "" || in.Password == "" {
		l.Errorf("Register validation failed: username, email and password are required")
		return nil, errInvalidArgument(MsgUsernameEmailPasswordReq)
	}

	if !validator.IsValidUsername(in.Username) {
		l.Errorf("Register validation failed: invalid username format: %s", in.Username)
		return nil, errInvalidArgument(MsgUsernameFormat)
	}

	if !validator.IsValidEmail(in.Email) {
		l.Errorf("Register validation failed: invalid email format: %s", in.Email)
		return nil, errInvalidArgument(MsgInvalidEmailFormat)
	}

	if !validator.IsStrongPassword(in.Password) {
		l.Errorf("Register validation failed: weak password for email: %s", in.Email)
		return nil, errInvalidArgument(MsgPasswordStrength)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		l.Errorf("Register failed to hash password: %v", err)
		return nil, errInternal(MsgFailedProcessPassword)
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
				return ErrUserAlreadyExists
			}
			l.Errorf("Register failed to create user: %v", err)
			return errInternal(MsgFailedCreateUser)
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
		return nil, errInternal(MsgFailedGenerateVerifToken)
	}
	if err := verificationRepo.SetThrottle(ctx, user.Email, 60*time.Second); err != nil {
		l.Errorf("Register failed to set verification throttle: %v", err)
	}

	// Email sending and event publishing are external/side-effecting work
	// that must not be bounded by the short RPC deadline (2s) — a slow Resend
	// would otherwise cancel the Kafka publish and surface as a 504 even though
	// the user row is already committed. Detach from the request context so the
	// RPC can return immediately, and give the email send its own generous
	// timeout. Both best-effort: failures are logged, never returned.
	verificationURL := l.svcCtx.Config.Email.FrontendBaseURL + "/verify-email?token=" + token
	emailCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	if err := l.svcCtx.EmailSender.Send(emailCtx, email.Email{
		To:      []string{user.Email},
		Subject: "Verify your email",
		HTML:    emailVerificationHTML(user.FullName, verificationURL),
	}); err != nil {
		l.Errorf("Register failed to send verification email to %s: %v", user.Email, err)
		// Don't fail registration if email delivery fails — user can resend.
	}

	l.Infof("Register successful for user %s (pending email verification)", user.ID)

	// Publish so downstream services can seed their local user_profiles read model.
	publishUserProfileUpdated(context.WithoutCancel(ctx), l.svcCtx.EventsPub, user)

	return &auth.RegisterResponse{
		RequiresVerification: true,
		Message:              "Account created. Check your email for a verification link to activate your account.",
	}, nil
}

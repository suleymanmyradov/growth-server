//go:build integration

package logic

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/email"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"github.com/testcontainers/testcontainers-go"
	tcpg "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ============================================
// Integration tests with real Postgres + Redis
//
// Run with: go test -tags=integration ./services/microservices/auth/rpc/internal/logic/ -v -timeout=120s
// Requires Docker.
// ============================================

// noopEmailSender is a no-op email.Sender for integration tests.
type noopEmailSender struct{}

func (n *noopEmailSender) Send(_ context.Context, _ email.Email) error { return nil }

// integrationTestEnv holds containers and a ServiceContext for integration tests.
type integrationTestEnv struct {
	svcCtx   *svc.ServiceContext
	pgPool   *pgxpool.Pool
	redisCli *redis.Client
	pgC      testcontainers.Container
	redisC   testcontainers.Container
}

func setupIntegrationTestEnv(t *testing.T) *integrationTestEnv {
	t.Helper()
	ctx := context.Background()

	// Start Postgres container
	pgContainer, err := tcpg.Run(ctx,
		"postgres:16-alpine",
		tcpg.WithDatabase("testdb"),
		tcpg.WithUsername("test"),
		tcpg.WithPassword("test"),
		tcpg.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = pgContainer.Terminate(ctx)
	})

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Start Redis container
	redisContainer, err := tcredis.Run(ctx, "redis:7-alpine")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = redisContainer.Terminate(ctx)
	})

	redisHost, err := redisContainer.Host(ctx)
	require.NoError(t, err)
	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)

	// Run migrations
	runMigrations(t, connStr)

	// Open a connection pool
	pool := postgres.MustOpenPool(connStr, 10, 5, time.Hour)

	// Create Redis client
	redisCli := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort.Port()),
	})

	// Create repository
	queries := db.New(pool)
	repo := repository.NewRepository(queries)
	txRunner := postgres.NewPgxTxRunner(pool)

	// Create token maker with Redis-backed revocation repo
	tokenRepo, err := repository.NewCmdableRedisRepository(redisCli)
	require.NoError(t, err)

	tokenMaker, err := jwt.NewTokenMaker(jwt.Config{
		Secret:                "test-secret-at-least-32-characters-long-xxxxx",
		Issuer:                "test-issuer",
		Audience:              "test-audience",
		AccessExpiryDuration:  15 * time.Minute,
		RefreshExpiryDuration: 7 * 24 * time.Hour,
	}, tokenRepo)
	require.NoError(t, err)

	cfg := config.Config{}
	cfg.JWT.AccessExpiryDuration = 15 * time.Minute
	cfg.JWT.RefreshExpiryDuration = 7 * 24 * time.Hour
	cfg.Email.FrontendBaseURL = "http://localhost:3000"

	svcCtx := &svc.ServiceContext{
		Config:      cfg,
		Repo:        repo,
		TokenMaker:  tokenMaker,
		TxRunner:    txRunner,
		RedisClient: redisCli,
		EmailSender: &noopEmailSender{},
	}

	return &integrationTestEnv{
		svcCtx:   svcCtx,
		pgPool:   pool,
		redisCli: redisCli,
		pgC:      pgContainer,
		redisC:   redisContainer,
	}
}

// runMigrations reads and executes all .up.sql migration files in order.
func runMigrations(t *testing.T, connStr string) {
	t.Helper()
	ctx := context.Background()

	// Find migrations directory relative to the test file
	migrationsDir := findMigrationsDir(t)

	// Find all .up.sql files
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, files)

	// Sort by filename (migrations are prefixed with numbers)
	sort.Strings(files)

	// Connect to the database
	conn, err := pgx.Connect(ctx, connStr)
	require.NoError(t, err)
	defer conn.Close(ctx)

	// Execute each migration file as a single multi-statement query.
	// pgx v5 sends the entire string to Postgres which handles multiple
	// statements separated by semicolons in one SimpleProtocol query.
	for _, file := range files {
		sql, err := os.ReadFile(file)
		require.NoError(t, err)

		// Use Exec which sends the query via the simple protocol when there
		// are no parameters, allowing Postgres to handle multi-statement SQL.
		_, err = conn.Exec(ctx, string(sql))
		if err != nil {
			t.Fatalf("migration %s failed: %v", filepath.Base(file), err)
		}
	}
	t.Logf("ran %d migration files", len(files))
}

// findMigrationsDir locates the sql/migrations_v2 directory relative to the test file.
func findMigrationsDir(t *testing.T) string {
	t.Helper()
	// The test file is in services/microservices/auth/rpc/internal/logic/
	// Migrations are in sql/migrations_v2/ relative to the repo root
	dir, err := os.Getwd()
	require.NoError(t, err)

	// Walk up to find the repo root (where sql/migrations_v2 lives)
	for i := 0; i < 10; i++ {
		candidate := filepath.Join(dir, "sql", "migrations_v2")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not find sql/migrations_v2 directory")
	return ""
}

// ============================================
// Integration test: Register → Login → Refresh Token
// ============================================

func TestIntegration_RegisterLoginRefresh(t *testing.T) {
	env := setupIntegrationTestEnv(t)
	defer env.pgPool.Close()
	defer env.redisCli.Close()

	ctx := context.Background()
	uniqueEmail := fmt.Sprintf("integration-%d@example.com", time.Now().UnixNano())
	username := fmt.Sprintf("user%d", time.Now().UnixNano()%10000)

	// Step 1: Register
	registerLogic := NewRegisterLogic(ctx, env.svcCtx)
	registerResp, err := registerLogic.Register(&auth.RegisterRequest{
		Username: username,
		Email:    uniqueEmail,
		Password: "Abcdef1!",
		FullName: "Integration Test User",
	})
	require.NoError(t, err)
	require.NotNil(t, registerResp)
	assert.True(t, registerResp.RequiresVerification)
	t.Logf("register: %s", registerResp.Message)

	// Step 2: Login should fail because email is not verified
	loginLogic := NewLoginLogic(ctx, env.svcCtx)
	_, err = loginLogic.Login(&auth.LoginRequest{
		Email:    uniqueEmail,
		Password: "Abcdef1!",
	})
	require.Error(t, err)
	st := statusFromError(t, err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
	t.Logf("login before verification correctly rejected: %s", st.Message())

	// Step 3: Verify the email by extracting the token from Redis
	// The register logic stores a verification token in Redis keyed by email
	verificationRepo := repository.NewVerificationRepo(env.redisCli)
	pending, err := verificationRepo.PendingForEmail(ctx, uniqueEmail)
	require.NoError(t, err)
	require.True(t, pending, "expected a pending verification token after register")

	// We need to get the actual token. Since we can't easily look it up by email,
	// let's manually verify the user in the DB and then test login.
	user, err := env.svcCtx.Repo.Users.GetUserByEmail(ctx, uniqueEmail)
	require.NoError(t, err)
	_, err = env.svcCtx.Repo.Users.SetEmailVerified(ctx, user.ID)
	require.NoError(t, err)

	// Step 4: Login should now succeed
	loginResp, err := loginLogic.Login(&auth.LoginRequest{
		Email:    uniqueEmail,
		Password: "Abcdef1!",
	})
	require.NoError(t, err)
	require.NotNil(t, loginResp)
	assert.NotEmpty(t, loginResp.AccessToken)
	assert.NotEmpty(t, loginResp.RefreshToken)
	assert.Equal(t, uniqueEmail, loginResp.User.Email)
	assert.True(t, loginResp.User.EmailVerified)
	t.Logf("login after verification succeeded: user=%s", loginResp.User.Id)

	// Step 5: Refresh the token
	refreshLogic := NewRefreshTokenLogic(ctx, env.svcCtx)
	refreshResp, err := refreshLogic.RefreshToken(&auth.RefreshRequest{
		RefreshToken: loginResp.RefreshToken,
	})
	require.NoError(t, err)
	require.NotNil(t, refreshResp)
	assert.NotEmpty(t, refreshResp.AccessToken)
	assert.NotEmpty(t, refreshResp.RefreshToken)
	assert.NotEqual(t, loginResp.RefreshToken, refreshResp.RefreshToken, "refresh token should be rotated")
	t.Logf("token refresh succeeded: new access token issued")

	// Step 6: Logout
	logoutLogic := NewLogoutLogic(ctx, env.svcCtx)
	_, err = logoutLogic.Logout(&auth.LogoutRequest{
		AccessToken:  loginResp.AccessToken,
		RefreshToken: loginResp.RefreshToken,
	})
	require.NoError(t, err)
	t.Logf("logout succeeded")

	// Step 7: After logout, the old refresh token should be rejected
	_, err = refreshLogic.RefreshToken(&auth.RefreshRequest{
		RefreshToken: loginResp.RefreshToken,
	})
	require.Error(t, err)
	st = statusFromError(t, err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	t.Logf("refresh after logout correctly rejected: %s", st.Message())

	// Step 8: The rotated refresh token should also be rejected (session revoked)
	_, err = refreshLogic.RefreshToken(&auth.RefreshRequest{
		RefreshToken: refreshResp.RefreshToken,
	})
	require.Error(t, err)
	st = statusFromError(t, err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	t.Logf("rotated refresh token after logout correctly rejected: %s", st.Message())
}

// ============================================
// Integration test: Forgot Password → Reset Password
// ============================================

func TestIntegration_ForgotResetPassword(t *testing.T) {
	env := setupIntegrationTestEnv(t)
	defer env.pgPool.Close()
	defer env.redisCli.Close()

	ctx := context.Background()
	uniqueEmail := fmt.Sprintf("reset-%d@example.com", time.Now().UnixNano())
	username := fmt.Sprintf("reset%d", time.Now().UnixNano()%10000)

	// Register and verify
	registerLogic := NewRegisterLogic(ctx, env.svcCtx)
	_, err := registerLogic.Register(&auth.RegisterRequest{
		Username: username,
		Email:    uniqueEmail,
		Password: "OldPass1!",
		FullName: "Reset Test User",
	})
	require.NoError(t, err)

	user, err := env.svcCtx.Repo.Users.GetUserByEmail(ctx, uniqueEmail)
	require.NoError(t, err)
	_, err = env.svcCtx.Repo.Users.SetEmailVerified(ctx, user.ID)
	require.NoError(t, err)

	// Forgot password
	forgotLogic := NewForgotPasswordLogic(ctx, env.svcCtx)
	_, err = forgotLogic.ForgotPassword(&auth.ForgotPasswordRequest{Email: uniqueEmail})
	require.NoError(t, err)

	// Extract the reset token from Redis by scanning keys
	// We need to find the token. Let's scan Redis for password reset keys.
	keys, err := env.redisCli.Keys(ctx, "auth:password-reset:*").Result()
	require.NoError(t, err)
	require.Len(t, keys, 1, "expected exactly one password reset token")
	token := strings.TrimPrefix(keys[0], "auth:password-reset:")

	// Reset password
	resetLogic := NewResetPasswordLogic(ctx, env.svcCtx)
	_, err = resetLogic.ResetPassword(&auth.ResetPasswordRequest{
		Token:       token,
		NewPassword: "NewPass1!",
	})
	require.NoError(t, err)
	t.Logf("password reset succeeded")

	// Login with new password should work
	loginLogic := NewLoginLogic(ctx, env.svcCtx)
	loginResp, err := loginLogic.Login(&auth.LoginRequest{
		Email:    uniqueEmail,
		Password: "NewPass1!",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, loginResp.AccessToken)
	t.Logf("login with new password succeeded")

	// Login with old password should fail
	_, err = loginLogic.Login(&auth.LoginRequest{
		Email:    uniqueEmail,
		Password: "OldPass1!",
	})
	require.Error(t, err)
	t.Logf("login with old password correctly rejected")
}

// statusFromError extracts the gRPC status from an error, failing the test if
// the error is not a gRPC status error.
func statusFromError(t *testing.T, err error) *status.Status {
	t.Helper()
	st, ok := status.FromError(err)
	require.True(t, ok, "expected error to be a gRPC status error, got: %v", err)
	return st
}

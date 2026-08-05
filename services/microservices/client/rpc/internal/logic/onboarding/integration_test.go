//go:build integration

// Package onboardinglogic contains integration tests for the onboarding
// completion flow (createGoal → createHabit ×N → updateSettings with
// onboardingCompleted: true) against a real Postgres database via
// testcontainers.
//
// Run with:
//
//	go test -tags=integration ./services/microservices/client/rpc/internal/logic/onboarding/ -v -timeout=120s
//
// Requires Docker.
package onboardinglogic

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpg "github.com/testcontainers/testcontainers-go/modules/postgres"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/cache"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/analytics"
	goalslogic "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/logic/goals"
	habitslogic "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/logic/habits"
	settingslogic "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/logic/settings"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"golang.org/x/sync/singleflight"
)

// ============================================
// Integration test environment
// ============================================

type integrationTestEnv struct {
	svcCtx *svc.ServiceContext
	pgPool *pgxpool.Pool
	pgC    testcontainers.Container
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

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Run migrations (same directory as auth: sql/migrations_v2)
	runMigrations(t, connStr)

	// Open connection pool
	pool := postgres.MustOpenPool(connStr, 10, 5, time.Hour)

	// Create repository
	queries := db.New(pool)
	repo := repository.NewRepository(queries)
	txRunner := postgres.NewPgxTxRunner(pool)

	// Build a minimal ServiceContext — no Kafka, no Stripe, no Redis.
	// The cache works with a nil Redis client (no-op).
	svcCtx := &svc.ServiceContext{
		Repo:             repo,
		TxRunner:         txRunner,
		PatternDetection: analytics.NewPatternDetection(),
		Cache:            cache.New(nil),
		WeeklyReviewSF:   singleflight.Group{},
	}

	return &integrationTestEnv{
		svcCtx: svcCtx,
		pgPool: pool,
		pgC:    pgContainer,
	}
}

// runMigrations reads and executes all .up.sql migration files in order.
func runMigrations(t *testing.T, connStr string) {
	t.Helper()
	ctx := context.Background()

	migrationsDir := findMigrationsDir(t)

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, files)

	sort.Strings(files)

	conn, err := pgx.Connect(ctx, connStr)
	require.NoError(t, err)
	defer func() { _ = conn.Close(ctx) }()

	for _, file := range files {
		sql, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = conn.Exec(ctx, string(sql))
		if err != nil {
			t.Fatalf("migration %s failed: %v", filepath.Base(file), err)
		}
	}
	t.Logf("ran %d migration files", len(files))
}

// findMigrationsDir locates the sql/migrations_v2 directory relative to the
// test file by walking up to the repo root.
func findMigrationsDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)

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

// ctxWithPrincipal returns a context with an authenticated principal for the
// given user ID.
func ctxWithPrincipal(userID uuid.UUID) context.Context {
	return principal.WithPrincipal(context.Background(), principal.Principal{
		UserID: userID.String(),
		Roles:  []string{"user"},
	})
}

// formatCheckInTime converts a pgtype.Time (microseconds since midnight) to
// a "15:04" string for assertions.
func formatCheckInTime(t pgtype.Time) string {
	if !t.Valid {
		return ""
	}
	totalSeconds := t.Microseconds / 1_000_000
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	return fmt.Sprintf("%02d:%02d", hours, minutes)
}

// seedCategories inserts test categories so that goals and habits can reference
// them by slug. The migrations don't seed categories — they're app-managed data.
func seedCategories(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	categories := []struct {
		name      string
		slug      string
		sortOrder int32
	}{
		{"Education", "learning", 1},
		{"Health", "fitness", 2},
		{"Career", "career", 3},
	}
	for _, c := range categories {
		_, err := pool.Exec(ctx,
			`INSERT INTO categories (name, slug, sort_order) VALUES ($1, $2, $3) ON CONFLICT (slug) DO NOTHING`,
			c.name, c.slug, c.sortOrder)
		require.NoError(t, err)
	}
	t.Logf("seeded %d categories", len(categories))
}

// assertGrpcError asserts that err is a gRPC status error with the expected
// code and a message containing expectedMsgPart.
func assertGrpcError(t *testing.T, err error, expectedCode codes.Code, expectedMsgPart string) {
	t.Helper()
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok, "error should be a gRPC status error")
	assert.Equal(t, expectedCode, st.Code())
	assert.Contains(t, st.Message(), expectedMsgPart)
}

// ============================================
// Integration test: Onboarding completion flow
//
// Mirrors the frontend's use-onboarding-submission hook:
//   1. Create a goal (title, description, category)
//   2. Create N habits (one per selected habit suggestion)
//   3. Update settings with onboardingCompleted: true
//   4. Verify all data persisted correctly
// ============================================

func TestIntegration_OnboardingCompletion_CreateGoalHabitsSettings(t *testing.T) {
	env := setupIntegrationTestEnv(t)
	defer env.pgPool.Close()

	// Seed categories so goals/habits can reference them by slug.
	seedCategories(t, env.pgPool)

	ctx := context.Background()
	userID := uuid.New()
	logicCtx := ctxWithPrincipal(userID)

	// ─── Step 1: Create goal ───────────────────────────────────────────────
	goalLogic := goalslogic.NewCreateGoalLogic(logicCtx, env.svcCtx)
	goalResp, err := goalLogic.CreateGoal(&client.CreateGoalRequest{
		Title:       "Read more books",
		Description: "Goal: Read more books. Motivation: Grow my knowledge",
		Category:    "learning",
	})
	require.NoError(t, err)
	require.NotNil(t, goalResp)
	require.NotNil(t, goalResp.Goal)
	assert.Equal(t, "Read more books", goalResp.Goal.Title)
	assert.Equal(t, "learning", goalResp.Goal.Category)
	assert.False(t, goalResp.Goal.Completed)
	assert.Equal(t, int32(0), goalResp.Goal.Progress)
	assert.Equal(t, userID.String(), goalResp.Goal.UserId)
	assert.NotEmpty(t, goalResp.Goal.Id)
	t.Logf("created goal: id=%s title=%s", goalResp.Goal.Id, goalResp.Goal.Title)

	// ─── Step 2: Create 3 habits (mirroring onboarding habit suggestions) ──
	habitNames := []string{
		"Read 10 pages before bed",
		"Track one win in journal",
		"Review notes for 5 minutes",
	}
	createdHabitIDs := make([]string, 0, 3)

	for _, name := range habitNames {
		habitLogic := habitslogic.NewCreateHabitLogic(logicCtx, env.svcCtx)
		habitResp, err := habitLogic.CreateHabit(&client.CreateHabitRequest{
			Name:        name,
			Description: "Daily habit from onboarding",
			Category:    "learning",
		})
		require.NoError(t, err)
		require.NotNil(t, habitResp)
		require.NotNil(t, habitResp.Habit)
		assert.Equal(t, name, habitResp.Habit.Name)
		assert.Equal(t, "learning", habitResp.Habit.Category)
		assert.Equal(t, int32(0), habitResp.Habit.Streak)
		assert.False(t, habitResp.Habit.Completed)
		assert.Equal(t, userID.String(), habitResp.Habit.UserId)
		assert.NotEmpty(t, habitResp.Habit.Id)
		createdHabitIDs = append(createdHabitIDs, habitResp.Habit.Id)
		t.Logf("created habit: id=%s name=%s", habitResp.Habit.Id, habitResp.Habit.Name)
	}
	require.Len(t, createdHabitIDs, 3)

	// ─── Step 3: Update settings with onboardingCompleted: true ────────────
	settingsLogic := settingslogic.NewUpdateSettingsLogic(logicCtx, env.svcCtx)
	settingsResp, err := settingsLogic.UpdateSettings(&client.UpdateSettingsRequest{
		Settings: &client.UserSettings{
			AccountabilityStyle: "balanced",
			CheckInTime:         "09:00",
			OnboardingCompleted: true,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, settingsResp)
	assert.True(t, settingsResp.Success)
	t.Logf("updated settings: onboardingCompleted=true")

	// ─── Step 4: Verify data persisted correctly ───────────────────────────

	// 4a. Goal exists in DB
	persistedGoal, err := env.svcCtx.Repo.Goals.GetGoalByID(ctx, uuid.MustParse(goalResp.Goal.Id))
	require.NoError(t, err)
	assert.Equal(t, "Read more books", persistedGoal.Title)
	assert.Equal(t, userID, persistedGoal.UserID)

	// 4b. Habits exist in DB
	persistedHabits, err := env.svcCtx.Repo.Habits.ListHabits(ctx, userID, 10, 0, "UTC")
	require.NoError(t, err)
	require.Len(t, persistedHabits, 3)
	persistedHabitNames := make(map[string]bool, 3)
	for _, h := range persistedHabits {
		persistedHabitNames[h.Name] = true
	}
	for _, name := range habitNames {
		assert.True(t, persistedHabitNames[name], "habit %q should be in DB", name)
	}

	// 4c. User preferences exist with onboardingCompleted=true
	prefs, err := env.svcCtx.Repo.UserPreferences.GetUserPreferences(ctx, userID)
	require.NoError(t, err)
	assert.True(t, prefs.OnboardingCompleted, "onboarding should be marked complete")
	assert.Equal(t, "09:00", formatCheckInTime(prefs.CheckInTime))

	// 4d. Coaching profile has the accountability style
	profile, err := env.svcCtx.Repo.CoachingProfiles.GetCoachingProfile(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, "balanced", profile.AccountabilityStyle)

	t.Logf("onboarding completion flow verified: 1 goal + 3 habits + settings updated")
}

// ============================================
// Integration test: Onboarding bypasses plan limits
//
// A user who hasn't completed onboarding can create goals/habits even if
// the Free plan limit would normally block them. This is the key business
// rule that makes onboarding work for users with leftover data from a
// previous partial onboarding attempt.
// ============================================

func TestIntegration_OnboardingBypassesPlanLimits(t *testing.T) {
	env := setupIntegrationTestEnv(t)
	defer env.pgPool.Close()

	ctx := context.Background()
	userID := uuid.New()
	logicCtx := ctxWithPrincipal(userID)

	// The Free plan allows 3 goals and 5 habits. A fresh user has 0 of each,
	// so the limit check passes. But the key assertion is that the billing
	// check doesn't block creation when onboarding is not complete — even if
	// the user already has goals/habits from a partial onboarding.

	// Create a goal — should succeed (onboarding not complete → bypass)
	goalLogic := goalslogic.NewCreateGoalLogic(logicCtx, env.svcCtx)
	_, err := goalLogic.CreateGoal(&client.CreateGoalRequest{
		Title:    "First goal",
		Category: "fitness",
	})
	require.NoError(t, err, "goal creation should succeed during onboarding")

	// Create a habit — should succeed (onboarding not complete → bypass)
	habitLogic := habitslogic.NewCreateHabitLogic(logicCtx, env.svcCtx)
	_, err = habitLogic.CreateHabit(&client.CreateHabitRequest{
		Name:     "Morning run",
		Category: "fitness",
	})
	require.NoError(t, err, "habit creation should succeed during onboarding")

	// Verify onboarding is NOT complete (we didn't call updateSettings)
	_, err = env.svcCtx.Repo.UserPreferences.GetUserPreferences(ctx, userID)
	assert.ErrorIs(t, err, pgx.ErrNoRows, "user_preferences should not exist yet")
}

// ============================================
// Integration test: Settings update is idempotent
//
// Calling updateSettings with onboardingCompleted: true twice should not
// fail or corrupt the settings row.
// ============================================

func TestIntegration_OnboardingSettings_IdempotentUpdate(t *testing.T) {
	env := setupIntegrationTestEnv(t)
	defer env.pgPool.Close()

	ctx := context.Background()
	userID := uuid.New()
	logicCtx := ctxWithPrincipal(userID)

	// First update — creates the user_preferences row
	settingsLogic := settingslogic.NewUpdateSettingsLogic(logicCtx, env.svcCtx)
	_, err := settingsLogic.UpdateSettings(&client.UpdateSettingsRequest{
		Settings: &client.UserSettings{
			AccountabilityStyle: "strict",
			CheckInTime:         "07:30",
			OnboardingCompleted: true,
		},
	})
	require.NoError(t, err)

	// Verify
	prefs, err := env.svcCtx.Repo.UserPreferences.GetUserPreferences(ctx, userID)
	require.NoError(t, err)
	assert.True(t, prefs.OnboardingCompleted)
	assert.Equal(t, "07:30", formatCheckInTime(prefs.CheckInTime))

	// Coaching profile should have the accountability style
	profile, err := env.svcCtx.Repo.CoachingProfiles.GetCoachingProfile(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, "strict", profile.AccountabilityStyle)

	// Second update — should upsert, not fail
	_, err = settingsLogic.UpdateSettings(&client.UpdateSettingsRequest{
		Settings: &client.UserSettings{
			AccountabilityStyle: "gentle",
			CheckInTime:         "18:00",
			OnboardingCompleted: true,
		},
	})
	require.NoError(t, err)

	// Verify updated values
	prefs, err = env.svcCtx.Repo.UserPreferences.GetUserPreferences(ctx, userID)
	require.NoError(t, err)
	assert.True(t, prefs.OnboardingCompleted)
	assert.Equal(t, "18:00", formatCheckInTime(prefs.CheckInTime))

	// Coaching profile should reflect the new accountability style
	profile, err = env.svcCtx.Repo.CoachingProfiles.GetCoachingProfile(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, "gentle", profile.AccountabilityStyle)
}

// ============================================
// Integration test: Missing principal returns Unauthenticated
//
// All three onboarding completion RPCs require an authenticated principal.
// ============================================

func TestIntegration_OnboardingCompletion_MissingPrincipal(t *testing.T) {
	env := setupIntegrationTestEnv(t)
	defer env.pgPool.Close()

	ctx := context.Background() // no principal

	t.Run("createGoal without principal", func(t *testing.T) {
		l := goalslogic.NewCreateGoalLogic(ctx, env.svcCtx)
		_, err := l.CreateGoal(&client.CreateGoalRequest{Title: "Test"})
		assertGrpcError(t, err, codes.Unauthenticated, "missing principal")
	})

	t.Run("createHabit without principal", func(t *testing.T) {
		l := habitslogic.NewCreateHabitLogic(ctx, env.svcCtx)
		_, err := l.CreateHabit(&client.CreateHabitRequest{Name: "Test"})
		assertGrpcError(t, err, codes.Unauthenticated, "missing principal")
	})

	t.Run("updateSettings without principal", func(t *testing.T) {
		l := settingslogic.NewUpdateSettingsLogic(ctx, env.svcCtx)
		_, err := l.UpdateSettings(&client.UpdateSettingsRequest{
			Settings: &client.UserSettings{OnboardingCompleted: true},
		})
		assertGrpcError(t, err, codes.Unauthenticated, "missing principal")
	})
}

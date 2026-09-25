package deletion

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
)

// integrationDSN returns a disposable local Postgres DSN from
// AUTH_DELETION_TEST_DSN, skipping when unset or pointing anywhere remote.
func integrationDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("AUTH_DELETION_TEST_DSN")
	if dsn == "" {
		t.Skip("AUTH_DELETION_TEST_DSN not set; skipping disposable-PG integration test")
	}
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Contains(t, []string{"localhost", "127.0.0.1", "::1"}, u.Hostname(),
		"integration test requires a local Postgres DSN")
	return dsn
}

// applyFixture applies the minimum real migrations needed for the outbox
// statements: the uuid v7 helper, the users table, and the outbox table.
func applyFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// Disposable DB only: start clean so reruns against the same DSN work.
	_, err := pool.Exec(context.Background(),
		`DROP TABLE IF EXISTS auth_deletion_outbox, users CASCADE`)
	require.NoError(t, err)
	root := filepath.Join("..", "..", "..", "..", "..", "..", "sql", "migrations_v2")
	for _, name := range []string{
		"000_setup.up.sql",
		"001_users.up.sql",
		"061_auth_deletion_outbox.up.sql",
	} {
		sql, err := os.ReadFile(filepath.Join(root, name))
		require.NoError(t, err)
		_, err = pool.Exec(context.Background(), string(sql))
		require.NoError(t, err, "apply %s", name)
	}
}

func seedUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	suffix := uuid.NewString()[:8]
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (username, email, password_hash, full_name)
		 VALUES ($1, $2, 'hash', 'Test User') RETURNING id`,
		"test_"+suffix, "test_"+suffix+"@example.com").Scan(&id)
	require.NoError(t, err)
	return id
}

func TestDeleteUserAtomicity_Integration(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), integrationDSN(t))
	require.NoError(t, err)
	defer pool.Close()
	applyFixture(t, pool)
	queries := db.New(pool)
	ctx := context.Background()

	t.Run("delete user and queue outbox row atomically", func(t *testing.T) {
		userID := seedUser(t, pool)
		require.NoError(t, queries.DeleteUser(ctx, userID))

		var userCount int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM users WHERE id = $1`, userID).Scan(&userCount))
		assert.Equal(t, 0, userCount)

		var outboxCount int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM auth_deletion_outbox`).Scan(&outboxCount))
		assert.Equal(t, 1, outboxCount)

		var queuedUser uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT user_id FROM auth_deletion_outbox LIMIT 1`).Scan(&queuedUser))
		assert.Equal(t, userID, queuedUser)
	})

	t.Run("outbox insert failure rolls back user delete", func(t *testing.T) {
		// Empty the table first so the always-false check validates, then it
		// rejects the outbox insert inside DeleteUser's single statement.
		_, err := pool.Exec(ctx, `DELETE FROM auth_deletion_outbox`)
		require.NoError(t, err)
		_, err = pool.Exec(ctx,
			`ALTER TABLE auth_deletion_outbox ADD CONSTRAINT test_outbox_blocked CHECK (false)`)
		require.NoError(t, err)
		defer func() {
			_, _ = pool.Exec(ctx,
				`ALTER TABLE auth_deletion_outbox DROP CONSTRAINT test_outbox_blocked`)
		}()

		userID := seedUser(t, pool)
		require.Error(t, queries.DeleteUser(ctx, userID))

		var userCount int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM users WHERE id = $1`, userID).Scan(&userCount))
		assert.Equal(t, 1, userCount, "user delete must roll back with the failed outbox insert")
	})
}

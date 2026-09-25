package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
)

type execCall struct {
	sql  string
	args []interface{}
}

type fakeRow struct {
	scan func(dest ...any) error
}

func (r fakeRow) Scan(dest ...any) error { return r.scan(dest...) }

// fakeDBTX records every call and can inject an error keyed on a SQL substring.
type fakeDBTX struct {
	mu          sync.Mutex
	execs       []execCall
	failOnSQL   string
	failErr     error
	processed   bool
	rowErr      error
	queryCalled bool
}

func (f *fakeDBTX) Exec(_ context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.execs = append(f.execs, execCall{sql: sql, args: args})
	if f.failOnSQL != "" && strings.Contains(sql, f.failOnSQL) {
		return pgconn.CommandTag{}, f.failErr
	}
	return pgconn.NewCommandTag("DELETE 1"), nil
}

func (f *fakeDBTX) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queryCalled = true
	return nil, errors.New("unexpected Query call")
}

func (f *fakeDBTX) QueryRow(_ context.Context, _ string, _ ...interface{}) pgx.Row {
	return fakeRow{scan: func(dest ...any) error {
		if f.rowErr != nil {
			return f.rowErr
		}
		if b, ok := dest[0].(*bool); ok {
			*b = f.processed
		}
		return nil
	}}
}

func (f *fakeDBTX) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("unexpected CopyFrom call")
}

func (f *fakeDBTX) execsContaining(sub string) []execCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []execCall
	for _, e := range f.execs {
		if strings.Contains(e.sql, sub) {
			out = append(out, e)
		}
	}
	return out
}

func userDeletedEnvelope(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	env, err := events.NewEnvelopeWithID(uuid.NewString(), events.TypeUserDeleted, events.UserDeleted{
		UserID: userID.String(),
	})
	require.NoError(t, err)
	raw, err := json.Marshal(env)
	require.NoError(t, err)
	return string(raw)
}

func newHandler(f *fakeDBTX) *AuthEventsHandler {
	queries := db.New(f)
	return NewAuthEventsHandler(repository.NewRepository(queries), queries)
}

func TestConsumeUserDeleted_DeletesReports(t *testing.T) {
	f := &fakeDBTX{}
	userID := uuid.New()
	require.NoError(t, newHandler(f).Consume(context.Background(), "", userDeletedEnvelope(t, userID)))

	for _, sqlPart := range []string{"DELETE FROM report_comments WHERE user_id", "DELETE FROM reports WHERE reporter_id"} {
		calls := f.execsContaining(sqlPart)
		require.Len(t, calls, 1, "expected one exec for %q", sqlPart)
		require.Equal(t, userID, calls[0].args[0])
	}
	assert.NotEmpty(t, f.execsContaining("INSERT INTO client_processed_events"), "event must be marked processed")
}

func TestConsumeUserDeleted_ReportsDeleteErrorSkipsMarkProcessed(t *testing.T) {
	for _, failOn := range []string{"DELETE FROM report_comments", "DELETE FROM reports WHERE"} {
		f := &fakeDBTX{failOnSQL: failOn, failErr: errors.New("db down")}
		err := newHandler(f).Consume(context.Background(), "", userDeletedEnvelope(t, uuid.New()))
		require.Error(t, err, "injected failure on %q", failOn)
		assert.Empty(t, f.execsContaining("INSERT INTO client_processed_events"))
	}
}

func TestConsumeUserDeleted_RedeliverySafe(t *testing.T) {
	f := &fakeDBTX{}
	h := newHandler(f)
	raw := userDeletedEnvelope(t, uuid.New())
	require.NoError(t, h.Consume(context.Background(), "", raw))
	require.NoError(t, h.Consume(context.Background(), "", raw))
	assert.Len(t, f.execsContaining("DELETE FROM report_comments"), 2)
}

func TestConsumeUserDeleted_DuplicateSkippedByDedup(t *testing.T) {
	f := &fakeDBTX{processed: true}
	require.NoError(t, newHandler(f).Consume(context.Background(), "", userDeletedEnvelope(t, uuid.New())))
	assert.Empty(t, f.execsContaining("DELETE FROM"))
}

func TestConsumeUserDeleted_OnlyTargetsEventUser(t *testing.T) {
	f := &fakeDBTX{}
	userID := uuid.New()
	other := uuid.New()
	require.NotEqual(t, userID, other)
	require.NoError(t, newHandler(f).Consume(context.Background(), "", userDeletedEnvelope(t, userID)))

	for _, e := range f.execsContaining("DELETE FROM report_comments") {
		assert.Equal(t, userID, e.args[0])
		assert.NotEqual(t, other, e.args[0])
	}
	for _, e := range f.execsContaining("DELETE FROM reports WHERE") {
		assert.Equal(t, userID, e.args[0])
	}
}

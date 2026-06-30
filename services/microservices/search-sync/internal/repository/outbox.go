package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxRow struct {
	ID          int64
	EntityType  string
	EntityID    uuid.UUID
	Operation   string
	Attempts    int
	LastError   *string
	AvailableAt time.Time
	CreatedAt   time.Time
}

type OutboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

// LockPending claims a batch using a visibility timeout: attempts is bumped
// and available_at pushed lockTimeout into the future, so a crashed worker's
// rows simply reappear once the timeout passes. workerID is kept for the
// call signature but no longer stored.
func (r *OutboxRepository) LockPending(ctx context.Context, batchSize int, workerID string, lockTimeout time.Duration) ([]OutboxRow, error) {
	_ = workerID
	query := `
		WITH next AS (
			SELECT id
			FROM search_outbox
			WHERE available_at <= now()
			ORDER BY id
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE search_outbox o
		SET attempts = attempts + 1,
		    available_at = now() + $2::interval
		FROM next
		WHERE o.id = next.id
		RETURNING o.id, o.entity_type, o.entity_id, o.operation, o.attempts, o.last_error, o.available_at, o.created_at`

	rows, err := r.pool.Query(ctx, query, batchSize, lockTimeout)
	if err != nil {
		return nil, fmt.Errorf("lock pending: %w", err)
	}
	defer rows.Close()

	var result []OutboxRow
	for rows.Next() {
		var row OutboxRow
		err := rows.Scan(
			&row.ID, &row.EntityType, &row.EntityID, &row.Operation,
			&row.Attempts, &row.LastError, &row.AvailableAt, &row.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan outbox row: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox rows: %w", err)
	}
	return result, nil
}

// MarkProcessed removes the row: the outbox is insert-only and processed
// rows have no further use.
func (r *OutboxRepository) MarkProcessed(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM search_outbox WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark processed: %w", err)
	}
	return nil
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, id int64, errMsg string, availableAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE search_outbox SET last_error = $2, available_at = $3 WHERE id = $1`,
		id, errMsg, availableAt,
	)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	return nil
}

// ReleaseStaleLocks is a no-op under the visibility-timeout scheme: claimed
// rows become visible again automatically when available_at passes.
func (r *OutboxRepository) ReleaseStaleLocks(ctx context.Context, lockTimeout time.Duration) error {
	return nil
}

func (r *OutboxRepository) GetArticle(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	query := `
		SELECT
			a.id,
			a.title,
			a.excerpt,
			a.content,
			a.author,
			a.published_at,
			a.created_at,
			a.updated_at,
			c.name AS category_name,
			c.slug AS category_slug
		FROM articles a
		LEFT JOIN categories c ON c.id = a.category_id
		WHERE a.id = $1`

	var doc struct {
		ID           uuid.UUID `json:"id"`
		Title        string    `json:"title"`
		Excerpt      *string   `json:"excerpt"`
		Content      string    `json:"content"`
		Author       string    `json:"author"`
		PublishedAt  time.Time `json:"published_at"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
		CategoryName *string   `json:"category_name"`
		CategorySlug *string   `json:"category_slug"`
	}

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&doc.ID, &doc.Title, &doc.Excerpt, &doc.Content, &doc.Author,
		&doc.PublishedAt, &doc.CreatedAt, &doc.UpdatedAt, &doc.CategoryName, &doc.CategorySlug,
	)
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"id":            docID("article", doc.ID),
		"entity_id":     doc.ID.String(),
		"type":          "article",
		"user_id":       nil,
		"title":         doc.Title,
		"description":   nil,
		"content":       doc.Content,
		"category":      nil,
		"category_slug": nil,
		"author":        doc.Author,
		"created_at":    doc.CreatedAt.Unix(),
		"updated_at":    doc.UpdatedAt.Unix(),
		"url":           fmt.Sprintf("/article/%s", doc.ID.String()),
		"visibility":    "public",
	}
	if doc.Excerpt != nil {
		result["description"] = *doc.Excerpt
	}
	if doc.CategoryName != nil {
		result["category"] = *doc.CategoryName
		result["category_slug"] = *doc.CategorySlug
	}
	return result, nil
}

func (r *OutboxRepository) GetGoal(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	query := `SELECT g.id, g.user_id, g.title, g.description, COALESCE(c.slug, '') AS category, g.status, g.created_at, g.updated_at
	FROM goals g LEFT JOIN categories c ON c.id = g.category_id
	WHERE g.id = $1`

	var doc struct {
		ID          uuid.UUID `json:"id"`
		UserID      uuid.UUID `json:"user_id"`
		Title       string    `json:"title"`
		Description *string   `json:"description"`
		Category    string    `json:"category"`
		Status      string    `json:"status"`
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   time.Time `json:"updated_at"`
	}

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&doc.ID, &doc.UserID, &doc.Title, &doc.Description, &doc.Category, &doc.Status, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"id":          docID("goal", doc.ID),
		"entity_id":   doc.ID.String(),
		"type":        "goal",
		"user_id":     doc.UserID.String(),
		"title":       doc.Title,
		"description": nil,
		"category":    doc.Category,
		"created_at":  doc.CreatedAt.Unix(),
		"updated_at":  doc.UpdatedAt.Unix(),
		"url":         "/goals",
		"visibility":  "private",
	}
	if doc.Description != nil {
		result["description"] = *doc.Description
	}
	return result, nil
}

func (r *OutboxRepository) GetHabit(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	query := `SELECT h.id, h.user_id, h.name, h.description, COALESCE(c.slug, '') AS category, h.created_at, h.updated_at
	FROM habits h LEFT JOIN categories c ON c.id = h.category_id
	WHERE h.id = $1`

	var doc struct {
		ID          uuid.UUID `json:"id"`
		UserID      uuid.UUID `json:"user_id"`
		Name        string    `json:"name"`
		Description *string   `json:"description"`
		Category    string    `json:"category"`
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   time.Time `json:"updated_at"`
	}

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&doc.ID, &doc.UserID, &doc.Name, &doc.Description, &doc.Category, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"id":          docID("habit", doc.ID),
		"entity_id":   doc.ID.String(),
		"type":        "habit",
		"user_id":     doc.UserID.String(),
		"title":       doc.Name,
		"description": nil,
		"category":    doc.Category,
		"created_at":  doc.CreatedAt.Unix(),
		"updated_at":  doc.UpdatedAt.Unix(),
		"url":         "/habits",
		"visibility":  "private",
	}
	if doc.Description != nil {
		result["description"] = *doc.Description
	}
	return result, nil
}

// GetCheckIn returns a user_memory doc for a check-in note. The insert trigger
// only enqueues rows with a non-empty note, so content is the note text.
// habit_name is carried as light metadata so the coach can attribute a snippet.
func (r *OutboxRepository) GetCheckIn(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	query := `SELECT c.id, c.user_id, c.note, c.local_date, c.created_at, COALESCE(h.name, '') AS habit_name
		FROM check_ins c
		LEFT JOIN habits h ON h.id = c.habit_id
		WHERE c.id = $1`

	var doc struct {
		ID        uuid.UUID `json:"id"`
		UserID    uuid.UUID `json:"user_id"`
		Note      *string   `json:"note"`
		LocalDate time.Time `json:"local_date"`
		CreatedAt time.Time `json:"created_at"`
		HabitName string    `json:"habit_name"`
	}

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&doc.ID, &doc.UserID, &doc.Note, &doc.LocalDate, &doc.CreatedAt, &doc.HabitName,
	)
	if err != nil {
		return nil, err
	}

	content := ""
	if doc.Note != nil {
		content = *doc.Note
	}
	return map[string]any{
		"id":          docID("check_in", doc.ID),
		"entity_id":   doc.ID.String(),
		"entity_type": "check_in",
		"user_id":     doc.UserID.String(),
		"content":     content,
		"habit_name":  doc.HabitName,
		"local_date":  doc.LocalDate.Unix(),
		"created_at":  doc.CreatedAt.Unix(),
	}, nil
}

// GetMessage returns a user_memory doc for a conversation message. user_id is
// joined from the parent conversation. role is carried as light metadata.
func (r *OutboxRepository) GetMessage(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	query := `SELECT m.id, conv.user_id, m.role, m.content, m.created_at
		FROM conversation_messages m
		JOIN conversations conv ON conv.id = m.conversation_id
		WHERE m.id = $1`

	var doc struct {
		ID        uuid.UUID `json:"id"`
		UserID    uuid.UUID `json:"user_id"`
		Role      string    `json:"role"`
		Content   string    `json:"content"`
		CreatedAt time.Time `json:"created_at"`
	}

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&doc.ID, &doc.UserID, &doc.Role, &doc.Content, &doc.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"id":          docID("conversation_message", doc.ID),
		"entity_id":   doc.ID.String(),
		"entity_type": "conversation_message",
		"user_id":     doc.UserID.String(),
		"content":     doc.Content,
		"role":        doc.Role,
		"created_at":  doc.CreatedAt.Unix(),
	}, nil
}

// GetWeeklyReview returns a user_memory doc for a weekly review's ai_summary.
// The upsert trigger only fires when ai_summary is non-empty.
func (r *OutboxRepository) GetWeeklyReview(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	query := `SELECT w.id, w.user_id, w.ai_summary, w.week_start, w.created_at
		FROM weekly_reviews w
		WHERE w.id = $1`

	var doc struct {
		ID        uuid.UUID `json:"id"`
		UserID    uuid.UUID `json:"user_id"`
		AISummary *string   `json:"ai_summary"`
		WeekStart time.Time `json:"week_start"`
		CreatedAt time.Time `json:"created_at"`
	}

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&doc.ID, &doc.UserID, &doc.AISummary, &doc.WeekStart, &doc.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	content := ""
	if doc.AISummary != nil {
		content = *doc.AISummary
	}
	return map[string]any{
		"id":          docID("weekly_review", doc.ID),
		"entity_id":   doc.ID.String(),
		"entity_type": "weekly_review",
		"user_id":     doc.UserID.String(),
		"content":     content,
		"week_start":  doc.WeekStart.Unix(),
		"created_at":  doc.CreatedAt.Unix(),
	}, nil
}

func (r *OutboxRepository) Backfill(ctx context.Context) error {
	queries := []string{
		`INSERT INTO search_outbox (entity_type, entity_id, operation)
		 SELECT 'article', id, 'upsert' FROM articles
		 ON CONFLICT DO NOTHING`,
		`INSERT INTO search_outbox (entity_type, entity_id, operation)
		 SELECT 'goal', id, 'upsert' FROM goals
		 ON CONFLICT DO NOTHING`,
		`INSERT INTO search_outbox (entity_type, entity_id, operation)
		 SELECT 'habit', id, 'upsert' FROM habits
		 ON CONFLICT DO NOTHING`,
		// Private memory index sources. check_ins only where a note exists; the
		// insert trigger has the same guard, so this matches steady-state.
		`INSERT INTO search_outbox (entity_type, entity_id, operation)
		 SELECT 'check_in', id, 'upsert' FROM check_ins
		 WHERE note IS NOT NULL AND note <> ''
		 ON CONFLICT DO NOTHING`,
		`INSERT INTO search_outbox (entity_type, entity_id, operation)
		 SELECT 'conversation_message', id, 'upsert' FROM conversation_messages
		 ON CONFLICT DO NOTHING`,
		`INSERT INTO search_outbox (entity_type, entity_id, operation)
		 SELECT 'weekly_review', id, 'upsert' FROM weekly_reviews
		 WHERE ai_summary IS NOT NULL AND ai_summary <> ''
		 ON CONFLICT DO NOTHING`,
	}

	for _, q := range queries {
		if _, err := r.pool.Exec(ctx, q); err != nil {
			return fmt.Errorf("backfill: %w", err)
		}
	}
	return nil
}

func (r *OutboxRepository) ListenNotify(ctx context.Context) (chan struct{}, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire conn for listen: %w", err)
	}

	_, err = conn.Exec(ctx, "LISTEN search_sync")
	if err != nil {
		conn.Release()
		return nil, fmt.Errorf("listen search_sync: %w", err)
	}

	ch := make(chan struct{}, 1)
	go func() {
		defer conn.Release()
		for {
			select {
			case <-ctx.Done():
				close(ch)
				return
			default:
			}
			_, err := conn.Conn().WaitForNotification(ctx)
			if err != nil {
				if ctx.Err() != nil {
					close(ch)
					return
				}
				continue
			}
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}()

	return ch, nil
}

func IsNoRows(err error) bool {
	return err == pgx.ErrNoRows
}

// docID builds the Meili primary-key for an indexed row. Meili document ids may
// only contain A-Za-z0-9-_ (no ':'), so the entity type and uuid are joined
// with '_'. This is the single source of truth for the id scheme used by both
// the getters (upsert) and the syncer (delete).
func docID(entityType string, id uuid.UUID) string {
	return fmt.Sprintf("%s_%s", entityType, id.String())
}

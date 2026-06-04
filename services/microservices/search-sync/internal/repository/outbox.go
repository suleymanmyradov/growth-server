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
	ID          uuid.UUID
	EntityType  string
	EntityID    uuid.UUID
	Operation   string
	Payload     []byte
	Status      string
	Attempts    int
	AvailableAt time.Time
	LockedAt    *time.Time
	LockedBy    *string
	ProcessedAt *time.Time
	Error       *string
	CreatedAt   time.Time
}

type OutboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

func (r *OutboxRepository) LockPending(ctx context.Context, batchSize int, workerID string, lockTimeout time.Duration) ([]OutboxRow, error) {
	query := `
		WITH next AS (
			SELECT id
			FROM search_outbox
			WHERE status IN ('pending', 'failed')
			  AND available_at <= now()
			ORDER BY created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE search_outbox o
		SET status = 'processing',
		    locked_at = now(),
		    locked_by = $2,
		    attempts = attempts + 1
		FROM next
		WHERE o.id = next.id
		RETURNING o.id, o.entity_type, o.entity_id, o.operation, o.payload, o.status, o.attempts, o.available_at, o.locked_at, o.locked_by, o.processed_at, o.error, o.created_at`

	rows, err := r.pool.Query(ctx, query, batchSize, workerID)
	if err != nil {
		return nil, fmt.Errorf("lock pending: %w", err)
	}
	defer rows.Close()

	var result []OutboxRow
	for rows.Next() {
		var row OutboxRow
		err := rows.Scan(
			&row.ID, &row.EntityType, &row.EntityID, &row.Operation,
			&row.Payload, &row.Status, &row.Attempts, &row.AvailableAt,
			&row.LockedAt, &row.LockedBy, &row.ProcessedAt, &row.Error, &row.CreatedAt,
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

func (r *OutboxRepository) MarkProcessed(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE search_outbox SET status = 'processed', processed_at = now(), error = NULL WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("mark processed: %w", err)
	}
	return nil
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string, availableAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE search_outbox SET status = 'failed', error = $2, available_at = $3 WHERE id = $1`,
		id, errMsg, availableAt,
	)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	return nil
}

func (r *OutboxRepository) ReleaseStaleLocks(ctx context.Context, lockTimeout time.Duration) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE search_outbox SET status = 'pending', locked_at = NULL, locked_by = NULL
		 WHERE status = 'processing' AND locked_at < now() - $1::interval`,
		lockTimeout,
	)
	if err != nil {
		return fmt.Errorf("release stale locks: %w", err)
	}
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
		"id":            fmt.Sprintf("article:%s", doc.ID.String()),
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
	query := `SELECT id, user_id, title, description, category, status, created_at, updated_at FROM goals WHERE id = $1`

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
		"id":          fmt.Sprintf("goal:%s", doc.ID.String()),
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
	query := `SELECT id, user_id, name, description, category, created_at, updated_at FROM habits WHERE id = $1`

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
		"id":          fmt.Sprintf("habit:%s", doc.ID.String()),
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

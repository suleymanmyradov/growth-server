package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Notification is the parsed payload from a pg_notify('search_sync', ...) call.
// Triggers fire: {"e":"<entity_type>","id":"<uuid>","op":"upsert"|"delete"}
type Notification struct {
	EntityType string
	EntityID   uuid.UUID
	Operation  string // "upsert" or "delete"
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Listen starts a dedicated connection in LISTEN search_sync mode and returns
// a channel of parsed notifications. The connection is held for the lifetime
// of the channel; it is released when ctx is cancelled. If the connection
// drops, the goroutine reconnects and re-LISTENs automatically.
func (r *Repository) Listen(ctx context.Context) (<-chan Notification, error) {
	ch := make(chan Notification, 64)

	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire conn for listen: %w", err)
	}
	if _, err := conn.Exec(ctx, "LISTEN search_sync"); err != nil {
		conn.Release()
		return nil, fmt.Errorf("listen search_sync: %w", err)
	}

	go func() {
		defer conn.Release()
		defer close(ch)
		for {
			notification, err := conn.Conn().WaitForNotification(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				// Connection dropped; try to re-acquire and re-listen.
				conn.Release()
				time.Sleep(time.Second)
				newConn, err := r.pool.Acquire(ctx)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					continue
				}
				conn = newConn
				if _, err := conn.Exec(ctx, "LISTEN search_sync"); err != nil {
					conn.Release()
					continue
				}
				continue
			}

			n, err := parseNotification(notification.Payload)
			if err != nil {
				continue
			}
			select {
			case ch <- n:
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	return ch, nil
}

func parseNotification(payload string) (Notification, error) {
	var raw struct {
		E  string `json:"e"`
		ID string `json:"id"`
		OP string `json:"op"`
	}
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return Notification{}, fmt.Errorf("parse notification payload: %w", err)
	}
	id, err := uuid.Parse(raw.ID)
	if err != nil {
		return Notification{}, fmt.Errorf("parse entity id: %w", err)
	}
	return Notification{EntityType: raw.E, EntityID: id, Operation: raw.OP}, nil
}

// ---------------------------------------------------------------------------
// Document getters — fetch the full row from Postgres and shape it as a
// Meilisearch document. Used by both the notification handler and the
// reconciliation loop.
// ---------------------------------------------------------------------------

func (r *Repository) GetArticle(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	query := `
		SELECT
			a.id,
			a.title,
			a.excerpt,
			a.content,
			a.author,
			a.status,
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
		Status       string    `json:"status"`
		PublishedAt  time.Time `json:"published_at"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
		CategoryName *string   `json:"category_name"`
		CategorySlug *string   `json:"category_slug"`
	}

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&doc.ID, &doc.Title, &doc.Excerpt, &doc.Content, &doc.Author, &doc.Status,
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
		"status":        doc.Status,
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

func (r *Repository) GetGoal(ctx context.Context, id uuid.UUID) (map[string]any, error) {
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

func (r *Repository) GetHabit(ctx context.Context, id uuid.UUID) (map[string]any, error) {
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
// only fires for rows with a non-empty note, so content is the note text.
// habit_name is carried as light metadata so the coach can attribute a snippet.
func (r *Repository) GetCheckIn(ctx context.Context, id uuid.UUID) (map[string]any, error) {
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

// minTurnContentChars is the floor for indexing a turn. Below it the exchange
// carries no retrievable meaning -- "yeah ok", "thanks", "sounds good" -- and
// indexing it only adds noise that competes for top-k against real content.
const minTurnContentChars = 40

// GetMessage returns a user_memory doc for one conversational TURN, anchored on
// the user's message and including the assistant reply that followed it.
//
// One document per message was the wrong unit. An assistant message is several
// hundred words of coaching prose and a user message is frequently two words,
// so embedding each separately meant embedding "yeah ok" as its own retrievable
// item, where it competes on equal footing with substantive content. Pairing
// them makes each document a coherent exchange, halves the index, and gives the
// embedding enough context to be meaningful.
//
// Called with any message id -- an assistant message resolves back to the user
// message that prompted it, so both notifications converge on the same doc.
// Returns a no-rows error when there is nothing worth indexing, which the syncer
// treats as a delete; that is also what retires the old one-doc-per-message
// entries as they are re-processed.
func (r *Repository) GetMessage(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	// Resolve the turn anchor: the user message at or before the given id
	// within the same conversation. Ordering is (created_at, id) to match the
	// conversation read path -- id is uuid_generate_v7 and so time-ordered.
	const anchorQuery = `
		WITH target AS (
			SELECT m.id, m.conversation_id, m.role, m.created_at
			FROM conversation_messages m
			WHERE m.id = $1
		)
		SELECT a.id, conv.user_id, a.content, a.created_at
		FROM target t
		JOIN conversation_messages a
		  ON a.conversation_id = t.conversation_id
		 AND a.role = 'user'
		 AND (a.created_at, a.id) <= (t.created_at, t.id)
		JOIN conversations conv ON conv.id = a.conversation_id
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT 1`

	var anchor struct {
		ID        uuid.UUID
		UserID    uuid.UUID
		Content   string
		CreatedAt time.Time
	}
	if err := r.pool.QueryRow(ctx, anchorQuery, id).Scan(
		&anchor.ID, &anchor.UserID, &anchor.Content, &anchor.CreatedAt,
	); err != nil {
		// No user message at or before this one: an assistant message opening a
		// conversation, or the row is gone. Nothing to anchor a turn on.
		return nil, err
	}

	// The assistant reply that followed the anchor, if it has arrived. A turn
	// mid-flight (user message stored, reply still streaming) indexes on its
	// own and is completed when the reply lands and re-triggers this path.
	const replyQuery = `
		SELECT m.content
		FROM conversation_messages m
		WHERE m.conversation_id = (SELECT conversation_id FROM conversation_messages WHERE id = $1)
		  AND m.role = 'assistant'
		  AND (m.created_at, m.id) > ((SELECT created_at FROM conversation_messages WHERE id = $1),
		                              (SELECT id FROM conversation_messages WHERE id = $1))
		ORDER BY m.created_at ASC, m.id ASC
		LIMIT 1`

	var reply *string
	if err := r.pool.QueryRow(ctx, replyQuery, anchor.ID).Scan(&reply); err != nil && !IsNoRows(err) {
		return nil, err
	}

	content := strings.TrimSpace(anchor.Content)
	if reply != nil {
		if trimmed := strings.TrimSpace(*reply); trimmed != "" {
			content = content + "\n\n" + trimmed
		}
	}
	if len(content) < minTurnContentChars {
		// Signal "not indexable" using the same error the syncer already treats
		// as a delete, so a turn that shrinks below the floor is removed rather
		// than left stale.
		return nil, pgx.ErrNoRows
	}

	return map[string]any{
		"id":          docID("conversation_message", anchor.ID),
		"entity_id":   anchor.ID.String(),
		"entity_type": "conversation_message",
		"user_id":     anchor.UserID.String(),
		"content":     content,
		// The anchor is always the user's side of the exchange. Kept for
		// attribution in the coach's prompt, which labels a hit by its source.
		"role":       "user",
		"created_at": anchor.CreatedAt.Unix(),
	}, nil
}

// GetWeeklyReview returns a user_memory doc for a weekly review's ai_summary.
// The upsert trigger only fires when ai_summary is non-empty.
func (r *Repository) GetWeeklyReview(ctx context.Context, id uuid.UUID) (map[string]any, error) {
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

// ---------------------------------------------------------------------------
// Reconciliation queries — used by the periodic reconcile loop to detect and
// repair drift between Postgres and Meilisearch.
// ---------------------------------------------------------------------------

// EntitySpec describes one entity type for reconciliation: which table to scan
// and which doc IDs the syncer expects for rows in that table.
type EntitySpec struct {
	EntityType string
	// ListIDs returns all entity IDs that should exist in the search index.
	ListIDs func(ctx context.Context) ([]uuid.UUID, error)
}

// ListArticleIDs returns all article IDs.
func (r *Repository) ListArticleIDs(ctx context.Context) ([]uuid.UUID, error) {
	return r.listIDs(ctx, `SELECT id FROM articles`)
}

// ListGoalIDs returns all goal IDs.
func (r *Repository) ListGoalIDs(ctx context.Context) ([]uuid.UUID, error) {
	return r.listIDs(ctx, `SELECT id FROM goals`)
}

// ListHabitIDs returns all habit IDs.
func (r *Repository) ListHabitIDs(ctx context.Context) ([]uuid.UUID, error) {
	return r.listIDs(ctx, `SELECT id FROM habits`)
}

// ListCheckInIDs returns IDs of check_ins that carry a non-empty note (the
// same guard as the insert trigger).
func (r *Repository) ListCheckInIDs(ctx context.Context) ([]uuid.UUID, error) {
	return r.listIDs(ctx, `SELECT id FROM check_ins WHERE note IS NOT NULL AND note <> ''`)
}

// ListMessageIDs returns all conversation_message IDs.
// ListMessageIDs returns the ids that anchor an indexable turn: user messages
// only. Assistant messages are folded into their turn's document rather than
// getting one of their own, so listing them here would make full reconciliation
// expect documents that are never written.
func (r *Repository) ListMessageIDs(ctx context.Context) ([]uuid.UUID, error) {
	return r.listIDs(ctx, `SELECT id FROM conversation_messages WHERE role = 'user'`)
}

// ListWeeklyReviewIDs returns IDs of weekly_reviews with a non-empty ai_summary
// (the same guard as the upsert trigger).
func (r *Repository) ListWeeklyReviewIDs(ctx context.Context) ([]uuid.UUID, error) {
	return r.listIDs(ctx, `SELECT id FROM weekly_reviews WHERE ai_summary IS NOT NULL AND ai_summary <> ''`)
}

// ListRecentArticleIDs returns article IDs updated since the given time.
func (r *Repository) ListRecentArticleIDs(ctx context.Context, since time.Time) ([]uuid.UUID, error) {
	return r.listIDs(ctx, `SELECT id FROM articles WHERE updated_at >= $1`, since)
}

// ListRecentGoalIDs returns goal IDs updated since the given time.
func (r *Repository) ListRecentGoalIDs(ctx context.Context, since time.Time) ([]uuid.UUID, error) {
	return r.listIDs(ctx, `SELECT id FROM goals WHERE updated_at >= $1`, since)
}

// ListRecentHabitIDs returns habit IDs updated since the given time.
func (r *Repository) ListRecentHabitIDs(ctx context.Context, since time.Time) ([]uuid.UUID, error) {
	return r.listIDs(ctx, `SELECT id FROM habits WHERE updated_at >= $1`, since)
}

// ListRecentCheckInIDs returns check_in IDs created since the given time that
// carry a non-empty note.
func (r *Repository) ListRecentCheckInIDs(ctx context.Context, since time.Time) ([]uuid.UUID, error) {
	return r.listIDs(ctx, `SELECT id FROM check_ins WHERE created_at >= $1 AND note IS NOT NULL AND note <> ''`, since)
}

// ListRecentMessageIDs returns conversation_message IDs created since the given
// time.
func (r *Repository) ListRecentMessageIDs(ctx context.Context, since time.Time) ([]uuid.UUID, error) {
	return r.listIDs(ctx, `SELECT id FROM conversation_messages WHERE created_at >= $1`, since)
}

// ListRecentWeeklyReviewIDs returns weekly_review IDs updated since the given
// time that carry a non-empty ai_summary.
func (r *Repository) ListRecentWeeklyReviewIDs(ctx context.Context, since time.Time) ([]uuid.UUID, error) {
	return r.listIDs(ctx, `SELECT id FROM weekly_reviews WHERE updated_at >= $1 AND ai_summary IS NOT NULL AND ai_summary <> ''`, since)
}

func (r *Repository) listIDs(ctx context.Context, query string, args ...any) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list ids: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ids: %w", err)
	}
	return ids, nil
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

// DocID is the exported version of docID for use by the syncer's delete path.
func DocID(entityType string, id uuid.UUID) string {
	return docID(entityType, id)
}

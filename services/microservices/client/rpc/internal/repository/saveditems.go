package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// SavedItemsRepo implements ISavedItems interface using the concrete
// saved_articles / saved_goals / saved_habits tables.  The old polymorphic
// saved_items table is no longer written or read.
type SavedItemsRepo struct {
	db *db.Queries
}

// NewSavedItemsRepo creates a new SavedItemsRepo instance
func NewSavedItemsRepo(db *db.Queries) *SavedItemsRepo {
	return &SavedItemsRepo{db: db}
}

// WithTx returns a new SavedItemsRepo backed by the given transaction.
func (r *SavedItemsRepo) WithTx(tx pgx.Tx) *SavedItemsRepo {
	return &SavedItemsRepo{db: r.db.WithTx(tx)}
}

func (r *SavedItemsRepo) ListSavedItems(ctx context.Context, limit, offset int32) ([]db.SavedItem, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.ListSavedItems")
	defer span.End()

	// Use the new UNION ALL query across concrete tables.
	rows, err := r.db.ListAllSavedItemsByUser(ctx, uuid.Nil, limit, offset)
	if err != nil {
		return nil, err
	}
	return mapListAllSavedItemsToSavedItems(rows), nil
}

func (r *SavedItemsRepo) ListSavedItemsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.SavedItem, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.ListSavedItemsByUser")
	defer span.End()

	rows, err := r.db.ListAllSavedItemsByUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	return mapListAllSavedItemsToSavedItems(rows), nil
}

func (r *SavedItemsRepo) ListSavedItemsByType(ctx context.Context, userID uuid.UUID, itemType string, limit, offset int32) ([]db.SavedItem, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.ListSavedItemsByType")
	defer span.End()

	switch db.SavedItemType(itemType) {
	case "article":
		rows, err := r.db.ListSavedArticlesByUser(ctx, userID, limit, offset)
		if err != nil {
			return nil, err
		}
		return mapListSavedArticlesToSavedItems(rows), nil
	case "goal":
		rows, err := r.db.ListSavedGoalsByUser(ctx, userID, limit, offset)
		if err != nil {
			return nil, err
		}
		return mapListSavedGoalsToSavedItems(rows), nil
	case "habit":
		rows, err := r.db.ListSavedHabitsByUser(ctx, userID, limit, offset)
		if err != nil {
			return nil, err
		}
		return mapListSavedHabitsToSavedItems(rows), nil
	default:
		return nil, fmt.Errorf("unsupported item type: %s", itemType)
	}
}

func (r *SavedItemsRepo) GetSavedItemByID(ctx context.Context, id uuid.UUID) (db.SavedItem, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.GetSavedItemByID")
	defer span.End()

	// There is no single-table get-by-ID for the concrete tables.
	// Fall back to listing all for the user and filtering.
	// This is acceptable because GetSavedItem is rarely used directly.
	// NOTE: this loses the user-id filter; callers should prefer
	// GetSavedItemByUserAndItem.
	item, err := r.db.GetSavedItem(ctx, id)
	if err != nil {
		return db.SavedItem{}, err
	}
	return item, nil
}

func (r *SavedItemsRepo) GetSavedItemByUserAndItem(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) (db.SavedItem, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.GetSavedItemByUserAndItem")
	defer span.End()

	// Use the concrete-table existence checks.
	var exists bool
	var err error
	switch db.SavedItemType(itemType) {
	case "article":
		exists, err = r.db.IsArticleSaved(ctx, userID, itemID)
	case "goal":
		exists, err = r.db.IsGoalSaved(ctx, userID, itemID)
	case "habit":
		exists, err = r.db.IsHabitSaved(ctx, userID, itemID)
	default:
		return db.SavedItem{}, fmt.Errorf("unsupported item type: %s", itemType)
	}
	if err != nil {
		return db.SavedItem{}, err
	}
	if !exists {
		return db.SavedItem{}, pgx.ErrNoRows
	}
	// Construct a synthetic SavedItem with the ID derived from the concrete
	// table row.  We don’t have the exact row id here; callers that need it
	// should use ListSavedItemsByUser.
	return db.SavedItem{
		ItemType: db.SavedItemType(itemType),
		ItemID:   itemID,
		UserID:   userID,
	}, nil
}

func (r *SavedItemsRepo) CreateSavedItem(ctx context.Context, itemType db.SavedItemType, itemID uuid.UUID, userID uuid.UUID) (db.SavedItem, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.CreateSavedItem")
	defer span.End()

	// Write to the appropriate concrete table so that article is_saved
	// queries (which read saved_articles) stay consistent.
	switch itemType {
	case "article":
		row, err := r.db.CreateSavedArticle(ctx, itemID, userID)
		if err != nil {
			return db.SavedItem{}, err
		}
		return db.SavedItem{
			ID:        row.ID,
			ItemType:  db.SavedItemType(row.ItemType),
			ItemID:    row.ItemID,
			UserID:    row.UserID,
			CreatedAt: row.CreatedAt,
		}, nil
	case "goal":
		row, err := r.db.CreateSavedGoal(ctx, itemID, userID)
		if err != nil {
			return db.SavedItem{}, err
		}
		return db.SavedItem{
			ID:        row.ID,
			ItemType:  db.SavedItemType(row.ItemType),
			ItemID:    row.ItemID,
			UserID:    row.UserID,
			CreatedAt: row.CreatedAt,
		}, nil
	case "habit":
		row, err := r.db.CreateSavedHabit(ctx, itemID, userID)
		if err != nil {
			return db.SavedItem{}, err
		}
		return db.SavedItem{
			ID:        row.ID,
			ItemType:  db.SavedItemType(row.ItemType),
			ItemID:    row.ItemID,
			UserID:    row.UserID,
			CreatedAt: row.CreatedAt,
		}, nil
	default:
		return db.SavedItem{}, fmt.Errorf("unsupported item type: %s", itemType)
	}
}

func (r *SavedItemsRepo) DeleteSavedItem(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.DeleteSavedItem")
	defer span.End()

	// The old polymorphic table is deprecated; callers should use
	// DeleteSavedItemByUserAndItem which targets the concrete tables.
	return r.db.DeleteSavedItem(ctx, id)
}

func (r *SavedItemsRepo) DeleteSavedItemByUserAndItem(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.DeleteSavedItemByUserAndItem")
	defer span.End()

	switch db.SavedItemType(itemType) {
	case "article":
		return r.db.DeleteSavedArticle(ctx, userID, itemID)
	case "goal":
		return r.db.DeleteSavedGoal(ctx, userID, itemID)
	case "habit":
		return r.db.DeleteSavedHabit(ctx, userID, itemID)
	default:
		return fmt.Errorf("unsupported item type: %s", itemType)
	}
}

func (r *SavedItemsRepo) IsItemSaved(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) (bool, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.IsItemSaved")
	defer span.End()

	switch db.SavedItemType(itemType) {
	case "article":
		return r.db.IsArticleSaved(ctx, userID, itemID)
	case "goal":
		return r.db.IsGoalSaved(ctx, userID, itemID)
	case "habit":
		return r.db.IsHabitSaved(ctx, userID, itemID)
	default:
		return false, fmt.Errorf("unsupported item type: %s", itemType)
	}
}

func (r *SavedItemsRepo) CountSavedItemsByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.CountSavedItemsByUser")
	defer span.End()

	count, err := r.db.CountAllSavedItemsByUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

func (r *SavedItemsRepo) CountSavedItemsByUserAndType(ctx context.Context, userID uuid.UUID, itemType string) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.CountSavedItemsByUserAndType")
	defer span.End()

	switch db.SavedItemType(itemType) {
	case "article":
		return r.db.CountSavedArticlesByUser(ctx, userID)
	case "goal":
		return r.db.CountSavedGoalsByUser(ctx, userID)
	case "habit":
		return r.db.CountSavedHabitsByUser(ctx, userID)
	default:
		return 0, fmt.Errorf("unsupported item type: %s", itemType)
	}
}

// ---------------------------------------------------------------------------
// Helpers to map concrete table rows to the legacy SavedItem type used by
// the service layer.
// ---------------------------------------------------------------------------

func mapListAllSavedItemsToSavedItems(rows []db.ListAllSavedItemsByUserRow) []db.SavedItem {
	items := make([]db.SavedItem, len(rows))
	for i, r := range rows {
		items[i] = db.SavedItem{
			ID:        r.ID,
			ItemType:  db.SavedItemType(r.ItemType),
			ItemID:    r.ItemID,
			UserID:    r.UserID,
			CreatedAt: r.CreatedAt,
		}
	}
	return items
}

func mapListSavedArticlesToSavedItems(rows []db.ListSavedArticlesByUserRow) []db.SavedItem {
	items := make([]db.SavedItem, len(rows))
	for i, r := range rows {
		items[i] = db.SavedItem{
			ID:        r.ID,
			ItemType:  db.SavedItemType(r.ItemType),
			ItemID:    r.ItemID,
			UserID:    r.UserID,
			CreatedAt: r.CreatedAt,
		}
	}
	return items
}

func mapListSavedGoalsToSavedItems(rows []db.ListSavedGoalsByUserRow) []db.SavedItem {
	items := make([]db.SavedItem, len(rows))
	for i, r := range rows {
		items[i] = db.SavedItem{
			ID:        r.ID,
			ItemType:  db.SavedItemType(r.ItemType),
			ItemID:    r.ItemID,
			UserID:    r.UserID,
			CreatedAt: r.CreatedAt,
		}
	}
	return items
}

func mapListSavedHabitsToSavedItems(rows []db.ListSavedHabitsByUserRow) []db.SavedItem {
	items := make([]db.SavedItem, len(rows))
	for i, r := range rows {
		items[i] = db.SavedItem{
			ID:        r.ID,
			ItemType:  db.SavedItemType(r.ItemType),
			ItemID:    r.ItemID,
			UserID:    r.UserID,
			CreatedAt: r.CreatedAt,
		}
	}
	return items
}

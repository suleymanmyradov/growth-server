package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// SavedItemsRepo implements ISavedItems on top of the three concrete
// saved_articles / saved_goals / saved_habits tables.
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

func (r *SavedItemsRepo) ListSavedItemsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]SavedItem, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.ListSavedItemsByUser")
	defer span.End()

	rows, err := r.db.ListAllSavedItemsByUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	items := make([]SavedItem, len(rows))
	for i, row := range rows {
		items[i] = SavedItem{
			ID:        row.ID,
			ItemType:  row.ItemType,
			ItemID:    row.ItemID,
			UserID:    row.UserID,
			CreatedAt: row.CreatedAt,
		}
	}
	return items, nil
}

func (r *SavedItemsRepo) ListSavedItemsByType(ctx context.Context, userID uuid.UUID, itemType string, limit, offset int32) ([]SavedItem, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.ListSavedItemsByType")
	defer span.End()

	switch itemType {
	case "article":
		rows, err := r.db.ListSavedArticlesByUser(ctx, userID, limit, offset)
		if err != nil {
			return nil, err
		}
		items := make([]SavedItem, len(rows))
		for i, row := range rows {
			items[i] = SavedItem{ID: row.ID, ItemType: row.ItemType, ItemID: row.ItemID, UserID: row.UserID, CreatedAt: row.CreatedAt}
		}
		return items, nil
	case "goal":
		rows, err := r.db.ListSavedGoalsByUser(ctx, userID, limit, offset)
		if err != nil {
			return nil, err
		}
		items := make([]SavedItem, len(rows))
		for i, row := range rows {
			items[i] = SavedItem{ID: row.ID, ItemType: row.ItemType, ItemID: row.ItemID, UserID: row.UserID, CreatedAt: row.CreatedAt}
		}
		return items, nil
	case "habit":
		rows, err := r.db.ListSavedHabitsByUser(ctx, userID, limit, offset)
		if err != nil {
			return nil, err
		}
		items := make([]SavedItem, len(rows))
		for i, row := range rows {
			items[i] = SavedItem{ID: row.ID, ItemType: row.ItemType, ItemID: row.ItemID, UserID: row.UserID, CreatedAt: row.CreatedAt}
		}
		return items, nil
	default:
		return nil, fmt.Errorf("unsupported item type: %s", itemType)
	}
}

func (r *SavedItemsRepo) CreateSavedItem(ctx context.Context, itemType string, itemID uuid.UUID, userID uuid.UUID) (SavedItem, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.CreateSavedItem")
	defer span.End()

	switch itemType {
	case "article":
		row, err := r.db.CreateSavedArticle(ctx, itemID, userID)
		if err != nil {
			return SavedItem{}, err
		}
		return SavedItem{ID: row.ID, ItemType: row.ItemType, ItemID: row.ItemID, UserID: row.UserID, CreatedAt: row.CreatedAt}, nil
	case "goal":
		row, err := r.db.CreateSavedGoal(ctx, itemID, userID)
		if err != nil {
			return SavedItem{}, err
		}
		return SavedItem{ID: row.ID, ItemType: row.ItemType, ItemID: row.ItemID, UserID: row.UserID, CreatedAt: row.CreatedAt}, nil
	case "habit":
		row, err := r.db.CreateSavedHabit(ctx, itemID, userID)
		if err != nil {
			return SavedItem{}, err
		}
		return SavedItem{ID: row.ID, ItemType: row.ItemType, ItemID: row.ItemID, UserID: row.UserID, CreatedAt: row.CreatedAt}, nil
	default:
		return SavedItem{}, fmt.Errorf("unsupported item type: %s", itemType)
	}
}

func (r *SavedItemsRepo) DeleteSavedItemByUserAndItem(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.DeleteSavedItemByUserAndItem")
	defer span.End()

	switch itemType {
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

	switch itemType {
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

	switch itemType {
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

package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// SavedItemsRepo implements ISavedItems interface
type SavedItemsRepo struct {
	db *db.Queries
}

// NewSavedItemsRepo creates a new SavedItemsRepo instance
func NewSavedItemsRepo(db *db.Queries) *SavedItemsRepo {
	return &SavedItemsRepo{db: db}
}

func (r *SavedItemsRepo) ListSavedItems(ctx context.Context, limit, offset int32) ([]db.SavedItem, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.ListSavedItems")
	defer span.End()

	return r.db.ListSavedItems(ctx, db.ListSavedItemsParams{
		Limit:  limit,
		Offset: offset,
	})
}

func (r *SavedItemsRepo) ListSavedItemsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.SavedItem, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.ListSavedItemsByUser")
	defer span.End()

	return r.db.ListSavedItemsByUser(ctx, db.ListSavedItemsByUserParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
}

func (r *SavedItemsRepo) ListSavedItemsByType(ctx context.Context, userID uuid.UUID, itemType string, limit, offset int32) ([]db.SavedItem, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.ListSavedItemsByType")
	defer span.End()

	return r.db.ListSavedItemsByType(ctx, db.ListSavedItemsByTypeParams{
		UserID:   userID,
		ItemType: itemType,
		Limit:    limit,
		Offset:   offset,
	})
}

func (r *SavedItemsRepo) GetSavedItem(ctx context.Context, id uuid.UUID) (db.SavedItem, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.GetSavedItem")
	defer span.End()

	return r.db.GetSavedItem(ctx, id)
}

func (r *SavedItemsRepo) GetSavedItemByUserAndItem(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) (db.SavedItem, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.GetSavedItemByUserAndItem")
	defer span.End()

	return r.db.GetSavedItemByUserAndItem(ctx, db.GetSavedItemByUserAndItemParams{
		UserID:   userID,
		ItemType: itemType,
		ItemID:   itemID,
	})
}

func (r *SavedItemsRepo) CreateSavedItem(ctx context.Context, params db.CreateSavedItemParams) (db.SavedItem, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.CreateSavedItem")
	defer span.End()

	return r.db.CreateSavedItem(ctx, params)
}

func (r *SavedItemsRepo) DeleteSavedItem(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.DeleteSavedItem")
	defer span.End()

	return r.db.DeleteSavedItem(ctx, id)
}

func (r *SavedItemsRepo) DeleteSavedItemByUserAndItem(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.DeleteSavedItemByUserAndItem")
	defer span.End()

	return r.db.DeleteSavedItemByUserAndItem(ctx, db.DeleteSavedItemByUserAndItemParams{
		UserID:   userID,
		ItemType: itemType,
		ItemID:   itemID,
	})
}

func (r *SavedItemsRepo) IsItemSaved(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) (bool, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.IsItemSaved")
	defer span.End()

	return r.db.IsItemSaved(ctx, db.IsItemSavedParams{
		UserID:   userID,
		ItemType: itemType,
		ItemID:   itemID,
	})
}

func (r *SavedItemsRepo) CountSavedItems(ctx context.Context) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.CountSavedItems")
	defer span.End()

	return r.db.CountSavedItems(ctx)
}

func (r *SavedItemsRepo) CountSavedItemsByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.CountSavedItemsByUser")
	defer span.End()

	return r.db.CountSavedItemsByUser(ctx, userID)
}

func (r *SavedItemsRepo) CountSavedItemsByUserAndType(ctx context.Context, userID uuid.UUID, itemType string) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SavedItemsRepo.CountSavedItemsByUserAndType")
	defer span.End()

	return r.db.CountSavedItemsByUserAndType(ctx, db.CountSavedItemsByUserAndTypeParams{
		UserID:   userID,
		ItemType: itemType,
	})
}

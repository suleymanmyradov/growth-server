package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// PlanAdjustmentSuggestionsRepo implements IPlanAdjustmentSuggestions interface
type PlanAdjustmentSuggestionsRepo struct {
	db *db.Queries
}

// NewPlanAdjustmentSuggestionsRepo creates a new PlanAdjustmentSuggestionsRepo instance
func NewPlanAdjustmentSuggestionsRepo(db *db.Queries) *PlanAdjustmentSuggestionsRepo {
	return &PlanAdjustmentSuggestionsRepo{db: db}
}

// WithTx returns a new PlanAdjustmentSuggestionsRepo backed by the given transaction.
func (r *PlanAdjustmentSuggestionsRepo) WithTx(tx pgx.Tx) *PlanAdjustmentSuggestionsRepo {
	return &PlanAdjustmentSuggestionsRepo{db: r.db.WithTx(tx)}
}

func (r *PlanAdjustmentSuggestionsRepo) CreatePlanAdjustmentSuggestion(ctx context.Context, params db.CreatePlanAdjustmentSuggestionParams) (db.PlanAdjustment, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.CreatePlanAdjustmentSuggestion")
	defer span.End()

	return r.db.CreatePlanAdjustmentSuggestion(ctx, params)
}

func (r *PlanAdjustmentSuggestionsRepo) GetPlanAdjustmentSuggestion(ctx context.Context, id uuid.UUID, userID uuid.UUID) (db.PlanAdjustment, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.GetPlanAdjustmentSuggestion")
	defer span.End()

	return r.db.GetPlanAdjustmentSuggestion(ctx, id, userID)
}

func (r *PlanAdjustmentSuggestionsRepo) ListPendingPlanAdjustmentSuggestions(ctx context.Context, userID uuid.UUID, limit int32, offset int32) ([]db.PlanAdjustment, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.ListPendingPlanAdjustmentSuggestions")
	defer span.End()

	return r.db.ListPendingPlanAdjustmentSuggestions(ctx, userID, limit, offset)
}

func (r *PlanAdjustmentSuggestionsRepo) ListAllPlanAdjustmentSuggestions(ctx context.Context, userID uuid.UUID, limit int32, offset int32) ([]db.PlanAdjustment, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.ListAllPlanAdjustmentSuggestions")
	defer span.End()

	return r.db.ListAllPlanAdjustmentSuggestions(ctx, userID, limit, offset)
}

func (r *PlanAdjustmentSuggestionsRepo) ListPlanAdjustmentSuggestionsByHabit(ctx context.Context, userID uuid.UUID, habitID uuid.NullUUID, limit int32, offset int32) ([]db.PlanAdjustment, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.ListPlanAdjustmentSuggestionsByHabit")
	defer span.End()

	return r.db.ListPlanAdjustmentSuggestionsByHabit(ctx, userID, habitID, limit, offset)
}

func (r *PlanAdjustmentSuggestionsRepo) ListPlanAdjustmentSuggestionsByGoal(ctx context.Context, userID uuid.UUID, goalID uuid.NullUUID, limit int32, offset int32) ([]db.PlanAdjustment, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.ListPlanAdjustmentSuggestionsByGoal")
	defer span.End()

	return r.db.ListPlanAdjustmentSuggestionsByGoal(ctx, userID, goalID, limit, offset)
}

func (r *PlanAdjustmentSuggestionsRepo) UpdatePlanAdjustmentSuggestionStatus(ctx context.Context, id uuid.UUID, userID uuid.UUID, status string) (db.PlanAdjustment, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.UpdatePlanAdjustmentSuggestionStatus")
	defer span.End()

	return r.db.UpdatePlanAdjustmentSuggestionStatus(ctx, id, userID, status)
}

func (r *PlanAdjustmentSuggestionsRepo) UpdatePlanAdjustmentSuggestion(ctx context.Context, params db.UpdatePlanAdjustmentSuggestionParams) (db.PlanAdjustment, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.UpdatePlanAdjustmentSuggestion")
	defer span.End()

	return r.db.UpdatePlanAdjustmentSuggestion(ctx, params)
}

func (r *PlanAdjustmentSuggestionsRepo) DeletePlanAdjustmentSuggestion(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.DeletePlanAdjustmentSuggestion")
	defer span.End()

	return r.db.DeletePlanAdjustmentSuggestion(ctx, id, userID)
}

func (r *PlanAdjustmentSuggestionsRepo) CountPendingPlanAdjustmentSuggestions(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.CountPendingPlanAdjustmentSuggestions")
	defer span.End()

	return r.db.CountPendingPlanAdjustmentSuggestions(ctx, userID)
}

func (r *PlanAdjustmentSuggestionsRepo) DismissOldPendingSuggestions(ctx context.Context, userID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.DismissOldPendingSuggestions")
	defer span.End()

	return r.db.DismissOldPendingSuggestions(ctx, userID)
}

func (r *PlanAdjustmentSuggestionsRepo) ApplyPlanAdjustmentSuggestion(ctx context.Context, id uuid.UUID, userID uuid.UUID) (db.PlanAdjustment, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.ApplyPlanAdjustmentSuggestion")
	defer span.End()

	return r.db.ApplyPlanAdjustmentSuggestion(ctx, id, userID)
}

package repository

import (
	"context"

	"github.com/google/uuid"
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

func (r *PlanAdjustmentSuggestionsRepo) CreatePlanAdjustmentSuggestion(ctx context.Context, params db.CreatePlanAdjustmentSuggestionParams) (db.PlanAdjustmentSuggestion, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.CreatePlanAdjustmentSuggestion")
	defer span.End()

	return r.db.CreatePlanAdjustmentSuggestion(ctx, params)
}

func (r *PlanAdjustmentSuggestionsRepo) GetPlanAdjustmentSuggestion(ctx context.Context, params db.GetPlanAdjustmentSuggestionParams) (db.PlanAdjustmentSuggestion, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.GetPlanAdjustmentSuggestion")
	defer span.End()

	return r.db.GetPlanAdjustmentSuggestion(ctx, params)
}

func (r *PlanAdjustmentSuggestionsRepo) ListPendingPlanAdjustmentSuggestions(ctx context.Context, params db.ListPendingPlanAdjustmentSuggestionsParams) ([]db.PlanAdjustmentSuggestion, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.ListPendingPlanAdjustmentSuggestions")
	defer span.End()

	return r.db.ListPendingPlanAdjustmentSuggestions(ctx, params)
}

func (r *PlanAdjustmentSuggestionsRepo) ListAllPlanAdjustmentSuggestions(ctx context.Context, params db.ListAllPlanAdjustmentSuggestionsParams) ([]db.PlanAdjustmentSuggestion, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.ListAllPlanAdjustmentSuggestions")
	defer span.End()

	return r.db.ListAllPlanAdjustmentSuggestions(ctx, params)
}

func (r *PlanAdjustmentSuggestionsRepo) ListPlanAdjustmentSuggestionsByHabit(ctx context.Context, params db.ListPlanAdjustmentSuggestionsByHabitParams) ([]db.PlanAdjustmentSuggestion, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.ListPlanAdjustmentSuggestionsByHabit")
	defer span.End()

	return r.db.ListPlanAdjustmentSuggestionsByHabit(ctx, params)
}

func (r *PlanAdjustmentSuggestionsRepo) ListPlanAdjustmentSuggestionsByGoal(ctx context.Context, params db.ListPlanAdjustmentSuggestionsByGoalParams) ([]db.PlanAdjustmentSuggestion, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.ListPlanAdjustmentSuggestionsByGoal")
	defer span.End()

	return r.db.ListPlanAdjustmentSuggestionsByGoal(ctx, params)
}

func (r *PlanAdjustmentSuggestionsRepo) UpdatePlanAdjustmentSuggestionStatus(ctx context.Context, params db.UpdatePlanAdjustmentSuggestionStatusParams) (db.PlanAdjustmentSuggestion, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.UpdatePlanAdjustmentSuggestionStatus")
	defer span.End()

	return r.db.UpdatePlanAdjustmentSuggestionStatus(ctx, params)
}

func (r *PlanAdjustmentSuggestionsRepo) UpdatePlanAdjustmentSuggestion(ctx context.Context, params db.UpdatePlanAdjustmentSuggestionParams) (db.PlanAdjustmentSuggestion, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.UpdatePlanAdjustmentSuggestion")
	defer span.End()

	return r.db.UpdatePlanAdjustmentSuggestion(ctx, params)
}

func (r *PlanAdjustmentSuggestionsRepo) DeletePlanAdjustmentSuggestion(ctx context.Context, params db.DeletePlanAdjustmentSuggestionParams) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.DeletePlanAdjustmentSuggestion")
	defer span.End()

	return r.db.DeletePlanAdjustmentSuggestion(ctx, params)
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

func (r *PlanAdjustmentSuggestionsRepo) ApplyPlanAdjustmentSuggestion(ctx context.Context, params db.ApplyPlanAdjustmentSuggestionParams) (db.PlanAdjustmentSuggestion, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "PlanAdjustmentSuggestionsRepo.ApplyPlanAdjustmentSuggestion")
	defer span.End()

	return r.db.ApplyPlanAdjustmentSuggestion(ctx, params)
}

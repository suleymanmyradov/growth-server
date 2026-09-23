package goalslogic

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type UpdateGoalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateGoalLogic {
	return &UpdateGoalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateGoalLogic) UpdateGoal(in *client.UpdateGoalRequest) (*client.UpdateGoalResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "UpdateGoalLogic.UpdateGoal")
	defer span.End()
	goalID, err := uuid.Parse(in.GoalId)
	if err != nil {
		l.Errorf("Invalid goal ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid goal id")
	}

	// Fetch the existing goal so we can preserve measurement fields that the
	// client didn't include in a partial update. The UpdateGoal SQL query
	// unconditionally writes all columns, so we must backfill from the current
	// row to avoid resetting measurement='manual' and values to 0 when the
	// client only wants to change e.g. the title.
	existing, err := l.svcCtx.Repo.Goals.GetGoalByID(ctx, goalID)
	if err != nil {
		l.Errorf("Failed to get goal: %v", err)
		return nil, status.Error(codes.NotFound, "goal not found")
	}

	// Ownership check: verify the caller owns this goal before mutating.
	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	if existing.UserID.String() != p.UserID {
		return nil, status.Error(codes.PermissionDenied, "access denied")
	}

	// Resolve measurement: use the request value if provided, otherwise keep
	// the existing measurement. Never default to "manual" on update — that
	// would silently change a typed goal back to manual.
	measurement := in.Measurement
	if measurement == "" {
		measurement = existing.Measurement
	}
	if !validMeasurement(measurement) {
		return nil, status.Error(codes.InvalidArgument, "measurement must be one of: binary, numeric, milestone, habit, manual")
	}

	// Backfill value fields from the existing row when the client sends
	// zeros (proto default for float64). This is imperfect — a client can't
	// explicitly set a value back to 0 — but it prevents the common case of
	// a partial title/description edit from wiping the measurement config.
	// The frontend always sends the full form, so this mainly protects
	// non-frontend callers.
	startValue := in.StartValue
	if startValue == 0 {
		startValue = numericToFloat(existing.StartValue)
	}
	currentValue := in.CurrentValue
	if currentValue == 0 {
		currentValue = numericToFloat(existing.CurrentValue)
	}
	targetValue := in.TargetValue
	if targetValue == 0 {
		targetValue = numericToFloat(existing.TargetValue)
	}
	unit := in.Unit
	if unit == "" {
		if existing.Unit != nil {
			unit = *existing.Unit
		}
	}

	// Validate type-specific requirements only when the measurement is
	// actually being set to that type (not when preserving the existing type
	// and the client didn't send value fields).
	if measurement == MeasurementHabit && len(in.RelatedHabitIds) == 0 {
		// Allow preserving existing habit links if the client didn't send any.
		// Only reject when the client explicitly sends an empty list for a
		// habit goal — but since proto repeated fields default to empty, we
		// can't distinguish "not sent" from "explicitly empty". So only reject
		// when switching TO habit from a different type.
		if existing.Measurement != MeasurementHabit {
			return nil, status.Error(codes.InvalidArgument, "habit goals require at least one linked habit")
		}
	}
	if measurement == MeasurementNumeric && targetValue == startValue {
		return nil, status.Error(codes.InvalidArgument, "numeric goals require a target value different from start value")
	}

	// Backfill the remaining columns from the existing row when the request
	// omits them — the UpdateGoal SQL writes every column unconditionally, so
	// a partial update would otherwise blank title/description/category/due_date.
	title := in.Title
	if title == "" {
		title = existing.Title
	}
	desc := existing.Description
	if in.Description != "" {
		desc = &in.Description
	}
	slug := in.Category
	if slug == "" {
		slug = existing.Category
	}
	dueTime := existing.DueDate
	if in.DueDate > 0 {
		dueTime = pgtype.Timestamptz{Time: time.Unix(in.DueDate, 0), Valid: true}
	}
	var unitPtr *string
	if unit != "" {
		unitPtr = &unit
	}

	params := db.UpdateGoalParams{
		ID:           goalID,
		Title:        title,
		Description:  desc,
		Slug:         slug,
		DueDate:      dueTime,
		Measurement:  measurement,
		StartValue:   floatToNumeric(startValue),
		CurrentValue: floatToNumeric(currentValue),
		TargetValue:  floatToNumeric(targetValue),
		Unit:         unitPtr,
	}

	// Perform the goal update, habit relink, milestone reconcile, and progress
	// recompute atomically inside a transaction. This is the operation doing
	// the most multi-row work (habit links + milestones + recompute), so it
	// must be atomic like the milestone/value RPCs.
	// relatedHabitIDs is what the response echoes back — the request list when
	// provided, otherwise the preserved existing links.
	var goal db.GetGoalRow
	var milestones []db.GoalMilestone
	relatedHabitIDs := in.RelatedHabitIds
	err = l.svcCtx.RunInTx(ctx, p.UserID, func(txRepo *repository.Repository) error {
		var uErr error
		goal, uErr = txRepo.Goals.UpdateGoal(ctx, params)
		if uErr != nil {
			return status.Error(codes.Internal, "failed to update goal")
		}

		// Replace goal-habit links only when the client sent a new set. An
		// empty list means "not provided" — preserve existing links instead of
		// wiping them (proto can't distinguish omitted from explicitly-empty).
		if len(in.RelatedHabitIds) > 0 {
			if hErr := txRepo.Goals.UnlinkAllGoalHabits(ctx, goalID); hErr != nil {
				return fmt.Errorf("unlink old goal-habits: %w", hErr)
			}
			habitIDs := parseHabitIDs(in.RelatedHabitIds)
			if len(habitIDs) > 0 {
				if lErr := txRepo.Goals.LinkGoalHabitsBatch(ctx, goalID, habitIDs); lErr != nil {
					return fmt.Errorf("link habits to goal: %w", lErr)
				}
			}
		} else {
			linked, lErr := txRepo.Goals.ListGoalHabitIDsByGoal(ctx, goalID)
			if lErr != nil {
				return fmt.Errorf("list goal-habit links: %w", lErr)
			}
			relatedHabitIDs = make([]string, 0, len(linked))
			for _, h := range linked {
				relatedHabitIDs = append(relatedHabitIDs, h.String())
			}
		}

		// Reconcile milestones by ID (not title). The new `milestones` field
		// (repeated MilestoneInput) carries optional IDs: a non-empty id
		// identifies an existing milestone to update (preserving done_at),
		// an empty id means create new. Milestones not in the list are deleted.
		// sort_order is rewritten to the list index for every entry.
		//
		// Falls back to the deprecated `milestone_titles` field for older
		// clients that don't send `milestones` yet.
		if measurement == MeasurementMilestone {
			if len(in.Milestones) > 0 {
				existingMs, lErr := txRepo.Goals.ListGoalMilestones(ctx, goalID)
				if lErr != nil {
					return fmt.Errorf("list goal milestones: %w", lErr)
				}
				existingByID := make(map[uuid.UUID]db.GoalMilestone, len(existingMs))
				for _, m := range existingMs {
					existingByID[m.ID] = m
				}
				seen := make(map[uuid.UUID]bool, len(in.Milestones))
				milestones = make([]db.GoalMilestone, 0, len(in.Milestones))
				for i, mi := range in.Milestones {
					if mi.Title == "" {
						continue
					}
					if mi.Id != "" {
						mID, pErr := uuid.Parse(mi.Id)
						if pErr != nil {
							return status.Error(codes.InvalidArgument, "invalid milestone id")
						}
						if _, ok := existingByID[mID]; ok {
							seen[mID] = true
							updated, uErr := txRepo.Goals.UpdateGoalMilestone(ctx, mID, goalID, mi.Title, int32(i))
							if uErr != nil {
								return fmt.Errorf("update goal milestone: %w", uErr)
							}
							milestones = append(milestones, updated)
							continue
						}
					}
					// New milestone (empty id or id not found).
					m, cErr := txRepo.Goals.CreateGoalMilestone(ctx, goalID, mi.Title, int32(i))
					if cErr != nil {
						return fmt.Errorf("create goal milestone: %w", cErr)
					}
					milestones = append(milestones, m)
				}
				// Delete milestones not in the new list.
				for _, m := range existingMs {
					if !seen[m.ID] {
						if dErr := txRepo.Goals.DeleteGoalMilestone(ctx, m.ID, goalID); dErr != nil {
							return fmt.Errorf("delete goal milestone: %w", dErr)
						}
					}
				}
			} else if len(in.MilestoneTitles) > 0 {
				// Deprecated path: reconcile by title (preserves done_at for
				// unchanged titles, loses it on rename).
				existingMs, lErr := txRepo.Goals.ListGoalMilestones(ctx, goalID)
				if lErr != nil {
					return fmt.Errorf("list goal milestones: %w", lErr)
				}
				existingByTitle := make(map[string]db.GoalMilestone, len(existingMs))
				for _, m := range existingMs {
					existingByTitle[m.Title] = m
				}
				seen := make(map[string]bool, len(in.MilestoneTitles))
				milestones = make([]db.GoalMilestone, 0, len(in.MilestoneTitles))
				for i, title := range in.MilestoneTitles {
					if title == "" {
						continue
					}
					seen[title] = true
					if m, ok := existingByTitle[title]; ok {
						// Update sort_order to match the new list position.
						updated, uErr := txRepo.Goals.UpdateGoalMilestone(ctx, m.ID, goalID, title, int32(i))
						if uErr != nil {
							return fmt.Errorf("update milestone sort_order: %w", uErr)
						}
						milestones = append(milestones, updated)
						continue
					}
					m, cErr := txRepo.Goals.CreateGoalMilestone(ctx, goalID, title, int32(i))
					if cErr != nil {
						return fmt.Errorf("create goal milestone: %w", cErr)
					}
					milestones = append(milestones, m)
				}
				for _, m := range existingMs {
					if !seen[m.Title] {
						if dErr := txRepo.Goals.DeleteGoalMilestone(ctx, m.ID, goalID); dErr != nil {
							return fmt.Errorf("delete goal milestone: %w", dErr)
						}
					}
				}
			} else {
				var lErr error
				milestones, lErr = txRepo.Goals.ListGoalMilestones(ctx, goalID)
				if lErr != nil {
					return fmt.Errorf("list goal milestones: %w", lErr)
				}
			}
		}

		// Recompute progress for all non-manual types.
		if measurement != MeasurementManual {
			var rErr error
			goal, rErr = RecomputeGoalProgressWithRepo(ctx, txRepo.Goals, txRepo.CheckIns, goalID)
			if rErr != nil {
				return fmt.Errorf("recompute goal progress: %w", rErr)
			}
		}
		return nil
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
			return nil, st.Err()
		}
		l.Errorf("Failed to update goal: %v", err)
		return nil, status.Error(codes.Internal, "failed to update goal")
	}

	l.svcCtx.InvalidatePersonalizationContext(ctx, goal.UserID)

	// Fire-and-forget publish goal_updated event so the notifications service
	// can reschedule the goal_deadline reminder for the new deadline.
	if l.svcCtx.EventsPub != nil {
		deadlineAt := ""
		if goal.DueDate.Valid {
			deadlineAt = goal.DueDate.Time.Format(time.RFC3339)
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			env, err := events.NewEnvelope(events.TypeGoalUpdated, events.GoalUpdated{
				UserID:     goal.UserID.String(),
				GoalID:     goal.ID.String(),
				Title:      goal.Title,
				DeadlineAt: deadlineAt,
			})
			if err != nil {
				logx.Errorf("envelope: %v", err)
				return
			}
			if err := l.svcCtx.EventsPub.Publish(ctx, env); err != nil {
				logx.Errorf("publish goal_updated event: %v", err)
			}
		}()
	}

	return &client.UpdateGoalResponse{
		Goal: goalToProto(goal, relatedHabitIDs, milestones),
	}, nil
}

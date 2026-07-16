package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
)

// AuthEventsHandler consumes auth events (user_deleted, user_profile_updated)
// from the growth.events topic and maintains the local user_profiles read model
// + cleans up client-owned tables on account deletion.
type AuthEventsHandler struct {
	repo *repository.Repository
	dbq  *db.Queries
}

// NewAuthEventsHandler creates a handler with the given dependencies.
func NewAuthEventsHandler(repo *repository.Repository, dbq *db.Queries) *AuthEventsHandler {
	return &AuthEventsHandler{repo: repo, dbq: dbq}
}

// Consume is the kq.ConsumeHandler callback.
func (h *AuthEventsHandler) Consume(ctx context.Context, _ string, raw string) error {
	var env events.Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		logx.WithContext(ctx).Errorf("invalid envelope: %v", err)
		return nil
	}

	// Dedup via processed_events so redeliveries don't double-process.
	eventID, err := uuid.Parse(env.EventID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid event ID %q: %v", env.EventID, err)
		return nil
	}
	processed, err := h.dbq.IsClientEventProcessed(ctx, eventID.String())
	if err != nil {
		logx.WithContext(ctx).Errorf("check processed_events: %v", err)
		// Non-fatal: proceed without dedup rather than blocking the queue.
	} else if processed {
		logx.WithContext(ctx).Infof("duplicate event %s, skipping", env.EventID)
		return nil
	}

	var handlerErr error
	switch events.EventType(env.EventType) {
	case events.TypeUserDeleted:
		handlerErr = h.onUserDeleted(ctx, env)
	case events.TypeUserProfileUpdated:
		handlerErr = h.onUserProfileUpdated(ctx, env)
	default:
		return nil
	}

	if handlerErr != nil {
		return handlerErr
	}

	// Mark event as processed only after successful handling.
	if err := h.dbq.MarkClientEventProcessed(ctx, eventID.String()); err != nil {
		logx.WithContext(ctx).Errorf("mark event %s processed: %v", env.EventID, err)
	}

	return nil
}

func (h *AuthEventsHandler) onUserDeleted(ctx context.Context, env events.Envelope) error {
	var p events.UserDeleted
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.WithContext(ctx).Errorf("unmarshal UserDeleted: %v", err)
		return nil
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid userID %q: %v", p.UserID, err)
		return nil
	}

	logx.WithContext(ctx).Infof("cleaning up client-owned data for user %s", userID)

	// Delete all client-owned rows for this user. Order doesn't matter much
	// since there are no cross-table FKs (we removed them in step 1).
	if err := h.dbq.DeleteHabitsByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete habits: %w", err)
	}
	if err := h.dbq.DeleteGoalsByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete goals: %w", err)
	}
	if err := h.dbq.DeleteCheckInsByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete check_ins: %w", err)
	}
	if err := h.dbq.DeleteWeeklyReviewsByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete weekly_reviews: %w", err)
	}
	if err := h.dbq.DeletePlanAdjustmentsByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete plan_adjustments: %w", err)
	}
	if err := h.dbq.DeleteSavedArticlesByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete saved_articles: %w", err)
	}
	if err := h.dbq.DeleteSavedGoalsByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete saved_goals: %w", err)
	}
	if err := h.dbq.DeleteSavedHabitsByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete saved_habits: %w", err)
	}
	if err := h.dbq.DeleteArticleLikesByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete article_likes: %w", err)
	}
	if err := h.dbq.DeleteArticleSharesByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete article_shares: %w", err)
	}
	if err := h.dbq.DeleteActivitiesByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete activities: %w", err)
	}
	if err := h.dbq.DeleteSubscriptionsByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete subscriptions: %w", err)
	}
	if err := h.dbq.DeleteUpgradeEventsByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete upgrade_events: %w", err)
	}

	// Delete user-owned preferences and coaching profile.
	if err := h.repo.UserPreferences.DeleteUserPreferences(ctx, userID); err != nil {
		return fmt.Errorf("delete user_preferences: %w", err)
	}
	if err := h.repo.CoachingProfiles.DeleteCoachingProfile(ctx, userID); err != nil {
		return fmt.Errorf("delete coaching_profile: %w", err)
	}

	// Delete the local user_profiles read model row.
	if err := h.dbq.DeleteUserProfile(ctx, userID); err != nil {
		return fmt.Errorf("delete user_profile: %w", err)
	}

	logx.WithContext(ctx).Infof("cleanup complete for user %s", userID)
	return nil
}

func (h *AuthEventsHandler) onUserProfileUpdated(ctx context.Context, env events.Envelope) error {
	var p events.UserProfileUpdated
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.WithContext(ctx).Errorf("unmarshal UserProfileUpdated: %v", err)
		return nil
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid userID %q: %v", p.UserID, err)
		return nil
	}

	logx.WithContext(ctx).Infof("upserting user_profiles read model for user %s", userID)

	var bio, location, website, avatar *string
	if p.Bio != "" {
		bio = &p.Bio
	}
	if p.Location != "" {
		location = &p.Location
	}
	if p.Website != "" {
		website = &p.Website
	}
	if p.Avatar != "" {
		avatar = &p.Avatar
	}

	params := db.UpsertUserProfileParams{
		ID:        userID,
		Username:  p.Username,
		FullName:  p.Name,
		Bio:       bio,
		Location:  location,
		Website:   website,
		Interests: p.Interests,
		AvatarUrl: avatar,
	}

	if err := h.dbq.UpsertUserProfile(ctx, params); err != nil {
		return fmt.Errorf("upsert user_profile: %w", err)
	}

	return nil
}

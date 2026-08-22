package notification

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
)

type Request struct {
	UserID           uuid.UUID
	Type             string
	Title            string
	Message          string
	Destination      string
	ResourceID       uuid.UUID
	DeduplicationKey string
	Metadata         map[string]any
	Push             bool
	Email            bool
}

func Create(ctx context.Context, repo *repository.Repository, req Request) (db.CreateNotificationRow, error) {
	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return db.CreateNotificationRow{}, fmt.Errorf("marshal notification metadata: %w", err)
	}

	params := db.CreateNotificationParams{
		Title:    req.Title,
		Message:  req.Message,
		Type:     req.Type,
		UserID:   req.UserID,
		Metadata: raw,
	}
	if req.Destination != "" {
		params.Destination = &req.Destination
	}
	if req.ResourceID != uuid.Nil {
		params.ResourceID = uuid.NullUUID{UUID: req.ResourceID, Valid: true}
	}
	if req.DeduplicationKey != "" {
		params.DeduplicationKey = &req.DeduplicationKey
	}

	created, err := repo.Notifications.Create(ctx, params)
	if err != nil {
		return db.CreateNotificationRow{}, fmt.Errorf("create notification: %w", err)
	}
	if req.Push {
		if _, err := repo.Deliveries.Create(ctx, created.ID, req.UserID, "push"); err != nil {
			return db.CreateNotificationRow{}, fmt.Errorf("create push delivery: %w", err)
		}
	}
	if req.Email {
		if _, err := repo.Deliveries.Create(ctx, created.ID, req.UserID, "email"); err != nil {
			return db.CreateNotificationRow{}, fmt.Errorf("create email delivery: %w", err)
		}
	}
	return created, nil
}

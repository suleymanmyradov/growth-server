// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package notifications

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"

	"github.com/zeromicro/go-zero/core/logx"
)

const broadcastChunkSize = 2000

// allowedBroadcastItemTypes mirrors the notifications.type CHECK constraint.
var allowedBroadcastItemTypes = map[string]bool{
	"habit_reminder":  true,
	"missed_check_in": true,
	"goal_deadline":   true,
	"achievement":     true,
	"weekly_review":   true,
	"encouragement":   true,
	"system":          true,
	"ai_feedback":     true,
}

type AdminBroadcastNotificationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBroadcastNotificationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBroadcastNotificationLogic {
	return &AdminBroadcastNotificationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminBroadcastNotificationLogic) AdminBroadcastNotification(req *types.BroadcastNotificationRequest) (resp *types.BroadcastNotificationResponse, err error) {
	resp = &types.BroadcastNotificationResponse{}

	// Validate input.
	req.Title = strings.TrimSpace(req.Title)
	req.Message = strings.TrimSpace(req.Message)
	req.ItemType = strings.TrimSpace(req.ItemType)
	req.Segment = strings.TrimSpace(req.Segment)
	if req.Title == "" || req.Message == "" {
		return nil, fmt.Errorf("title and message are required")
	}
	if !allowedBroadcastItemTypes[req.ItemType] {
		return nil, fmt.Errorf("invalid itemType %q", req.ItemType)
	}
	segment := req.Segment
	if segment == "" {
		segment = "all"
	}
	if segment != "all" && segment != "premium" && segment != "free" {
		return nil, fmt.Errorf("invalid segment %q (want all|premium|free)", segment)
	}

	if l.svcCtx.EventsPub == nil {
		return nil, fmt.Errorf("broadcast publisher not configured (kafka)")
	}

	// Resolve the full audience from auth.
	allUserIDs, err := l.svcCtx.AuthRpc.ListUserIds(l.ctx, &authservice.ListUserIdsRequest{})
	if err != nil {
		return nil, fmt.Errorf("list user ids: %w", err)
	}
	if len(allUserIDs.UserIds) == 0 {
		resp.BroadcastID = uuid.NewString()
		resp.AudienceSize = 0
		resp.Chunks = 0
		return resp, nil
	}

	// Filter by segment using billing subscription statuses.
	targetIDs, err := l.resolveSegment(allUserIDs.UserIds, segment)
	if err != nil {
		return nil, err
	}

	broadcastID := uuid.NewString()
	if len(targetIDs) == 0 {
		resp.BroadcastID = broadcastID
		resp.AudienceSize = 0
		resp.Chunks = 0
		return resp, nil
	}

	// Chunk and publish.
	chunks := chunkIDs(targetIDs, broadcastChunkSize)
	chunkTotal := len(chunks)
	for i, chunk := range chunks {
		payload := events.BroadcastNotificationRequested{
			BroadcastID: broadcastID,
			ChunkIndex:  i + 1,
			ChunkTotal:  chunkTotal,
			Title:       req.Title,
			Message:     req.Message,
			Type:        req.ItemType,
			UserIDs:     chunk,
		}
		env, envErr := events.NewEnvelope(events.TypeBroadcastNotificationRequested, payload)
		if envErr != nil {
			return nil, fmt.Errorf("build envelope: %w", envErr)
		}
		if pubErr := l.svcCtx.EventsPub.Publish(l.ctx, env); pubErr != nil {
			return nil, fmt.Errorf("publish chunk %d/%d: %w", i+1, chunkTotal, pubErr)
		}
	}

	l.Infof("broadcast %s: queued %d users in %d chunks (segment=%s)", broadcastID, len(targetIDs), chunkTotal, segment)

	resp.BroadcastID = broadcastID
	resp.AudienceSize = len(targetIDs)
	resp.Chunks = chunkTotal
	return resp, nil
}

// resolveSegment returns the subset of allUserIDs matching the requested
// segment. "premium" = active/trialing subscription with a non-free plan code;
// "free" = everyone else; "all" = unchanged.
func (l *AdminBroadcastNotificationLogic) resolveSegment(allUserIDs []string, segment string) ([]string, error) {
	if segment == "all" {
		return allUserIDs, nil
	}

	statuses, err := l.svcCtx.BillingRpc.ListSubscriptionStatuses(l.ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list subscription statuses: %w", err)
	}

	premium := make(map[string]bool, len(statuses.Statuses))
	for _, s := range statuses.Statuses {
		if s == nil {
			continue
		}
		if (s.Status == "active" || s.Status == "trialing") && s.PlanCode != "free" && s.PlanCode != "" {
			premium[s.UserId] = true
		}
	}

	out := make([]string, 0, len(allUserIDs))
	for _, id := range allUserIDs {
		isPremium := premium[id]
		if segment == "premium" && isPremium {
			out = append(out, id)
		} else if segment == "free" && !isPremium {
			out = append(out, id)
		}
	}
	return out, nil
}

func chunkIDs(ids []string, size int) [][]string {
	if size <= 0 || len(ids) == 0 {
		return nil
	}
	if len(ids) <= size {
		return [][]string{ids}
	}
	var chunks [][]string
	for i := 0; i < len(ids); i += size {
		end := i + size
		if end > len(ids) {
			end = len(ids)
		}
		chunks = append(chunks, ids[i:end])
	}
	return chunks
}

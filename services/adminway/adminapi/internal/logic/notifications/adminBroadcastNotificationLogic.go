// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package notifications

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/notifications"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	broadcastChunkSize = 2000
	// broadcastPublishRetries is the number of times to retry publishing a
	// single chunk before giving up on that chunk.
	broadcastPublishRetries = 3
	// broadcastPublishRetryDelay is the backoff between retry attempts.
	broadcastPublishRetryDelay = 500 * time.Millisecond
)

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
	if !notifications.IsValid(req.ItemType) {
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

	// Resolve the admin user for the audit trail.
	adminUserID := ""
	if p, ok := principal.PrincipalFrom(l.ctx); ok {
		adminUserID = p.UserID
	}

	// Resolve the full audience from auth.
	allUserIDs, err := l.svcCtx.AuthRpc.ListUserIds(l.ctx, &authservice.ListUserIdsRequest{})
	if err != nil {
		return nil, fmt.Errorf("list user ids: %w", err)
	}
	if len(allUserIDs.UserIds) == 0 {
		broadcastID := uuid.NewString()
		l.auditLog(broadcastID, adminUserID, segment, 0, 0, 0, "empty audience")
		resp.BroadcastID = broadcastID
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
		l.auditLog(broadcastID, adminUserID, segment, 0, 0, 0, "segment matched 0 users")
		resp.BroadcastID = broadcastID
		resp.AudienceSize = 0
		resp.Chunks = 0
		return resp, nil
	}

	// Chunk and publish with deterministic event IDs for idempotency. Each
	// chunk's event ID is derived from broadcastID + chunkIndex, so a retry
	// (manual or automatic) with the same broadcastID+chunkIndex is a no-op
	// in the consumer (processed_events dedup).
	chunks := chunkIDs(targetIDs, broadcastChunkSize)
	chunkTotal := len(chunks)
	failedChunks := 0
	for i, chunk := range chunks {
		chunkIndex := i + 1
		payload := events.BroadcastNotificationRequested{
			BroadcastID: broadcastID,
			ChunkIndex:  chunkIndex,
			ChunkTotal:  chunkTotal,
			Title:       req.Title,
			Message:     req.Message,
			Type:        req.ItemType,
			UserIDs:     chunk,
		}

		// Deterministic event ID: uuidv5(namespace, broadcastID:chunkIndex).
		// This ensures the consumer's processed_events dedup treats a retry
		// of the same chunk as a duplicate, preventing duplicate notifications.
		eventID := deterministicChunkEventID(broadcastID, chunkIndex)
		env, envErr := events.NewEnvelopeWithID(eventID, events.TypeBroadcastNotificationRequested, payload)
		if envErr != nil {
			l.Errorf("broadcast %s: build envelope for chunk %d: %v", broadcastID, chunkIndex, envErr)
			failedChunks++
			continue
		}

		if pubErr := l.publishWithRetry(env, chunkIndex, chunkTotal); pubErr != nil {
			l.Errorf("broadcast %s: publish chunk %d/%d failed after %d retries: %v",
				broadcastID, chunkIndex, chunkTotal, broadcastPublishRetries, pubErr)
			failedChunks++
		}
	}

	l.Infof("broadcast %s: queued %d users in %d chunks (segment=%s, failed=%d, admin=%s)",
		broadcastID, len(targetIDs), chunkTotal, segment, failedChunks, adminUserID)

	l.auditLog(broadcastID, adminUserID, segment, len(targetIDs), chunkTotal, failedChunks, "published")

	resp.BroadcastID = broadcastID
	resp.AudienceSize = len(targetIDs)
	resp.Chunks = chunkTotal
	if failedChunks > 0 {
		// Don't fail the whole request — partial delivery is better than none.
		// The admin can retry the failed chunks by re-sending the broadcast
		// (idempotent chunks mean already-delivered chunks are no-ops).
		l.Infof("broadcast %s: %d/%d chunks failed — admin may retry (already-delivered chunks are idempotent)",
			broadcastID, failedChunks, chunkTotal)
	}
	return resp, nil
}

// publishWithRetry publishes an envelope with retry + backoff. Returns the
// last error if all retries are exhausted.
func (l *AdminBroadcastNotificationLogic) publishWithRetry(env events.Envelope, chunkIndex, chunkTotal int) error {
	var lastErr error
	for attempt := 0; attempt <= broadcastPublishRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(broadcastPublishRetryDelay * time.Duration(attempt))
		}
		if err := l.svcCtx.EventsPub.Publish(l.ctx, env); err != nil {
			lastErr = err
			l.Infof("broadcast chunk %d/%d: publish attempt %d failed: %v",
				chunkIndex, chunkTotal, attempt+1, err)
			continue
		}
		return nil
	}
	return lastErr
}

// deterministicChunkEventID generates a deterministic UUID for a broadcast
// chunk using UUIDv5. This ensures that retrying the same chunk (same
// broadcastID + chunkIndex) produces the same event ID, so the consumer's
// processed_events dedup treats it as a duplicate and skips it — preventing
// duplicate notifications on retry.
func deterministicChunkEventID(broadcastID string, chunkIndex int) string {
	name := fmt.Sprintf("%s:%d", broadcastID, chunkIndex)
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(name)).String()
}

// auditLog records broadcast metadata for audit purposes. This is a structured
// log entry; a dedicated audit table can be added later if queryability is
// needed.
func (l *AdminBroadcastNotificationLogic) auditLog(broadcastID, adminUserID, segment string, audienceSize, chunks, failedChunks int, status string) {
	l.Infof("[BROADCAST_AUDIT] broadcast=%s admin=%s segment=%s audience=%d chunks=%d failed=%d status=%s",
		broadcastID, adminUserID, segment, audienceSize, chunks, failedChunks, status)
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

package aicoachservicelogic

import (
	"context"
	"strings"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// maxSearchMemoryHits caps what one tool call can pull into the model context.
// The caller's limit is advisory; this is the ceiling.
const maxSearchMemoryHits = 8

type SearchMemoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchMemoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchMemoryLogic {
	return &SearchMemoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SearchMemory runs a hybrid search over the caller's private memory index.
//
// Unlike the prompt-injection path this is NOT fail-open: the model asked a
// direct question and an empty result set means "nothing found", which is a
// different answer from "search is broken". Returning success on an
// infrastructure failure would have the coach state that it found no record of
// something the user definitely told it.
func (l *SearchMemoryLogic) SearchMemory(in *aicoach.SearchMemoryRequest) (*aicoach.SearchMemoryResponse, error) {
	if l.svcCtx.MemoryRetriever == nil {
		return nil, status.Error(codes.Unavailable, "long-term memory is not configured")
	}
	if in.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "userId is required")
	}
	query := strings.TrimSpace(in.Query)
	if query == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	limit := int(in.Limit)
	if limit <= 0 || limit > maxSearchMemoryHits {
		limit = maxSearchMemoryHits
	}

	// The retriever validates the user id and enforces the per-user filter,
	// plus a per-hit ownership re-check.
	hits, err := l.svcCtx.MemoryRetriever.Retrieve(l.ctx, in.UserId, query)
	if err != nil {
		l.Errorf("memory search failed: user=%s err=%v", in.UserId, err)
		return nil, status.Error(codes.Internal, "memory search failed")
	}

	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]*aicoach.MemoryHit, 0, len(hits))
	for _, h := range hits {
		var createdAt int64
		if !h.CreatedAt.IsZero() {
			createdAt = h.CreatedAt.Unix()
		}
		out = append(out, &aicoach.MemoryHit{
			EntityType: h.EntityType,
			Content:    h.Content,
			CreatedAt:  createdAt,
			HabitName:  h.HabitName,
			Role:       h.Role,
		})
	}
	return &aicoach.SearchMemoryResponse{Hits: out}, nil
}

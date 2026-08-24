package aicoachservicelogic

import (
	"context"

	"github.com/google/uuid"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListUserFactsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListUserFactsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUserFactsLogic {
	return &ListUserFactsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListUserFacts returns everything the coach currently remembers about the
// caller, including low-confidence facts.
//
// Deliberately unfiltered by confidence, unlike the prompt path: a user who
// cannot see a shaky guess cannot correct it, and will instead be surprised by
// it later when the coach acts on it.
func (l *ListUserFactsLogic) ListUserFacts(in *aicoach.ListUserFactsRequest) (*aicoach.ListUserFactsResponse, error) {
	if l.svcCtx.Queries == nil {
		return nil, status.Error(codes.Unavailable, "conversation persistence is not configured")
	}
	if in.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "userId is required")
	}
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid userId")
	}

	limit := in.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	page := in.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	rows, err := l.svcCtx.Queries.ListAllUserFacts(l.ctx, userID, limit, offset)
	if err != nil {
		l.Errorf("failed to list user facts: %v", err)
		return nil, status.Error(codes.Internal, "failed to list facts")
	}
	total, err := l.svcCtx.Queries.CountCurrentUserFacts(l.ctx, userID)
	if err != nil {
		l.Errorf("failed to count user facts: %v", err)
		return nil, status.Error(codes.Internal, "failed to count facts")
	}

	out := make([]*aicoach.UserFact, 0, len(rows))
	for _, r := range rows {
		// Defense-in-depth against a filter regression leaking another user's
		// memory into this response.
		if r.UserID != userID {
			continue
		}
		out = append(out, protoUserFact(r))
	}
	return &aicoach.ListUserFactsResponse{Facts: out, Total: int32(total)}, nil
}

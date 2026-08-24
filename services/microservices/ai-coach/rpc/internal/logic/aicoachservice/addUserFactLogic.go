package aicoachservicelogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/facts"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AddUserFactLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddUserFactLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddUserFactLogic {
	return &AddUserFactLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AddUserFact stores a fact the user wrote themselves, optionally replacing one
// the coach had wrong.
//
// User-authored facts carry confidence 1.0 and bypass the extraction confidence
// floor: on the subject of themselves, the user is authoritative and the model
// is not.
func (l *AddUserFactLogic) AddUserFact(in *aicoach.AddUserFactRequest) (*aicoach.AddUserFactResponse, error) {
	if l.svcCtx.FactStore == nil {
		return nil, status.Error(codes.Unavailable, "long-term memory is not configured")
	}
	if in.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "userId is required")
	}
	if !facts.ValidCategories[in.Category] {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported category %q", in.Category)
	}

	created, ok, err := l.svcCtx.FactStore.Record(l.ctx, in.UserId, facts.Candidate{
		Fact:         in.Fact,
		Category:     in.Category,
		Confidence:   1,
		UserAuthored: true,
		SupersedesID: in.SupersedesId,
	})
	if err != nil {
		l.Errorf("failed to add user fact: %v", err)
		return nil, status.Error(codes.InvalidArgument, "could not save that fact")
	}
	if !ok {
		// Already remembered verbatim. Idempotent from the user's point of
		// view: the thing they asked to be remembered is remembered.
		return &aicoach.AddUserFactResponse{}, nil
	}
	return &aicoach.AddUserFactResponse{Fact: protoUserFact(created)}, nil
}

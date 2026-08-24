package aicoachservicelogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ForgetUserFactLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewForgetUserFactLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ForgetUserFactLogic {
	return &ForgetUserFactLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ForgetUserFact deletes one remembered fact. A hard delete, not a
// supersession: retaining something a user asked to be forgotten defeats the
// request.
func (l *ForgetUserFactLogic) ForgetUserFact(in *aicoach.ForgetUserFactRequest) (*aicoach.ForgetUserFactResponse, error) {
	if l.svcCtx.FactStore == nil {
		return nil, status.Error(codes.Unavailable, "long-term memory is not configured")
	}
	if in.UserId == "" || in.FactId == "" {
		return nil, status.Error(codes.InvalidArgument, "userId and factId are required")
	}
	if err := l.svcCtx.FactStore.Forget(l.ctx, in.UserId, in.FactId); err != nil {
		l.Errorf("failed to forget user fact: %v", err)
		return nil, status.Error(codes.Internal, "could not forget that fact")
	}
	return &aicoach.ForgetUserFactResponse{}, nil
}

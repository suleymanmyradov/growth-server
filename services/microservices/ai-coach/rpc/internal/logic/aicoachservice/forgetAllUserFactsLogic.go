package aicoachservicelogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ForgetAllUserFactsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewForgetAllUserFactsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ForgetAllUserFactsLogic {
	return &ForgetAllUserFactsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ForgetAllUserFacts wipes the curated tier for one user. Backs both "disable
// long-term memory" and account deletion.
func (l *ForgetAllUserFactsLogic) ForgetAllUserFacts(in *aicoach.ForgetAllUserFactsRequest) (*aicoach.ForgetAllUserFactsResponse, error) {
	if l.svcCtx.FactStore == nil {
		return nil, status.Error(codes.Unavailable, "long-term memory is not configured")
	}
	if in.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "userId is required")
	}
	if err := l.svcCtx.FactStore.ForgetAll(l.ctx, in.UserId); err != nil {
		l.Errorf("failed to forget all user facts: %v", err)
		return nil, status.Error(codes.Internal, "could not clear your memory")
	}
	return &aicoach.ForgetAllUserFactsResponse{}, nil
}

// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package personalization

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type StreamPersonalizedCoachingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewStreamPersonalizedCoachingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StreamPersonalizedCoachingLogic {
	return &StreamPersonalizedCoachingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *StreamPersonalizedCoachingLogic) StreamPersonalizedCoaching(req *types.GeneratePersonalizedCoachingRequest, client chan<- *types.GeneratePersonalizedCoachingResponse) error {
	// todo: add your logic here and delete this line

	return nil
}

package billing

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientbilling "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/billingservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type TrackUpgradeEventLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTrackUpgradeEventLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TrackUpgradeEventLogic {
	return &TrackUpgradeEventLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TrackUpgradeEventLogic) TrackUpgradeEvent(req *types.TrackUpgradeEventRequest) (resp *types.TrackUpgradeEventResponse, err error) {
	rpcResp, err := l.svcCtx.BillingRpc.TrackUpgradeEvent(l.ctx, &clientbilling.TrackUpgradeEventRequest{
		EventType:       req.EventType,
		Surface:         req.Surface,
		Trigger:         req.Trigger,
		PlanCode:        req.PlanCode,
		BillingInterval: req.BillingInterval,
		FeedbackReason:  req.FeedbackReason,
		FeedbackNote:    req.FeedbackNote,
		MetadataJson:    req.MetadataJson,
	})
	if err != nil {
		return nil, err
	}

	return &types.TrackUpgradeEventResponse{
		EventId: rpcResp.EventId,
	}, nil
}

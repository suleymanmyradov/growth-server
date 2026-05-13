// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package saved

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientsaved "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/saved"

	"github.com/zeromicro/go-zero/core/logx"
)

type SaveItemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSaveItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveItemLogic {
	return &SaveItemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SaveItemLogic) SaveItem(req *types.SaveItemRequest) (resp *types.SavedItemResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, nil
	}

	rpcResp, err := l.svcCtx.SavedRpc.SaveItem(l.ctx, &clientsaved.SaveItemRequest{
		UserId:   p.UserID,
		ItemType: req.ItemType,
		ItemId:   req.ItemId,
	})
	if err != nil {
		return nil, err
	}

	return &types.SavedItemResponse{
		Data: types.SavedItem{
			Id:       rpcResp.SavedId,
			ItemType: req.ItemType,
			ItemId:   req.ItemId,
			UserId:   p.UserID,
		},
	}, nil
}

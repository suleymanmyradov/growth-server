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

type ListSavedLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListSavedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSavedLogic {
	return &ListSavedLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListSavedLogic) ListSaved(req *types.PageRequest) (resp *types.SavedItemsResponse, err error) {
	_, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return &types.SavedItemsResponse{Data: []types.SavedItem{}}, nil
	}

	rpcResp, err := l.svcCtx.SavedRpc.ListSaved(l.ctx, &clientsaved.ListSavedRequest{
		Limit:  int32(req.Limit),
		Offset: int32((req.Page - 1) * req.Limit),
	})
	if err != nil {
		return nil, err
	}

	items := make([]types.SavedItem, 0, len(rpcResp.Items))
	for _, item := range rpcResp.Items {
		items = append(items, types.SavedItem{
			Id:       item.Id,
			ItemType: item.ItemType,
			ItemId:   item.ItemId,
			UserId:   item.UserId,
		})
	}

	totalPages := int(rpcResp.TotalCount) / req.Limit
	if int(rpcResp.TotalCount)%req.Limit > 0 {
		totalPages++
	}

	return &types.SavedItemsResponse{
		Data: items,
		Page: types.PageResponse{
			Total:      int64(rpcResp.TotalCount),
			Page:       req.Page,
			Limit:      req.Limit,
			TotalPages: totalPages,
		},
	}, nil
}

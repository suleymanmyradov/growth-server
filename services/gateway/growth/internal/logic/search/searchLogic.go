// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package search

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	searchpb "github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/pb/search"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchLogic {
	return &SearchLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchLogic) Search(req *types.SearchRequest) (resp *types.SearchResponse, err error) {
	userID := ""
	if p, ok := principal.PrincipalFrom(l.ctx); ok {
		userID = p.UserID
	}

	var rpcTypes []string
	if req.ItemType != "" {
		rpcTypes = []string{req.ItemType}
	}

	offset := (req.Page - 1) * req.Limit
	if offset < 0 {
		offset = 0
	}

	rpcResp, err := l.svcCtx.SearchRpc.Search(l.ctx, &searchpb.SearchRequest{
		Query:  req.Q,
		UserId: userID,
		Types:  rpcTypes,
		Limit:  int32(req.Limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}

	var results []types.SearchResult
	for _, r := range rpcResp.Results {
		results = append(results, types.SearchResult{
			Id:          r.Id,
			ItemType:    r.Type,
			Title:       r.Title,
			Description: r.Description,
			Score:       float64(r.Score),
			Highlight:   r.Highlighted,
		})
	}

	totalPages := int(rpcResp.Total) / req.Limit
	if int(rpcResp.Total)%req.Limit > 0 {
		totalPages++
	}

	return &types.SearchResponse{
		Data: results,
		Page: types.PageResponse{
			Total:      int64(rpcResp.Total),
			Page:       req.Page,
			Limit:      req.Limit,
			TotalPages: totalPages,
		},
	}, nil
}

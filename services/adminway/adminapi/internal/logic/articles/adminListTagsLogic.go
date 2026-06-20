package articles

import (
	"context"
	"fmt"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clientarticles "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/articles"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminListTagsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminListTagsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminListTagsLogic {
	return &AdminListTagsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminListTagsLogic) AdminListTags() (resp *types.TagsResponse, err error) {
	rpcResp, err := l.svcCtx.ArticlesRpc.ListTags(l.ctx, &clientarticles.ListTagsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tags via rpc: %w", err)
	}

	tags := make([]types.Tag, 0, len(rpcResp.Tags))
	for _, t := range rpcResp.Tags {
		tags = append(tags, types.Tag{
			Id:   t.Id,
			Name: t.Name,
			Slug: t.Slug,
		})
	}

	return &types.TagsResponse{
		Data: tags,
	}, nil
}

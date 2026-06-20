package tags

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clienttags "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/tags"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminDeleteTagLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminDeleteTagLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDeleteTagLogic {
	return &AdminDeleteTagLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminDeleteTagLogic) AdminDeleteTag(req *types.ArticleRequest) (resp *types.EmptyResponse, err error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "tag id is required")
	}

	if _, err := l.svcCtx.TagsRpc.DeleteTag(l.ctx, &clienttags.DeleteTagRequest{
		TagId: req.Id,
	}); err != nil {
		return nil, fmt.Errorf("failed to delete tag via rpc: %w", err)
	}

	return &types.EmptyResponse{}, nil
}

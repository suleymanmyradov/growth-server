package tags

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clienttags "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/tags"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminCreateTagLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminCreateTagLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCreateTagLogic {
	return &AdminCreateTagLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminCreateTagLogic) AdminCreateTag(req *types.CreateTagRequest) (resp *types.TagResponse, err error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	rpcResp, err := l.svcCtx.TagsRpc.CreateTag(l.ctx, &clienttags.CreateTagRequest{
		Name: req.Name,
		Slug: req.Slug,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create tag via rpc: %w", err)
	}

	if rpcResp.Tag == nil {
		return nil, status.Error(codes.Internal, "tag creation returned nil")
	}

	return &types.TagResponse{
		Data: mapTag(rpcResp.Tag),
	}, nil
}

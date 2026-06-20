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

type AdminUpdateTagLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminUpdateTagLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUpdateTagLogic {
	return &AdminUpdateTagLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminUpdateTagLogic) AdminUpdateTag(req *types.UpdateTagRequest) (resp *types.TagResponse, err error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "tag id is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	rpcResp, err := l.svcCtx.TagsRpc.UpdateTag(l.ctx, &clienttags.UpdateTagRequest{
		TagId: req.Id,
		Name:  req.Name,
		Slug:  req.Slug,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update tag via rpc: %w", err)
	}

	if rpcResp.Tag == nil {
		return nil, status.Error(codes.Internal, "tag update returned nil")
	}

	return &types.TagResponse{
		Data: mapTag(rpcResp.Tag),
	}, nil
}

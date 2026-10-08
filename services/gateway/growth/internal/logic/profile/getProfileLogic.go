// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package profile

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/logic/files"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProfileLogic {
	return &GetProfileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProfileLogic) GetProfile() (resp *types.ProfileResponse, err error) {
	_, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "login required to view profile")
	}

	rpcResp, err := l.svcCtx.AuthRpc.GetProfile(l.ctx, &authservice.GetProfileRequest{})
	if err != nil {
		return nil, err
	}

	if rpcResp.User == nil {
		return &types.ProfileResponse{Data: types.Profile{}}, nil
	}

	return &types.ProfileResponse{
		Data: types.Profile{
			Id:            rpcResp.User.Id,
			FullName:      rpcResp.User.FullName,
			Username:      rpcResp.User.Username,
			Email:         rpcResp.User.Email,
			Bio:           rpcResp.User.Bio,
			Location:      rpcResp.User.Location,
			Website:       rpcResp.User.Website,
			Interests:     rpcResp.User.Interests,
			AvatarUrl:     files.ResolveAvatarURL(l.ctx, l.svcCtx.FileManagerRpc, rpcResp.User.AvatarUrl),
			CreatedAt:     rpcResp.User.CreatedAt,
			UpdatedAt:     rpcResp.User.UpdatedAt,
			EmailVerified: rpcResp.User.EmailVerified,
		},
	}, nil
}

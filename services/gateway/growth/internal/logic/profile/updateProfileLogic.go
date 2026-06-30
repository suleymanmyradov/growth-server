// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package profile

import (
	"context"
	"fmt"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProfileLogic {
	return &UpdateProfileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateProfileLogic) UpdateProfile(req *types.UpdateProfileRequest) (resp *types.ProfileResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, fmt.Errorf("unauthenticated")
	}

	rpcResp, err := l.svcCtx.AuthRpc.UpdateProfile(l.ctx, &authservice.UpdateProfileRequest{
		UserId:    p.UserID,
		FullName:  req.FullName,
		Bio:       req.Bio,
		Location:  req.Location,
		Website:   req.Website,
		Interests: req.Interests,
		AvatarUrl: req.AvatarUrl,
	})
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
			AvatarUrl:     rpcResp.User.AvatarUrl,
			CreatedAt:     rpcResp.User.CreatedAt,
			UpdatedAt:     rpcResp.User.UpdatedAt,
			EmailVerified: rpcResp.User.EmailVerified,
		},
	}, nil
}

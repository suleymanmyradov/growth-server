package logic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListUserIdsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListUserIdsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUserIdsLogic {
	return &ListUserIdsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

const listUserIdsPageSize = 5000

// Admin / internal: enumerate user ids (used by adminway for broadcasts).
// The request cursor is currently ignored; the RPC pages internally and
// returns the complete id list in one response. This is acceptable for the
// current user-base size; if it grows large, switch to streaming or honor the
// cursor for client-side pagination.
func (l *ListUserIdsLogic) ListUserIds(_ *auth.ListUserIdsRequest) (*auth.ListUserIdsResponse, error) {
	var all []string
	cursor := uuid.Nil
	for {
		ids, err := l.svcCtx.Repo.Users.ListUserIds(l.ctx, cursor, listUserIdsPageSize)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			all = append(all, id.String())
		}
		if len(ids) < listUserIdsPageSize {
			break
		}
		cursor = ids[len(ids)-1]
	}

	return &auth.ListUserIdsResponse{UserIds: all}, nil
}

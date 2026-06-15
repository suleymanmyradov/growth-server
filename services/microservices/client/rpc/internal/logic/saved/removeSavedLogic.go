package savedlogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RemoveSavedLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRemoveSavedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RemoveSavedLogic {
	return &RemoveSavedLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RemoveSavedLogic) RemoveSaved(in *client.RemoveSavedRequest) (*client.RemoveSavedResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "RemoveSavedLogic.RemoveSaved")
	defer span.End()
	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return nil, status.Error(codes.Internal, "invalid user id")
	}

	savedID, err := uuid.Parse(in.SavedId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid saved id")
	}

	pool := l.svcCtx.Pool()

	// The savedId is the row id in one of the three concrete tables.
	// Try deleting from each table until we get a hit.
	var deleted bool
	for _, table := range []string{"saved_articles", "saved_goals", "saved_habits"} {
		tag, err := pool.Exec(ctx,
			"DELETE FROM "+table+" WHERE id = $1 AND user_id = $2",
			savedID, userID,
		)
		if err != nil {
			l.Errorf("Failed to delete from %s: %v", table, err)
			return nil, status.Error(codes.Internal, "failed to remove saved item")
		}
		if tag.RowsAffected() > 0 {
			deleted = true
			break
		}
	}

	if !deleted {
		return nil, status.Error(codes.NotFound, "saved item not found")
	}

	return &client.RemoveSavedResponse{
		Success: true,
	}, nil
}

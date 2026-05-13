package savedlogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetSavedStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSavedStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSavedStatsLogic {
	return &GetSavedStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetSavedStatsLogic) GetSavedStats(in *client.GetSavedStatsRequest) (*client.GetSavedStatsResponse, error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	totalSaved, err := l.svcCtx.Repo.SavedItems.CountSavedItemsByUser(l.ctx, userID)
	if err != nil {
		l.Errorf("Failed to count saved items: %v", err)
		return nil, err
	}

	typeCounts := map[string]int32{}
	for _, itemType := range []string{"article", "goal", "habit"} {
		count, err := l.svcCtx.Repo.SavedItems.CountSavedItemsByUserAndType(l.ctx, userID, itemType)
		if err != nil {
			l.Errorf("Failed to count saved items for type %s: %v", itemType, err)
			return nil, err
		}
		typeCounts[itemType] = int32(count)
	}

	return &client.GetSavedStatsResponse{
		TotalSaved:       int32(totalSaved),
		TotalCollections: 0,
		TypeCounts:       typeCounts,
	}, nil
}

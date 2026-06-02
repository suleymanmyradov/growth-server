package settingslogic

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

type DeleteAccountLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteAccountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteAccountLogic {
	return &DeleteAccountLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteAccountLogic) DeleteAccount(in *client.DeleteAccountRequest) (*client.DeleteAccountResponse, error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
return nil, status.Error(codes.Internal, "invalid user id")
	}

	if l.svcCtx.Authz != nil {
		if err := l.svcCtx.Authz.CheckPrincipal(l.ctx); err != nil {
			return nil, err
		}
	}

	l.Infof("Deleting account for user %s", userID)

	return &client.DeleteAccountResponse{
		Success: true,
	}, nil
}

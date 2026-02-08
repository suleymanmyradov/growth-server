package conversationsservicelogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/conversations/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/conversations/rpc/pb/conversations"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetConversationSuggestionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetConversationSuggestionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConversationSuggestionsLogic {
	return &GetConversationSuggestionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AI features
func (l *GetConversationSuggestionsLogic) GetConversationSuggestions(in *conversations.GetConversationSuggestionsRequest) (*conversations.GetConversationSuggestionsResponse, error) {
	// todo: add your logic here and delete this line

	return &conversations.GetConversationSuggestionsResponse{}, nil
}

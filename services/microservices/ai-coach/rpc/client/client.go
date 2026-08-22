package client

import (
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/conversationservice"
	"github.com/zeromicro/go-zero/zrpc"
)

// Service aggregates every sub-service client exposed by the ai-coach RPC into
// a single struct. Consumers (e.g. the gateway) only need to import this one
// package instead of each individual sub-service package.
//
// AICoachService and ConversationService are constructed from separate
// zrpc.Client instances so they can have independent timeouts: AI generation
// calls are long-running (90s) while conversation calls are fast DB CRUD (3s).
type Service struct {
	AICoachService      aicoachservice.AICoachService
	ConversationService conversationservice.ConversationService
}

// NewAICoachClientService constructs the aggregate ai-coach client from two
// zrpc.Client instances: aiCoachCli should be configured with a long timeout
// (AI generation can take 90s+); conversationCli should use the default short
// timeout (conversation methods are fast DB operations).
func NewAICoachClientService(aiCoachCli, conversationCli zrpc.Client) *Service {
	return &Service{
		AICoachService:      aicoachservice.NewAICoachService(aiCoachCli),
		ConversationService: conversationservice.NewConversationService(conversationCli),
	}
}

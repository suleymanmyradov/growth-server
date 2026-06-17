package articles

import (
	"time"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

func formatTime(unix int64) string {
	if unix <= 0 {
		return ""
	}
	return time.Unix(unix, 0).Format(time.RFC3339)
}

func mapCategory(rpcCategory *client.ArticleCategory) *types.ArticleCategory {
	if rpcCategory == nil {
		return nil
	}
	return &types.ArticleCategory{
		Id:   rpcCategory.Id,
		Name: rpcCategory.Name,
		Slug: rpcCategory.Slug,
	}
}

package categories

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

func mapCategory(c *client.Category) types.Category {
	return types.Category{
		Id:        c.Id,
		Name:      c.Name,
		Slug:      c.Slug,
		SortOrder: int(c.SortOrder),
		CreatedAt: formatTime(c.CreatedAt),
		UpdatedAt: formatTime(c.UpdatedAt),
	}
}

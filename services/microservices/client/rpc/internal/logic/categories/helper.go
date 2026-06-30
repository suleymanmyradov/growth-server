package categorieslogic

import (
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

func convertCategory(c db.Category) *client.Category {
	return &client.Category{
		Id:        c.ID.String(),
		Name:      c.Name,
		Slug:      c.Slug,
		SortOrder: c.SortOrder,
		CreatedAt: c.CreatedAt.Time.Unix(),
		UpdatedAt: c.UpdatedAt.Time.Unix(),
	}
}

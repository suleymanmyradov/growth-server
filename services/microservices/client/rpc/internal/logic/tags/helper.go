package tagslogic

import (
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

func convertTag(t db.Tag) *client.Tag {
	return &client.Tag{
		Id:   t.ID.String(),
		Name: t.Name,
		Slug: t.Slug,
	}
}

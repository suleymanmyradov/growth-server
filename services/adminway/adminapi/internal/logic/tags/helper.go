package tags

import (
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

func mapTag(t *client.Tag) types.Tag {
	return types.Tag{
		Id:   t.Id,
		Name: t.Name,
		Slug: t.Slug,
	}
}

package sitesettingslogic

import (
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

func convertSiteSetting(s db.SiteSetting) *client.SiteSetting {
	return &client.SiteSetting{
		Key:       s.Key,
		Value:     s.Value,
		CreatedAt: s.CreatedAt.Time.Unix(),
		UpdatedAt: s.UpdatedAt.Time.Unix(),
	}
}

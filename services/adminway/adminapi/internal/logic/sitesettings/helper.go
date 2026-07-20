package sitesettings

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

func mapSiteSetting(s *client.SiteSetting) types.SiteSettingItem {
	return types.SiteSettingItem{
		Key:       s.Key,
		Value:     string(s.Value),
		UpdatedAt: formatTime(s.UpdatedAt),
	}
}

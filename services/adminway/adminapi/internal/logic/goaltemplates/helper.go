package goaltemplates

import (
	"time"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clientgoaltemplates "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goaltemplates"
)

func formatTime(unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).UTC().Format(time.RFC3339)
}

func goalTemplateProtoToItem(m *clientgoaltemplates.GoalTemplate) types.GoalTemplateItem {
	item := types.GoalTemplateItem{
		Id:          m.Id,
		Title:       m.Title,
		Description: m.Description,
		SortOrder:   m.SortOrder,
		IsActive:    m.IsActive,
		CreatedAt:   formatTime(m.CreatedAt),
		UpdatedAt:   formatTime(m.UpdatedAt),
	}
	if m.Category != nil {
		item.Category = &types.TemplateCategory{
			Id:   m.Category.Id,
			Name: m.Category.Name,
			Slug: m.Category.Slug,
		}
	}
	return item
}

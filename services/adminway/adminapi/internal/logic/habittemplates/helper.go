package habittemplates

import (
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
)

func formatTime(unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).UTC().Format(time.RFC3339)
}

func parseOptionalUUID(s *string) (uuid.NullUUID, error) {
	if s == nil || *s == "" {
		return uuid.NullUUID{}, nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return uuid.NullUUID{}, err
	}
	return uuid.NullUUID{UUID: id, Valid: true}, nil
}

func habitTemplateRowToItem(row db.AdminListHabitTemplatesRow) types.HabitTemplateItem {
	item := types.HabitTemplateItem{
		Id:        row.ID.String(),
		Name:      row.Name,
		SortOrder: row.SortOrder,
		IsActive:  row.IsActive,
		CreatedAt: formatTime(row.CreatedAt.Time.Unix()),
		UpdatedAt: formatTime(row.UpdatedAt.Time.Unix()),
	}
	if row.Description != nil {
		item.Description = *row.Description
	}
	if row.CategoryIDJoined.Valid && row.CategoryIDJoined.UUID != uuid.Nil {
		name := ""
		if row.CategoryName != nil {
			name = *row.CategoryName
		}
		slug := ""
		if row.CategorySlug != nil {
			slug = *row.CategorySlug
		}
		item.Category = &types.TemplateCategory{
			Id:   row.CategoryIDJoined.UUID.String(),
			Name: name,
			Slug: slug,
		}
	}
	return item
}

func habitTemplateGetRowToItem(row db.AdminGetHabitTemplateRow) types.HabitTemplateItem {
	item := types.HabitTemplateItem{
		Id:        row.ID.String(),
		Name:      row.Name,
		SortOrder: row.SortOrder,
		IsActive:  row.IsActive,
		CreatedAt: formatTime(row.CreatedAt.Time.Unix()),
		UpdatedAt: formatTime(row.UpdatedAt.Time.Unix()),
	}
	if row.Description != nil {
		item.Description = *row.Description
	}
	if row.CategoryIDJoined.Valid && row.CategoryIDJoined.UUID != uuid.Nil {
		name := ""
		if row.CategoryName != nil {
			name = *row.CategoryName
		}
		slug := ""
		if row.CategorySlug != nil {
			slug = *row.CategorySlug
		}
		item.Category = &types.TemplateCategory{
			Id:   row.CategoryIDJoined.UUID.String(),
			Name: name,
			Slug: slug,
		}
	}
	return item
}

func habitTemplateModelToItem(m db.HabitTemplate) types.HabitTemplateItem {
	item := types.HabitTemplateItem{
		Id:        m.ID.String(),
		Name:      m.Name,
		SortOrder: m.SortOrder,
		IsActive:  m.IsActive,
		CreatedAt: formatTime(m.CreatedAt.Time.Unix()),
		UpdatedAt: formatTime(m.UpdatedAt.Time.Unix()),
	}
	if m.Description != nil {
		item.Description = *m.Description
	}
	return item
}

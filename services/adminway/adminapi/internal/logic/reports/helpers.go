package reports

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

func reportToType(r *client.ReportItem) types.ReportItem {
	if r == nil {
		return types.ReportItem{}
	}
	attachments := r.Attachments
	if attachments == nil {
		attachments = []string{}
	}
	return types.ReportItem{
		Id:          r.Id,
		ReporterId:  r.ReporterId,
		TargetId:    r.TargetId,
		TargetType:  r.TargetType,
		Category:    r.Category,
		Title:       r.Title,
		Description: r.Description,
		Email:       r.Email,
		Status:      r.Status,
		AdminNotes:  r.AdminNotes,
		CloseReason: r.CloseReason,
		Attachments: attachments,
		CreatedAt:   formatTime(r.CreatedAt),
		UpdatedAt:   formatTime(r.UpdatedAt),
	}
}

func commentToType(c *client.ReportComment) types.ReportCommentItem {
	if c == nil {
		return types.ReportCommentItem{}
	}
	return types.ReportCommentItem{
		Id:        c.Id,
		ReportId:  c.ReportId,
		UserId:    c.UserId,
		Comment:   c.Comment,
		IsAdmin:   c.IsAdmin,
		CreatedAt: formatTime(c.CreatedAt),
	}
}

func totalPages(total, limit int) int {
	if limit <= 0 {
		return 1
	}
	p := total / limit
	if total%limit > 0 {
		p++
	}
	if p == 0 {
		p = 1
	}
	return p
}

package reportlogic

import (
	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

// reportToPb converts a db.Report row to the protobuf ReportItem.
func reportToPb(r db.Report) *client.ReportItem {
	var targetID string
	if r.TargetID != nil {
		targetID = *r.TargetID
	}
	var email string
	if r.Email != nil {
		email = *r.Email
	}
	var adminNotes string
	if r.AdminNotes != nil {
		adminNotes = *r.AdminNotes
	}
	var closeReason string
	if r.CloseReason != nil {
		closeReason = *r.CloseReason
	}
	attachments := r.Attachments
	if attachments == nil {
		attachments = []string{}
	}
	return &client.ReportItem{
		Id:          r.ID.String(),
		ReporterId:  r.ReporterID.String(),
		TargetId:    targetID,
		TargetType:  r.TargetType,
		Category:    r.Category,
		Title:       r.Title,
		Description: r.Description,
		Email:       email,
		Status:      r.Status,
		AdminNotes:  adminNotes,
		CloseReason: closeReason,
		Attachments: attachments,
		CreatedAt:   r.CreatedAt.Time.Unix(),
		UpdatedAt:   r.UpdatedAt.Time.Unix(),
	}
}

// commentToPb converts a db.ReportComment row to the protobuf ReportComment.
func commentToPb(c db.ReportComment) *client.ReportComment {
	return &client.ReportComment{
		Id:        c.ID.String(),
		ReportId:  c.ReportID.String(),
		UserId:    c.UserID.String(),
		Comment:   c.Comment,
		IsAdmin:   c.IsAdmin,
		CreatedAt: c.CreatedAt.Time.Unix(),
	}
}

// parseReporterID parses a reporter ID string, returning uuid.Nil on empty/invalid.
func parseReporterID(s string) uuid.UUID {
	if s == "" {
		return uuid.Nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}

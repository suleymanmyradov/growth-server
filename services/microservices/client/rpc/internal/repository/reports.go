package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// ReportsRepo implements IReports interface
type ReportsRepo struct {
	db *db.Queries
}

// NewReportsRepo creates a new ReportsRepo instance
func NewReportsRepo(db *db.Queries) *ReportsRepo {
	return &ReportsRepo{db: db}
}

// WithTx returns a new ReportsRepo backed by the given transaction.
func (r *ReportsRepo) WithTx(tx pgx.Tx) *ReportsRepo {
	return &ReportsRepo{db: r.db.WithTx(tx)}
}

func (r *ReportsRepo) CreateReport(ctx context.Context, params db.CreateReportParams) (db.Report, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ReportsRepo.CreateReport")
	defer span.End()
	return r.db.CreateReport(ctx, params)
}

func (r *ReportsRepo) GetReportByID(ctx context.Context, id uuid.UUID) (db.Report, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ReportsRepo.GetReportByID")
	defer span.End()
	return r.db.GetReportByID(ctx, id)
}

func (r *ReportsRepo) GetReportStatus(ctx context.Context, id uuid.UUID) (db.GetReportStatusRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ReportsRepo.GetReportStatus")
	defer span.End()
	return r.db.GetReportStatus(ctx, id)
}

func (r *ReportsRepo) ListReports(ctx context.Context, params db.ListReportsParams) ([]db.Report, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ReportsRepo.ListReports")
	defer span.End()
	return r.db.ListReports(ctx, params)
}

func (r *ReportsRepo) CountReports(ctx context.Context, status, category string, reporterID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ReportsRepo.CountReports")
	defer span.End()
	return r.db.CountReports(ctx, status, category, reporterID)
}

func (r *ReportsRepo) UpdateReportStatus(ctx context.Context, id uuid.UUID, status string, adminNotes *string) (db.Report, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ReportsRepo.UpdateReportStatus")
	defer span.End()
	return r.db.UpdateReportStatus(ctx, id, status, adminNotes)
}

func (r *ReportsRepo) CloseReport(ctx context.Context, id uuid.UUID, closeReason *string, adminNotes *string) (db.Report, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ReportsRepo.CloseReport")
	defer span.End()
	return r.db.CloseReport(ctx, id, closeReason, adminNotes)
}

func (r *ReportsRepo) CreateReportComment(ctx context.Context, reportID, userID uuid.UUID, comment string, isAdmin bool) (db.ReportComment, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ReportsRepo.CreateReportComment")
	defer span.End()
	return r.db.CreateReportComment(ctx, reportID, userID, comment, isAdmin)
}

func (r *ReportsRepo) ListReportComments(ctx context.Context, reportID uuid.UUID) ([]db.ReportComment, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ReportsRepo.ListReportComments")
	defer span.End()
	return r.db.ListReportComments(ctx, reportID)
}

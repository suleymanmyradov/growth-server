// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package profile

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"
	clientactivity "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/activity"
	clientcheckin "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/checkinservice"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"
	clienthabits "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/habits"
	clientpersonalization "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/personalizationservice"
	clientsaved "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/saved"
	clientsettings "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/settings"
	clientweeklyreview "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/weeklyreviewservice"
	fileManagerClient "github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/fileManagerClient"
	notificationsClient "github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/notificationsClient"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

type ExportDataLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExportDataLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportDataLogic {
	return &ExportDataLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ExportDataLogic) ExportData() (resp *types.ExportDataResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	// Gather data from all services in parallel. Each goroutine writes to its
	// own variable; the results are merged after all complete.
	type result struct {
		key string
		val any
		err error
	}

	results := make(chan result, 10)

	// Auth profile
	go func() {
		profileResp, err := l.svcCtx.AuthRpc.GetProfile(l.ctx, &authservice.GetProfileRequest{})
		if err != nil {
			results <- result{key: "profile", err: err}
			return
		}
		if profileResp.User != nil {
			b, _ := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(profileResp.User)
			var v any
			_ = json.Unmarshal(b, &v)
			results <- result{key: "profile", val: v}
		} else {
			results <- result{key: "profile", val: nil}
		}
	}()

	// Habits
	go func() {
		r, err := l.svcCtx.ClientRpc.Habits.ListHabits(l.ctx, &clienthabits.ListHabitsRequest{Page: 1, Limit: 1000})
		if err != nil {
			results <- result{key: "habits", err: err}
			return
		}
		b, _ := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(r)
		var v any
		_ = json.Unmarshal(b, &v)
		results <- result{key: "habits", val: v}
	}()

	// Goals
	go func() {
		r, err := l.svcCtx.ClientRpc.Goals.ListGoals(l.ctx, &clientgoals.ListGoalsRequest{Page: 1, Limit: 1000})
		if err != nil {
			results <- result{key: "goals", err: err}
			return
		}
		b, _ := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(r)
		var v any
		_ = json.Unmarshal(b, &v)
		results <- result{key: "goals", val: v}
	}()

	// Check-ins
	go func() {
		r, err := l.svcCtx.ClientRpc.CheckInService.GetCheckInHistory(l.ctx, &clientcheckin.GetCheckInHistoryRequest{UserId: p.UserID, Page: 1, Limit: 1000})
		if err != nil {
			results <- result{key: "checkIns", err: err}
			return
		}
		b, _ := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(r)
		var v any
		_ = json.Unmarshal(b, &v)
		results <- result{key: "checkIns", val: v}
	}()

	// Weekly reviews
	go func() {
		r, err := l.svcCtx.ClientRpc.WeeklyReviewService.ListWeeklyReviews(l.ctx, &clientweeklyreview.ListWeeklyReviewsRequest{UserId: p.UserID, Page: 1, Limit: 1000})
		if err != nil {
			results <- result{key: "weeklyReviews", err: err}
			return
		}
		b, _ := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(r)
		var v any
		_ = json.Unmarshal(b, &v)
		results <- result{key: "weeklyReviews", val: v}
	}()

	// Saved items
	go func() {
		r, err := l.svcCtx.ClientRpc.Saved.ListSaved(l.ctx, &clientsaved.ListSavedRequest{Limit: 1000, Offset: 0})
		if err != nil {
			results <- result{key: "savedItems", err: err}
			return
		}
		b, _ := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(r)
		var v any
		_ = json.Unmarshal(b, &v)
		results <- result{key: "savedItems", val: v}
	}()

	// Activity feed
	go func() {
		r, err := l.svcCtx.ClientRpc.Activity.GetActivityFeed(l.ctx, &clientactivity.GetActivityFeedRequest{UserId: p.UserID, Limit: 1000, Offset: 0})
		if err != nil {
			results <- result{key: "activities", err: err}
			return
		}
		b, _ := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(r)
		var v any
		_ = json.Unmarshal(b, &v)
		results <- result{key: "activities", val: v}
	}()

	// Settings
	go func() {
		r, err := l.svcCtx.ClientRpc.Settings.GetSettings(l.ctx, &clientsettings.GetSettingsRequest{})
		if err != nil {
			results <- result{key: "settings", err: err}
			return
		}
		b, _ := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(r)
		var v any
		_ = json.Unmarshal(b, &v)
		results <- result{key: "settings", val: v}
	}()

	// Coaching profile
	go func() {
		r, err := l.svcCtx.ClientRpc.PersonalizationService.GetCoachingProfile(l.ctx, &clientpersonalization.GetCoachingProfileRequest{UserId: p.UserID})
		if err != nil {
			results <- result{key: "coachingProfile", err: err}
			return
		}
		b, _ := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(r)
		var v any
		_ = json.Unmarshal(b, &v)
		results <- result{key: "coachingProfile", val: v}
	}()

	// Notifications
	go func() {
		r, err := l.svcCtx.NotificationsRpc.ListNotifications(l.ctx, &notificationsClient.ListNotificationsRequest{Limit: 1000, Offset: 0})
		if err != nil {
			results <- result{key: "notifications", err: err}
			return
		}
		b, _ := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(r)
		var v any
		_ = json.Unmarshal(b, &v)
		results <- result{key: "notifications", val: v}
	}()

	// Collect results
	exportData := make(map[string]any)
	exportData["exportedAt"] = time.Now().UTC().Format(time.RFC3339)
	exportData["userId"] = p.UserID

	var errs []error
	for i := 0; i < 10; i++ {
		r := <-results
		if r.err != nil {
			l.Errorf("export: failed to fetch %s: %v", r.key, r.err)
			errs = append(errs, fmt.Errorf("%s: %w", r.key, r.err))
			continue
		}
		exportData[r.key] = r.val
	}

	if len(errs) == 10 {
		return nil, status.Error(codes.Internal, "failed to export any data")
	}

	jsonBytes, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal export data")
	}

	filename := fmt.Sprintf("export-%s-%d.json", p.UserID, time.Now().Unix())
	uploadResp, err := l.svcCtx.FileManagerRpc.UploadFile(l.ctx, &fileManagerClient.UploadFileRequest{
		Data:        jsonBytes,
		Filename:    filename,
		ContentType: "application/json",
		Folder:      "exports",
	})
	if err != nil {
		l.Errorf("export: failed to upload to file manager: %v", err)
		return nil, status.Error(codes.Internal, "failed to upload export file")
	}

	// The exports/ prefix is not publicly readable, so hand back a short-lived
	// presigned URL instead of the bucket's public URL.
	presignResp, err := l.svcCtx.FileManagerRpc.GetPresignedURL(l.ctx, &fileManagerClient.GetPresignedURLRequest{
		Key:           uploadResp.Key,
		ExpirySeconds: 900,
	})
	if err != nil {
		l.Errorf("export: failed to presign download url: %v", err)
		return nil, status.Error(codes.Internal, "failed to generate export download link")
	}

	return &types.ExportDataResponse{
		DownloadUrl: presignResp.Url,
	}, nil
}

package weeklyreviewservicelogic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type StreamWeeklyReviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStreamWeeklyReviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StreamWeeklyReviewLogic {
	return &StreamWeeklyReviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *StreamWeeklyReviewLogic) StreamWeeklyReview(in *client.GenerateWeeklyReviewRequest, stream client.WeeklyReviewService_StreamWeeklyReviewServer) error {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "StreamWeeklyReviewLogic.StreamWeeklyReview")
	defer span.End()

	prepStart := time.Now()

	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid user ID")
	}

	settings, err := l.svcCtx.Repo.UserSettings.GetUserSettings(ctx, userID)
	if err != nil {
		l.Infof("failed to get user settings, using UTC: %v", err)
	}

	loc := time.UTC
	if settings.Timezone != "" {
		var err error
		loc, err = loadLocationCached(settings.Timezone)
		if err != nil {
			l.Infof("invalid timezone %s, using UTC: %v", settings.Timezone, err)
			loc = time.UTC
		}
	}

	weekStart, weekEnd, err := resolveWeekBounds(in.WeekStart, loc)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid weekStart")
	}

	// Same cooldown check as the unary version.
	if in.ForceRegenerate {
		cooldown := l.svcCtx.Config.WeeklyReview.RegenerationCooldown
		if cooldown == 0 {
			cooldown = time.Hour
		}
		existing, err := l.svcCtx.Repo.WeeklyReviews.GetWeeklyReview(ctx, userID, weekStart)
		if err == nil && existing.ID != uuid.Nil {
			timeSinceGeneration := time.Since(existing.GeneratedAt.Time)
			if timeSinceGeneration < cooldown {
				remaining := cooldown - timeSinceGeneration
				return status.Errorf(codes.ResourceExhausted, "please wait %s before regenerating", remaining.Round(time.Second))
			}
		}
	} else {
		existing, err := l.svcCtx.Repo.WeeklyReviews.GetWeeklyReview(ctx, userID, weekStart)
		if err == nil && existing.ID != uuid.Nil {
			l.Infof("returning cached review from DB (no regeneration needed)")
			return stream.Send(&client.WeeklyReviewStreamChunk{
				Complete: true,
				Review:   dbReviewToProto(existing),
			})
		}
	}

	t0 := time.Now()
	// Compute stats (same as unary).
	stats, err := l.computeWeeklyStats(ctx, userID, weekStart, weekEnd)
	if err != nil {
		l.Errorf("compute weekly stats: %v", err)
		return status.Error(codes.Internal, "failed to compute weekly stats")
	}
	l.Infof("prep: computeWeeklyStats took %v", time.Since(t0))

	// Fetch check-ins, habits, streaks, patterns (same as unary).
	t1 := time.Now()
	weekCheckIns, err := fetchAllPages(checkInPageSize, func(limit, offset int32) ([]db.CheckIn, error) {
		return l.svcCtx.Repo.CheckIns.GetCheckInHistory(ctx, userID, weekStart, weekEnd, limit, offset)
	})
	if err != nil {
		l.Infof("failed to get all week check-ins (using %d fetched): %v", len(weekCheckIns), err)
	}
	if weekCheckIns == nil {
		weekCheckIns = []db.CheckIn{}
	}
	l.Infof("prep: fetchCheckIns took %v (%d check-ins)", time.Since(t1), len(weekCheckIns))

	t2 := time.Now()
	weekHabits, err := fetchAllPages(habitPageSize, func(limit, offset int32) ([]db.GetHabitRow, error) {
		return l.svcCtx.Repo.Habits.ListHabits(ctx, userID, limit, offset)
	})
	if err != nil {
		l.Infof("failed to get all habits (using %d fetched): %v", len(weekHabits), err)
	}
	if weekHabits == nil {
		weekHabits = []db.GetHabitRow{}
	}

	streakRows, err := l.svcCtx.Repo.Habits.GetHabitStreaks(ctx, userID)
	if err != nil {
		l.Infof("failed to get habit streaks: %v", err)
		streakRows = []db.GetHabitStreaksRow{}
	}
	l.Infof("prep: fetchHabits+streaks took %v (%d habits, %d streaks)", time.Since(t2), len(weekHabits), len(streakRows))

	streakByHabit := make(map[uuid.UUID]int32, len(streakRows))
	for _, s := range streakRows {
		streakByHabit[s.HabitID] = s.Streak
	}

	t3 := time.Now()
	patternInsights := l.svcCtx.PatternDetection.AnalyzeFullFromData(weekCheckIns, weekHabits, streakByHabit, loc)
	l.Infof("prep: patternDetection took %v", time.Since(t3))

	// Personalization context.
	preferredTone := "supportive"
	difficultyPreference := "adaptive"
	commonBlockers := []string{}

	t4 := time.Now()
	coachingProfile, err := l.svcCtx.Repo.CoachingProfiles.GetCoachingProfile(ctx, userID)
	if err == nil && coachingProfile.UserID != uuid.Nil {
		if coachingProfile.PreferredTone != "" {
			preferredTone = string(coachingProfile.PreferredTone)
		}
		if coachingProfile.DifficultyPreference != "" {
			difficultyPreference = string(coachingProfile.DifficultyPreference)
		}
		if len(coachingProfile.CommonBlockers) > 0 {
			var blockers []string
			if err := json.Unmarshal(coachingProfile.CommonBlockers, &blockers); err == nil {
				commonBlockers = blockers
			}
		}
	}

	accountabilityStyle := "balanced"
	if settings.AccountabilityStyle != "" {
		accountabilityStyle = string(settings.AccountabilityStyle)
	}

	goals, err := fetchAllPages(goalPageSize, func(limit, offset int32) ([]db.GetGoalRow, error) {
		return l.svcCtx.Repo.Goals.ListGoals(ctx, userID, limit, offset)
	})
	if err != nil {
		l.Infof("failed to get all goals (using %d fetched): %v", len(goals), err)
	}
	l.Infof("prep: fetchCoachingProfile+goals took %v (%d goals)", time.Since(t4), len(goals))

	goalTitles := make([]string, len(goals))
	for i, g := range goals {
		goalTitles[i] = g.Title
	}

	detectedPatterns := make([]string, 0, 6+len(patternInsights.RiskFactors))
	if patternInsights.CompletionPattern != "" {
		detectedPatterns = append(detectedPatterns, "Completion pattern: "+patternInsights.CompletionPattern)
	}
	if patternInsights.BestTimeOfDay != "" {
		detectedPatterns = append(detectedPatterns, "Best time of day: "+patternInsights.BestTimeOfDay)
	}
	if patternInsights.HardestTimeOfDay != "" {
		detectedPatterns = append(detectedPatterns, "Hardest time of day: "+patternInsights.HardestTimeOfDay)
	}
	if patternInsights.MoodEnergyCorrelation != "" && patternInsights.MoodEnergyCorrelation != "no_data" {
		detectedPatterns = append(detectedPatterns, "Mood/energy correlation: "+patternInsights.MoodEnergyCorrelation)
	}
	if patternInsights.StreakPattern != "" && patternInsights.StreakPattern != "no_data" {
		detectedPatterns = append(detectedPatterns, "Streak pattern: "+patternInsights.StreakPattern)
	}
	for _, risk := range patternInsights.RiskFactors {
		detectedPatterns = append(detectedPatterns, "Risk factor: "+risk)
	}

	habitBreakdowns := make([]*aicoachservice.HabitBreakdown, len(stats.habitBreakdowns))
	for i, h := range stats.habitBreakdowns {
		habitBreakdowns[i] = &aicoachservice.HabitBreakdown{
			HabitId:        h.HabitID,
			HabitName:      h.HabitName,
			Category:       h.Category,
			CompletedCount: int32(h.CompletedCount),
			MissedCount:    int32(h.MissedCount),
			CompletionRate: float32(h.CompletionRate),
		}
	}
	blockerStats := make([]*aicoachservice.BlockerStat, len(stats.blockerStats))
	for i, b := range stats.blockerStats {
		blockerStats[i] = &aicoachservice.BlockerStat{Blocker: b.Blocker, Count: int32(b.Count)}
	}
	moodStats := make([]*aicoachservice.MoodStat, len(stats.moodStats))
	for i, m := range stats.moodStats {
		moodStats[i] = &aicoachservice.MoodStat{Mood: m.Mood, Count: int32(m.Count)}
	}
	energyStats := make([]*aicoachservice.EnergyStat, len(stats.energyStats))
	for i, e := range stats.energyStats {
		energyStats[i] = &aicoachservice.EnergyStat{Energy: e.Energy, Count: int32(e.Count)}
	}

	// Call the ai-coach streaming RPC.
	l.Infof("prep: total prep time before ai-coach call: %v", time.Since(prepStart))
	streamStart := time.Now()
	aiStream, err := l.svcCtx.AICoachRpc.StreamWeeklyReview(ctx, &aicoachservice.WeeklyReviewRequest{
		UserId:               in.UserId,
		AccountabilityStyle:  accountabilityStyle,
		PreferredTone:        preferredTone,
		DifficultyPreference: difficultyPreference,
		CommonBlockers:       commonBlockers,
		Goals:                goalTitles,
		TotalHabits:          int32(stats.totalHabits),
		CompletionRate:       float32(stats.completionRate),
		CompletedCheckIns:    int32(stats.completedCheckIns),
		MissedCheckIns:       int32(stats.missedCheckIns),
		BestDay:              stats.bestDay,
		HardestDay:           stats.hardestDay,
		TopBlocker:           stats.topBlocker,
		HabitBreakdowns:      habitBreakdowns,
		BlockerStats:         blockerStats,
		MoodStats:            moodStats,
		EnergyStats:          energyStats,
		DetectedPatterns:     detectedPatterns,
	})
	if err != nil {
		l.Errorf("AI stream RPC failed after %v: %v", time.Since(streamStart), err)
		return status.Error(codes.Internal, "failed to start AI stream")
	}
	l.Infof("ai-coach stream opened after %v", time.Since(streamStart))

	// Forward chunks from ai-coach to the client.
	var aiSummary string
	var suggestedAdjustments []*aicoachservice.WeeklyReviewAdjustment
	var nextWeekPlan *aicoachservice.NextWeekPlan

	// gRPC server streams are NOT safe for concurrent SendMsg. The heartbeat
	// goroutine below and this goroutine both send on the same stream, so every
	// send must be serialized through this mutex to avoid corrupting the stream.
	var sendMu sync.Mutex
	send := func(chunk *client.WeeklyReviewStreamChunk) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		return stream.Send(chunk)
	}

	// Start a heartbeat goroutine that sends empty delta chunks every 15
	// seconds to keep the SSE connection alive while the AI generates the
	// structured JSON (which can take 60+ seconds with no output).
	heartbeatDone := make(chan struct{})
	var heartbeatWG sync.WaitGroup
	var stopHeartbeatOnce sync.Once
	stopHeartbeat := func() {
		stopHeartbeatOnce.Do(func() { close(heartbeatDone) })
		heartbeatWG.Wait()
	}
	// Always stop the heartbeat, even on error returns.
	defer stopHeartbeat()

	heartbeatWG.Add(1)
	go func() {
		defer heartbeatWG.Done()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatDone:
				return
			case <-ticker.C:
				// Send an empty delta as a keepalive. The gateway handler
				// skips empty deltas, but the flush keeps the connection alive.
				_ = send(&client.WeeklyReviewStreamChunk{Delta: ""})
			}
		}
	}()

	chunkCount := 0
	for {
		chunk, recvErr := aiStream.Recv()
		if recvErr != nil {
			// io.EOF means the stream ended normally (the ai-coach returned
			// without sending a complete chunk). This can happen if the AI
			// call failed and the ai-coach returned an error via the stream.
			// If we already received a complete chunk, this is fine.
			if recvErr == io.EOF {
				l.Infof("ai-coach stream ended (EOF) after %d chunks, %v elapsed", chunkCount, time.Since(streamStart))
				break
			}
			ctxErr := ctx.Err()
			l.Errorf("ai-coach stream recv error after %d chunks, %v elapsed: err=%v (type=%T), ctx.Err()=%v, aiSummaryLen=%d",
				chunkCount, time.Since(streamStart), recvErr, recvErr, ctxErr, len(aiSummary))
			return status.Errorf(codes.Internal, "AI stream interrupted: %v", recvErr)
		}
		chunkCount++

		if chunk.Complete {
			if chunk.Review != nil {
				aiSummary = chunk.Review.AiSummary
				suggestedAdjustments = chunk.Review.SuggestedAdjustments
				nextWeekPlan = chunk.Review.NextWeekPlan
			}
			break
		}

		// Forward the finalizing signal.
		if chunk.Finalizing {
			if sendErr := send(&client.WeeklyReviewStreamChunk{Finalizing: true}); sendErr != nil {
				l.Errorf("stream send finalizing error: %v", sendErr)
				return sendErr
			}
			continue
		}

		// Forward the text delta.
		if chunk.Delta != "" {
			if sendErr := send(&client.WeeklyReviewStreamChunk{Delta: chunk.Delta}); sendErr != nil {
				l.Errorf("stream send error: %v", sendErr)
				return sendErr
			}
		}
	}

	// Stop the heartbeat before persisting and sending the final Complete
	// chunk, so no keepalive send can race with (or follow) the final send.
	stopHeartbeat()

	// Normalize adjustment types.
	for _, adj := range suggestedAdjustments {
		if adj != nil {
			adj.AdjustmentType = normalizeAdjustmentType(adj.AdjustmentType)
		}
	}

	// Persist to DB (same as unary).
	moodSummaryJSON, err := json.Marshal(stats.moodMap)
	if err != nil {
		l.Errorf("failed to marshal mood summary: %v", err)
		return status.Error(codes.Internal, "failed to serialize mood summary")
	}
	energySummaryJSON, err := json.Marshal(stats.energyMap)
	if err != nil {
		l.Errorf("failed to marshal energy summary: %v", err)
		return status.Error(codes.Internal, "failed to serialize energy summary")
	}
	habitBreakdownJSON, err := json.Marshal(stats.habitBreakdownsForDB)
	if err != nil {
		l.Errorf("failed to marshal habit breakdown: %v", err)
		return status.Error(codes.Internal, "failed to serialize habit breakdown")
	}
	suggestedAdjustmentsJSON, err := json.Marshal(suggestedAdjustments)
	if err != nil {
		l.Errorf("failed to marshal suggested adjustments: %v", err)
		return status.Error(codes.Internal, "failed to serialize suggested adjustments")
	}
	nextWeekPlanJSON, err := json.Marshal(nextWeekPlan)
	if err != nil {
		l.Errorf("failed to marshal next week plan: %v", err)
		return status.Error(codes.Internal, "failed to serialize next week plan")
	}

	weekStartDate := pgtype.Date{Time: weekStart, Valid: true}
	var completionRate pgtype.Numeric
	if err := completionRate.Scan(fmt.Sprintf("%.2f", stats.completionRate)); err != nil {
		l.Infof("failed to scan completion rate: %v", err)
	}

	var bestDay *string
	if stats.bestDay != "" {
		bestDay = &stats.bestDay
	}
	var hardestDay *string
	if stats.hardestDay != "" {
		hardestDay = &stats.hardestDay
	}
	var topBlocker *string
	if stats.topBlocker != "" {
		topBlocker = &stats.topBlocker
	}
	var aiSummaryPtr *string
	if aiSummary != "" {
		aiSummaryPtr = &aiSummary
	}

	params := db.CreateWeeklyReviewParams{
		UserID:               userID,
		WeekStart:            weekStartDate,
		TotalHabits:          int32(stats.totalHabits),
		CompletedCheckIns:    int32(stats.completedCheckIns),
		MissedCheckIns:       int32(stats.missedCheckIns),
		CompletionRate:       completionRate,
		BestDay:              bestDay,
		HardestDay:           hardestDay,
		TopBlocker:           topBlocker,
		MoodSummary:          moodSummaryJSON,
		EnergySummary:        energySummaryJSON,
		HabitBreakdown:       habitBreakdownJSON,
		AiSummary:            aiSummaryPtr,
		SuggestedAdjustments: suggestedAdjustmentsJSON,
		NextWeekPlan:         nextWeekPlanJSON,
	}

	review, err := l.svcCtx.Repo.WeeklyReviews.CreateWeeklyReview(ctx, params)
	if err != nil {
		l.Errorf("failed to save weekly review: %v", err)
		return status.Error(codes.Internal, "failed to save weekly review")
	}

	// Create plan adjustment suggestions in background (same as unary).
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logx.Errorf("panic while creating plan adjustment suggestions: %v", r)
			}
		}()

		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		for _, adjustment := range suggestedAdjustments {
			if adjustment == nil || adjustment.AdjustmentType == "keep_same" {
				continue
			}

			var goalID, habitID uuid.NullUUID
			if adjustment.HabitId != "" {
				if habitUUID, err := uuid.Parse(adjustment.HabitId); err == nil {
					habitID = uuid.NullUUID{UUID: habitUUID, Valid: true}
				}
			}

			metadata := map[string]string{
				"source":          "weekly_review",
				"week_start":      weekStart.Format(time.RFC3339),
				"habit_name":      adjustment.HabitName,
				"adjustment_type": adjustment.AdjustmentType,
				"ai_generated":    "true",
			}
			metadataJSON, _ := json.Marshal(metadata)

			_, err := l.svcCtx.Repo.PlanAdjustmentSuggestions.CreatePlanAdjustmentSuggestion(bgCtx, db.CreatePlanAdjustmentSuggestionParams{
				UserID:         userID,
				GoalID:         goalID,
				HabitID:        habitID,
				Source:         "weekly_review",
				AdjustmentType: adjustment.AdjustmentType,
				Reason:         adjustment.Reason,
				Suggestion:     adjustment.Suggestion,
				Metadata:       metadataJSON,
				WeekStart:      pgtype.Date{Time: weekStart, Valid: true},
			})
			if err != nil {
				logx.Errorf("failed to create plan adjustment suggestion: %v", err)
			}
		}
	}()

	// Send the final complete chunk with the persisted review.
	return send(&client.WeeklyReviewStreamChunk{
		Complete: true,
		Review:   dbReviewToProto(review),
	})
}

// computeWeeklyStats delegates to the same method on GenerateWeeklyReviewLogic.
// We embed the same receiver type by copying the method via a shared helper.
func (l *StreamWeeklyReviewLogic) computeWeeklyStats(ctx context.Context, userID uuid.UUID, start, end time.Time) (weeklyStats, error) {
	gen := NewGenerateWeeklyReviewLogic(ctx, l.svcCtx)
	return gen.computeWeeklyStats(ctx, userID, start, end)
}

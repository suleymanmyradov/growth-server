package aicoachservicelogic

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/prompts"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/logx"
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

func (l *StreamWeeklyReviewLogic) StreamWeeklyReview(in *aicoach.WeeklyReviewRequest, stream aicoach.AICoachService_StreamWeeklyReviewServer) error {
	if l.svcCtx.AIClient == nil {
		return stream.Send(&aicoach.WeeklyReviewStreamChunk{
			Complete: true,
			Review: &aicoach.WeeklyReviewResponse{
				AiSummary: "AI service is not configured. Please contact support.",
			},
		})
	}

	// Convert proto habit breakdowns to prompt input.
	habitBreakdowns := make([]prompts.HabitBreakdownInput, len(in.HabitBreakdowns))
	for i, h := range in.HabitBreakdowns {
		habitBreakdowns[i] = prompts.HabitBreakdownInput{
			HabitID:        h.HabitId,
			HabitName:      h.HabitName,
			Category:       h.Category,
			CompletedCount: int(h.CompletedCount),
			MissedCount:    int(h.MissedCount),
			CompletionRate: float64(h.CompletionRate),
		}
	}

	blockerStats := make([]prompts.BlockerInput, len(in.BlockerStats))
	for i, b := range in.BlockerStats {
		blockerStats[i] = prompts.BlockerInput{
			Blocker: b.Blocker,
			Count:   int(b.Count),
		}
	}

	moodStats := make([]prompts.MoodInput, len(in.MoodStats))
	for i, m := range in.MoodStats {
		moodStats[i] = prompts.MoodInput{
			Mood:  m.Mood,
			Count: int(m.Count),
		}
	}

	energyStats := make([]prompts.EnergyInput, len(in.EnergyStats))
	for i, e := range in.EnergyStats {
		energyStats[i] = prompts.EnergyInput{
			Energy: e.Energy,
			Count:  int(e.Count),
		}
	}

	input := prompts.WeeklyReviewInput{
		AccountabilityStyle:  in.AccountabilityStyle,
		PreferredTone:        in.PreferredTone,
		DifficultyPreference: in.DifficultyPreference,
		CommonBlockers:       in.CommonBlockers,
		Goals:                in.Goals,
		TotalHabits:          int(in.TotalHabits),
		CompletionRate:       float64(in.CompletionRate),
		CompletedCheckIns:    int(in.CompletedCheckIns),
		MissedCheckIns:       int(in.MissedCheckIns),
		BestDay:              in.BestDay,
		HardestDay:           in.HardestDay,
		TopBlocker:           in.TopBlocker,
		HabitBreakdowns:      habitBreakdowns,
		BlockerStats:         blockerStats,
		MoodStats:            moodStats,
		EnergyStats:          energyStats,
		DetectedPatterns:     in.DetectedPatterns,
	}

	systemPrompt := prompts.BuildWeeklyReviewStreamSystemPrompt(in.AccountabilityStyle, in.PreferredTone, in.DifficultyPreference)
	userPrompt := prompts.BuildWeeklyReviewUserPrompt(input)

	aiStream, err := l.svcCtx.AIClient.Stream(l.ctx, ai.GenerateRequest{
		ModelProfile: ai.ModelCheapLong,
		System:       systemPrompt,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: userPrompt},
		},
		Metadata: ai.Metadata{
			UserID:  in.UserId,
			Feature: "weekly_review_stream",
		},
	})
	if err != nil {
		l.Errorf("AI stream open failed: %v", err)
		return stream.Send(&aicoach.WeeklyReviewStreamChunk{
			Complete: true,
			Review: &aicoach.WeeklyReviewResponse{
				AiSummary: "I couldn't generate your weekly review right now. Please try again later.",
			},
		})
	}
	defer aiStream.Close()

	// The model emits the human-readable summary first, then the
	// JSONDelimiter ("|||JSON|||"), then a JSON object with structured
	// fields. We must forward ONLY the summary text as deltas to the
	// client — the delimiter and JSON must never reach the user's UI.
	//
	// The delimiter may span multiple deltas, so we buffer incoming text
	// and only forward the portion that is guaranteed to be before any
	// possible delimiter occurrence. Once the delimiter is found, we send
	// a "finalizing" signal (so the frontend can show a parsing indicator)
	// and accumulate the rest internally as JSON.
	var fullText strings.Builder
	var pending strings.Builder // un-forwarded text (before delimiter found)
	delim := prompts.JSONDelimiter
	delimFound := false

	for {
		chunk, recvErr := aiStream.Recv()
		if recvErr != nil {
			if recvErr == io.EOF {
				break
			}
			l.Errorf("AI stream recv error: %v", recvErr)
			break
		}

		if chunk.Delta == "" {
			continue
		}

		fullText.WriteString(chunk.Delta)

		// Once the delimiter has been found, all remaining text is JSON —
		// accumulate internally, never forward.
		if delimFound {
			continue
		}

		pending.WriteString(chunk.Delta)
		s := pending.String()

		if idx := strings.Index(s, delim); idx >= 0 {
			// Delimiter found — forward the text before it.
			before := s[:idx]
			if before != "" {
				if err := stream.Send(&aicoach.WeeklyReviewStreamChunk{
					Delta: before,
				}); err != nil {
					return err
				}
			}
			// Signal the client that the summary text is done and the
			// backend is now parsing the structured JSON.
			if err := stream.Send(&aicoach.WeeklyReviewStreamChunk{
				Finalizing: true,
			}); err != nil {
				return err
			}
			delimFound = true
			pending.Reset()
			continue
		}

		// No delimiter yet. Forward the safe portion of the buffer, but
		// hold back any trailing suffix that could be the start of a
		// partial delimiter (it might be completed by the next delta).
		maxHold := len(delim) - 1
		if maxHold > len(s) {
			maxHold = len(s)
		}
		safeLen := len(s)
		for i := maxHold; i >= 1; i-- {
			if strings.HasPrefix(delim, s[len(s)-i:]) {
				safeLen = len(s) - i
				break
			}
		}
		if safeLen > 0 {
			if err := stream.Send(&aicoach.WeeklyReviewStreamChunk{
				Delta: s[:safeLen],
			}); err != nil {
				return err
			}
		}
		pending.Reset()
		pending.WriteString(s[safeLen:])
	}

	// If the model never emitted the delimiter, forward any remaining
	// buffered text so the user sees the full summary.
	if !delimFound && pending.Len() > 0 {
		if err := stream.Send(&aicoach.WeeklyReviewStreamChunk{
			Delta: pending.String(),
		}); err != nil {
			return err
		}
	}

	raw := fullText.String()
	l.Infof("weekly review stream complete: user=%s, %d chars, delimiterFound=%v", in.UserId, len(raw), delimFound)

	// Split on the JSON delimiter — text before is the summary, text after is structured JSON.
	parts := strings.SplitN(raw, prompts.JSONDelimiter, 2)
	summary := strings.TrimSpace(parts[0])

	review := &aicoach.WeeklyReviewResponse{
		AiSummary: summary,
	}

	if len(parts) == 2 {
		jsonPart := strings.TrimSpace(parts[1])
		var structured prompts.WeeklyReviewStructuredOutput
		if err := json.Unmarshal([]byte(jsonPart), &structured); err == nil {
			// Map structured output to proto.
			adjustments := make([]*aicoach.WeeklyReviewAdjustment, len(structured.SuggestedAdjustments))
			for i, a := range structured.SuggestedAdjustments {
				adjustments[i] = &aicoach.WeeklyReviewAdjustment{
					HabitId:        a.HabitID,
					HabitName:      a.HabitName,
					Reason:         a.Reason,
					Suggestion:     a.Suggestion,
					AdjustmentType: a.AdjustmentType,
				}
			}
			review.SuggestedAdjustments = adjustments
			review.NextWeekPlan = &aicoach.NextWeekPlan{
				Focus:           structured.NextWeekPlan.Focus,
				Commitments:     structured.NextWeekPlan.Commitments,
				Risks:           structured.NextWeekPlan.Risks,
				RecoveryActions: structured.NextWeekPlan.RecoveryActions,
			}
		} else {
			l.Errorf("failed to parse weekly review JSON: %v", err)
		}
	}

	return stream.Send(&aicoach.WeeklyReviewStreamChunk{
		Complete: true,
		Review:   review,
	})
}

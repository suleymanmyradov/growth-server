package personalization

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/types"
	aicoachrpc "github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client"
	aicoachservice "github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"
)

// ============================================
// Mock AICoachService
// ============================================

type mockAICoachService struct {
	generateOnboardingHabitsFn func(ctx context.Context, in *aicoachservice.GenerateOnboardingHabitsRequest) (*aicoachservice.GenerateOnboardingHabitsResponse, error)
	lastRequest                *aicoachservice.GenerateOnboardingHabitsRequest
	searchMemoryFn             func(ctx context.Context, in *aicoachservice.SearchMemoryRequest) (*aicoachservice.SearchMemoryResponse, error)
}

func (m *mockAICoachService) GenerateCheckInFeedback(ctx context.Context, in *aicoachservice.CheckInFeedbackRequest, opts ...grpc.CallOption) (*aicoachservice.CheckInFeedbackResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAICoachService) GeneratePersonalizedCoaching(ctx context.Context, in *aicoachservice.PersonalizedCoachingRequest, opts ...grpc.CallOption) (*aicoachservice.PersonalizedCoachingResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAICoachService) GenerateWeeklyReview(ctx context.Context, in *aicoachservice.WeeklyReviewRequest, opts ...grpc.CallOption) (*aicoachservice.WeeklyReviewResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAICoachService) StreamWeeklyReview(ctx context.Context, in *aicoachservice.WeeklyReviewRequest, opts ...grpc.CallOption) (aicoach.AICoachService_StreamWeeklyReviewClient, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAICoachService) StreamPersonalizedCoaching(ctx context.Context, in *aicoachservice.PersonalizedCoachingRequest, opts ...grpc.CallOption) (aicoach.AICoachService_StreamPersonalizedCoachingClient, error) {
	return nil, errors.New("not implemented")
}

// Long-term memory RPCs. searchMemoryFn is settable so the
// search_past_conversations tool can be exercised; the fact-management RPCs are
// not used from this package.
func (m *mockAICoachService) SearchMemory(ctx context.Context, in *aicoachservice.SearchMemoryRequest, opts ...grpc.CallOption) (*aicoachservice.SearchMemoryResponse, error) {
	if m.searchMemoryFn != nil {
		return m.searchMemoryFn(ctx, in)
	}
	return nil, errors.New("not implemented")
}
func (m *mockAICoachService) ListUserFacts(ctx context.Context, in *aicoachservice.ListUserFactsRequest, opts ...grpc.CallOption) (*aicoachservice.ListUserFactsResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAICoachService) AddUserFact(ctx context.Context, in *aicoachservice.AddUserFactRequest, opts ...grpc.CallOption) (*aicoachservice.AddUserFactResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAICoachService) ForgetUserFact(ctx context.Context, in *aicoachservice.ForgetUserFactRequest, opts ...grpc.CallOption) (*aicoachservice.ForgetUserFactResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAICoachService) ForgetAllUserFacts(ctx context.Context, in *aicoachservice.ForgetAllUserFactsRequest, opts ...grpc.CallOption) (*aicoachservice.ForgetAllUserFactsResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAICoachService) GenerateOnboardingHabits(ctx context.Context, in *aicoachservice.GenerateOnboardingHabitsRequest, opts ...grpc.CallOption) (*aicoachservice.GenerateOnboardingHabitsResponse, error) {
	m.lastRequest = in
	if m.generateOnboardingHabitsFn != nil {
		return m.generateOnboardingHabitsFn(ctx, in)
	}
	return &aicoachservice.GenerateOnboardingHabitsResponse{
		Habits: []*aicoach.OnboardingHabitSuggestion{
			{Name: "Walk 15 min", Description: "After lunch"},
			{Name: "Read 10 pages", Description: "Before bed"},
			{Name: "Log one win", Description: "In journal"},
		},
	}, nil
}
func (m *mockAICoachService) Transcribe(ctx context.Context, in *aicoachservice.TranscribeRequest, opts ...grpc.CallOption) (*aicoachservice.TranscribeResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAICoachService) Synthesize(ctx context.Context, in *aicoachservice.SynthesizeRequest, opts ...grpc.CallOption) (*aicoachservice.SynthesizeResponse, error) {
	return nil, errors.New("not implemented")
}

// ============================================
// Helpers
// ============================================

// ctxWithPrincipal returns a context with an authenticated principal.
func ctxWithPrincipal(userID string) context.Context {
	return principal.WithPrincipal(context.Background(), principal.Principal{
		UserID: userID,
	})
}

// ============================================
// Tests
// ============================================

func TestGenerateOnboardingHabits_Success(t *testing.T) {
	mockSvc := &mockAICoachService{}
	svcCtx := &svc.ServiceContext{
		AICoachRpc: &aicoachrpc.Service{AICoachService: mockSvc},
	}

	l := NewGenerateOnboardingHabitsLogic(ctxWithPrincipal("user-123"), svcCtx)
	resp, err := l.GenerateOnboardingHabits(&types.GenerateOnboardingHabitsRequest{
		GoalTitle:           "Run a 5k",
		GoalCategory:        "fitness",
		Motivation:          "Feel healthier",
		Blocker:             "Time",
		DailyMinutes:        30,
		AccountabilityStyle: "balanced",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Data, 3)
	assert.Equal(t, "Walk 15 min", resp.Data[0].Name)
	assert.Equal(t, "After lunch", resp.Data[0].Description)
}

func TestGenerateOnboardingHabits_PassesUserIDToRPC(t *testing.T) {
	mockSvc := &mockAICoachService{}
	svcCtx := &svc.ServiceContext{
		AICoachRpc: &aicoachrpc.Service{AICoachService: mockSvc},
	}

	l := NewGenerateOnboardingHabitsLogic(ctxWithPrincipal("user-abc"), svcCtx)
	_, err := l.GenerateOnboardingHabits(&types.GenerateOnboardingHabitsRequest{
		GoalTitle:    "Test",
		DailyMinutes: 30,
	})
	require.NoError(t, err)

	require.NotNil(t, mockSvc.lastRequest)
	assert.Equal(t, "user-abc", mockSvc.lastRequest.UserId)
}

func TestGenerateOnboardingHabits_PassesAllFieldsToRPC(t *testing.T) {
	mockSvc := &mockAICoachService{}
	svcCtx := &svc.ServiceContext{
		AICoachRpc: &aicoachrpc.Service{AICoachService: mockSvc},
	}

	l := NewGenerateOnboardingHabitsLogic(ctxWithPrincipal("user-1"), svcCtx)
	_, err := l.GenerateOnboardingHabits(&types.GenerateOnboardingHabitsRequest{
		GoalTitle:           "Read more",
		GoalCategory:        "learning",
		Motivation:          "Grow knowledge",
		Blocker:             "Distraction",
		DailyMinutes:        45,
		AccountabilityStyle: "strict",
	})
	require.NoError(t, err)

	require.NotNil(t, mockSvc.lastRequest)
	assert.Equal(t, "Read more", mockSvc.lastRequest.GoalTitle)
	assert.Equal(t, "learning", mockSvc.lastRequest.GoalCategory)
	assert.Equal(t, "Grow knowledge", mockSvc.lastRequest.Motivation)
	assert.Equal(t, "Distraction", mockSvc.lastRequest.Blocker)
	assert.Equal(t, int32(45), mockSvc.lastRequest.DailyMinutes)
	assert.Equal(t, "strict", mockSvc.lastRequest.AccountabilityStyle)
}

func TestGenerateOnboardingHabits_EmptyAccountabilityDefaultsToBalanced(t *testing.T) {
	mockSvc := &mockAICoachService{}
	svcCtx := &svc.ServiceContext{
		AICoachRpc: &aicoachrpc.Service{AICoachService: mockSvc},
	}

	l := NewGenerateOnboardingHabitsLogic(ctxWithPrincipal("user-1"), svcCtx)
	_, err := l.GenerateOnboardingHabits(&types.GenerateOnboardingHabitsRequest{
		GoalTitle:           "Test",
		DailyMinutes:        30,
		AccountabilityStyle: "", // empty → should default to "balanced"
	})
	require.NoError(t, err)

	require.NotNil(t, mockSvc.lastRequest)
	assert.Equal(t, "balanced", mockSvc.lastRequest.AccountabilityStyle)
}

func TestGenerateOnboardingHabits_MissingPrincipalReturnsUnauthenticated(t *testing.T) {
	mockSvc := &mockAICoachService{}
	svcCtx := &svc.ServiceContext{
		AICoachRpc: &aicoachrpc.Service{AICoachService: mockSvc},
	}

	// No principal in context
	l := NewGenerateOnboardingHabitsLogic(context.Background(), svcCtx)
	_, err := l.GenerateOnboardingHabits(&types.GenerateOnboardingHabitsRequest{
		GoalTitle:    "Test",
		DailyMinutes: 30,
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	// The RPC should not have been called
	assert.Nil(t, mockSvc.lastRequest)
}

func TestGenerateOnboardingHabits_RPCErrorPropagates(t *testing.T) {
	mockSvc := &mockAICoachService{
		generateOnboardingHabitsFn: func(_ context.Context, _ *aicoachservice.GenerateOnboardingHabitsRequest) (*aicoachservice.GenerateOnboardingHabitsResponse, error) {
			return nil, status.Error(codes.Unavailable, "ai-coach service down")
		},
	}
	svcCtx := &svc.ServiceContext{
		AICoachRpc: &aicoachrpc.Service{AICoachService: mockSvc},
	}

	l := NewGenerateOnboardingHabitsLogic(ctxWithPrincipal("user-1"), svcCtx)
	_, err := l.GenerateOnboardingHabits(&types.GenerateOnboardingHabitsRequest{
		GoalTitle:    "Test",
		DailyMinutes: 30,
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unavailable, st.Code())
	assert.Contains(t, st.Message(), "ai-coach service down")
}

func TestGenerateOnboardingHabits_MapsHabitsToResponseTypes(t *testing.T) {
	mockSvc := &mockAICoachService{
		generateOnboardingHabitsFn: func(_ context.Context, _ *aicoachservice.GenerateOnboardingHabitsRequest) (*aicoachservice.GenerateOnboardingHabitsResponse, error) {
			return &aicoachservice.GenerateOnboardingHabitsResponse{
				Habits: []*aicoach.OnboardingHabitSuggestion{
					{Name: "Custom Habit A", Description: "Custom desc A"},
					{Name: "Custom Habit B", Description: "Custom desc B"},
				},
			}, nil
		},
	}
	svcCtx := &svc.ServiceContext{
		AICoachRpc: &aicoachrpc.Service{AICoachService: mockSvc},
	}

	l := NewGenerateOnboardingHabitsLogic(ctxWithPrincipal("user-1"), svcCtx)
	resp, err := l.GenerateOnboardingHabits(&types.GenerateOnboardingHabitsRequest{
		GoalTitle:    "Test",
		DailyMinutes: 30,
	})
	require.NoError(t, err)
	require.Len(t, resp.Data, 2)
	assert.Equal(t, "Custom Habit A", resp.Data[0].Name)
	assert.Equal(t, "Custom desc A", resp.Data[0].Description)
	assert.Equal(t, "Custom Habit B", resp.Data[1].Name)
	assert.Equal(t, "Custom desc B", resp.Data[1].Description)
}

func TestGenerateOnboardingHabits_EmptyHabitsFromRPC(t *testing.T) {
	mockSvc := &mockAICoachService{
		generateOnboardingHabitsFn: func(_ context.Context, _ *aicoachservice.GenerateOnboardingHabitsRequest) (*aicoachservice.GenerateOnboardingHabitsResponse, error) {
			return &aicoachservice.GenerateOnboardingHabitsResponse{Habits: nil}, nil
		},
	}
	svcCtx := &svc.ServiceContext{
		AICoachRpc: &aicoachrpc.Service{AICoachService: mockSvc},
	}

	l := NewGenerateOnboardingHabitsLogic(ctxWithPrincipal("user-1"), svcCtx)
	resp, err := l.GenerateOnboardingHabits(&types.GenerateOnboardingHabitsRequest{
		GoalTitle:    "Test",
		DailyMinutes: 30,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Data)
}

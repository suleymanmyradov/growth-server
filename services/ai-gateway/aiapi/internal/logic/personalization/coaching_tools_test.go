package personalization

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	clientarticles "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/articles"
	clientcheckin "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/checkinservice"
	clienthabits "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/habits"
	clientpb "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	searchpb "github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/pb/search"
	searchservice "github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/searchservice"
)

// ============================================
// Proposal tool tests (pure functions, no RPC)
// ============================================

func TestProposeCreateGoalTool(t *testing.T) {
	tool := proposeCreateGoalTool()
	ctx := context.Background()

	t.Run("valid input", func(t *testing.T) {
		out, err := tool.Execute(ctx, `{"title":"Read 12 books","category":"learning","dueDate":"2026-12-31"}`)
		require.NoError(t, err)
		assert.Contains(t, out, `"action":"create_goal"`)
		assert.Contains(t, out, `"title":"Read 12 books"`)
		assert.Contains(t, out, `"category":"learning"`)
		assert.Contains(t, out, `"dueDate":"2026-12-31"`)
		// ID should be a non-empty UUID.
		assert.Contains(t, out, `"id":"`)
	})

	t.Run("missing title", func(t *testing.T) {
		_, err := tool.Execute(ctx, `{"category":"learning"}`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "title is required")
	})

	t.Run("empty input", func(t *testing.T) {
		_, err := tool.Execute(ctx, `{}`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "title is required")
	})
}

func TestProposeUpdateGoalTool(t *testing.T) {
	tool := proposeUpdateGoalTool()
	ctx := context.Background()

	t.Run("valid input", func(t *testing.T) {
		out, err := tool.Execute(ctx, `{"goalId":"abc-123","title":"Updated title"}`)
		require.NoError(t, err)
		assert.Contains(t, out, `"action":"update_goal"`)
		assert.Contains(t, out, `"goalId":"abc-123"`)
		assert.Contains(t, out, `"title":"Updated title"`)
	})

	t.Run("missing goalId", func(t *testing.T) {
		_, err := tool.Execute(ctx, `{"title":"Updated title"}`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "goalId is required")
	})
}

func TestProposeDeleteGoalTool(t *testing.T) {
	tool := proposeDeleteGoalTool()
	ctx := context.Background()

	t.Run("valid input", func(t *testing.T) {
		out, err := tool.Execute(ctx, `{"goalId":"abc-123"}`)
		require.NoError(t, err)
		assert.Contains(t, out, `"action":"delete_goal"`)
		assert.Contains(t, out, `"goalId":"abc-123"`)
	})

	t.Run("missing goalId", func(t *testing.T) {
		_, err := tool.Execute(ctx, `{}`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "goalId is required")
	})
}

func TestProposeCreateHabitTool(t *testing.T) {
	tool := proposeCreateHabitTool()
	ctx := context.Background()

	t.Run("valid input", func(t *testing.T) {
		out, err := tool.Execute(ctx, `{"name":"Meditate","description":"10 min daily","category":"mindfulness"}`)
		require.NoError(t, err)
		assert.Contains(t, out, `"action":"create_habit"`)
		assert.Contains(t, out, `"name":"Meditate"`)
		assert.Contains(t, out, `"description":"10 min daily"`)
	})

	t.Run("missing name", func(t *testing.T) {
		_, err := tool.Execute(ctx, `{"category":"mindfulness"}`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})
}

func TestProposeUpdateHabitTool(t *testing.T) {
	tool := proposeUpdateHabitTool()
	ctx := context.Background()

	t.Run("valid input", func(t *testing.T) {
		out, err := tool.Execute(ctx, `{"habitId":"h-123","name":"Meditate 15 min"}`)
		require.NoError(t, err)
		assert.Contains(t, out, `"action":"update_habit"`)
		assert.Contains(t, out, `"habitId":"h-123"`)
		assert.Contains(t, out, `"name":"Meditate 15 min"`)
	})

	t.Run("missing habitId", func(t *testing.T) {
		_, err := tool.Execute(ctx, `{"name":"Meditate"}`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "habitId is required")
	})
}

func TestProposeDeleteHabitTool(t *testing.T) {
	tool := proposeDeleteHabitTool()
	ctx := context.Background()

	t.Run("valid input", func(t *testing.T) {
		out, err := tool.Execute(ctx, `{"habitId":"h-123"}`)
		require.NoError(t, err)
		assert.Contains(t, out, `"action":"delete_habit"`)
		assert.Contains(t, out, `"habitId":"h-123"`)
	})

	t.Run("missing habitId", func(t *testing.T) {
		_, err := tool.Execute(ctx, `{}`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "habitId is required")
	})
}

func TestProposalOutputIDsAreUnique(t *testing.T) {
	tool := proposeCreateGoalTool()
	ctx := context.Background()

	out1, err := tool.Execute(ctx, `{"title":"Goal 1"}`)
	require.NoError(t, err)
	out2, err := tool.Execute(ctx, `{"title":"Goal 2"}`)
	require.NoError(t, err)
	assert.NotEqual(t, out1, out2, "two proposals should have different IDs")
}

// ============================================
// search_articles tool tests (with mocks)
// ============================================

// --- Mock SearchService ---

type mockSearchService struct {
	searchFn func(ctx context.Context, in *searchservice.SearchRequest) (*searchservice.SearchResponse, error)
}

func (m *mockSearchService) Search(ctx context.Context, in *searchservice.SearchRequest, _ ...grpc.CallOption) (*searchservice.SearchResponse, error) {
	return m.searchFn(ctx, in)
}
func (m *mockSearchService) GetSearchSuggestions(_ context.Context, _ *searchservice.GetSearchSuggestionsRequest, _ ...grpc.CallOption) (*searchservice.GetSearchSuggestionsResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockSearchService) GetSearchFilters(_ context.Context, _ *searchservice.GetSearchFiltersRequest, _ ...grpc.CallOption) (*searchservice.GetSearchFiltersResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockSearchService) GetSearchHistory(_ context.Context, _ *searchservice.GetSearchHistoryRequest, _ ...grpc.CallOption) (*searchservice.GetSearchHistoryResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockSearchService) SaveSearch(_ context.Context, _ *searchservice.SaveSearchRequest, _ ...grpc.CallOption) (*searchservice.SaveSearchResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockSearchService) ClearSearchHistory(_ context.Context, _ *searchservice.ClearSearchHistoryRequest, _ ...grpc.CallOption) (*searchservice.ClearSearchHistoryResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockSearchService) DeleteSearchHistoryItem(_ context.Context, _ *searchservice.DeleteSearchHistoryItemRequest, _ ...grpc.CallOption) (*searchservice.DeleteSearchHistoryItemResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockSearchService) GetTrendingSearches(_ context.Context, _ *searchservice.GetTrendingSearchesRequest, _ ...grpc.CallOption) (*searchservice.GetTrendingSearchesResponse, error) {
	return nil, errors.New("not implemented")
}

// --- Mock Articles ---

type mockArticles struct {
	getByIdsFn func(ctx context.Context, in *clientarticles.GetArticlesByIdsRequest) (*clientarticles.GetArticlesByIdsResponse, error)
}

func (m *mockArticles) ListArticles(_ context.Context, _ *clientarticles.ListArticlesRequest, _ ...grpc.CallOption) (*clientarticles.ListArticlesResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockArticles) GetArticle(_ context.Context, _ *clientarticles.GetArticleRequest, _ ...grpc.CallOption) (*clientarticles.GetArticleResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockArticles) CreateArticle(_ context.Context, _ *clientarticles.CreateArticleRequest, _ ...grpc.CallOption) (*clientarticles.CreateArticleResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockArticles) UpdateArticle(_ context.Context, _ *clientarticles.UpdateArticleRequest, _ ...grpc.CallOption) (*clientarticles.UpdateArticleResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockArticles) DeleteArticle(_ context.Context, _ *clientarticles.DeleteArticleRequest, _ ...grpc.CallOption) (*clientarticles.DeleteArticleResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockArticles) LikeArticle(_ context.Context, _ *clientarticles.LikeArticleRequest, _ ...grpc.CallOption) (*clientarticles.LikeArticleResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockArticles) ShareArticle(_ context.Context, _ *clientarticles.ShareArticleRequest, _ ...grpc.CallOption) (*clientarticles.ShareArticleResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockArticles) GetAuthorArticles(_ context.Context, _ *clientarticles.GetAuthorArticlesRequest, _ ...grpc.CallOption) (*clientarticles.GetAuthorArticlesResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockArticles) ListTags(_ context.Context, _ *clientarticles.ListTagsRequest, _ ...grpc.CallOption) (*clientarticles.ListTagsResponse, error) {
	return nil, errors.New("not implemented")
}
func (m *mockArticles) GetArticlesByIds(ctx context.Context, in *clientarticles.GetArticlesByIdsRequest, _ ...grpc.CallOption) (*clientarticles.GetArticlesByIdsResponse, error) {
	return m.getByIdsFn(ctx, in)
}
func (m *mockArticles) GetFeaturedArticle(_ context.Context, _ *clientarticles.GetFeaturedArticleRequest, _ ...grpc.CallOption) (*clientarticles.GetFeaturedArticleResponse, error) {
	return nil, errors.New("not implemented")
}

func TestSearchArticlesTool(t *testing.T) {
	ctx := context.Background()

	t.Run("returns hydrated articles", func(t *testing.T) {
		search := &mockSearchService{
			searchFn: func(_ context.Context, in *searchservice.SearchRequest) (*searchservice.SearchResponse, error) {
				assert.Equal(t, "discipline", in.Query)
				assert.Equal(t, []string{"articles"}, in.Types)
				assert.Equal(t, "published", in.Status)
				return &searchservice.SearchResponse{
					Results: []*searchpb.SearchResult{
						{Id: "art-1", Title: "Building Discipline", Description: "A guide", Url: "/articles/art-1"},
						{Id: "art-2", Title: "Habit Stacking", Description: "How to", Url: "/articles/art-2"},
					},
				}, nil
			},
		}
		articles := &mockArticles{
			getByIdsFn: func(_ context.Context, in *clientarticles.GetArticlesByIdsRequest) (*clientarticles.GetArticlesByIdsResponse, error) {
				assert.ElementsMatch(t, []string{"art-1", "art-2"}, in.Ids)
				return &clientarticles.GetArticlesByIdsResponse{
					Articles: []*clientpb.Article{
						{Id: "art-1", Title: "Building Discipline", Summary: "A comprehensive guide", ReadTime: 5, Category: &clientpb.ArticleCategory{Name: "Mindset"}},
						{Id: "art-2", Title: "Habit Stacking", Summary: "How to stack habits", ReadTime: 3, Category: &clientpb.ArticleCategory{Name: "Habits"}},
					},
				}, nil
			},
		}

		tool := searchArticlesTool(search, articles)
		out, err := tool.Execute(ctx, `{"query":"discipline"}`)
		require.NoError(t, err)
		assert.Contains(t, out, `"id":"art-1"`)
		assert.Contains(t, out, `"title":"Building Discipline"`)
		assert.Contains(t, out, `"summary":"A comprehensive guide"`)
		assert.Contains(t, out, `"readTime":5`)
		assert.Contains(t, out, `"category":"Mindset"`)
		assert.Contains(t, out, `"url":"/articles/art-1"`)
	})

	t.Run("falls back to search metadata when hydration fails", func(t *testing.T) {
		search := &mockSearchService{
			searchFn: func(_ context.Context, _ *searchservice.SearchRequest) (*searchservice.SearchResponse, error) {
				return &searchservice.SearchResponse{
					Results: []*searchpb.SearchResult{
						{Id: "art-1", Title: "Building Discipline", Description: "A guide", Url: "/articles/art-1"},
					},
				}, nil
			},
		}
		articles := &mockArticles{
			getByIdsFn: func(_ context.Context, _ *clientarticles.GetArticlesByIdsRequest) (*clientarticles.GetArticlesByIdsResponse, error) {
				return nil, errors.New("rpc unavailable")
			},
		}

		tool := searchArticlesTool(search, articles)
		out, err := tool.Execute(ctx, `{"query":"discipline"}`)
		require.NoError(t, err)
		assert.Contains(t, out, `"id":"art-1"`)
		assert.Contains(t, out, `"title":"Building Discipline"`)
		assert.Contains(t, out, `"summary":"A guide"`)
		assert.Contains(t, out, `"url":"/articles/art-1"`)
	})

	t.Run("empty query returns error", func(t *testing.T) {
		tool := searchArticlesTool(&mockSearchService{}, &mockArticles{})
		_, err := tool.Execute(ctx, `{"query":""}`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "query is required")
	})

	t.Run("nil search service returns error", func(t *testing.T) {
		tool := searchArticlesTool(nil, &mockArticles{})
		_, err := tool.Execute(ctx, `{"query":"discipline"}`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "search service not configured")
	})

	t.Run("no results returns empty array", func(t *testing.T) {
		search := &mockSearchService{
			searchFn: func(_ context.Context, _ *searchservice.SearchRequest) (*searchservice.SearchResponse, error) {
				return &searchservice.SearchResponse{Results: nil}, nil
			},
		}
		tool := searchArticlesTool(search, &mockArticles{})
		out, err := tool.Execute(ctx, `{"query":"nonexistent"}`)
		require.NoError(t, err)
		assert.Contains(t, out, `"articles":[]`)
	})
}

type mockCheckInService struct {
	clientcheckin.CheckInService
	requests []*clientcheckin.GetCheckInHistoryRequest
}

func (m *mockCheckInService) GetCheckInHistory(_ context.Context, in *clientcheckin.GetCheckInHistoryRequest, _ ...grpc.CallOption) (*clientcheckin.GetCheckInHistoryResponse, error) {
	m.requests = append(m.requests, in)
	return &clientcheckin.GetCheckInHistoryResponse{
		CheckIns: []*clientpb.CheckIn{{
			HabitId:   in.HabitId,
			Status:    "completed",
			Note:      "Felt focused",
			CreatedAt: 1_700_000_000,
		}},
	}, nil
}

type mockHabitsService struct {
	clienthabits.Habits
}

func (m *mockHabitsService) ListHabits(_ context.Context, _ *clienthabits.ListHabitsRequest, _ ...grpc.CallOption) (*clienthabits.ListHabitsResponse, error) {
	return &clienthabits.ListHabitsResponse{Habits: []*clientpb.Habit{
		{Id: "habit-1", Name: "Morning walk"},
		{Id: "habit-2", Name: "Read nightly"},
	}}, nil
}

func TestGetRecentCheckInsToolFiltersAndLabelsHabits(t *testing.T) {
	checkIns := &mockCheckInService{}
	tool := getRecentCheckInsTool("user-1", checkIns, &mockHabitsService{})

	out, err := tool.Execute(context.Background(), `{"habitIds":["habit-1","habit-2"]}`)
	require.NoError(t, err)
	require.Len(t, checkIns.requests, 2)
	assert.Equal(t, "habit-1", checkIns.requests[0].HabitId)
	assert.Equal(t, "habit-2", checkIns.requests[1].HabitId)
	assert.Contains(t, out, `"habitId":"habit-1"`)
	assert.Contains(t, out, `"habitName":"Morning walk"`)
	assert.Contains(t, out, `"habitId":"habit-2"`)
	assert.Contains(t, out, `"habitName":"Read nightly"`)
	assert.Contains(t, out, `"note":"Felt focused"`)
}

// ============================================
// BuildCoachingTools integration test
// ============================================

func TestBuildCoachingToolsIncludesAllTools(t *testing.T) {
	tools := BuildCoachingTools("user-123", CoachingToolDeps{
		Search:   &mockSearchService{},
		Articles: &mockArticles{},
	})

	names := make(map[string]bool, len(tools))
	for _, tool := range tools {
		names[tool.Name()] = true
	}

	// Original retrieval tools.
	assert.True(t, names["get_active_goals"])
	assert.True(t, names["get_active_habits"])
	assert.True(t, names["get_recent_check_ins"])
	assert.True(t, names["get_latest_weekly_review"])
	assert.True(t, names["get_pending_suggestions"])
	assert.True(t, names["get_coaching_profile"])
	// Detail retrieval tools.
	assert.True(t, names["get_goal"])
	assert.True(t, names["get_habit"])
	// Proposal tools.
	assert.True(t, names["propose_create_goal"])
	assert.True(t, names["propose_update_goal"])
	assert.True(t, names["propose_delete_goal"])
	assert.True(t, names["propose_create_habit"])
	assert.True(t, names["propose_update_habit"])
	assert.True(t, names["propose_delete_habit"])
	// Article search.
	assert.True(t, names["search_articles"])
}

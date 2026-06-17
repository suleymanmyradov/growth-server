package saved

import (
	"context"
	"sync"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"
	clienthabits "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/habits"
	clientsaved "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/saved"
	pbclient "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListSavedDetailedLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListSavedDetailedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSavedDetailedLogic {
	return &ListSavedDetailedLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListSavedDetailedLogic) ListSavedDetailed(req *types.PageRequest) (resp *types.SavedItemsDetailedResponse, err error) {
	_, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return &types.SavedItemsDetailedResponse{Data: []types.SavedItemDetailed{}}, nil
	}

	// 1. Get saved items
	rpcResp, err := l.svcCtx.SavedRpc.ListSaved(l.ctx, &clientsaved.ListSavedRequest{
		Limit:  int32(req.Limit),
		Offset: int32((req.Page - 1) * req.Limit),
	})
	if err != nil {
		return nil, err
	}

	// 2. Fetch all user habits and goals in parallel (single call each)
	var habitsResp *pbclient.ListHabitsResponse
	var goalsResp *pbclient.ListGoalsResponse
	var habitsErr, goalsErr error

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		habitsResp, habitsErr = l.svcCtx.HabitsRpc.ListHabits(l.ctx, &clienthabits.ListHabitsRequest{
			Limit: 1000,
		})
	}()
	go func() {
		defer wg.Done()
		goalsResp, goalsErr = l.svcCtx.GoalsRpc.ListGoals(l.ctx, &clientgoals.ListGoalsRequest{
			Limit: 1000,
		})
	}()
	wg.Wait()

	// Build lookup maps
	habitMap := make(map[string]*pbclient.Habit)
	if habitsErr == nil && habitsResp != nil {
		for _, h := range habitsResp.Habits {
			habitMap[h.Id] = h
		}
	}

	goalMap := make(map[string]*pbclient.Goal)
	if goalsErr == nil && goalsResp != nil {
		for _, g := range goalsResp.Goals {
			goalMap[g.Id] = g
		}
	}

	// 3. Fetch articles in parallel (one RPC per saved article)
	var articleMu sync.Mutex
	articleMap := make(map[string]*pbclient.Article)
	var articleWg sync.WaitGroup

	for _, item := range rpcResp.Items {
		if item.ItemType == "article" {
			articleWg.Add(1)
			go func(id string) {
				defer articleWg.Done()
				articleResp, err := l.svcCtx.ArticlesRpc.GetArticle(l.ctx, &pbclient.GetArticleRequest{ArticleId: id})
				if err != nil || articleResp == nil {
					return
				}
				articleMu.Lock()
				articleMap[id] = articleResp.Article
				articleMu.Unlock()
			}(item.ItemId)
		}
	}
	articleWg.Wait()

	// 4. Build hydrated response
	items := make([]types.SavedItemDetailed, 0, len(rpcResp.Items))
	for _, item := range rpcResp.Items {
		detailed := types.SavedItemDetailed{
			Id:        item.Id,
			ItemType:  item.ItemType,
			ItemId:    item.ItemId,
			UserId:    item.UserId,
			CreatedAt: time.Unix(item.SavedAt, 0).Format(time.RFC3339),
		}

		switch item.ItemType {
		case "article":
			if a := articleMap[item.ItemId]; a != nil {
				detailed.Article = types.Article{
					Id:          a.Id,
					Title:       a.Title,
					Excerpt:     a.Summary,
					Content:     a.Content,
					ReadTime:    int(a.ReadTime),
					ImageUrl:    a.CoverImage,
					Author:      a.AuthorId,
					PublishedAt: formatUnix(a.PublishedAt),
					CreatedAt:   formatUnix(a.CreatedAt),
					UpdatedAt:   formatUnix(a.UpdatedAt),
					Tags:        a.Tags,
				}
				if a.Category != nil {
					detailed.Article.Category = &types.ArticleCategory{
						Id:   a.Category.Id,
						Name: a.Category.Name,
						Slug: a.Category.Slug,
					}
				}
			}
		case "habit":
			if h := habitMap[item.ItemId]; h != nil {
				detailed.Habit = types.Habit{
					Id:          h.Id,
					Name:        h.Name,
					Description: h.Description,
					Category:    h.Category,
					Streak:      int(h.Streak),
					Completed:   h.Completed,
					CreatedAt:   formatUnix(h.CreatedAt),
					UpdatedAt:   formatUnix(h.UpdatedAt),
				}
			}
		case "goal":
			if g := goalMap[item.ItemId]; g != nil {
				detailed.Goal = types.Goal{
					Id:          g.Id,
					Title:       g.Title,
					Description: g.Description,
					Category:    g.Category,
					Progress:    int(g.Progress),
					Completed:   g.Completed,
					DueDate:     formatUnix(g.DueDate),
					CreatedAt:   formatUnix(g.CreatedAt),
					UpdatedAt:   formatUnix(g.UpdatedAt),
				}
			}
		}

		items = append(items, detailed)
	}

	totalPages := int(rpcResp.TotalCount) / req.Limit
	if int(rpcResp.TotalCount)%req.Limit > 0 {
		totalPages++
	}

	return &types.SavedItemsDetailedResponse{
		Data: items,
		Page: types.PageResponse{
			Total:      int64(rpcResp.TotalCount),
			Page:       req.Page,
			Limit:      req.Limit,
			TotalPages: totalPages,
		},
	}, nil
}

func formatUnix(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).Format(time.RFC3339)
}

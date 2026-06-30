package articleslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SearchArticlesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchArticlesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchArticlesLogic {
	return &SearchArticlesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SearchArticlesLogic) SearchArticles(in *client.SearchArticlesRequest) (*client.SearchArticlesResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "SearchArticlesLogic.SearchArticles")
	defer span.End()
	if in.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	limit := int32(20)
	offset := int32(0)
	if in.Limit > 0 {
		limit = in.Limit
	}
	if in.Offset > 0 {
		offset = in.Offset
	}

	rows, err := l.svcCtx.Repo.Articles.SearchArticles(ctx, in.Query, in.Status, limit, offset)
	if err != nil {
		l.Errorf("search articles failed: %v", err)
		return nil, status.Error(codes.Internal, "search articles failed")
	}

	pbArticles := make([]*client.Article, len(rows))
	for i, a := range rows {
		pbArticles[i] = convertSearchRowToPbArticle(a)
	}

	if len(pbArticles) > 0 {
		articleIDs := make([]uuid.UUID, 0, len(pbArticles))
		for _, a := range pbArticles {
			if id, err := uuid.Parse(a.Id); err == nil {
				articleIDs = append(articleIDs, id)
			}
		}
		tagRows, err := l.svcCtx.Repo.Articles.GetTagsByArticleIDs(ctx, articleIDs)
		if err != nil {
			l.Errorf("failed to get article tags: %v", err)
		} else {
			tagMap := make(map[string][]string)
			for _, t := range tagRows {
				tagMap[t.ArticleID.String()] = append(tagMap[t.ArticleID.String()], t.Name)
			}
			for _, a := range pbArticles {
				tags := tagMap[a.Id]
				if tags == nil {
					tags = []string{}
				}
				a.Tags = tags
			}
		}
	}

	totalCount, _ := l.svcCtx.Repo.Articles.CountSearchArticles(ctx, in.Query, in.Status)

	return &client.SearchArticlesResponse{
		Articles:   pbArticles,
		TotalCount: int32(totalCount),
	}, nil
}

package articleslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetArticlesByIdsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetArticlesByIdsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetArticlesByIdsLogic {
	return &GetArticlesByIdsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

const maxGetArticlesByIds = 100

// GetArticlesByIds hydrates article IDs into full Article protos (with
// category, like count and tags). Articles are returned in the order of the
// requested IDs; unknown or deleted IDs are silently skipped so callers
// holding a stale ID list (e.g. from a search index) get partial results
// rather than an error.
func (l *GetArticlesByIdsLogic) GetArticlesByIds(in *client.GetArticlesByIdsRequest) (*client.GetArticlesByIdsResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetArticlesByIdsLogic.GetArticlesByIds")
	defer span.End()

	if len(in.Ids) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ids is required")
	}
	if len(in.Ids) > maxGetArticlesByIds {
		return nil, status.Errorf(codes.InvalidArgument, "too many ids: max %d", maxGetArticlesByIds)
	}

	ids := make([]uuid.UUID, 0, len(in.Ids))
	for _, raw := range in.Ids {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid article id: %s", raw)
		}
		ids = append(ids, id)
	}

	rows, err := l.svcCtx.Repo.Articles.GetArticlesByIDs(ctx, ids)
	if err != nil {
		l.Errorf("get articles by ids failed: %v", err)
		return nil, status.Error(codes.Internal, "get articles by ids failed")
	}

	byID := make(map[string]db.GetArticlesByIDsRow, len(rows))
	for _, r := range rows {
		byID[r.ID.String()] = r
	}

	// Rebuild the article list in the order of the requested IDs.
	pbArticles := make([]*client.Article, 0, len(rows))
	for _, id := range ids {
		row, ok := byID[id.String()]
		if !ok {
			continue
		}
		pbArticles = append(pbArticles, convertGetArticlesByIDsRowToPbArticle(row))
	}

	// Attach tags in bulk (same pattern as ListArticles).
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

	return &client.GetArticlesByIdsResponse{Articles: pbArticles}, nil
}

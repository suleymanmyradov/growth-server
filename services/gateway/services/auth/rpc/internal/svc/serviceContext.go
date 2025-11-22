package svc

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/stores/redis"

	authcache "github.com/suleymanmyradov/growth-server/services/gateway/services/auth/domain/cache"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/shared/repository"
)

type ServiceContext struct {
	Config config.Config
	Repo   *repository.BaseRepository
	Cache  authcache.Cache
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.MustConnect("postgres", c.Database.DataSource)
	redisClient := redis.MustNewRedis(c.Redis)

	cache := authcache.NewCache(redisClient)

	return &ServiceContext{
		Config: c,
		Repo:   repository.NewBaseRepository(db),
		Cache:  cache,
	}
}

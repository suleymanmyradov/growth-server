// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/suleymanmyradov/growth-server/backend/services/gateway/internal/config"
	"github.com/suleymanmyradov/growth-server/backend/services/gateway/internal/middleware"
	"github.com/suleymanmyradov/growth-server/backend/services/gateway/internal/repository"
)

type ServiceContext struct {
	Config          config.Config
	DB              *sqlx.DB
	JWTMiddleware   *middleware.JWTMiddleware
	UserRepo        *repository.UserRepository
	ProfileRepo     *repository.ProfileRepository
}

func NewServiceContext(c config.Config) *ServiceContext {
	// Initialize database connection
	db, err := sqlx.Connect("postgres", c.DataSource)
	if err != nil {
		panic(err)
	}

	// Initialize JWT middleware
	jwtMiddleware := middleware.NewJWTMiddleware(
		c.Auth.AccessSecret,
		c.Auth.RefreshSecret,
		c.Auth.AccessExpire,
		c.Auth.RefreshExpire,
	)

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	profileRepo := repository.NewProfileRepository(db)

	return &ServiceContext{
		Config:        c,
		DB:            db,
		JWTMiddleware: jwtMiddleware,
		UserRepo:      userRepo,
		ProfileRepo:   profileRepo,
	}
}

package clientservice

import (
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/activity"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/articles"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/billingservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/categories"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/checkinservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/habits"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/personalizationservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/report"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/saved"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/settings"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/tags"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/weeklyreviewservice"
	"github.com/zeromicro/go-zero/zrpc"
)

// Service aggregates every sub-service client exposed by the client RPC into a
// single struct. Consumers (e.g. the gateway) only need to import this one
// package instead of each individual sub-service package.
type Service struct {
	Activity               activity.Activity
	Articles               articles.Articles
	BillingService         billingservice.BillingService
	Categories             categories.Categories
	CheckInService         checkinservice.CheckInService
	Goals                  goals.Goals
	Habits                 habits.Habits
	PersonalizationService personalizationservice.PersonalizationService
	Report                 report.Report
	Saved                  saved.Saved
	Settings               settings.Settings
	Tags                   tags.Tags
	WeeklyReviewService    weeklyreviewservice.WeeklyReviewService
}

// NewClientService constructs every sub-service client from a single
// zrpc.Client. All sub-services share the same client (and therefore the same
// timeout/interceptor configuration).
func NewClientService(cli zrpc.Client) *Service {
	return &Service{
		Activity:               activity.NewActivity(cli),
		Articles:               articles.NewArticles(cli),
		BillingService:         billingservice.NewBillingService(cli),
		Categories:             categories.NewCategories(cli),
		CheckInService:         checkinservice.NewCheckInService(cli),
		Goals:                  goals.NewGoals(cli),
		Habits:                 habits.NewHabits(cli),
		PersonalizationService: personalizationservice.NewPersonalizationService(cli),
		Report:                 report.NewReport(cli),
		Saved:                  saved.NewSaved(cli),
		Settings:               settings.NewSettings(cli),
		Tags:                   tags.NewTags(cli),
		WeeklyReviewService:    weeklyreviewservice.NewWeeklyReviewService(cli),
	}
}

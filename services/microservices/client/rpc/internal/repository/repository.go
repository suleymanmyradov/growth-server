package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
)

type IArticles interface {
	ListArticles(ctx context.Context, status string, limit, offset int32) ([]db.ListArticlesRow, error)
	ListArticlesWithSaved(ctx context.Context, status string, limit, offset int32, userID uuid.UUID) ([]db.ListArticlesWithSavedRow, error)
	ListArticlesByCategorySlug(ctx context.Context, slug string, status string, limit, offset int32) ([]db.ListArticlesByCategorySlugRow, error)
	ListArticlesByCategorySlugWithSaved(ctx context.Context, slug string, status string, limit, offset int32, userID uuid.UUID) ([]db.ListArticlesByCategorySlugWithSavedRow, error)
	ListArticlesByAuthor(ctx context.Context, author string, status string, limit, offset int32) ([]db.ListArticlesByAuthorRow, error)
	ListArticlesByAuthorWithSaved(ctx context.Context, author string, status string, limit, offset int32, userID uuid.UUID) ([]db.ListArticlesByAuthorWithSavedRow, error)
	GetArticlesByIDs(ctx context.Context, ids []uuid.UUID) ([]db.GetArticlesByIDsRow, error)
	GetArticleByID(ctx context.Context, id uuid.UUID, status string) (db.GetArticleRow, error)
	GetArticleByIDWithSaved(ctx context.Context, id uuid.UUID, userID uuid.UUID, status string) (db.GetArticleWithSavedRow, error)
	GetArticleByTitle(ctx context.Context, title string) (db.GetArticleByTitleRow, error)
	GetFeaturedArticle(ctx context.Context) (db.GetFeaturedArticleRow, error)
	CreateArticle(ctx context.Context, params db.CreateArticleParams) (db.CreateArticleRow, error)
	UpdateArticle(ctx context.Context, params db.UpdateArticleParams) (db.UpdateArticleRow, error)
	DeleteArticle(ctx context.Context, id uuid.UUID) error
	CountArticles(ctx context.Context, status string) (int64, error)
	CountArticlesByCategorySlug(ctx context.Context, slug string, status string) (int64, error)
	CountArticlesByCategoryID(ctx context.Context, id uuid.UUID) (int64, error)
	CreateArticleShare(ctx context.Context, articleID uuid.UUID, userID uuid.UUID, platform string) (db.ArticleShare, error)
	CreateArticleLike(ctx context.Context, articleID uuid.UUID, userID uuid.UUID) (db.ArticleLike, error)
	DeleteArticleLike(ctx context.Context, articleID uuid.UUID, userID uuid.UUID) error
	CountArticleLikes(ctx context.Context, articleID uuid.UUID) (int64, error)
	IsArticleLikedByUser(ctx context.Context, articleID uuid.UUID, userID uuid.UUID) (bool, error)
	UpsertTags(ctx context.Context, names []string, slugs []string) ([]db.UpsertTagsRow, error)
	DeleteArticleTags(ctx context.Context, articleID uuid.UUID) error
	LinkArticleTags(ctx context.Context, articleID uuid.UUID, tagNames []string) error
	GetTagsByArticleIDs(ctx context.Context, articleIDs []uuid.UUID) ([]db.GetTagsByArticleIDsRow, error)
	ListTags(ctx context.Context) ([]db.ListTagsRow, error)
}

type ITags interface {
	CreateTag(ctx context.Context, name string, slug string) (db.Tag, error)
	GetTag(ctx context.Context, id uuid.UUID) (db.Tag, error)
	GetTagBySlug(ctx context.Context, slug string) (db.Tag, error)
	UpdateTag(ctx context.Context, id uuid.UUID, name string, slug string) (db.Tag, error)
	DeleteTag(ctx context.Context, id uuid.UUID) error
	CountTagUsage(ctx context.Context, id uuid.UUID) (int64, error)
}

// SavedItem is the uniform view over the three concrete saved tables.
type SavedItem struct {
	ID        uuid.UUID
	ItemType  string
	ItemID    uuid.UUID
	UserID    uuid.UUID
	CreatedAt pgtype.Timestamptz
}

type ISavedItems interface {
	ListSavedItemsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]SavedItem, error)
	ListSavedItemsByType(ctx context.Context, userID uuid.UUID, itemType string, limit, offset int32) ([]SavedItem, error)
	CreateSavedItem(ctx context.Context, itemType string, itemID uuid.UUID, userID uuid.UUID) (SavedItem, error)
	DeleteSavedItemByUserAndItem(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) error
	IsItemSaved(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) (bool, error)
	CountSavedItemsByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountSavedItemsByUserAndType(ctx context.Context, userID uuid.UUID, itemType string) (int64, error)
}

type IActivities interface {
	ListActivities(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Activity, error)
	ListActivitiesByType(ctx context.Context, userID uuid.UUID, itemType string, limit, offset int32) ([]db.Activity, error)
	GetActivityByID(ctx context.Context, id uuid.UUID) (db.Activity, error)
	CreateActivity(ctx context.Context, params db.CreateActivityParams) (db.Activity, error)
	LogActivity(ctx context.Context, params db.LogActivityParams) (db.Activity, error)
	DeleteActivity(ctx context.Context, id uuid.UUID) error
	DeleteActivitiesByUser(ctx context.Context, userID uuid.UUID) error
	CountActivitiesByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountActivitiesByUserAndType(ctx context.Context, userID uuid.UUID, itemType string) (int64, error)
	GetActivityFeed(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Activity, error)
	GetActivityStats(ctx context.Context, userID uuid.UUID) (db.GetActivityStatsRow, error)
	GetStreaks(ctx context.Context, userID uuid.UUID) (db.GetStreaksRow, error)
	GetAchievements(ctx context.Context, userID uuid.UUID) ([]db.GetAchievementsRow, error)
	GetActivityCalendar(ctx context.Context, userID uuid.UUID, year, month int32) ([]db.GetActivityCalendarRow, error)
}

type IUserPreferences interface {
	GetUserPreferences(ctx context.Context, userID uuid.UUID) (db.UserPreference, error)
	CreateUserPreferences(ctx context.Context, theme string, language string, timezone string, userID uuid.UUID) (db.UserPreference, error)
	UpdateUserPreferences(ctx context.Context, userID uuid.UUID, theme string, language string, timezone string) (db.UserPreference, error)
	UpdateOnboardingCompleted(ctx context.Context, userID uuid.UUID, checkInTime pgtype.Time, onboardingCompleted bool) (db.UserPreference, error)
	DeleteUserPreferences(ctx context.Context, userID uuid.UUID) error
}

type IUsers interface {
	GetUserProfileByID(ctx context.Context, id uuid.UUID) (db.GetUserProfileByIDRow, error)
}

type IHabits interface {
	ListHabits(ctx context.Context, userID uuid.UUID, limit, offset int32, timezone string) ([]db.GetHabitRow, error)
	GetHabitByID(ctx context.Context, id uuid.UUID, timezone string) (db.GetHabitRow, error)
	CreateHabit(ctx context.Context, name string, description *string, category string, userID uuid.UUID) (db.GetHabitRow, error)
	UpdateHabit(ctx context.Context, params db.UpdateHabitParams) (db.GetHabitRow, error)
	DeleteHabit(ctx context.Context, id uuid.UUID) error
	GetHabitStreak(ctx context.Context, habitID, userID uuid.UUID, timezone string) (int32, error)
	GetHabitStreaks(ctx context.Context, userID uuid.UUID, timezone string) ([]db.GetHabitStreaksRow, error)
	ResetTodayHabits(ctx context.Context, userID uuid.UUID, timezone string) (int64, error)
	CountHabitsByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	ListHabitHistory(ctx context.Context, userID uuid.UUID, timezone string) ([]db.ListHabitHistoryRow, error)
}

type IGoals interface {
	ListGoals(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.GetGoalRow, error)
	GetGoalByID(ctx context.Context, id uuid.UUID) (db.GetGoalRow, error)
	CreateGoal(ctx context.Context, params db.CreateGoalParams) (db.GetGoalRow, error)
	UpdateGoal(ctx context.Context, params db.UpdateGoalParams) (db.GetGoalRow, error)
	DeleteGoal(ctx context.Context, id uuid.UUID) error
	ToggleGoal(ctx context.Context, id uuid.UUID) (db.GetGoalRow, error)
	UpdateGoalProgress(ctx context.Context, id uuid.UUID, progress int32) (db.GetGoalRow, error)
	LogGoalValue(ctx context.Context, id uuid.UUID, currentValue pgtype.Numeric) (db.GetGoalRow, error)
	RecomputeGoalProgress(ctx context.Context, id uuid.UUID, progress int32) (db.GetGoalRow, error)
	CountGoalsByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountActiveGoalsByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	ListGoalHabitIDs(ctx context.Context, userID uuid.UUID) ([]db.ListGoalHabitIDsRow, error)
	ListGoalHabitIDsByGoal(ctx context.Context, goalID uuid.UUID) ([]uuid.UUID, error)
	ListGoalIDsByHabit(ctx context.Context, habitID uuid.UUID) ([]uuid.UUID, error)
	UnlinkAllGoalHabits(ctx context.Context, goalID uuid.UUID) error
	LinkGoalHabitsBatch(ctx context.Context, goalID uuid.UUID, habitIDs []uuid.UUID) error
	CountGoalMilestones(ctx context.Context, goalID uuid.UUID) (db.CountGoalMilestonesRow, error)
	ListGoalMilestones(ctx context.Context, goalID uuid.UUID) ([]db.GoalMilestone, error)
	ListGoalMilestonesByGoals(ctx context.Context, goalIDs []uuid.UUID) ([]db.GoalMilestone, error)
	CreateGoalMilestone(ctx context.Context, goalID uuid.UUID, title string, sortOrder int32) (db.GoalMilestone, error)
	UpdateGoalMilestone(ctx context.Context, id, goalID uuid.UUID, title string, sortOrder int32) (db.GoalMilestone, error)
	ToggleGoalMilestone(ctx context.Context, id, goalID uuid.UUID) (db.GoalMilestone, error)
	DeleteGoalMilestone(ctx context.Context, id, goalID uuid.UUID) error
}

type ICategories interface {
	ListCategories(ctx context.Context) ([]db.Category, error)
	GetCategoryByID(ctx context.Context, id uuid.UUID) (db.Category, error)
	GetCategoryBySlug(ctx context.Context, slug string) (db.Category, error)
	CreateCategory(ctx context.Context, name string, slug string, sortOrder int32) (db.Category, error)
	UpdateCategory(ctx context.Context, id uuid.UUID, name string, slug string, sortOrder int32) (db.Category, error)
	DeleteCategory(ctx context.Context, id uuid.UUID) error
	CountCategories(ctx context.Context) (int64, error)
	GetCategoriesByIDs(ctx context.Context, ids []uuid.UUID) ([]db.Category, error)
	ReorderCategories(ctx context.Context, ids []uuid.UUID, sortOrders []int32) error
	// Admin list for template management (called by adminway via gRPC).
	AdminListCategories(ctx context.Context) ([]db.AdminListCategoriesRow, error)
}

type ICheckIns interface {
	CreateCheckIn(ctx context.Context, params db.CreateCheckInParams) (db.CheckIn, error)
	UpsertCheckIn(ctx context.Context, params db.UpsertCheckInParams) (db.CheckIn, error)
	GetTodayCheckIns(ctx context.Context, userID uuid.UUID, timezone string) ([]db.CheckIn, error)
	GetCheckInsByHabit(ctx context.Context, habitID, userID uuid.UUID, limit, offset int32) ([]db.CheckIn, error)
	GetCheckInsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.CheckIn, error)
	GetCheckInHistory(ctx context.Context, userID uuid.UUID, start, end time.Time, limit, offset int32) ([]db.CheckIn, error)
	GetCheckInsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.CheckIn, error)
	HasCheckedInToday(ctx context.Context, userID, habitID uuid.UUID, timezone string) (bool, error)
	CountCheckInsByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountCheckInsByHabit(ctx context.Context, habitID uuid.UUID) (int64, error)
	CountCompletedCheckInDays(ctx context.Context, habitIDs []uuid.UUID, from, to pgtype.Date) (int64, error)
	DeleteTodayCheckIn(ctx context.Context, userID, habitID uuid.UUID, timezone string) (int64, error)
}

type IWeeklyReviews interface {
	CreateWeeklyReview(ctx context.Context, params db.CreateWeeklyReviewParams) (db.GetWeeklyReviewRow, error)
	GetWeeklyReview(ctx context.Context, userID uuid.UUID, weekStart time.Time) (db.GetWeeklyReviewRow, error)
	GetCurrentWeeklyReview(ctx context.Context, userID uuid.UUID) (db.GetWeeklyReviewRow, error)
	ListWeeklyReviews(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.GetWeeklyReviewRow, error)
	CountWeeklyReviews(ctx context.Context, userID uuid.UUID) (int64, error)
	GetCheckInStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetCheckInStatsForWeekRow, error)
	GetDailyCheckInStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetDailyCheckInStatsForWeekRow, error)
	GetBlockerStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetBlockerStatsForWeekRow, error)
	GetMoodStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetMoodStatsForWeekRow, error)
	GetEnergyStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetEnergyStatsForWeekRow, error)
}

type ICoachingProfiles interface {
	GetCoachingProfile(ctx context.Context, userID uuid.UUID) (db.GetCoachingProfileRow, error)
	UpsertCoachingProfile(ctx context.Context, params db.UpsertCoachingProfileParams) (db.UpsertCoachingProfileRow, error)
	UpdateCoachingProfilePreferences(ctx context.Context, userID uuid.UUID, accountabilityStyle string, preferredTone string, difficultyPreference string) (db.UpdateCoachingProfilePreferencesRow, error)
	UpdateCoachingProfileBlockers(ctx context.Context, userID uuid.UUID, commonBlockers []byte) (db.UpdateCoachingProfileBlockersRow, error)
	UpdateCoachingProfileNotes(ctx context.Context, userID uuid.UUID, coachingNotes []byte) (db.UpdateCoachingProfileNotesRow, error)
	UpdateCoachingProfileContextRefresh(ctx context.Context, userID uuid.UUID) (db.UpdateCoachingProfileContextRefreshRow, error)
	DeleteCoachingProfile(ctx context.Context, userID uuid.UUID) error
}

type IBilling interface {
	ListActivePlans(ctx context.Context) ([]db.Plan, error)
	GetPlanByCode(ctx context.Context, code string) (db.Plan, error)
	GetUserSubscription(ctx context.Context, userID uuid.UUID) (db.GetUserSubscriptionRow, error)
	GetOrCreateUserSubscription(ctx context.Context, userID uuid.UUID) (db.GetUserSubscriptionRow, error)
	GetUserSubscriptionByStripeCustomerID(ctx context.Context, stripeCustomerID *string) (db.GetUserSubscriptionByStripeCustomerIDRow, error)
	CreateDefaultFreeSubscription(ctx context.Context, userID uuid.UUID) (db.Subscription, error)
	UpsertUserSubscription(ctx context.Context, params db.UpsertUserSubscriptionParams) (db.Subscription, error)
	CreateUpgradeEvent(ctx context.Context, params db.CreateUpgradeEventParams) (db.CreateUpgradeEventRow, error)
	ComputeEntitlements(ctx context.Context, sub db.GetUserSubscriptionRow, userID uuid.UUID) (*EntitlementsResult, error)
	IsStripeEventProcessed(ctx context.Context, stripeEventID string) (bool, error)
	MarkStripeEventProcessed(ctx context.Context, stripeEventID string) error
	ListExpiredActiveSubscriptions(ctx context.Context, limit int32) ([]db.ListExpiredActiveSubscriptionsRow, error)
	ListSubscriptionStatuses(ctx context.Context) ([]db.ListSubscriptionStatusesRow, error)
	// RevenueCat
	GetUserSubscriptionByUserID(ctx context.Context, userID uuid.UUID) (db.GetUserSubscriptionByUserIDRow, error)
	SetRevenueCatCustomerID(ctx context.Context, userID uuid.UUID, revenuecatCustomerID *string) error
	IsRevenueCatEventProcessed(ctx context.Context, eventID string) (bool, error)
	MarkRevenueCatEventProcessed(ctx context.Context, eventID string) error
}

type IPlanAdjustmentSuggestions interface {
	CreatePlanAdjustmentSuggestion(ctx context.Context, params db.CreatePlanAdjustmentSuggestionParams) (db.PlanAdjustment, error)
	GetPlanAdjustmentSuggestion(ctx context.Context, id uuid.UUID, userID uuid.UUID) (db.PlanAdjustment, error)
	ListPendingPlanAdjustmentSuggestions(ctx context.Context, userID uuid.UUID, limit int32, offset int32) ([]db.PlanAdjustment, error)
	ListAllPlanAdjustmentSuggestions(ctx context.Context, userID uuid.UUID, limit int32, offset int32) ([]db.PlanAdjustment, error)
	ListPlanAdjustmentSuggestionsByHabit(ctx context.Context, userID uuid.UUID, habitID uuid.NullUUID, limit int32, offset int32) ([]db.PlanAdjustment, error)
	ListPlanAdjustmentSuggestionsByGoal(ctx context.Context, userID uuid.UUID, goalID uuid.NullUUID, limit int32, offset int32) ([]db.PlanAdjustment, error)
	UpdatePlanAdjustmentSuggestionStatus(ctx context.Context, id uuid.UUID, userID uuid.UUID, status string) (db.PlanAdjustment, error)
	UpdatePlanAdjustmentSuggestion(ctx context.Context, params db.UpdatePlanAdjustmentSuggestionParams) (db.PlanAdjustment, error)
	DeletePlanAdjustmentSuggestion(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	CountPendingPlanAdjustmentSuggestions(ctx context.Context, userID uuid.UUID) (int64, error)
	DismissOldPendingSuggestions(ctx context.Context, userID uuid.UUID) error
	ApplyPlanAdjustmentSuggestion(ctx context.Context, id uuid.UUID, userID uuid.UUID) (db.PlanAdjustment, error)
}

type ISiteSettings interface {
	GetSiteSetting(ctx context.Context, key string) (db.SiteSetting, error)
	ListSiteSettings(ctx context.Context, keys []string) ([]db.SiteSetting, error)
	ListAllSiteSettings(ctx context.Context) ([]db.SiteSetting, error)
	UpsertSiteSetting(ctx context.Context, key string, value []byte) (db.SiteSetting, error)
	DeleteSiteSetting(ctx context.Context, key string) error
}

type IHabitTemplates interface {
	ListHabitTemplates(ctx context.Context) ([]db.ListHabitTemplatesRow, error)
	// Admin CRUD (called by adminway via gRPC).
	AdminListHabitTemplates(ctx context.Context) ([]db.AdminListHabitTemplatesRow, error)
	AdminGetHabitTemplate(ctx context.Context, id uuid.UUID) (db.AdminGetHabitTemplateRow, error)
	AdminCreateHabitTemplate(ctx context.Context, params db.AdminCreateHabitTemplateParams) (db.HabitTemplate, error)
	AdminUpdateHabitTemplate(ctx context.Context, params db.AdminUpdateHabitTemplateParams) (db.HabitTemplate, error)
	AdminDeleteHabitTemplate(ctx context.Context, id uuid.UUID) error
}

type IGoalTemplates interface {
	ListGoalTemplates(ctx context.Context) ([]db.ListGoalTemplatesRow, error)
	// Admin CRUD (called by adminway via gRPC).
	AdminListGoalTemplates(ctx context.Context) ([]db.AdminListGoalTemplatesRow, error)
	AdminGetGoalTemplate(ctx context.Context, id uuid.UUID) (db.AdminGetGoalTemplateRow, error)
	AdminCreateGoalTemplate(ctx context.Context, params db.AdminCreateGoalTemplateParams) (db.GoalTemplate, error)
	AdminUpdateGoalTemplate(ctx context.Context, params db.AdminUpdateGoalTemplateParams) (db.GoalTemplate, error)
	AdminDeleteGoalTemplate(ctx context.Context, id uuid.UUID) error
}

type IReports interface {
	CreateReport(ctx context.Context, params db.CreateReportParams) (db.Report, error)
	GetReportByID(ctx context.Context, id uuid.UUID) (db.Report, error)
	GetReportStatus(ctx context.Context, id uuid.UUID) (db.GetReportStatusRow, error)
	ListReports(ctx context.Context, params db.ListReportsParams) ([]db.Report, error)
	CountReports(ctx context.Context, status, category string, reporterID uuid.UUID) (int64, error)
	UpdateReportStatus(ctx context.Context, id uuid.UUID, status string, adminNotes *string) (db.Report, error)
	CloseReport(ctx context.Context, id uuid.UUID, closeReason *string, adminNotes *string) (db.Report, error)
	CreateReportComment(ctx context.Context, reportID, userID uuid.UUID, comment string, isAdmin bool) (db.ReportComment, error)
	ListReportComments(ctx context.Context, reportID uuid.UUID) ([]db.ReportComment, error)
}

type Repository struct {
	Articles                  IArticles
	Tags                      ITags
	SavedItems                ISavedItems
	Activities                IActivities
	UserPreferences           IUserPreferences
	Users                     IUsers
	Habits                    IHabits
	Goals                     IGoals
	Categories                ICategories
	CheckIns                  ICheckIns
	WeeklyReviews             IWeeklyReviews
	CoachingProfiles          ICoachingProfiles
	PlanAdjustmentSuggestions IPlanAdjustmentSuggestions
	Billing                   IBilling
	SiteSettings              ISiteSettings
	HabitTemplates            IHabitTemplates
	GoalTemplates             IGoalTemplates
	Reports                   IReports
}

func NewRepository(db *db.Queries) *Repository {
	habits := NewHabitsRepo(db)
	goals := NewGoalsRepo(db)
	planAdjustments := NewPlanAdjustmentSuggestionsRepo(db)

	return &Repository{
		Articles:                  NewArticlesRepo(db),
		Tags:                      NewTagsRepo(db),
		SavedItems:                NewSavedItemsRepo(db),
		Activities:                NewActivitiesRepo(db),
		UserPreferences:           NewUserPreferencesRepo(db),
		Users:                     NewUsersRepo(db),
		Habits:                    habits,
		Goals:                     goals,
		Categories:                NewCategoriesRepo(db),
		CheckIns:                  NewCheckInsRepo(db),
		WeeklyReviews:             NewWeeklyReviewsRepo(db),
		CoachingProfiles:          NewCoachingProfilesRepo(db),
		PlanAdjustmentSuggestions: planAdjustments,
		Billing:                   NewBillingRepo(db, habits, goals, planAdjustments),
		SiteSettings:              NewSiteSettingsRepo(db),
		HabitTemplates:            NewHabitTemplatesRepo(db),
		GoalTemplates:             NewGoalTemplatesRepo(db),
		Reports:                   NewReportsRepo(db),
	}
}

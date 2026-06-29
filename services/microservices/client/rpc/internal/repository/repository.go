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
	SearchArticles(ctx context.Context, query string, status string, limit, offset int32) ([]db.SearchArticlesRow, error)
	GetArticleByID(ctx context.Context, id uuid.UUID, status string) (db.GetArticleRow, error)
	GetArticleByIDWithSaved(ctx context.Context, id uuid.UUID, userID uuid.UUID, status string) (db.GetArticleWithSavedRow, error)
	GetArticleByTitle(ctx context.Context, title string) (db.GetArticleByTitleRow, error)
	CreateArticle(ctx context.Context, params db.CreateArticleParams) (db.CreateArticleRow, error)
	UpdateArticle(ctx context.Context, params db.UpdateArticleParams) (db.UpdateArticleRow, error)
	DeleteArticle(ctx context.Context, id uuid.UUID) error
	CountArticles(ctx context.Context, status string) (int64, error)
	CountArticlesByCategorySlug(ctx context.Context, slug string, status string) (int64, error)
	CountSearchArticles(ctx context.Context, query string, status string) (int64, error)
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

type IUserSettings interface {
	GetUserSettings(ctx context.Context, userID uuid.UUID) (db.UserSetting, error)
	CreateUserSettings(ctx context.Context, params db.CreateUserSettingsParams) (db.UserSetting, error)
	UpdateUserSettings(ctx context.Context, params db.UpdateUserSettingsParams) (db.UserSetting, error)
	UpdateOnboardingSettings(ctx context.Context, userID uuid.UUID, accountabilityStyle string, checkInTime pgtype.Time, onboardingCompleted bool) (db.UserSetting, error)
	DeleteUserSettings(ctx context.Context, userID uuid.UUID) error
}

type IUsers interface {
	GetUserProfileByID(ctx context.Context, id uuid.UUID) (db.GetUserProfileByIDRow, error)
}

type IHabits interface {
	ListHabits(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.GetHabitRow, error)
	GetHabitByID(ctx context.Context, id uuid.UUID) (db.GetHabitRow, error)
	CreateHabit(ctx context.Context, name string, description *string, category string, userID uuid.UUID) (db.GetHabitRow, error)
	UpdateHabit(ctx context.Context, id uuid.UUID, name string, description *string, category string) (db.GetHabitRow, error)
	DeleteHabit(ctx context.Context, id uuid.UUID) error
	GetHabitStreak(ctx context.Context, habitID, userID uuid.UUID) (int32, error)
	GetHabitStreaks(ctx context.Context, userID uuid.UUID) ([]db.GetHabitStreaksRow, error)
	ResetTodayHabits(ctx context.Context, userID uuid.UUID) (int64, error)
	CountHabitsByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	ListHabitHistory(ctx context.Context, userID uuid.UUID) ([]db.ListHabitHistoryRow, error)
}

type IGoals interface {
	ListGoals(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.GetGoalRow, error)
	GetGoalByID(ctx context.Context, id uuid.UUID) (db.GetGoalRow, error)
	CreateGoal(ctx context.Context, params db.CreateGoalParams) (db.GetGoalRow, error)
	UpdateGoal(ctx context.Context, params db.UpdateGoalParams) (db.GetGoalRow, error)
	DeleteGoal(ctx context.Context, id uuid.UUID) error
	ToggleGoal(ctx context.Context, id uuid.UUID) (db.GetGoalRow, error)
	UpdateGoalProgress(ctx context.Context, id uuid.UUID, progress int32) (db.GetGoalRow, error)
	CountGoalsByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	ListGoalHabitIDs(ctx context.Context, userID uuid.UUID) ([]db.ListGoalHabitIDsRow, error)
	ListGoalHabitIDsByGoal(ctx context.Context, goalID uuid.UUID) ([]uuid.UUID, error)
	UnlinkAllGoalHabits(ctx context.Context, goalID uuid.UUID) error
	LinkGoalHabitsBatch(ctx context.Context, goalID uuid.UUID, habitIDs []uuid.UUID) error
}

type ICategories interface {
	ListCategories(ctx context.Context) ([]db.Category, error)
	GetCategoryByID(ctx context.Context, id uuid.UUID) (db.Category, error)
	GetCategoryBySlug(ctx context.Context, slug string) (db.Category, error)
	CreateCategory(ctx context.Context, name string, slug string, sortOrder int32) (db.Category, error)
	UpdateCategory(ctx context.Context, id uuid.UUID, name string, slug string, sortOrder int32) (db.Category, error)
	DeleteCategory(ctx context.Context, id uuid.UUID) error
	CountCategories(ctx context.Context) (int64, error)
	CountArticlesByCategory(ctx context.Context, id uuid.UUID) (int64, error)
	ReorderCategories(ctx context.Context, ids []uuid.UUID, sortOrders []int32) error
}

type ICheckIns interface {
	CreateCheckIn(ctx context.Context, params db.CreateCheckInParams) (db.CheckIn, error)
	GetTodayCheckIns(ctx context.Context, userID uuid.UUID) ([]db.CheckIn, error)
	GetCheckInsByHabit(ctx context.Context, habitID, userID uuid.UUID, limit, offset int32) ([]db.CheckIn, error)
	GetCheckInsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.CheckIn, error)
	GetCheckInHistory(ctx context.Context, userID uuid.UUID, start, end time.Time, limit, offset int32) ([]db.CheckIn, error)
	GetCheckInsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.CheckIn, error)
	HasCheckedInToday(ctx context.Context, userID, habitID uuid.UUID) (bool, error)
	CountCheckInsByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountCheckInsByHabit(ctx context.Context, habitID uuid.UUID) (int64, error)
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
	UpsertCoachingProfile(ctx context.Context, params db.UpsertCoachingProfileParams) (db.GetCoachingProfileRow, error)
	UpdateCoachingProfilePreferences(ctx context.Context, userID uuid.UUID, accountabilityStyle string, preferredTone string, difficultyPreference string) (db.GetCoachingProfileRow, error)
	UpdateCoachingProfileBlockers(ctx context.Context, userID uuid.UUID, commonBlockers []byte) (db.GetCoachingProfileRow, error)
	UpdateCoachingProfileNotes(ctx context.Context, userID uuid.UUID, coachingNotes []byte) (db.GetCoachingProfileRow, error)
	UpdateCoachingProfileContextRefresh(ctx context.Context, userID uuid.UUID) (db.GetCoachingProfileRow, error)
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
	CountActiveGoalsForUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountActiveHabitsForUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountPendingPlanAdjustmentsForUser(ctx context.Context, userID uuid.UUID) (int64, error)
	ComputeEntitlements(ctx context.Context, sub db.GetUserSubscriptionRow, userID uuid.UUID) (*EntitlementsResult, error)
	IsStripeEventProcessed(ctx context.Context, stripeEventID string) (bool, error)
	MarkStripeEventProcessed(ctx context.Context, stripeEventID string) error
	ListExpiredActiveSubscriptions(ctx context.Context, limit int32) ([]db.ListExpiredActiveSubscriptionsRow, error)
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

type Repository struct {
	Articles                  IArticles
	Tags                      ITags
	SavedItems                ISavedItems
	Activities                IActivities
	UserSettings              IUserSettings
	Users                     IUsers
	Habits                    IHabits
	Goals                     IGoals
	Categories                ICategories
	CheckIns                  ICheckIns
	WeeklyReviews             IWeeklyReviews
	CoachingProfiles          ICoachingProfiles
	PlanAdjustmentSuggestions IPlanAdjustmentSuggestions
	Billing                   IBilling
}

func NewRepository(db *db.Queries) *Repository {

	return &Repository{
		Articles:                  NewArticlesRepo(db),
		Tags:                      NewTagsRepo(db),
		SavedItems:                NewSavedItemsRepo(db),
		Activities:                NewActivitiesRepo(db),
		UserSettings:              NewUserSettingsRepo(db),
		Users:                     NewUsersRepo(db),
		Habits:                    NewHabitsRepo(db),
		Goals:                     NewGoalsRepo(db),
		Categories:                NewCategoriesRepo(db),
		CheckIns:                  NewCheckInsRepo(db),
		WeeklyReviews:             NewWeeklyReviewsRepo(db),
		CoachingProfiles:          NewCoachingProfilesRepo(db),
		PlanAdjustmentSuggestions: NewPlanAdjustmentSuggestionsRepo(db),
		Billing:                   NewBillingRepo(db),
	}
}

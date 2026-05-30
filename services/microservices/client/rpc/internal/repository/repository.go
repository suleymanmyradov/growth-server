package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
)

type IArticles interface {
	ListArticles(ctx context.Context, limit, offset int32) ([]db.ListArticlesRow, error)
	ListArticlesByCategorySlug(ctx context.Context, slug string, limit, offset int32) ([]db.ListArticlesByCategorySlugRow, error)
	ListArticlesByAuthor(ctx context.Context, author string, limit, offset int32) ([]db.ListArticlesByAuthorRow, error)
	SearchArticles(ctx context.Context, query string, limit, offset int32) ([]db.SearchArticlesRow, error)
	GetArticleByID(ctx context.Context, id uuid.UUID) (db.GetArticleRow, error)
	GetArticleByTitle(ctx context.Context, title string) (db.GetArticleByTitleRow, error)
	CreateArticle(ctx context.Context, params db.CreateArticleParams) (db.CreateArticleRow, error)
	UpdateArticle(ctx context.Context, params db.UpdateArticleParams) (db.UpdateArticleRow, error)
	DeleteArticle(ctx context.Context, id uuid.UUID) error
	CountArticles(ctx context.Context) (int64, error)
	CountArticlesByCategorySlug(ctx context.Context, slug string) (int64, error)
	CreateArticleShare(ctx context.Context, articleID uuid.UUID, userID uuid.UUID, platform string) (db.ArticleShare, error)
}

type ISavedItems interface {
	ListSavedItems(ctx context.Context, limit, offset int32) ([]db.SavedItem, error)
	ListSavedItemsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.SavedItem, error)
	ListSavedItemsByType(ctx context.Context, userID uuid.UUID, itemType string, limit, offset int32) ([]db.SavedItem, error)
	GetSavedItemByID(ctx context.Context, id uuid.UUID) (db.SavedItem, error)
	GetSavedItemByUserAndItem(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) (db.SavedItem, error)
	CreateSavedItem(ctx context.Context, itemType db.SavedItemType, itemID uuid.UUID, userID uuid.UUID) (db.SavedItem, error)
	DeleteSavedItem(ctx context.Context, id uuid.UUID) error
	DeleteSavedItemByUserAndItem(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) error
	IsItemSaved(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) (bool, error)
	CountSavedItems(ctx context.Context) (int64, error)
	CountSavedItemsByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountSavedItemsByUserAndType(ctx context.Context, userID uuid.UUID, itemType string) (int64, error)
}

type IActivities interface {
	ListActivities(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Activity, error)
	ListActivitiesByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Activity, error)
	ListActivitiesByType(ctx context.Context, userID uuid.UUID, itemType string, limit, offset int32) ([]db.Activity, error)
	GetActivityByID(ctx context.Context, id uuid.UUID) (db.Activity, error)
	CreateActivity(ctx context.Context, params db.CreateActivityParams) (db.Activity, error)
	LogActivity(ctx context.Context, params db.LogActivityParams) (db.Activity, error)
	DeleteActivity(ctx context.Context, id uuid.UUID) error
	DeleteActivitiesByUser(ctx context.Context, userID uuid.UUID) error
	CountActivities(ctx context.Context) (int64, error)
	CountActivitiesByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountActivitiesByUserAndType(ctx context.Context, userID uuid.UUID, itemType string) (int64, error)
	GetActivityFeed(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Activity, error)
	GetActivityStats(ctx context.Context, userID uuid.UUID) (db.GetActivityStatsRow, error)
	GetStreaks(ctx context.Context, userID uuid.UUID) (db.GetStreaksRow, error)
	GetAchievements(ctx context.Context, userID uuid.UUID) ([]db.GetAchievementsRow, error)
	GetActivityCalendar(ctx context.Context, userID uuid.UUID, year, month int32) ([]db.GetActivityCalendarRow, error)
}

type IUserSettings interface {
	GetUserSettings(ctx context.Context, userID uuid.UUID) (db.GetUserSettingsRow, error)
	GetUserSettingsByID(ctx context.Context, id uuid.UUID) (db.GetUserSettingsByIDRow, error)
	CreateUserSettings(ctx context.Context, params db.CreateUserSettingsParams) (db.CreateUserSettingsRow, error)
	UpdateUserSettings(ctx context.Context, params db.UpdateUserSettingsParams) (db.UpdateUserSettingsRow, error)
	UpdateOnboardingSettings(ctx context.Context, userID uuid.UUID, accountabilityStyle db.AccountabilityStyleType, checkInTime pgtype.Time, onboardingCompleted bool) (db.UpdateOnboardingSettingsRow, error)
	DeleteUserSettings(ctx context.Context, userID uuid.UUID) error
	CountUserSettings(ctx context.Context) (int64, error)
}

type IHabits interface {
	ListHabits(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Habit, error)
	GetHabitByID(ctx context.Context, id uuid.UUID) (db.Habit, error)
	CreateHabit(ctx context.Context, name string, description *string, category string, userID uuid.UUID) (db.Habit, error)
	UpdateHabit(ctx context.Context, id uuid.UUID, name string, description *string, category string) (db.Habit, error)
	DeleteHabit(ctx context.Context, id uuid.UUID) error
	ToggleHabit(ctx context.Context, id uuid.UUID) (db.Habit, error)
	UpdateHabitStreak(ctx context.Context, id uuid.UUID, streak int32) (db.Habit, error)
	MarkHabitCompleted(ctx context.Context, id uuid.UUID) (db.Habit, error)
	ResetTodayHabits(ctx context.Context, userID uuid.UUID) (int64, error)
	CountHabitsByUser(ctx context.Context, userID uuid.UUID) (int64, error)
}

type IGoals interface {
	ListGoals(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Goal, error)
	GetGoalByID(ctx context.Context, id uuid.UUID) (db.Goal, error)
	CreateGoal(ctx context.Context, params db.CreateGoalParams) (db.Goal, error)
	UpdateGoal(ctx context.Context, params db.UpdateGoalParams) (db.Goal, error)
	DeleteGoal(ctx context.Context, id uuid.UUID) error
	ToggleGoal(ctx context.Context, id uuid.UUID) (db.Goal, error)
	UpdateGoalProgress(ctx context.Context, id uuid.UUID, progress int32) (db.Goal, error)
	CountGoalsByUser(ctx context.Context, userID uuid.UUID) (int64, error)
}

type ICategories interface {
	ListCategories(ctx context.Context, entityType db.EntityType) ([]db.Category, error)
	ListAllCategories(ctx context.Context) ([]db.Category, error)
	GetCategoryByID(ctx context.Context, id uuid.UUID) (db.Category, error)
	GetCategoryBySlug(ctx context.Context, slug string, entityType db.EntityType) (db.Category, error)
	CreateCategory(ctx context.Context, name string, slug string, entityType db.EntityType, sortOrder int32) (db.Category, error)
	UpdateCategory(ctx context.Context, id uuid.UUID, name string, slug string, sortOrder int32) (db.Category, error)
	DeleteCategory(ctx context.Context, id uuid.UUID) error
	CountCategoriesByType(ctx context.Context, entityType db.EntityType) (int64, error)
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
	CreateWeeklyReview(ctx context.Context, params db.CreateWeeklyReviewParams) (db.WeeklyReview, error)
	GetWeeklyReview(ctx context.Context, userID uuid.UUID, weekStart time.Time) (db.WeeklyReview, error)
	GetCurrentWeeklyReview(ctx context.Context, userID uuid.UUID) (db.WeeklyReview, error)
	ListWeeklyReviews(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.WeeklyReview, error)
	CountWeeklyReviews(ctx context.Context, userID uuid.UUID) (int64, error)
	GetCheckInStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetCheckInStatsForWeekRow, error)
	GetDailyCheckInStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetDailyCheckInStatsForWeekRow, error)
	GetBlockerStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetBlockerStatsForWeekRow, error)
	GetMoodStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetMoodStatsForWeekRow, error)
	GetEnergyStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetEnergyStatsForWeekRow, error)
}

type ICoachingProfiles interface {
	GetCoachingProfile(ctx context.Context, userID uuid.UUID) (db.UserCoachingProfile, error)
	UpsertCoachingProfile(ctx context.Context, params db.UpsertCoachingProfileParams) (db.UserCoachingProfile, error)
	UpdateCoachingProfilePreferences(ctx context.Context, userID uuid.UUID, accountabilityStyle db.AccountabilityStyleType, preferredTone db.CoachToneType, difficultyPreference db.DifficultyLevelType) (db.UserCoachingProfile, error)
	UpdateCoachingProfileBlockers(ctx context.Context, userID uuid.UUID, commonBlockers []byte) (db.UserCoachingProfile, error)
	UpdateCoachingProfileNotes(ctx context.Context, userID uuid.UUID, coachingNotes []byte) (db.UserCoachingProfile, error)
	UpdateCoachingProfileContextRefresh(ctx context.Context, userID uuid.UUID) (db.UserCoachingProfile, error)
	DeleteCoachingProfile(ctx context.Context, userID uuid.UUID) error
}

type IBilling interface {
	ListActivePlans(ctx context.Context) ([]db.Plan, error)
	GetPlanByCode(ctx context.Context, code string) (db.Plan, error)
	GetUserSubscription(ctx context.Context, userID uuid.UUID) (db.GetUserSubscriptionRow, error)
	GetOrCreateUserSubscription(ctx context.Context, userID uuid.UUID) (db.GetUserSubscriptionRow, error)
	GetUserSubscriptionByStripeCustomerID(ctx context.Context, stripeCustomerID *string) (db.GetUserSubscriptionByStripeCustomerIDRow, error)
	CreateDefaultFreeSubscription(ctx context.Context, userID uuid.UUID) (db.UserSubscription, error)
	UpsertUserSubscription(ctx context.Context, params db.UpsertUserSubscriptionParams) (db.UserSubscription, error)
	CreateUpgradeEvent(ctx context.Context, params db.CreateUpgradeEventParams) (db.CreateUpgradeEventRow, error)
	CountActiveGoalsForUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountActiveHabitsForUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountPendingPlanAdjustmentsForUser(ctx context.Context, userID uuid.UUID) (int64, error)
	ComputeEntitlements(ctx context.Context, sub db.GetUserSubscriptionRow, userID uuid.UUID) (*EntitlementsResult, error)
}

type IPlanAdjustmentSuggestions interface {
	CreatePlanAdjustmentSuggestion(ctx context.Context, params db.CreatePlanAdjustmentSuggestionParams) (db.PlanAdjustmentSuggestion, error)
	GetPlanAdjustmentSuggestion(ctx context.Context, id uuid.UUID, userID uuid.UUID) (db.PlanAdjustmentSuggestion, error)
	ListPendingPlanAdjustmentSuggestions(ctx context.Context, userID uuid.UUID, limit int32, offset int32) ([]db.PlanAdjustmentSuggestion, error)
	ListAllPlanAdjustmentSuggestions(ctx context.Context, userID uuid.UUID, limit int32, offset int32) ([]db.PlanAdjustmentSuggestion, error)
	ListPlanAdjustmentSuggestionsByHabit(ctx context.Context, userID uuid.UUID, habitID uuid.NullUUID, limit int32, offset int32) ([]db.PlanAdjustmentSuggestion, error)
	ListPlanAdjustmentSuggestionsByGoal(ctx context.Context, userID uuid.UUID, goalID uuid.NullUUID, limit int32, offset int32) ([]db.PlanAdjustmentSuggestion, error)
	UpdatePlanAdjustmentSuggestionStatus(ctx context.Context, id uuid.UUID, userID uuid.UUID, status db.PlanAdjustmentStatusType) (db.PlanAdjustmentSuggestion, error)
	UpdatePlanAdjustmentSuggestion(ctx context.Context, params db.UpdatePlanAdjustmentSuggestionParams) (db.PlanAdjustmentSuggestion, error)
	DeletePlanAdjustmentSuggestion(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	CountPendingPlanAdjustmentSuggestions(ctx context.Context, userID uuid.UUID) (int64, error)
	DismissOldPendingSuggestions(ctx context.Context, userID uuid.UUID) error
	ApplyPlanAdjustmentSuggestion(ctx context.Context, id uuid.UUID, userID uuid.UUID) (db.PlanAdjustmentSuggestion, error)
}

type Repository struct {
	Articles                  IArticles
	SavedItems                ISavedItems
	Activities                IActivities
	UserSettings              IUserSettings
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
		SavedItems:                NewSavedItemsRepo(db),
		Activities:                NewActivitiesRepo(db),
		UserSettings:              NewUserSettingsRepo(db),
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

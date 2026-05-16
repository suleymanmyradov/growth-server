package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
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
	CreateArticleShare(ctx context.Context, params db.CreateArticleShareParams) (db.ArticleShare, error)
}

type ISavedItems interface {
	ListSavedItems(ctx context.Context, limit, offset int32) ([]db.SavedItem, error)
	ListSavedItemsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.SavedItem, error)
	ListSavedItemsByType(ctx context.Context, userID uuid.UUID, itemType string, limit, offset int32) ([]db.SavedItem, error)
	GetSavedItemByID(ctx context.Context, id uuid.UUID) (db.SavedItem, error)
	GetSavedItemByUserAndItem(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) (db.SavedItem, error)
	CreateSavedItem(ctx context.Context, params db.CreateSavedItemParams) (db.SavedItem, error)
	DeleteSavedItem(ctx context.Context, id uuid.UUID) error
	DeleteSavedItemByUserAndItem(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) error
	IsItemSaved(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) (bool, error)
	CountSavedItems(ctx context.Context) (int64, error)
	CountSavedItemsByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountSavedItemsByUserAndType(ctx context.Context, userID uuid.UUID, itemType string) (int64, error)
}

type IActivities interface {
	ListActivities(ctx context.Context, limit, offset int32) ([]db.Activity, error)
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
	UpdateOnboardingSettings(ctx context.Context, params db.UpdateOnboardingSettingsParams) (db.UpdateOnboardingSettingsRow, error)
	DeleteUserSettings(ctx context.Context, userID uuid.UUID) error
	CountUserSettings(ctx context.Context) (int64, error)
}

type IHabits interface {
	ListHabits(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Habit, error)
	GetHabitByID(ctx context.Context, id uuid.UUID) (db.Habit, error)
	CreateHabit(ctx context.Context, params db.CreateHabitParams) (db.Habit, error)
	UpdateHabit(ctx context.Context, params db.UpdateHabitParams) (db.Habit, error)
	DeleteHabit(ctx context.Context, id uuid.UUID) error
	ToggleHabit(ctx context.Context, id uuid.UUID) (db.Habit, error)
	UpdateHabitStreak(ctx context.Context, id uuid.UUID, streak int32) (db.Habit, error)
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
	UpdateGoalProgress(ctx context.Context, params db.UpdateGoalProgressParams) (db.Goal, error)
	CountGoalsByUser(ctx context.Context, userID uuid.UUID) (int64, error)
}

type ICategories interface {
	ListCategories(ctx context.Context, entityType db.EntityType) ([]db.Category, error)
	ListAllCategories(ctx context.Context) ([]db.Category, error)
	GetCategoryByID(ctx context.Context, id uuid.UUID) (db.Category, error)
	GetCategoryBySlug(ctx context.Context, slug string, entityType db.EntityType) (db.Category, error)
	CreateCategory(ctx context.Context, params db.CreateCategoryParams) (db.Category, error)
	UpdateCategory(ctx context.Context, params db.UpdateCategoryParams) (db.Category, error)
	DeleteCategory(ctx context.Context, id uuid.UUID) error
	CountCategoriesByType(ctx context.Context, entityType db.EntityType) (int64, error)
}

type ICheckIns interface {
	CreateCheckIn(ctx context.Context, params db.CreateCheckInParams) (db.CheckIn, error)
	GetTodayCheckIns(ctx context.Context, userID uuid.UUID) ([]db.CheckIn, error)
	GetCheckInsByHabit(ctx context.Context, habitID uuid.UUID, limit, offset int32) ([]db.CheckIn, error)
	GetCheckInsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.CheckIn, error)
	GetCheckInsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.CheckIn, error)
	HasCheckedInToday(ctx context.Context, userID, habitID uuid.UUID) (bool, error)
	CountCheckInsByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountCheckInsByHabit(ctx context.Context, habitID uuid.UUID) (int64, error)
}

type Repository struct {
	Articles     IArticles
	SavedItems   ISavedItems
	Activities   IActivities
	UserSettings IUserSettings
	Habits       IHabits
	Goals        IGoals
	Categories   ICategories
	CheckIns     ICheckIns
}

func NewRepository(db *db.Queries) *Repository {
	return &Repository{
		Articles:     NewArticlesRepo(db),
		SavedItems:   NewSavedItemsRepo(db),
		Activities:   NewActivitiesRepo(db),
		UserSettings: NewUserSettingsRepo(db),
		Habits:       NewHabitsRepo(db),
		Goals:        NewGoalsRepo(db),
		Categories:   NewCategoriesRepo(db),
		CheckIns:     NewCheckInsRepo(db),
	}
}

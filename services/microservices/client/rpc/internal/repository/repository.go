package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
)

// IArticles defines the interface for article repository operations
type IArticles interface {
	ListArticles(ctx context.Context, limit, offset int32) ([]db.ListArticlesRow, error)
	ListArticlesByCategory(ctx context.Context, category string, limit, offset int32) ([]db.ListArticlesByCategoryRow, error)
	ListArticlesByAuthor(ctx context.Context, author string, limit, offset int32) ([]db.ListArticlesByAuthorRow, error)
	SearchArticles(ctx context.Context, query string, limit, offset int32) ([]db.SearchArticlesRow, error)
	GetArticle(ctx context.Context, id uuid.UUID) (db.GetArticleRow, error)
	GetArticleByTitle(ctx context.Context, title string) (db.GetArticleByTitleRow, error)
	CreateArticle(ctx context.Context, params db.CreateArticleParams) (db.CreateArticleRow, error)
	UpdateArticle(ctx context.Context, params db.UpdateArticleParams) (db.UpdateArticleRow, error)
	DeleteArticle(ctx context.Context, id uuid.UUID) error
	CountArticles(ctx context.Context) (int64, error)
	CountArticlesByCategory(ctx context.Context, category string) (int64, error)
	CreateArticleShare(ctx context.Context, params db.CreateArticleShareParams) (db.ArticleShare, error)
}

// ISavedItems defines the interface for saved items repository operations
type ISavedItems interface {
	ListSavedItems(ctx context.Context, limit, offset int32) ([]db.SavedItem, error)
	ListSavedItemsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.SavedItem, error)
	ListSavedItemsByType(ctx context.Context, userID uuid.UUID, itemType string, limit, offset int32) ([]db.SavedItem, error)
	GetSavedItem(ctx context.Context, id uuid.UUID) (db.SavedItem, error)
	GetSavedItemByUserAndItem(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) (db.SavedItem, error)
	CreateSavedItem(ctx context.Context, params db.CreateSavedItemParams) (db.SavedItem, error)
	DeleteSavedItem(ctx context.Context, id uuid.UUID) error
	DeleteSavedItemByUserAndItem(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) error
	IsItemSaved(ctx context.Context, userID uuid.UUID, itemType string, itemID uuid.UUID) (bool, error)
	CountSavedItems(ctx context.Context) (int64, error)
	CountSavedItemsByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountSavedItemsByUserAndType(ctx context.Context, userID uuid.UUID, itemType string) (int64, error)
}

// INotifications defines the interface for notifications repository operations
type INotifications interface {
	ListNotifications(ctx context.Context, limit, offset int32) ([]db.Notification, error)
	ListNotificationsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Notification, error)
	ListUnreadNotifications(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Notification, error)
	ListNotificationsByType(ctx context.Context, userID uuid.UUID, itemType string, limit, offset int32) ([]db.Notification, error)
	ListNotificationsForUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Notification, error)
	GetNotification(ctx context.Context, id uuid.UUID) (db.Notification, error)
	CreateNotification(ctx context.Context, params db.CreateNotificationParams) (db.Notification, error)
	MarkNotificationRead(ctx context.Context, id uuid.UUID) (db.Notification, error)
	MarkAllNotificationsRead(ctx context.Context, userID uuid.UUID) error
	DeleteNotification(ctx context.Context, id uuid.UUID) error
	DeleteAllNotificationsByUser(ctx context.Context, userID uuid.UUID) error
	CountNotifications(ctx context.Context) (int64, error)
	CountNotificationsByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountUnreadNotifications(ctx context.Context, userID uuid.UUID) (int64, error)
	GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, error)
}

// IActivities defines the interface for activities repository operations
type IActivities interface {
	ListActivities(ctx context.Context, limit, offset int32) ([]db.Activity, error)
	ListActivitiesByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Activity, error)
	ListActivitiesByType(ctx context.Context, userID uuid.UUID, itemType string, limit, offset int32) ([]db.Activity, error)
	GetActivity(ctx context.Context, id uuid.UUID) (db.Activity, error)
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

// IUserSettings defines the interface for user settings repository operations
type IUserSettings interface {
	GetUserSettings(ctx context.Context, userID uuid.UUID) (db.UserSetting, error)
	GetUserSettingsByID(ctx context.Context, id uuid.UUID) (db.UserSetting, error)
	CreateUserSettings(ctx context.Context, params db.CreateUserSettingsParams) (db.UserSetting, error)
	UpdateUserSettings(ctx context.Context, params db.UpdateUserSettingsParams) (db.UserSetting, error)
	DeleteUserSettings(ctx context.Context, userID uuid.UUID) error
	CountUserSettings(ctx context.Context) (int64, error)
}

// Repository is the main repository struct that holds all repository implementations
type Repository struct {
	Articles      IArticles
	SavedItems    ISavedItems
	Notifications INotifications
	Activities    IActivities
	UserSettings  IUserSettings
}

// NewRepository creates a new Repository instance with all repository implementations
func NewRepository(db *db.Queries) *Repository {
	return &Repository{
		Articles:      NewArticlesRepo(db),
		SavedItems:    NewSavedItemsRepo(db),
		Notifications: NewNotificationsRepo(db),
		Activities:    NewActivitiesRepo(db),
		UserSettings:  NewUserSettingsRepo(db),
	}
}

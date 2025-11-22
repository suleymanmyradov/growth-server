package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Base model with common fields
type BaseModel struct {
	ID        uuid.UUID `db:"id" json:"id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// User model
type User struct {
	BaseModel
	Username     string `db:"username" json:"username"`
	Email        string `db:"email" json:"email"`
	PasswordHash string `db:"password_hash" json:"-"`
	FullName     string `db:"full_name" json:"full_name"`
}

// Profile model
type Profile struct {
	BaseModel
	UserID    uuid.UUID   `db:"user_id" json:"user_id"`
	Bio       *string     `db:"bio" json:"bio"`
	Location  *string     `db:"location" json:"location"`
	Website   *string     `db:"website" json:"website"`
	Interests StringArray `db:"interests" json:"interests"`
	AvatarURL *string     `db:"avatar_url" json:"avatar_url"`
}

// Habit model
type Habit struct {
	BaseModel
	Name        string    `db:"name" json:"name"`
	Description *string   `db:"description" json:"description"`
	Streak      int       `db:"streak" json:"streak"`
	Completed   bool      `db:"completed" json:"completed"`
	Category    string    `db:"category" json:"category"`
	UserID      uuid.UUID `db:"user_id" json:"user_id"`
}

// Goal model
type Goal struct {
	BaseModel
	Title       string     `db:"title" json:"title"`
	Description *string    `db:"description" json:"description"`
	Category    string     `db:"category" json:"category"`
	DueDate     *time.Time `db:"due_date" json:"due_date"`
	Progress    int        `db:"progress" json:"progress"`
	Completed   bool       `db:"completed" json:"completed"`
	UserID      uuid.UUID  `db:"user_id" json:"user_id"`
}

// Article model
type Article struct {
	BaseModel
	Title       string   `db:"title" json:"title"`
	Content     string   `db:"content" json:"content"`
	Summary     string   `db:"summary" json:"summary"`
	Author      string   `db:"author" json:"author"`
	Tags        []string `db:"tags" json:"tags"`
	Published   bool     `db:"published" json:"published"`
	ViewCount   int      `db:"view_count" json:"view_count"`
}

// Conversation model
type Conversation struct {
	BaseModel
	UserID      uuid.UUID `db:"user_id" json:"user_id"`
	Title       string    `db:"title" json:"title"`
	IsAI        bool      `db:"is_ai" json:"is_ai"`
	LastMessage *string   `db:"last_message" json:"last_message"`
}

// Message model
type Message struct {
	BaseModel
	ConversationID uuid.UUID `db:"conversation_id" json:"conversation_id"`
	Content        string    `db:"content" json:"content"`
	IsFromAI       bool      `db:"is_from_ai" json:"is_from_ai"`
}

// Notification model
type Notification struct {
	BaseModel
	UserID  uuid.UUID `db:"user_id" json:"user_id"`
	Title   string    `db:"title" json:"title"`
	Message string    `db:"message" json:"message"`
	Read    bool      `db:"read" json:"read"`
	Type    string    `db:"type" json:"type"`
}

// Activity model
type Activity struct {
	BaseModel
	UserID      uuid.UUID `db:"user_id" json:"user_id"`
	Action      string    `db:"action" json:"action"`
	TargetType  string    `db:"target_type" json:"target_type"`
	TargetID    uuid.UUID `db:"target_id" json:"target_id"`
	Description string    `db:"description" json:"description"`
}

// UserSettings model
type UserSettings struct {
	BaseModel
	UserID                 uuid.UUID `db:"user_id" json:"user_id"`
	EmailNotifications     bool      `db:"email_notifications" json:"email_notifications"`
	PushNotifications      bool      `db:"push_notifications" json:"push_notifications"`
	DailyReminder          bool      `db:"daily_reminder" json:"daily_reminder"`
	PrivacyProfile         bool      `db:"privacy_profile" json:"privacy_profile"`
	PrivacyActivity        bool      `db:"privacy_activity" json:"privacy_activity"`
	Theme                  string    `db:"theme" json:"theme"`
	Language               string    `db:"language" json:"language"`
	Timezone               string    `db:"timezone" json:"timezone"`
}

// SavedItem model
type SavedItem struct {
	BaseModel
	UserID      uuid.UUID `db:"user_id" json:"user_id"`
	ItemType    string    `db:"item_type" json:"item_type"`
	ItemID      uuid.UUID `db:"item_id" json:"item_id"`
	Title       string    `db:"title" json:"title"`
	Description *string   `db:"description" json:"description"`
}

// StringArray is a custom type for handling PostgreSQL string arrays
type StringArray []string

func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = StringArray{}
		return nil
	}
	
	switch v := value.(type) {
	case []byte:
		return a.scanBytes(v)
	case string:
		return a.scanBytes([]byte(v))
	default:
		*a = StringArray{}
		return nil
	}
}

func (a *StringArray) scanBytes(src []byte) error {
	var arr []string
	if len(src) > 0 {
		err := json.Unmarshal(src, &arr)
		if err != nil {
			*a = StringArray{}
			return err
		}
	}
	*a = StringArray(arr)
	return nil
}

func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	return json.Marshal([]string(a))
}

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/suleymanmyradov/growth-server/shared/models"
)

// BaseRepository provides common database operations
type BaseRepository struct {
	db *sqlx.DB
}

func NewBaseRepository(db *sqlx.DB) *BaseRepository {
	return &BaseRepository{db: db}
}

// Create creates a new record
func (r *BaseRepository) Create(ctx context.Context, query string, args interface{}) error {
	_, err := r.db.NamedExecContext(ctx, query, args)
	if err != nil {
		logx.Errorf("Failed to create record: %v", err)
		return fmt.Errorf("failed to create record: %w", err)
	}
	return nil
}

// GetByID retrieves a record by ID
func (r *BaseRepository) GetByID(ctx context.Context, dest interface{}, query string, id interface{}) error {
	err := r.db.GetContext(ctx, dest, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("record not found")
		}
		logx.Errorf("Failed to get record by ID: %v", err)
		return fmt.Errorf("failed to get record by ID: %w", err)
	}
	return nil
}

// Update updates a record
func (r *BaseRepository) Update(ctx context.Context, query string, args interface{}) error {
	_, err := r.db.NamedExecContext(ctx, query, args)
	if err != nil {
		logx.Errorf("Failed to update record: %v", err)
		return fmt.Errorf("failed to update record: %w", err)
	}
	return nil
}

// Delete deletes a record by ID
func (r *BaseRepository) Delete(ctx context.Context, query string, id interface{}) error {
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		logx.Errorf("Failed to delete record: %v", err)
		return fmt.Errorf("failed to delete record: %w", err)
	}
	return nil
}

// List retrieves multiple records
func (r *BaseRepository) List(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	err := r.db.SelectContext(ctx, dest, query, args...)
	if err != nil {
		logx.Errorf("Failed to list records: %v", err)
		return fmt.Errorf("failed to list records: %w", err)
	}
	return nil
}

// Transaction executes a function within a database transaction
func (r *BaseRepository) Transaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		logx.Errorf("Failed to begin transaction: %v", err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	err = fn(tx)
	return err
}

// Common queries
const (
	InsertUserQuery = `
		INSERT INTO users (id, username, email, password_hash, full_name, created_at, updated_at)
		VALUES (:id, :username, :email, :password_hash, :full_name, :created_at, :updated_at)`

	SelectUserByIDQuery = `
		SELECT id, username, email, password_hash, full_name, created_at, updated_at
		FROM users WHERE id = $1`

	SelectUserByEmailQuery = `
		SELECT id, username, email, password_hash, full_name, created_at, updated_at
		FROM users WHERE email = $1`

	UpdateUserQuery = `
		UPDATE users 
		SET username = :username, email = :email, full_name = :full_name, updated_at = :updated_at
		WHERE id = :id`

	DeleteUserQuery = `DELETE FROM users WHERE id = $1`

	InsertProfileQuery = `
		INSERT INTO profiles (id, user_id, bio, location, website, interests, avatar_url, created_at, updated_at)
		VALUES (:id, :user_id, :bio, :location, :website, :interests, :avatar_url, :created_at, :updated_at)`

	SelectProfileByUserIDQuery = `
		SELECT id, user_id, bio, location, website, interests, avatar_url, created_at, updated_at
		FROM profiles WHERE user_id = $1`

	UpdateProfileQuery = `
		UPDATE profiles 
		SET bio = :bio, location = :location, website = :website, interests = :interests, avatar_url = :avatar_url, updated_at = :updated_at
		WHERE user_id = :user_id`
)

// Helper function to set timestamps
func SetCreateTimestamps(model interface{}) {
	now := time.Now()
	switch m := model.(type) {
	case *models.User:
		m.CreatedAt = now
		m.UpdatedAt = now
	case *models.Profile:
		m.CreatedAt = now
		m.UpdatedAt = now
	case *models.Habit:
		m.CreatedAt = now
		m.UpdatedAt = now
	case *models.Goal:
		m.CreatedAt = now
		m.UpdatedAt = now
	case *models.Article:
		m.CreatedAt = now
		m.UpdatedAt = now
	case *models.Conversation:
		m.CreatedAt = now
		m.UpdatedAt = now
	case *models.Message:
		m.CreatedAt = now
		m.UpdatedAt = now
	case *models.Notification:
		m.CreatedAt = now
		m.UpdatedAt = now
	case *models.Activity:
		m.CreatedAt = now
		m.UpdatedAt = now
	case *models.UserSettings:
		m.CreatedAt = now
		m.UpdatedAt = now
	case *models.SavedItem:
		m.CreatedAt = now
		m.UpdatedAt = now
	}
}

func SetUpdateTimestamp(model interface{}) {
	now := time.Now()
	switch m := model.(type) {
	case *models.User:
		m.UpdatedAt = now
	case *models.Profile:
		m.UpdatedAt = now
	case *models.Habit:
		m.UpdatedAt = now
	case *models.Goal:
		m.UpdatedAt = now
	case *models.Article:
		m.UpdatedAt = now
	case *models.Conversation:
		m.UpdatedAt = now
	case *models.Message:
		m.UpdatedAt = now
	case *models.Notification:
		m.UpdatedAt = now
	case *models.Activity:
		m.UpdatedAt = now
	case *models.UserSettings:
		m.UpdatedAt = now
	case *models.SavedItem:
		m.UpdatedAt = now
	}
}

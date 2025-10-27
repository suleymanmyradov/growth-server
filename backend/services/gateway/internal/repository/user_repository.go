package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/suleymanmyradov/growth-server/backend/services/gateway/internal/model"
)

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

// CreateUser creates a new user in the database
func (r *UserRepository) CreateUser(user *model.User) error {
	query := `
		INSERT INTO users (id, username, email, password_hash, full_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`

	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}

	err := r.db.QueryRow(query, user.ID, user.Username, user.Email, user.PasswordHash, user.FullName, user.CreatedAt, user.UpdatedAt).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		logx.Errorf("Failed to create user: %v", err)
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUserByEmail retrieves a user by email
func (r *UserRepository) GetUserByEmail(email string) (*model.User, error) {
	query := `
		SELECT id, username, email, password_hash, full_name, created_at, updated_at
		FROM users
		WHERE email = $1`

	var user model.User
	err := r.db.Get(&user, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		logx.Errorf("Failed to get user by email: %v", err)
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (r *UserRepository) GetUserByUsername(username string) (*model.User, error) {
	query := `
		SELECT id, username, email, password_hash, full_name, created_at, updated_at
		FROM users
		WHERE username = $1`

	var user model.User
	err := r.db.Get(&user, query, username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		logx.Errorf("Failed to get user by username: %v", err)
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID
func (r *UserRepository) GetUserByID(id uuid.UUID) (*model.User, error) {
	query := `
		SELECT id, username, email, password_hash, full_name, created_at, updated_at
		FROM users
		WHERE id = $1`

	var user model.User
	err := r.db.Get(&user, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		logx.Errorf("Failed to get user by ID: %v", err)
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return &user, nil
}

// CheckEmailExists checks if an email already exists
func (r *UserRepository) CheckEmailExists(email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`
	
	var exists bool
	err := r.db.Get(&exists, query, email)
	if err != nil {
		logx.Errorf("Failed to check email existence: %v", err)
		return false, fmt.Errorf("failed to check email existence: %w", err)
	}

	return exists, nil
}

// CheckUsernameExists checks if a username already exists
func (r *UserRepository) CheckUsernameExists(username string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`
	
	var exists bool
	err := r.db.Get(&exists, query, username)
	if err != nil {
		logx.Errorf("Failed to check username existence: %v", err)
		return false, fmt.Errorf("failed to check username existence: %w", err)
	}

	return exists, nil
}

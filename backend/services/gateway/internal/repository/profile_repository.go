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

type ProfileRepository struct {
	db *sqlx.DB
}

func NewProfileRepository(db *sqlx.DB) *ProfileRepository {
	return &ProfileRepository{db: db}
}

// CreateProfile creates a new profile for a user
func (r *ProfileRepository) CreateProfile(profile *model.Profile) error {
	query := `
		INSERT INTO profiles (id, user_id, bio, location, website, interests, avatar_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at`

	now := time.Now()
	profile.CreatedAt = now
	profile.UpdatedAt = now

	if profile.ID == uuid.Nil {
		profile.ID = uuid.New()
	}

	err := r.db.QueryRow(query, profile.ID, profile.UserID, profile.Bio, profile.Location, profile.Website, profile.Interests, profile.AvatarURL, profile.CreatedAt, profile.UpdatedAt).Scan(&profile.ID, &profile.CreatedAt, &profile.UpdatedAt)
	if err != nil {
		logx.Errorf("Failed to create profile: %v", err)
		return fmt.Errorf("failed to create profile: %w", err)
	}

	return nil
}

// GetProfileByUserID retrieves a profile by user ID
func (r *ProfileRepository) GetProfileByUserID(userID uuid.UUID) (*model.Profile, error) {
	query := `
		SELECT id, user_id, bio, location, website, interests, avatar_url, created_at, updated_at
		FROM profiles
		WHERE user_id = $1`

	var profile model.Profile
	err := r.db.Get(&profile, query, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("profile not found")
		}
		logx.Errorf("Failed to get profile by user ID: %v", err)
		return nil, fmt.Errorf("failed to get profile by user ID: %w", err)
	}

	return &profile, nil
}

// UpdateProfile updates an existing profile
func (r *ProfileRepository) UpdateProfile(profile *model.Profile) error {
	query := `
		UPDATE profiles
		SET bio = $2, location = $3, website = $4, interests = $5, avatar_url = $6, updated_at = $7
		WHERE user_id = $1
		RETURNING updated_at`

	profile.UpdatedAt = time.Now()

	err := r.db.QueryRow(query, profile.UserID, profile.Bio, profile.Location, profile.Website, profile.Interests, profile.AvatarURL, profile.UpdatedAt).Scan(&profile.UpdatedAt)
	if err != nil {
		logx.Errorf("Failed to update profile: %v", err)
		return fmt.Errorf("failed to update profile: %w", err)
	}

	return nil
}

// EnsureProfileExists creates a profile if it doesn't exist
func (r *ProfileRepository) EnsureProfileExists(userID uuid.UUID) error {
	// Check if profile exists
	_, err := r.GetProfileByUserID(userID)
	if err == nil {
		return nil // Profile already exists
	}

	// Create new profile with default values
	profile := &model.Profile{
		UserID:    userID,
		Interests: model.StringArray{},
	}

	return r.CreateProfile(profile)
}

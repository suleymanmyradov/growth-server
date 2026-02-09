package repository

import (
    "context"

    "github.com/google/uuid"
    "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
)

// IUsers defines the interface for user repository operations
type IUsers interface {
    CreateUser(ctx context.Context, params db.CreateUserParams) (db.User, error)
    GetUserByEmail(ctx context.Context, email string) (db.User, error)
    GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
    GetUserByUsername(ctx context.Context, username string) (db.User, error)
    UpdateUserPassword(ctx context.Context, params db.UpdateUserPasswordParams) (db.User, error)
    UpdateUserFullName(ctx context.Context, params db.UpdateUserFullNameParams) (db.User, error)
}

// IProfiles defines the interface for profile repository operations
type IProfiles interface {
    CreateProfile(ctx context.Context, params db.CreateProfileParams) (db.Profile, error)
    GetProfileByUserID(ctx context.Context, userID uuid.UUID) (db.Profile, error)
    UpdateProfile(ctx context.Context, params db.UpdateProfileParams) (db.Profile, error)
}

// Repository aggregates all auth repositories
type Repository struct {
    Users    IUsers
    Profiles IProfiles
}

// NewRepository creates a new Repository instance with all implementations
func NewRepository(dbq *db.Queries) *Repository {
    return &Repository{
        Users:    NewUsersRepo(dbq),
        Profiles: NewProfilesRepo(dbq),
    }
}

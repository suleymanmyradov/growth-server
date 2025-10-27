package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           uuid.UUID `db:"id"`
	Username     string    `db:"username"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	FullName     string    `db:"full_name"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

type Profile struct {
	ID        uuid.UUID   `db:"id"`
	UserID    uuid.UUID   `db:"user_id"`
	Bio       *string     `db:"bio"`
	Location  *string     `db:"location"`
	Website   *string     `db:"website"`
	Interests StringArray `db:"interests"`
	AvatarURL *string     `db:"avatar_url"`
	CreatedAt time.Time   `db:"created_at"`
	UpdatedAt time.Time   `db:"updated_at"`
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

// HashPassword hashes the password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash compares a password with its hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

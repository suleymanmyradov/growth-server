package validator

import (
	"regexp"
	"time"
	"unicode"

	"github.com/google/uuid"
)

var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	usernameRegex = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
)

// IsValidEmail checks if the provided email string matches a valid email format.
func IsValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

// IsValidUsername checks if the username matches the users.username_format
// CHECK constraint: lowercase, starts with a letter, only [a-z0-9_-].
// Callers should also enforce length bounds (DB column is varchar(50)).
func IsValidUsername(username string) bool {
	return usernameRegex.MatchString(username)
}

// IsStrongPassword checks if the password meets strength requirements:
// - At least 8 characters
// - Contains at least one uppercase letter
// - Contains at least one lowercase letter
// - Contains at least one number
// - Contains at least one special character
func IsStrongPassword(password string) bool {
	if len(password) < 8 {
		return false
	}
	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}
	return hasUpper && hasLower && hasNumber && hasSpecial
}

// IsNotEmpty returns true if the string is non-empty after trimming whitespace.
func IsNotEmpty(s string) bool {
	return len(s) > 0 && len(regexp.MustCompile(`^\s*$`).ReplaceAllString(s, "")) > 0
}

// IsValidTimezone returns true if the string is a valid IANA timezone name
// (e.g. "Europe/Berlin", "UTC"). Invalid values must be rejected on write —
// they would otherwise poison every `AT TIME ZONE` query downstream and turn
// habit endpoints into 500s. Deploy images ship tzdata, so LoadLocation is
// the authority. The 50-char bound matches the varchar(50) columns that
// store timezone (user_preferences, reminder_state, notification_devices).
func IsValidTimezone(tz string) bool {
	if len(tz) == 0 || len(tz) > 50 {
		return false
	}
	_, err := time.LoadLocation(tz)
	return err == nil
}

// IsValidUUID returns true if the string is a valid UUID.
func IsValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// IsOneOf returns true if the string is contained in the allowed set.
func IsOneOf(s string, allowed ...string) bool {
	for _, a := range allowed {
		if s == a {
			return true
		}
	}
	return false
}

// MinLength returns true if the string length is >= min.
func MinLength(s string, min int) bool {
	return len(s) >= min
}

// MaxLength returns true if the string length is <= max.
func MaxLength(s string, max int) bool {
	return len(s) <= max
}

// InRange returns true if the integer is within [min, max].
func InRange(n, min, max int) bool {
	return n >= min && n <= max
}

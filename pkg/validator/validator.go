package validator

import (
	"regexp"
	"unicode"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// IsValidEmail checks if the provided email string matches a valid email format.
func IsValidEmail(email string) bool {
	return emailRegex.MatchString(email)
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

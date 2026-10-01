package validator

import (
	"testing"
)

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		email string
		want  bool
	}{
		{"user@example.com", true},
		{"user.name@example.com", true},
		{"user+tag@example.co.uk", true},
		{"user@sub.example.com", true},
		{"", false},
		{"invalid", false},
		{"@example.com", false},
		{"user@", false},
		{"user@.com", false},
		{"user@com", false},
		{"user example.com", false},
		{"user@exa mple.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			if got := IsValidEmail(tt.email); got != tt.want {
				t.Errorf("IsValidEmail(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func TestIsValidTimezone(t *testing.T) {
	tests := []struct {
		tz   string
		want bool
	}{
		{"UTC", true},
		{"Europe/Berlin", true},
		{"America/New_York", true},
		{"Asia/Tokyo", true},
		{"Etc/GMT+5", true},
		{"", false},
		{"garbage", false},
		{"Europe/Nowhere", false},
		{"UTC+05:00", false}, // offsets aren't IANA names
		{" Europe/Berlin", false},
		{"Europe/Berlin ", false},
		{"../../etc/passwd", false},
		{"/absolute/path", false},
		{string(make([]byte, 51)), false}, // over the varchar(50) column bound
	}

	for _, tt := range tests {
		t.Run(tt.tz, func(t *testing.T) {
			if got := IsValidTimezone(tt.tz); got != tt.want {
				t.Errorf("IsValidTimezone(%q) = %v, want %v", tt.tz, got, tt.want)
			}
		})
	}
}

func TestIsStrongPassword(t *testing.T) {
	tests := []struct {
		password string
		want     bool
	}{
		{"StrongP@ssw0rd", true},
		{"Another1!Pass", true},
		{"C0mplex#Pw", true},
		{"", false},
		{"short", false},
		{"nouppercase1!", false},
		{"NOLOWERCASE1!", false},
		{"NoNumber!!", false},
		{"NoSpecial1", false},
		{"nospecial1!", false}, // Missing uppercase
		{"NOLOWER1!", false},   // Missing lowercase
		{"noupper1!", false},   // Missing uppercase
		{"NoUpper!", false},    // Missing digit
		{"12345678", false},    // Only digits
		{"abcdefgh", false},    // Only lowercase
		{"ABCDEFGH", false},    // Only uppercase
		{"!@#$%^&*", false},    // Only special
		{"Passw0rd", false},    // Missing special
		{"password1!", false},  // Missing uppercase
		{"PASSWORD1!", false},  // Missing lowercase
		{"Password!", false},   // Missing digit
	}

	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			if got := IsStrongPassword(tt.password); got != tt.want {
				t.Errorf("IsStrongPassword(%q) = %v, want %v", tt.password, got, tt.want)
			}
		})
	}
}

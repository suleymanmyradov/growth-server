package prompts

import (
	"fmt"
	"strings"
)

// Max lengths for user-controlled prompt fields to prevent token blow-up
// and make injection attacks less effective.
const (
	MaxFieldHabitName   = 200
	MaxFieldStatus      = 50
	MaxFieldMood        = 100
	MaxFieldEnergy      = 100
	MaxFieldBlocker     = 500
	MaxFieldNote        = 1000
	MaxFieldGoal        = 200
	MaxFieldHabit       = 200
	MaxFieldPattern     = 200
	MaxFieldUserMessage = 2000
)

// SanitizeTemplateInput strips Go template action markers from user input
// so that untrusted strings cannot be interpreted as template directives.
func SanitizeTemplateInput(s string) string {
	s = strings.ReplaceAll(s, "{{", "")
	s = strings.ReplaceAll(s, "}}", "")
	return s
}

// WrapUserContent wraps raw user content in structural XML-like delimiters
// to separate it from system instructions. This reduces the risk of prompt
// injection because the model treats delimited blocks as data, not commands.
func WrapUserContent(label, content string) string {
	return fmt.Sprintf("<user-data label=%q>\n%s\n</user-data>", label, content)
}

// Truncate shortens a string to maxLen runes, appending "..." if truncated.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

// SanitizeAndTruncate applies both sanitization and length truncation.
func SanitizeAndTruncate(s string, maxLen int) string {
	return Truncate(SanitizeTemplateInput(s), maxLen)
}

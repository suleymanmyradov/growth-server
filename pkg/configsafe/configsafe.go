// Package configsafe provides helpers for safely logging configuration structs
// without leaking secrets, API keys, or database passwords.
package configsafe

import (
	"fmt"
	"reflect"
	"strings"
)

// MaskedValue is a string value that masks its content when formatted.
type MaskedValue string

func (m MaskedValue) String() string {
	v := string(m)
	if v == "" {
		return ""
	}
	if len(v) <= 4 {
		return "****"
	}
	return v[:2] + strings.Repeat("*", len(v)-4) + v[len(v)-2:]
}

func (m MaskedValue) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", m.String())), nil
}

// MaskSecrets walks a struct and replaces any field tagged with `secret:"true"`
// with a masked string representation. It returns a map that is safe to log.
func MaskSecrets(cfg interface{}) map[string]interface{} {
	val := reflect.ValueOf(cfg)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	return maskValue(val, "")
}

func maskValue(v reflect.Value, prefix string) map[string]interface{} {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}

	result := make(map[string]interface{})
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		// Skip unexported fields
		if !fv.CanInterface() {
			continue
		}

		name := field.Name
		if prefix != "" {
			name = prefix + "." + name
		}

		// Check for secret tag on the field
		if field.Tag.Get("secret") == "true" {
			if fv.Kind() == reflect.String {
				result[field.Name] = MaskedValue(fv.String()).String()
			} else {
				result[field.Name] = "****"
			}
			continue
		}

		// Recurse into nested structs
		if fv.Kind() == reflect.Struct {
			nested := maskValue(fv, "")
			if len(nested) > 0 {
				result[field.Name] = nested
			}
			continue
		}

		// Handle string fields that look like secrets/keys by name
		if fv.Kind() == reflect.String && isSensitiveName(field.Name) {
			result[field.Name] = MaskedValue(fv.String()).String()
			continue
		}

		// Default: include the value as-is
		result[field.Name] = fv.Interface()
	}
	return result
}

var sensitiveNames = []string{
	"Secret", "secret", "Password", "password", "Key", "key",
	"Token", "token", "APIKey", "apiKey", "api_key",
	"StripeSecretKey", "StripeWebhookSecret",
	"Datasource", "datasource",
}

func isSensitiveName(name string) bool {
	for _, s := range sensitiveNames {
		if strings.Contains(name, s) {
			return true
		}
	}
	return false
}

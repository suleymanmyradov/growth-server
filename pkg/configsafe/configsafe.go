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
// or any field with a sensitive name (Secret, Password, Pass, Key, Token, Datasource)
// with a masked string representation. It returns a map that is safe to log.
func MaskSecrets(cfg interface{}) map[string]interface{} {
	val := reflect.ValueOf(cfg)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil
	}
	return maskStruct(val)
}

func maskStruct(v reflect.Value) map[string]interface{} {
	result := make(map[string]interface{})
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		// Skip unexported fields
		if !fv.CanInterface() {
			continue
		}

		// Flatten anonymous (embedded) structs into the parent map
		if field.Anonymous && fv.Kind() == reflect.Struct {
			nested := maskStruct(fv)
			for k, v := range nested {
				result[k] = v
			}
			continue
		}

		// Check for explicit secret tag on the field
		if field.Tag.Get("secret") == "true" {
			result[field.Name] = maskReflectValue(fv)
			continue
		}

		// Recurse into nested structs
		if fv.Kind() == reflect.Struct {
			result[field.Name] = maskStruct(fv)
			continue
		}

		// Handle slices
		if fv.Kind() == reflect.Slice || fv.Kind() == reflect.Array {
			result[field.Name] = maskSlice(fv)
			continue
		}

		// Handle maps
		if fv.Kind() == reflect.Map {
			result[field.Name] = maskMap(fv)
			continue
		}

		// Handle string fields that look like secrets/keys by name
		if fv.Kind() == reflect.String && isSensitiveName(field.Name) {
			result[field.Name] = MaskedValue(fv.String())
			continue
		}

		// Default: include the value as-is
		result[field.Name] = fv.Interface()
	}
	return result
}

func maskReflectValue(v reflect.Value) interface{} {
	switch v.Kind() {
	case reflect.String:
		return MaskedValue(v.String())
	case reflect.Slice, reflect.Array:
		return maskSlice(v)
	case reflect.Map:
		return maskMap(v)
	case reflect.Struct:
		return maskStruct(v)
	case reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		return maskReflectValue(v.Elem())
	default:
		return "****"
	}
}

func maskSlice(v reflect.Value) interface{} {
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return v.Interface()
	}
	result := make([]interface{}, v.Len())
	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		if elem.Kind() == reflect.Struct {
			result[i] = maskStruct(elem)
		} else {
			result[i] = elem.Interface()
		}
	}
	return result
}

func maskMap(v reflect.Value) interface{} {
	if v.Kind() != reflect.Map {
		return v.Interface()
	}
	result := make(map[string]interface{})
	for _, key := range v.MapKeys() {
		result[fmt.Sprint(key.Interface())] = v.MapIndex(key).Interface()
	}
	return result
}

var sensitiveNames = []string{
	"Secret", "secret", "Password", "password", "Pass", "pass", "Key", "key",
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

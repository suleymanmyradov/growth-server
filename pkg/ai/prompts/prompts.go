package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

// Template is a typed prompt template that renders a Go text/template
// from embedded .tmpl files.
type Template[T any] struct {
	name string
	tmpl *template.Template
}

// NewTemplate creates a new Template from the given name and embedded FS.
// The name must match a .tmpl file in the embedded FS.
func NewTemplate[T any](fs embed.FS, name string) (*Template[T], error) {
	content, err := fs.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("prompts.NewTemplate: read %q: %w", name, err)
	}

	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("prompts.NewTemplate: parse %q: %w", name, err)
	}

	return &Template[T]{name: name, tmpl: tmpl}, nil
}

// Render executes the template with the given input and returns the rendered string.
func (t *Template[T]) Render(in T) (string, error) {
	var buf bytes.Buffer
	if err := t.tmpl.Execute(&buf, in); err != nil {
		return "", fmt.Errorf("prompts.Render %q: %w", t.name, err)
	}
	return buf.String(), nil
}

// Name returns the template filename.
func (t *Template[T]) Name() string {
	return t.name
}

// LoadTemplate is a convenience function that creates and returns a Template.
func LoadTemplate[T any](fs embed.FS, name string) (*Template[T], error) {
	return NewTemplate[T](fs, name)
}

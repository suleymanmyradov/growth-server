package prompts

import (
	"embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed *.tmpl
var templateFS embed.FS

type ExampleInput struct {
	Name  string
	Topic string
}

func TestLoadTemplate(t *testing.T) {
	tmpl, err := LoadTemplate[ExampleInput](templateFS, "example.v1.tmpl")
	require.NoError(t, err)
	assert.Equal(t, "example.v1.tmpl", tmpl.Name())
}

func TestRender(t *testing.T) {
	tmpl, err := LoadTemplate[ExampleInput](templateFS, "example.v1.tmpl")
	require.NoError(t, err)

	result, err := tmpl.Render(ExampleInput{Name: "Alice", Topic: "Go testing"})
	require.NoError(t, err)
	assert.Contains(t, result, "Alice")
	assert.Contains(t, result, "Go testing")
}

func TestRender_MissingTemplate(t *testing.T) {
	_, err := LoadTemplate[ExampleInput](templateFS, "nonexistent.tmpl")
	assert.Error(t, err)
}

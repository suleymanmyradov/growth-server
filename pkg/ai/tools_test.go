package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type EchoInput struct {
	Text string `json:"text" jsonschema:"required,description=The text to echo"`
}

type EchoOutput struct {
	Echo string `json:"echo"`
}

var EchoTool = NewTool[EchoInput, EchoOutput](ToolSpec{
	Name:        "echo",
	Description: "Echoes back the input text.",
	Handler: func(_ context.Context, in EchoInput) (EchoOutput, error) {
		return EchoOutput{Echo: in.Text}, nil
	},
})

func TestNewTool_Info(t *testing.T) {
	info := EchoTool.Info()
	assert.Equal(t, "echo", info.Name)
	assert.Equal(t, "Echoes back the input text.", info.Desc)
	assert.NotNil(t, info.ParamsOneOf)
}

func TestNewTool_Execute(t *testing.T) {
	result, err := EchoTool.Execute(context.Background(), `{"text":"hello"}`)
	require.NoError(t, err)
	assert.Contains(t, result, "hello")
}

func TestNewTool_Execute_InvalidJSON(t *testing.T) {
	_, err := EchoTool.Execute(context.Background(), `not json`)
	assert.Error(t, err)
}

func TestNewTool_Name(t *testing.T) {
	assert.Equal(t, "echo", EchoTool.Name())
}

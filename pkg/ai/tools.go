package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/cloudwego/eino/schema"
	einojsonschema "github.com/eino-contrib/jsonschema"
	ischema "github.com/invopop/jsonschema"
)

// Tool is the interface every tool must implement. Tools are passed into
// AgentRequest.Tools per call. pkg/ai does NOT own business-logic tools —
// those live in each microservice. Only the abstraction lives here.
type Tool interface {
	// Info returns the Eino ToolInfo for model binding.
	Info() *schema.ToolInfo
	// Execute runs the tool handler with JSON input and returns JSON output.
	Execute(ctx context.Context, argsJSON string) (string, error)
	// Name returns the tool's unique name.
	Name() string
}

// ToolSpec defines a declarative tool.
type ToolSpec struct {
	Name        string
	Description string
	Handler     any // func(ctx context.Context, in I) (O, error)
}

// tool implements the Tool interface.
type tool[I any, O any] struct {
	spec    ToolSpec
	info    *schema.ToolInfo
	handler func(ctx context.Context, in I) (O, error)
}

// NewTool creates a Tool from a ToolSpec. The Handler must be
// func(ctx context.Context, in I) (O, error). JSON schema for the input
// type I is derived automatically via reflection.
func NewTool[I any, O any](spec ToolSpec) Tool {
	handler, ok := spec.Handler.(func(ctx context.Context, in I) (O, error))
	if !ok {
		panic(fmt.Sprintf("ai.NewTool: Handler must be func(context.Context, %T) (%T, error), got %T",
			*Izero[I](), *Ozero[O](), spec.Handler))
	}

	// Use invopop/jsonschema for reflection, then convert to eino-contrib/jsonschema.
	r := &ischema.Reflector{
		AllowAdditionalProperties: false,
	}
	invopopSchema := r.Reflect(*Izero[I]())

	einoSchema := convertToEinoJSONSchema(invopopSchema)

	paramsOneOf := schema.NewParamsOneOfByJSONSchema(einoSchema)

	info := &schema.ToolInfo{
		Name:        spec.Name,
		Desc:        spec.Description,
		ParamsOneOf: paramsOneOf,
	}

	return &tool[I, O]{
		spec:    spec,
		info:    info,
		handler: handler,
	}
}

func (t *tool[I, O]) Info() *schema.ToolInfo {
	return t.info
}

func (t *tool[I, O]) Execute(ctx context.Context, argsJSON string) (string, error) {
	var in I
	if err := json.Unmarshal([]byte(argsJSON), &in); err != nil {
		return "", fmt.Errorf("ai.Tool %q: unmarshal input: %w", t.spec.Name, err)
	}
	out, err := t.handler(ctx, in)
	if err != nil {
		return "", fmt.Errorf("ai.Tool %q: execute: %w", t.spec.Name, err)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("ai.Tool %q: marshal output: %w", t.spec.Name, err)
	}
	return string(b), nil
}

func (t *tool[I, O]) Name() string {
	return t.spec.Name
}

// Izero returns a zero value of I for schema reflection.
func Izero[I any]() *I { var v I; return &v }

// Ozero returns a zero value of O for schema reflection.
func Ozero[O any]() *O { var v O; return &v }

// convertToEinoJSONSchema converts an invopop/jsonschema Schema to an
// eino-contrib/jsonschema Schema via JSON round-trip.
func convertToEinoJSONSchema(s *ischema.Schema) *einojsonschema.Schema {
	b, err := json.Marshal(s)
	if err != nil {
		return nil
	}
	var es einojsonschema.Schema
	if err := json.Unmarshal(b, &es); err != nil {
		return nil
	}
	return &es
}

// ensure Tool interface is used
var _ Tool = (*tool[struct{}, struct{}])(nil)

// suppress unused import
var _ = reflect.TypeOf

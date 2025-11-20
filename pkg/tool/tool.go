package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"giai/pkg/types"
)

// Callable adapts plain functions into Tool implementations.
type Callable func(ctx context.Context, input map[string]any, tc *ToolContext) (any, error)

// Func is a lightweight Tool implementation that supports EnhancedTool features.
type Func struct {
	BaseTool
	fn Callable
}

// NewFunc creates a new Tool from a function.
func NewFunc(name, description string, fn Callable) *Func {
	f := &Func{
		BaseTool: NewBaseTool(name, description),
		fn:       fn,
	}
	// Default schema for freeform input
	f.SchemaVal = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"input": map[string]any{
				"type":        "string",
				"description": "freeform input",
			},
		},
		"required": []string{"input"},
	}
	return f
}

// Execute runs the wrapped function.
func (f *Func) Execute(ctx context.Context, input map[string]any, tc *ToolContext) (any, error) {
	if f.fn == nil {
		return nil, fmt.Errorf("tool %s has no implementation", f.Name())
	}
	return f.fn(ctx, input, tc)
}

// Fluent setters for configuration

func (f *Func) WithSchema(schema map[string]any) *Func {
	f.SchemaVal = schema
	return f
}

func (f *Func) WithPrompt(prompt string) *Func {
	f.PromptVal = prompt
	return f
}

func (f *Func) WithTimeout(d time.Duration) *Func {
	f.TimeoutVal = d
	return f
}

func (f *Func) WithPriority(p int) *Func {
	f.PriorityVal = p
	return f
}

func (f *Func) WithRetry(policy *RetryPolicy) *Func {
	f.RetryPolicyVal = policy
	return f
}

func (f *Func) WithApproval(required bool) *Func {
	f.RequiresApprovalVal = required
	return f
}

// Struct is a tool that uses a struct for input validation/parsing.
type Struct[T any] struct {
	BaseTool
	fn func(context.Context, T, *ToolContext) (any, error)
}

// NewStruct creates a tool from a struct type; schema is generated from the struct fields.
func NewStruct[T any](name, description string, fn func(context.Context, T, *ToolContext) (any, error)) *Struct[T] {
	var zero T
	s := &Struct[T]{
		BaseTool: NewBaseTool(name, description),
		fn:       fn,
	}
	s.SchemaVal = GenerateSchema(zero) // Assumes GenerateSchema is in schema.go
	return s
}

func (s *Struct[T]) Execute(ctx context.Context, input map[string]any, tc *ToolContext) (any, error) {
	var args T
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input for tool %s: %w", s.Name(), err)
	}

	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("failed to parse arguments for tool %s: %w", s.Name(), err)
	}
	return s.fn(ctx, args, tc)
}

// Fluent setters for Struct

func (s *Struct[T]) WithPrompt(prompt string) *Struct[T] {
	s.PromptVal = prompt
	return s
}

func (s *Struct[T]) WithTimeout(d time.Duration) *Struct[T] {
	s.TimeoutVal = d
	return s
}

func (s *Struct[T]) WithPriority(p int) *Struct[T] {
	s.PriorityVal = p
	return s
}

// Format renders a readable list for prompt injection.
func Format(tools []Tool) string {
	if len(tools) == 0 {
		return "no tools available"
	}
	parts := make([]string, 0, len(tools))
	for _, t := range tools {
		parts = append(parts, fmt.Sprintf("- %s: %s", t.Name(), t.Description()))
	}
	return strings.Join(parts, "\n")
}

// Find returns a tool by name or nil when missing (case-insensitive).
func Find(tools []Tool, name string) Tool {
	for _, t := range tools {
		if strings.EqualFold(t.Name(), name) {
			return t
		}
	}
	return nil
}

// ValidateInput performs a basic required-field check based on the tool schema.
func ValidateInput(tool Tool, input map[string]any) error {
	schema := tool.InputSchema()
	if schema == nil {
		return nil
	}

	required, ok := schema["required"].([]string)
	if !ok {
		if raw, okAny := schema["required"].([]any); okAny {
			for _, v := range raw {
				if s, okStr := v.(string); okStr {
					required = append(required, s)
				}
			}
		}
	}

	for _, field := range required {
		if _, exists := input[field]; !exists {
			return fmt.Errorf("missing required field: %s", field)
		}
	}
	return nil
}

// ToDefinition converts a Tool into a types.ToolDefinition for LLM providers.
func ToDefinition(t Tool) types.ToolDefinition {
	return types.ToolDefinition{
		Type: "function",
		Function: struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Parameters  any    `json:"parameters"`
		}{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.InputSchema(),
		},
	}
}

// ToDefinitions converts a list of Tools to provider tool definitions.
func ToDefinitions(tools []Tool) []types.ToolDefinition {
	res := make([]types.ToolDefinition, len(tools))
	for i, t := range tools {
		res[i] = ToDefinition(t)
	}
	return res
}

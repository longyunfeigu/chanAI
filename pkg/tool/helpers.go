package tool

import (
	"fmt"
	"strings"

	"giai/pkg/types"
)

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

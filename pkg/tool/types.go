package tool

import (
	"context"
	"time"
)

// Tool represents a basic callable capability.
type Tool interface {
	// Name returns the unique name of the tool.
	Name() string

	// Description returns a human-readable description.
	Description() string

	// InputSchema returns the JSON schema for validation.
	InputSchema() map[string]any

	// Execute runs the tool logic.
	Execute(ctx context.Context, input map[string]any, tc *ToolContext) (any, error)

	// Prompt returns a usage prompt for the LLM (optional).
	Prompt() string
}

// EnhancedTool extends Tool with advanced orchestration capabilities.
type EnhancedTool interface {
	Tool

	// IsLongRunning returns true if the tool returns a resource ID and completes asynchronously.
	IsLongRunning() bool

	// Timeout returns the execution timeout. Return 0 for default.
	Timeout() time.Duration

	// Priority returns the scheduling priority (higher is more important).
	Priority() int

	// RequiresApproval returns true if human-in-the-loop is needed.
	RequiresApproval() bool

	// RetryPolicy returns the retry configuration. Return nil for no retries.
	RetryPolicy() *RetryPolicy
}

// RetryPolicy defines how tool execution should be retried on failure.
type RetryPolicy struct {
	MaxRetries        int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
	RetryableErrors   []string // Substrings or error types to match
}

// DefaultRetryPolicy returns a standard retry configuration.
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:        3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        5 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

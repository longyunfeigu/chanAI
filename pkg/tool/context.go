package tool

import (
	"context"
)

// ToolContext carries metadata and services for tool execution.
type ToolContext struct {
	// Identity info
	AgentID     string
	SessionID   string
	ExecutionID string // Unique ID for this execution

	// Context
	Context context.Context

	// Metadata for arbitrary values
	Metadata map[string]any

	// Services (Interfaces for loose coupling)
	Logger  Logger
	Storage Storage
}

// Logger interface to avoid heavy dependencies
type Logger interface {
	Info(msg string, keysAndValues ...any)
	Error(msg string, keysAndValues ...any)
	Debug(msg string, keysAndValues ...any)
}

// Storage interface for tools that need persistence (like long-running tools)
type Storage interface {
	Get(ctx context.Context, key string) (any, error)
	Set(ctx context.Context, key string, value any) error
}

// Option defines a function to configure ToolContext
type Option func(*ToolContext)

func NewToolContext(opts ...Option) *ToolContext {
	tc := &ToolContext{
		Metadata: make(map[string]any),
		Context:  context.Background(),
	}
	for _, opt := range opts {
		opt(tc)
	}
	return tc
}

func WithAgentID(id string) Option {
	return func(tc *ToolContext) {
		tc.AgentID = id
	}
}

func WithSessionID(id string) Option {
	return func(tc *ToolContext) {
		tc.SessionID = id
	}
}

func WithLogger(l Logger) Option {
	return func(tc *ToolContext) {
		tc.Logger = l
	}
}

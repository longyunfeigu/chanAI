package provider

import (
	"context"
	"giai/pkg/types"
)

// ChatOptions contains configurable parameters for chat generation.
type ChatOptions struct {
	Model       string
	Temperature float64
	MaxTokens   int
	TopP        float64
	Stop        []string
	Tools       []types.ToolDefinition
	Stream      bool
}

// Option is a functional option for configuring ChatOptions.
type Option func(*ChatOptions)

func WithTemperature(t float64) Option {
	return func(o *ChatOptions) {
		o.Temperature = t
	}
}

func WithModel(m string) Option {
	return func(o *ChatOptions) {
		o.Model = m
	}
}

// ChatChunk represents a piece of a streamed response.
type ChatChunk struct {
	Content      string
	ToolCall     *types.ToolCall // Partial tool call
	FinishReason string
	Usage        *types.Usage // Usually only available in the last chunk
	ID           string
	Error        error // To handle stream errors gracefully
}

// ChatModel defines the interface for interacting with Chat LLMs.
type ChatModel interface {
	// Name returns the provider name (e.g., "openai", "anthropic").
	Name() string

	// Chat sends a list of messages and returns a complete response.
	Chat(ctx context.Context, messages []types.Message, opts ...Option) (*types.ChatResponse, error)

	// Stream sends a list of messages and returns a channel of chunks.
	Stream(ctx context.Context, messages []types.Message, opts ...Option) (<-chan ChatChunk, error)
}

package openai

import (
	"context"
	"os"
	"testing"

	"giai/pkg/provider"
	"giai/pkg/types"
)

func TestNewChatModel(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "Empty API Key",
			cfg:     Config{},
			wantErr: true,
		},
		{
			name:    "Valid Config",
			cfg:     Config{APIKey: "test-key"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewChatModel(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewChatModel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Error("NewChatModel() returned nil success")
			}
		})
	}
}

// --- Live Tests below ---

func getLiveClient(t *testing.T) provider.ChatModel {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping live test: OPENAI_API_KEY not set")
	}

	cfg := Config{
		APIKey:  apiKey,
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
		Model:   os.Getenv("OPENAI_MODEL"),
	}

	client, err := NewChatModel(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	return client
}

// TestLive_Chat runs against the real OpenAI API.
func TestLive_Chat(t *testing.T) {
	client := getLiveClient(t)
	ctx := context.Background()

	msgs := []types.Message{
		{Role: types.RoleUser, Content: "Hello, reply with 'LIVE TEST OK'"},
	}

	t.Logf("Sending Chat request...")
	resp, err := client.Chat(ctx, msgs)
	if err != nil {
		t.Fatalf("Live Chat() error = %v", err)
	}

	t.Logf("Response: %s", resp.Message.Content)
	if resp.Message.Content == "" {
		t.Error("Received empty content from OpenAI")
	}
}

// TestLive_Stream runs streaming against the real OpenAI API.
func TestLive_Stream(t *testing.T) {
	client := getLiveClient(t)
	ctx := context.Background()

	msgs := []types.Message{
		{Role: types.RoleUser, Content: "Count from 1 to 5, separated by spaces."},
	}

	t.Logf("Sending Stream request...")
	stream, err := client.Stream(ctx, msgs)
	if err != nil {
		t.Fatalf("Live Stream() error = %v", err)
	}

	var content string
	for chunk := range stream {
		if chunk.Error != nil {
			t.Errorf("Stream error: %v", chunk.Error)
		}
		if chunk.Content != "" {
			content += chunk.Content
			t.Logf("Chunk: %q", chunk.Content) // Uncomment to see chunks
		}
	}

	t.Logf("Full Stream Content: %s", content)
	if content == "" {
		t.Error("Stream received no content")
	}
}

// TestLive_ToolCall runs tool calling against the real OpenAI API.
func TestLive_ToolCall(t *testing.T) {
	client := getLiveClient(t)
	ctx := context.Background()

	// Define a dummy weather tool
	tools := []types.ToolDefinition{
		{
			Type: "function",
			Function: struct {
				Name        string `json:"name"`
				Description string `json:"description,omitempty"`
				Parameters  any    `json:"parameters"`
			}{
				Name:        "get_current_weather",
				Description: "Get the current weather in a given location",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The city and state, e.g. San Francisco, CA",
						},
						"unit": map[string]interface{}{
							"type": "string",
							"enum": []string{"celsius", "fahrenheit"},
						},
					},
					"required": []string{"location"},
				},
			},
		},
	}

	msgs := []types.Message{
		{Role: types.RoleUser, Content: "What's the weather like in Shanghai?"},
	}

	t.Logf("Sending ToolCall request...")
	resp, err := client.Chat(ctx, msgs, func(o *provider.ChatOptions) {
		o.Tools = tools
		o.Model = "gpt-4" // Ensure we use a model supporting tools if default is old
		// If user env var OPENAI_MODEL is set, it overrides this anyway in NewChatModel?
		// No, NewChatModel sets defaultModel, but prepareRequest applies options.
		// Actually, in Chat(), we call prepareRequest which applies options.
		// But wait, NewChatModel takes config model. prepareRequest uses m.defaultModel unless opts override?
		// Let's check prepareRequest logic:
		// options.Model starts as m.defaultModel.
		// Then we apply opts.
		// So if we want to force a capable model for this test, we can use WithModel option if we implemented it properly.
	})

	if err != nil {
		t.Fatalf("Live Chat() with tools error = %v", err)
	}

	t.Logf("FinishReason: %s", resp.FinishReason)

	if len(resp.Message.ToolCalls) == 0 {
		t.Logf("Content received instead of tool: %s", resp.Message.Content)
		t.Error("Expected tool call, got none")
	} else {
		for _, tc := range resp.Message.ToolCalls {
			t.Logf("ToolCall: %s(%s)", tc.Function.Name, tc.Function.Arguments)
			if tc.Function.Name != "get_current_weather" {
				t.Errorf("Expected function get_current_weather, got %s", tc.Function.Name)
			}
		}
	}
}

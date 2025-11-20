package openrouter

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
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping live test: OPENROUTER_API_KEY not set")
	}

	cfg := Config{
		APIKey:      apiKey,
		BaseURL:     os.Getenv("OPENROUTER_BASE_URL"),
		Model:       os.Getenv("OPENROUTER_MODEL"),
		Referer:     os.Getenv("OPENROUTER_REFERER"),
		AppName:     os.Getenv("OPENROUTER_APP_NAME"),
		Temperature: 0.7,
	}

	client, err := NewChatModel(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	return client
}

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
		t.Error("Received empty content from OpenRouter")
	}
}

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
			t.Logf("Chunk: %q", chunk.Content)
		}
	}

	t.Logf("Full Stream Content: %s", content)
	if content == "" {
		t.Error("Stream received no content")
	}
}

func TestLive_ToolCall(t *testing.T) {
	client := getLiveClient(t)
	ctx := context.Background()

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
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{
							"type":        "string",
							"description": "The city and state, e.g. San Francisco, CA",
						},
						"unit": map[string]any{
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
		o.Model = "openai/gpt-4o-mini"
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

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"giai/pkg/agent"
	"giai/pkg/provider"
	"giai/pkg/provider/echo"
	"giai/pkg/provider/openai"
	"giai/pkg/provider/openrouter"
	"giai/pkg/tool"
)

func main() {
	ctx := context.Background()

	llm := initProvider()

	tools := []tool.Tool{
		tool.NewFunc("clock", "Returns the current UTC time", func(ctx context.Context, input map[string]any, tc *tool.ToolContext) (any, error) {
			return time.Now().UTC().Format(time.RFC3339), nil
		}).WithSchema(map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}),
		tool.NewFunc("echo", "Echo back the provided input", func(ctx context.Context, input map[string]any, tc *tool.ToolContext) (any, error) {
			if v, ok := input["input"].(string); ok {
				return v, nil
			}
			return "", fmt.Errorf("input must be a string")
		}),
	}

	ag, err := agent.New(agent.Config{
		Provider: llm,
		Tools:    tools,
	})
	if err != nil {
		log.Fatalf("failed to build agent: %v", err)
	}

	userInput := "Hello, introduce yourself."
	fmt.Printf("User: %s\n", userInput)

	// Test Streaming
	fmt.Print("Agent: ")
	_, err = ag.RunStream(ctx, userInput, func(delta string) {
		fmt.Print(delta)
	})
	if err != nil {
		log.Fatalf("\nagent run failed: %v", err)
	}
	fmt.Println() // Newline after stream

	// Test Memory
	fmt.Println("\n--- Conversation History ---")
	for _, msg := range ag.History() {
		fmt.Printf("[%s]: %s\n", msg.Role, msg.Content)
	}
}

// initProvider selects OpenAI chat model when credentials are present, otherwise falls back to a local echo provider.
func initProvider() provider.ChatModel {
	if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" {
		llm, err := openrouter.NewChatModel(openrouter.Config{
			APIKey:      apiKey,
			BaseURL:     os.Getenv("OPENROUTER_BASE_URL"),
			Model:       os.Getenv("OPENROUTER_MODEL"),
			Referer:     os.Getenv("OPENROUTER_REFERER"),
			AppName:     os.Getenv("OPENROUTER_APP_NAME"),
			Temperature: 0.7,
		})
		if err != nil {
			log.Printf("openrouter init failed, trying OpenAI: %v", err)
		} else {
			fmt.Println("Using OpenRouter provider.")
			return llm
		}
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("No OPENAI_API_KEY found, using Echo provider.")
		return echo.New("EchoAgent")
	}

	llm, err := openai.NewChatModel(openai.Config{
		APIKey:      apiKey,
		BaseURL:     os.Getenv("OPENAI_BASE_URL"),
		Model:       os.Getenv("OPENAI_MODEL"),
		Temperature: 0.7,
	})
	if err != nil {
		log.Printf("openai init failed, falling back to echo provider: %v", err)
		return echo.New("EchoAgent")
	}

	return llm
}

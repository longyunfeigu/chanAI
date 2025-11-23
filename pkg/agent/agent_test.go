package agent

import (
	"context"
	"os"
	"strings"
	"testing"

	"giai/pkg/prompt"
	"giai/pkg/provider/echo"
	"giai/pkg/provider/openai"
	"giai/pkg/tool"
)

// addArgs is the input struct for the add tool.
type addArgs struct {
	A int `json:"a"`
	B int `json:"b"`
}

func TestAgentRunSimpleEcho(t *testing.T) {
	ctx := context.Background()
	prov := echo.New("test")
	ag, err := New(Config{Provider: prov})
	if err != nil {
		t.Fatalf("New agent failed: %v", err)
	}

	reply, err := ag.Run(ctx, "hello")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(reply, "hello") {
		t.Fatalf("expected reply to contain input, got: %q", reply)
	}
}

func TestAgentRunWithToolCalling(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set; skipping OpenAI integration test")
	}

	ctx := context.Background()
	prov, err := openai.NewChatModel(openai.Config{
		APIKey:      apiKey,
		BaseURL:     os.Getenv("OPENAI_BASE_URL"),
		Model:       os.Getenv("OPENAI_MODEL"),
		Temperature: 0,
	})
	if err != nil {
		t.Fatalf("failed to init openai provider: %v", err)
	}

	addTool := tool.NewStruct("add", "add two numbers", func(ctx context.Context, args addArgs, tc *tool.ToolContext) (any, error) {
		return args.A + args.B, nil
	})

	sys := prompt.NewTemplate("You are a test agent. You can call a tool named `add` which adds two integers. When the user asks you to compute 1+2, you MUST call the `add` tool with a=1 and b=2, then respond with the result in natural language.")

	ag, err := New(Config{
		Provider:     prov,
		Tools:        []tool.Tool{addTool},
		SystemPrompt: sys,
		MaxSteps:     4,
	})
	if err != nil {
		t.Fatalf("New agent failed: %v", err)
	}

	reply, err := ag.Run(ctx, "Please compute 1 + 2 using your tools.")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(reply, "3") {
		t.Fatalf("expected reply to contain '3', got: %q", reply)
	}
}

func TestAgentRunStreamEcho(t *testing.T) {
	ctx := context.Background()
	prov := echo.New("stream")
	ag, err := New(Config{Provider: prov})
	if err != nil {
		t.Fatalf("New agent failed: %v", err)
	}

	var got strings.Builder
	reply, err := ag.RunStream(ctx, "hello streaming", func(delta string) {
		got.WriteString(delta)
	})
	if err != nil {
		t.Fatalf("RunStream failed: %v", err)
	}
	if got.Len() == 0 {
		t.Fatalf("expected non-empty streamed content")
	}
	if reply == "" {
		t.Fatalf("expected non-empty final reply")
	}
}

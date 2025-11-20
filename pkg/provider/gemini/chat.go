package gemini

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"giai/pkg/provider"
	"giai/pkg/types"
)

// Config contains Gemini credential and runtime options.
type Config struct {
	APIKey      string
	Model       string // e.g., "gemini-pro"
	Temperature float64
}

// ChatModel implements provider.ChatModel using Google Gemini.
type ChatModel struct {
	client             *genai.Client
	defaultModel       string
	defaultTemperature float64
}

const (
	defaultModel       = "gemini-pro"
	defaultTemperature = 0.5
)

// NewChatModel builds a Gemini chat provider.
func NewChatModel(ctx context.Context, cfg Config) (provider.ChatModel, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("gemini api key is required")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(cfg.APIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	modelName := cfg.Model
	if modelName == "" {
		modelName = defaultModel
	}

	temp := cfg.Temperature
	if temp == 0 {
		temp = defaultTemperature
	}

	return &ChatModel{
		client:             client,
		defaultModel:       modelName,
		defaultTemperature: temp,
	}, nil
}

func (m *ChatModel) Name() string {
	return "gemini"
}

// Chat implements provider.ChatModel.Chat
func (m *ChatModel) Chat(ctx context.Context, messages []types.Message, opts ...provider.Option) (*types.ChatResponse, error) {
	model, cs, err := m.prepareSession(messages, opts)
	if err != nil {
		return nil, err
	}

	// Send the last message. In Gemini SDK, History contains previous messages,
	// and SendMessage takes the new parts.
	// However, our types.Message list includes the last message.
	// So we need to pop the last message content.
	if len(messages) == 0 {
		return nil, errors.New("no messages to send")
	}

	lastMsg := messages[len(messages)-1]
	if lastMsg.Role != types.RoleUser {
		// Gemini chat usually expects User input to drive the turn.
		// But if it's tool output, we also send it.
	}

	// Convert the last message to parts
	parts := toGeminiParts(lastMsg)

	resp, err := cs.SendMessage(ctx, parts...)
	if err != nil {
		return nil, err
	}

	return toChatResponse(resp), nil
}

// Stream implements provider.ChatModel.Stream
func (m *ChatModel) Stream(ctx context.Context, messages []types.Message, opts ...provider.Option) (<-chan provider.ChatChunk, error) {
	_, cs, err := m.prepareSession(messages, opts)
	if err != nil {
		return nil, err
	}

	if len(messages) == 0 {
		return nil, errors.New("no messages to send")
	}
	lastMsg := messages[len(messages)-1]
	parts := toGeminiParts(lastMsg)

	iter := cs.SendMessageStream(ctx, parts...)
	ch := make(chan provider.ChatChunk)

	go func() {
		defer close(ch)
		for {
			resp, err := iter.Next()
			if err == iterator.Done {
				return
			}
			if err != nil {
				ch <- provider.ChatChunk{Error: err}
				return
			}

			// Convert Gemini response chunk to our ChatChunk
			// Gemini chunks can contain multiple candidates/parts
			if len(resp.Candidates) > 0 {
				cand := resp.Candidates[0]
				if cand.Content != nil {
					var sb strings.Builder
					for _, part := range cand.Content.Parts {
						if txt, ok := part.(genai.Text); ok {
							sb.WriteString(string(txt))
						}
					}
					chunk := provider.ChatChunk{
						Content: sb.String(),
					}
					// TODO: Handle ToolCalls in stream if Gemini supports it this way
					ch <- chunk
				}
			}
		}
	}()

	return ch, nil
}

// prepareSession creates a ChatSession with history populated.
func (m *ChatModel) prepareSession(messages []types.Message, opts []provider.Option) (*genai.GenerativeModel, *genai.ChatSession, error) {
	// 1. Apply options
	options := &provider.ChatOptions{
		Model:       m.defaultModel,
		Temperature: m.defaultTemperature,
	}
	for _, o := range opts {
		o(options)
	}

	// 2. Configure Model
	gm := m.client.GenerativeModel(options.Model)
	gm.SetTemperature(float32(options.Temperature))
	if options.MaxTokens > 0 {
		gm.SetMaxOutputTokens(int32(options.MaxTokens))
	}
	// Handle Tools
	if len(options.Tools) > 0 {
		// Mapping types.ToolDefinition to gemini.Tool is non-trivial due to schema differences.
		// For now, we leave this as a TODO or implement basic FunctionDeclaration mapping.
		// gm.Tools = convertToGeminiTools(options.Tools)
	}

	// 3. Build History
	// Gemini ChatSession manages history. We need to feed all BUT the last message as history.
	cs := gm.StartChat()
	
	if len(messages) > 1 {
		history := messages[:len(messages)-1]
		geminiHistory := make([]*genai.Content, 0, len(history))
		
		for _, msg := range history {
			role := "user"
			if msg.Role == types.RoleAssistant {
				role = "model" // Gemini uses "model" instead of "assistant"
			} else if msg.Role == types.RoleTool {
				role = "function" // or user? Tool outputs are tricky in Gemini
			} else if msg.Role == types.RoleSystem {
				// Gemini Pro doesn't strictly have "system" role in Chat History yet, 
				// usually passed as SystemInstruction in model config or merged into first user message.
				// Recent SDKs added SystemInstruction support.
				gm.SystemInstruction = &genai.Content{
					Parts: []genai.Part{genai.Text(msg.Content)},
				}
				continue // Don't add to chat history
			}

			content := &genai.Content{
				Role: role,
				Parts: toGeminiParts(msg),
			}
			geminiHistory = append(geminiHistory, content)
		}
		cs.History = geminiHistory
	}

	return gm, cs, nil
}

// Helpers

func toGeminiParts(msg types.Message) []genai.Part {
	var parts []genai.Part
	if msg.Content != "" {
		parts = append(parts, genai.Text(msg.Content))
	}
	// TODO: Handle msg.ToolCalls -> genai.FunctionCall
	// TODO: Handle msg.ToolCallID/Content -> genai.FunctionResponse
	return parts
}

func toChatResponse(resp *genai.GenerateContentResponse) *types.ChatResponse {
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return &types.ChatResponse{
			Message: types.Message{Role: types.RoleAssistant, Content: ""},
		}
	}

	cand := resp.Candidates[0]
	var sb strings.Builder
	
	// A candidate can have multiple parts (text, function calls)
	// We need to separate them.
	// types.Message has Content (string) and ToolCalls ([]ToolCall)
	
	msg := types.Message{
		Role: types.RoleAssistant,
	}

	for _, part := range cand.Content.Parts {
		switch p := part.(type) {
		case genai.Text:
			sb.WriteString(string(p))
		case genai.FunctionCall:
			// Convert to types.ToolCall
			// tc := types.ToolCall{ ... }
			// msg.ToolCalls = append(msg.ToolCalls, tc)
		}
	}
	msg.Content = sb.String()

	return &types.ChatResponse{
		Message:      msg,
		FinishReason: toFinishReason(cand.FinishReason),
		// Usage: Usage is not always available in standard response struct easily?
		// It is in resp.UsageMetadata
	}
}

func toFinishReason(fr genai.FinishReason) string {
	switch fr {
	case genai.FinishReasonStop:
		return "stop"
	case genai.FinishReasonMaxTokens:
		return "length"
	default:
		return fmt.Sprintf("unknown:%d", fr)
	}
}

package openai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	goopenai "github.com/sashabaranov/go-openai"

	"giai/pkg/provider"
	"giai/pkg/types"
)

// Config contains OpenAI credential and runtime options.
type Config struct {
	APIKey      string
	BaseURL     string
	Model       string
	HTTPClient  *http.Client
	Temperature float64 // Default temperature
}

// ChatModel implements provider.ChatModel using OpenAI chat completions.
type ChatModel struct {
	client             *goopenai.Client
	defaultModel       string
	defaultTemperature float64
}

const (
	defaultTemperature = 0.7
	defaultModel       = goopenai.GPT4
)

// NewChatModel builds a chat completion provider.
func NewChatModel(cfg Config) (provider.ChatModel, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("openai api key is required")
	}

	apiCfg := goopenai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		apiCfg.BaseURL = cfg.BaseURL
	}
	if cfg.HTTPClient != nil {
		apiCfg.HTTPClient = cfg.HTTPClient
	}

	modelName := cfg.Model
	if strings.TrimSpace(modelName) == "" {
		modelName = defaultModel
	}

	temp := cfg.Temperature
	if temp == 0 {
		temp = defaultTemperature
	}

	return &ChatModel{
		client:             goopenai.NewClientWithConfig(apiCfg),
		defaultModel:       modelName,
		defaultTemperature: temp,
	}, nil
}

func (m *ChatModel) Name() string {
	return "openai"
}

func (m *ChatModel) prepareRequest(messages []types.Message, opts []provider.Option) (goopenai.ChatCompletionRequest, error) {
	// 1. Apply options
	options := &provider.ChatOptions{
		Model:       m.defaultModel,
		Temperature: m.defaultTemperature,
	}
	for _, o := range opts {
		o(options)
	}

	// 2. Convert Messages
	openaiMsgs := make([]goopenai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		oMsg := goopenai.ChatCompletionMessage{
			Content: msg.Content,
			Name:    msg.Name,
		}

		switch msg.Role {
		case types.RoleSystem:
			oMsg.Role = goopenai.ChatMessageRoleSystem
		case types.RoleUser:
			oMsg.Role = goopenai.ChatMessageRoleUser
		case types.RoleAssistant:
			oMsg.Role = goopenai.ChatMessageRoleAssistant
			if len(msg.ToolCalls) > 0 {
				oMsg.ToolCalls = convertToOpenAIToolCalls(msg.ToolCalls)
			}
		case types.RoleTool:
			oMsg.Role = goopenai.ChatMessageRoleTool
			oMsg.ToolCallID = msg.ToolCallID
		default:
			oMsg.Role = goopenai.ChatMessageRoleUser // Fallback
		}
		openaiMsgs[i] = oMsg
	}

	// 3. Build Request
	req := goopenai.ChatCompletionRequest{
		Model:       options.Model,
		Messages:    openaiMsgs,
		Temperature: float32(options.Temperature),
		MaxTokens:   options.MaxTokens,
		Stop:        options.Stop,
	}

	// 4. Handle Tools
	if len(options.Tools) > 0 {
		req.Tools = make([]goopenai.Tool, len(options.Tools))
		for i, t := range options.Tools {
			// Using reflection or marshaling to handle the 'Parameters' field which is 'any'
			// go-openai expects json.RawMessage or a struct for schema.
			// Let's try to marshal types.ToolDefinition to match goopenai.Tool structure
			req.Tools[i] = goopenai.Tool{
				Type: goopenai.ToolType(t.Type),
				Function: &goopenai.FunctionDefinition{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					Parameters:  t.Function.Parameters,
				},
			}
		}
	}

	return req, nil
}

// Chat implements provider.ChatModel.Chat
func (m *ChatModel) Chat(ctx context.Context, messages []types.Message, opts ...provider.Option) (*types.ChatResponse, error) {
	req, err := m.prepareRequest(messages, opts)
	if err != nil {
		return nil, err
	}

	resp, err := m.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("openai: no choices returned")
	}

	choice := resp.Choices[0]

	// Convert response back to types.Message
	chatMsg := types.Message{
		Role:    types.RoleAssistant,
		Content: choice.Message.Content,
	}
	if len(choice.Message.ToolCalls) > 0 {
		chatMsg.ToolCalls = convertFromOpenAIToolCalls(choice.Message.ToolCalls)
	}

	return &types.ChatResponse{
		Message:      chatMsg,
		FinishReason: string(choice.FinishReason),
		Usage: types.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

// Stream implements provider.ChatModel.Stream
func (m *ChatModel) Stream(ctx context.Context, messages []types.Message, opts ...provider.Option) (<-chan provider.ChatChunk, error) {
	req, err := m.prepareRequest(messages, opts)
	if err != nil {
		return nil, err
	}
	req.Stream = true

	stream, err := m.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, err
	}

	ch := make(chan provider.ChatChunk)
	go func() {
		defer close(ch)
		defer stream.Close()

		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				ch <- provider.ChatChunk{Error: err}
				return
			}

			if len(resp.Choices) > 0 {
				choice := resp.Choices[0]
				chunk := provider.ChatChunk{
					Content:      choice.Delta.Content,
					ID:           resp.ID,
					FinishReason: string(choice.FinishReason),
				}

				if len(choice.Delta.ToolCalls) > 0 {
					// Streaming tool calls usually come as fragments
					// We just pass them through for now. The consumer needs to aggregate them.
					tc := choice.Delta.ToolCalls[0]
					chunk.ToolCall = &types.ToolCall{
						// Index: tc.Index, // types.ToolCall doesn't have index yet, might need it for parallel calls
						ID:   tc.ID,
						Type: string(tc.Type),
						Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				}

				ch <- chunk
			}
		}
	}()

	return ch, nil
}

// Helpers

func convertToOpenAIToolCalls(tcs []types.ToolCall) []goopenai.ToolCall {
	res := make([]goopenai.ToolCall, len(tcs))
	for i, tc := range tcs {
		res[i] = goopenai.ToolCall{
			ID:   tc.ID,
			Type: goopenai.ToolType(tc.Type),
			Function: goopenai.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return res
}

func convertFromOpenAIToolCalls(tcs []goopenai.ToolCall) []types.ToolCall {
	res := make([]types.ToolCall, len(tcs))
	for i, tc := range tcs {
		res[i] = types.ToolCall{
			ID:   tc.ID,
			Type: string(tc.Type),
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return res
}

// Ensure interface compliance
var _ provider.ChatModel = (*ChatModel)(nil)

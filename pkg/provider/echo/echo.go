package echo

import (
	"context"
	"strings"

	"giai/pkg/provider"
	"giai/pkg/types"
)

// ChatModel is a deterministic echo provider useful for tests and fallbacks.
type ChatModel struct {
	Prefix string
}

// New returns a new echo provider.
func New(prefix string) provider.ChatModel {
	return &ChatModel{Prefix: prefix}
}

func (p *ChatModel) Name() string {
	if p.Prefix == "" {
		return "echo"
	}
	return "echo-" + strings.ReplaceAll(p.Prefix, " ", "_")
}

// Chat implements provider.ChatModel
func (p *ChatModel) Chat(ctx context.Context, messages []types.Message, opts ...provider.Option) (*types.ChatResponse, error) {
	var sb strings.Builder
	if p.Prefix != "" {
		sb.WriteString(strings.TrimSpace(p.Prefix))
		sb.WriteString(" ")
	}

	for _, msg := range messages {
		sb.WriteString(msg.Content)
		sb.WriteString("\n")
	}

	responseContent := sb.String()

	return &types.ChatResponse{
		Message: types.Message{
			Role:    types.RoleAssistant,
			Content: responseContent,
		},
		FinishReason: "stop",
		Usage: types.Usage{
			PromptTokens:     len(responseContent),
			CompletionTokens: len(responseContent),
			TotalTokens:      len(responseContent) * 2,
		},
	}, nil
}

// Stream implements provider.ChatModel
func (p *ChatModel) Stream(ctx context.Context, messages []types.Message, opts ...provider.Option) (<-chan provider.ChatChunk, error) {
	ch := make(chan provider.ChatChunk)

	go func() {
		defer close(ch)

		// Just generate the full response and send it in chunks (simulated)
		resp, err := p.Chat(ctx, messages, opts...)
		if err != nil {
			ch <- provider.ChatChunk{Error: err}
			return
		}

		// Simulate streaming by words
		words := strings.Split(resp.Message.Content, " ")
		for _, word := range words {
			ch <- provider.ChatChunk{
				Content: word + " ",
			}
		}

		ch <- provider.ChatChunk{
			FinishReason: "stop",
			Usage:        &resp.Usage,
		}
	}()

	return ch, nil
}

var _ provider.ChatModel = (*ChatModel)(nil)

package agent

import (
	"context"
	"fmt"
	"strings"

	"giai/pkg/memory"
	"giai/pkg/prompt"
	"giai/pkg/provider"
	"giai/pkg/tool"
	"giai/pkg/types"
)

// Config describes how an Agent is assembled.
type Config struct {
	Provider     provider.ChatModel // Changed interface
	Tools        []tool.Tool
	Memory       memory.Memory
	SystemPrompt prompt.Template
}

// Agent coordinates a model, tools, and memory.
type Agent struct {
	provider     provider.ChatModel
	tools        []tool.Tool
	toolIndex    map[string]tool.Tool
	memory       memory.Memory
	systemPrompt prompt.Template
}

const defaultSystemPrompt = `You are a helpful AI assistant.`

// New builds an Agent and wires defaults.
func New(cfg Config) (*Agent, error) {
	if cfg.Provider == nil {
		return nil, fmt.Errorf("provider is required")
	}

	mem := cfg.Memory
	if mem == nil {
		mem = memory.NewInMemory()
	}

	promptTemplate := cfg.SystemPrompt
	if promptTemplate.Text == "" {
		promptTemplate = prompt.NewTemplate(defaultSystemPrompt)
	}

	index := make(map[string]tool.Tool, len(cfg.Tools))
	for _, t := range cfg.Tools {
		index[t.Name()] = t
	}

	return &Agent{
		provider:     cfg.Provider,
		tools:        cfg.Tools,
		toolIndex:    index,
		memory:       mem,
		systemPrompt: promptTemplate,
	}, nil
}

// Run sends user input through prompting and the provider, recording the turn in memory.
func (a *Agent) Run(ctx context.Context, input string) (string, error) {
	// Add user input to memory
	userMsg := types.Message{Role: types.RoleUser, Content: input}
	a.memory.Add(userMsg)

	// Build full context (System + History)
	fullMessages := []types.Message{
		{Role: types.RoleSystem, Content: a.systemPrompt.Render(nil)},
	}
	fullMessages = append(fullMessages, a.memory.History()...)

	// Call LLM
	resp, err := a.provider.Chat(ctx, fullMessages)
	if err != nil {
		return "", err
	}

	// Save response
	a.memory.Add(resp.Message)

	return resp.Message.Content, nil
}

// RunStream streams the provider response, optionally forwarding deltas, and stores the final message.
func (a *Agent) RunStream(ctx context.Context, input string, onDelta func(string)) (string, error) {
	// Add user input to memory
	a.memory.Add(types.Message{Role: types.RoleUser, Content: input})

	fullMessages := []types.Message{
		{Role: types.RoleSystem, Content: a.systemPrompt.Render(nil)},
	}
	fullMessages = append(fullMessages, a.memory.History()...)

	chunks, err := a.provider.Stream(ctx, fullMessages)
	if err != nil {
		return "", err
	}

	var fullContent strings.Builder

	for chunk := range chunks {
		if chunk.Error != nil {
			return "", chunk.Error
		}
		if chunk.Content != "" {
			fullContent.WriteString(chunk.Content)
			if onDelta != nil {
				onDelta(chunk.Content)
			}
		}
	}

	finalReply := fullContent.String()
	a.memory.Add(types.Message{Role: types.RoleAssistant, Content: finalReply})

	return finalReply, nil
}

// UseTool allows manual tool invocation; typical planners can wrap this.
func (a *Agent) UseTool(ctx context.Context, name, input string) (string, error) {
	t, ok := a.toolIndex[name]
	if !ok {
		return "", fmt.Errorf("tool %q not found", name)
	}
	res, err := t.Run(ctx, input)
	if err != nil {
		return "", err
	}
	// Note: UseTool in this simple agent just records the execution,
	// it doesn't necessarily feed it back to the LLM unless part of a Run loop.
	// We'll update this in Phase 4 (ReAct Loop).
	a.memory.Add(types.Message{
		Role:    types.RoleTool,
		Content: res,
	})
	return res, nil
}

// History returns a copy of the remembered conversation.
func (a *Agent) History() []types.Message {
	return a.memory.History()
}

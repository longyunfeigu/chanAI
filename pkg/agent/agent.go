package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"giai/pkg/memory"
	"giai/pkg/prompt"
	"giai/pkg/provider"
	"giai/pkg/sandbox"
	"giai/pkg/tool"
	"giai/pkg/types"
)

// Config describes how an Agent is assembled.
type Config struct {
	ID           string
	Provider     provider.ChatModel
	Tools        []tool.Tool
	Sandbox      sandbox.Sandbox
	Memory       memory.Memory
	SystemPrompt prompt.Template
	MaxSteps     int
	Executor     *tool.Executor
}

// Agent coordinates a model, tools, memory and sandbox.
type Agent struct {
	id           string
	provider     provider.ChatModel
	tools        []tool.Tool
	toolIndex    map[string]tool.Tool
	toolDefs     []types.ToolDefinition
	sandbox      sandbox.Sandbox
	memory       memory.Memory
	systemPrompt prompt.Template
	maxSteps     int
	executor     *tool.Executor
	mu           sync.RWMutex
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

	maxSteps := cfg.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 6
	}

	index := make(map[string]tool.Tool, len(cfg.Tools))
	for _, t := range cfg.Tools {
		index[t.Name()] = t
	}

	sp := buildSystemPrompt(cfg.SystemPrompt, cfg.Tools)

	agent := &Agent{
		id:           cfg.ID,
		provider:     cfg.Provider,
		tools:        cfg.Tools,
		toolIndex:    index,
		toolDefs:     buildToolDefinitions(cfg.Tools),
		sandbox:      cfg.Sandbox,
		memory:       mem,
		systemPrompt: sp,
		maxSteps:     maxSteps,
	}

	if cfg.Executor != nil {
		agent.executor = cfg.Executor
	}

	return agent, nil
}

// Run executes a single turn with automatic tool calling.
func (a *Agent) Run(ctx context.Context, input string) (string, error) {
	a.mu.Lock()
	a.memory.Add(types.Message{Role: types.RoleUser, Content: input})
	a.mu.Unlock()

	for step := 0; step < a.maxSteps; step++ {
		msgs := a.buildMessages()
		resp, err := a.provider.Chat(ctx, msgs, provider.WithTools(a.toolDefs))
		if err != nil {
			return "", err
		}

		a.mu.Lock()
		a.memory.Add(resp.Message)
		a.mu.Unlock()

		if len(resp.Message.ToolCalls) == 0 {
			return resp.Message.Content, nil
		}

		if err := a.handleToolCalls(ctx, resp.Message.ToolCalls); err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("agent reached max steps (%d) without final answer", a.maxSteps)
}

// RunStream streams the provider response as a simple chat turn.
func (a *Agent) RunStream(ctx context.Context, input string, onDelta func(string)) (string, error) {
	a.mu.Lock()
	a.memory.Add(types.Message{Role: types.RoleUser, Content: input})
	a.mu.Unlock()

	msgs := a.buildMessages()
	chunks, err := a.provider.Stream(ctx, msgs, provider.WithTools(a.toolDefs))
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
	a.mu.Lock()
	a.memory.Add(types.Message{Role: types.RoleAssistant, Content: finalReply})
	a.mu.Unlock()

	return finalReply, nil
}

// UseTool allows manual tool invocation.
func (a *Agent) UseTool(ctx context.Context, name string, input map[string]any) (any, error) {
	t, ok := a.toolIndex[name]
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}

	if err := tool.ValidateInput(t, input); err != nil {
		return nil, err
	}

	tc := tool.NewToolContext(tool.WithAgentID(a.id))
	if a.sandbox != nil {
		tc.Sandbox = a.sandbox
	}
	tc.Context = ctx

	res, err := t.Execute(ctx, input, tc)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.memory.Add(types.Message{
		Role:    types.RoleTool,
		Content: fmt.Sprintf("%v", res),
	})
	a.mu.Unlock()

	return res, nil
}

// History returns a copy of the remembered conversation.
func (a *Agent) History() []types.Message {
	return a.memory.History()
}

func (a *Agent) buildMessages() []types.Message {
	system := types.Message{Role: types.RoleSystem, Content: a.systemPrompt.Render(nil)}
	history := a.memory.History()
	msgs := make([]types.Message, 0, 1+len(history))
	msgs = append(msgs, system)
	msgs = append(msgs, history...)
	return msgs
}

func (a *Agent) handleToolCalls(ctx context.Context, calls []types.ToolCall) error {
	for _, tc := range calls {
		t, ok := a.toolIndex[tc.Function.Name]
		if !ok {
			msg := fmt.Sprintf("tool %s not found", tc.Function.Name)
			a.mu.Lock()
			a.memory.Add(types.Message{Role: types.RoleTool, ToolCallID: tc.ID, Content: msg})
			a.mu.Unlock()
			continue
		}

		var input map[string]any
		if strings.TrimSpace(tc.Function.Arguments) != "" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				msg := fmt.Sprintf("invalid arguments for tool %s: %v", t.Name(), err)
				a.mu.Lock()
				a.memory.Add(types.Message{Role: types.RoleTool, ToolCallID: tc.ID, Content: msg})
				a.mu.Unlock()
				continue
			}
		} else {
			input = map[string]any{}
		}

		if err := tool.ValidateInput(t, input); err != nil {
			msg := fmt.Sprintf("tool %s input validation failed: %v", t.Name(), err)
			a.mu.Lock()
			a.memory.Add(types.Message{Role: types.RoleTool, ToolCallID: tc.ID, Content: msg})
			a.mu.Unlock()
			continue
		}

		exec := a.executor
		if exec == nil {
			exec = tool.NewExecutor(tool.ExecutorConfig{})
			a.executor = exec
		}

		tcxt := tool.NewToolContext(tool.WithAgentID(a.id))
		if a.sandbox != nil {
			tcxt.Sandbox = a.sandbox
		}
		tcxt.Context = ctx

		res := exec.Execute(ctx, &tool.ExecuteRequest{
			Tool:   t,
			Input:  input,
			Context: tcxt,
		})

		var content string
		if res.Error != nil {
			content = fmt.Sprintf("tool %s error: %v", t.Name(), res.Error)
		} else if res.Output != nil {
			if b, err := json.Marshal(res.Output); err == nil {
				content = string(b)
			} else {
				content = fmt.Sprintf("%v", res.Output)
			}
		}

		if content == "" {
			content = fmt.Sprintf("tool %s completed", t.Name())
		}

		a.mu.Lock()
		a.memory.Add(types.Message{Role: types.RoleTool, ToolCallID: tc.ID, Content: content})
		a.mu.Unlock()
	}

	return nil
}

func buildSystemPrompt(base prompt.Template, tools []tool.Tool) prompt.Template {
	text := strings.TrimSpace(base.Text)
	if text == "" {
		text = defaultSystemPrompt
	}

	manual := buildToolManual(tools)
	if manual != "" {
		if !strings.HasSuffix(text, "\n") {
			text += "\n\n"
		}
		text += manual
	}

	return prompt.NewTemplate(text)
}

func buildToolManual(tools []tool.Tool) string {
	if len(tools) == 0 {
		return ""
	}

	var lines []string
	for _, t := range tools {
		name := t.Name()
		promptText := strings.TrimSpace(t.Prompt())
		desc := strings.TrimSpace(t.Description())

		summary := ""
		if promptText != "" {
			parts := strings.Split(promptText, "\n")
			if len(parts) > 0 {
				summary = strings.TrimSpace(parts[0])
			}
		}
		if summary == "" {
			summary = desc
		}
		if summary == "" {
			summary = "No detailed manual; infer from tool name and input schema."
		}

		lines = append(lines, fmt.Sprintf("- `%s`: %s", name, summary))
	}

	if len(lines) == 0 {
		return ""
	}

	head := "### Tools\n\nThe following tools are available. Use them when needed instead of only replying in natural language.\n\n"
	return head + strings.Join(lines, "\n")
}

func buildToolDefinitions(tools []tool.Tool) []types.ToolDefinition {
	if len(tools) == 0 {
		return nil
	}

	defs := make([]types.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		def := types.ToolDefinition{Type: "function"}
		def.Function.Name = t.Name()
		def.Function.Description = t.Description()
		def.Function.Parameters = t.InputSchema()
		defs = append(defs, def)
	}
	return defs
}


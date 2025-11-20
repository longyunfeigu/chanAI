package types

// Role identifies who authored a message in the conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall represents a request from the model to call a specific function.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // usually "function"
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string arguments
	} `json:"function"`
}

// ToolDefinition describes a tool available to the model.
// It matches the OpenAI tools schema.
type ToolDefinition struct {
	Type     string `json:"type"` // "function"
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Parameters  any    `json:"parameters"` // JSON Schema
	} `json:"function"`
}

// Usage represents token usage statistics.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Message is a single chat turn.
// It is designed to be flexible enough to handle various LLM APIs.
type Message struct {
	Role       Role        `json:"role"`
	Content    string      `json:"content"`
	Name       string      `json:"name,omitempty"`       // Optional: author name
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"` // For RoleAssistant: tools the model wants to call
	ToolCallID string      `json:"tool_call_id,omitempty"` // For RoleTool: the ID of the call this message responds to
}

// ChatResponse represents the full response from a ChatModel.
type ChatResponse struct {
	Message      Message
	FinishReason string // stop, length, tool_calls, content_filter
	Usage        Usage
}

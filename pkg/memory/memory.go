package memory

import (
	"strings"
	"sync"

	"giai/pkg/types"
)

// Memory defines how conversation state is stored.
type Memory interface {
	Add(message types.Message)
	History() []types.Message
	Reset()
}

// InMemory is a simple thread-safe memory backend.
type InMemory struct {
	mu       sync.RWMutex
	messages []types.Message
}

// NewInMemory creates an empty memory store.
func NewInMemory() *InMemory {
	return &InMemory{messages: make([]types.Message, 0, 8)}
}

// Add appends a message to history.
func (m *InMemory) Add(message types.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, message)
}

// History returns a copy of the conversation so callers cannot mutate internal state.
func (m *InMemory) History() []types.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]types.Message, len(m.messages))
	copy(out, m.messages)
	return out
}

// Reset clears the conversation.
func (m *InMemory) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = m.messages[:0]
}

// FormatHistory renders a simple bullet list of the conversation for prompts.
func FormatHistory(messages []types.Message) string {
	if len(messages) == 0 {
		return ""
	}
	lines := make([]string, 0, len(messages))
	for _, msg := range messages {
		lines = append(lines, string(msg.Role)+": "+msg.Content)
	}
	return strings.Join(lines, "\n")
}

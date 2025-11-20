package tool

import (
	"context"
	"time"
)

// BaseTool implements the common fields of EnhancedTool.
// Embed this struct to get default implementations.
type BaseTool struct {
	NameVal        string
	DescVal        string
	SchemaVal      map[string]any
	PromptVal      string
	
	IsLongRunningVal  bool
	TimeoutVal        time.Duration
	PriorityVal       int
	RequiresApprovalVal bool
	RetryPolicyVal    *RetryPolicy
}

func NewBaseTool(name, desc string) BaseTool {
	return BaseTool{
		NameVal:        name,
		DescVal:        desc,
		TimeoutVal:     30 * time.Second,
		PriorityVal:    0,
		RetryPolicyVal: DefaultRetryPolicy(),
	}
}

func (b *BaseTool) Name() string                { return b.NameVal }
func (b *BaseTool) Description() string         { return b.DescVal }
func (b *BaseTool) InputSchema() map[string]any { return b.SchemaVal }
func (b *BaseTool) Prompt() string              { return b.PromptVal }

func (b *BaseTool) IsLongRunning() bool         { return b.IsLongRunningVal }
func (b *BaseTool) Timeout() time.Duration      { return b.TimeoutVal }
func (b *BaseTool) Priority() int               { return b.PriorityVal }
func (b *BaseTool) RequiresApproval() bool      { return b.RequiresApprovalVal }
func (b *BaseTool) RetryPolicy() *RetryPolicy   { return b.RetryPolicyVal }

// Execute must be implemented by the embedding struct.
func (b *BaseTool) Execute(ctx context.Context, input map[string]any, tc *ToolContext) (any, error) {
	return nil, nil
}

package builtin

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"giai/pkg/tool"
)

type Grep struct {
	tool.BaseTool
}

func NewGrep() *Grep {
	t := &Grep{
		BaseTool: tool.NewBaseTool(
			"grep",
			"Search for patterns in files using ripgrep (rg).",
		),
	}

	t.TimeoutVal = 1 * time.Minute

	t.SchemaVal = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "The regex pattern to search for.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Directory or file to search in (defaults to current dir).",
			},
			"glob": map[string]any{
				"type":        "string",
				"description": "Glob pattern to filter files (e.g., '*.go').",
			},
			"case_insensitive": map[string]any{
				"type":        "boolean",
				"description": "Perform case-insensitive search.",
			},
			"context_lines": map[string]any{
				"type":        "integer",
				"description": "Number of context lines to show (default 0).",
			},
		},
		"required": []string{"pattern"},
	}

	return t
}

func (t *Grep) Execute(ctx context.Context, input map[string]any, tc *tool.ToolContext) (any, error) {
	pattern, ok := input["pattern"].(string)
	if !ok {
		return nil, fmt.Errorf("pattern must be a string")
	}

	searchPath, _ := input["path"].(string)
	if searchPath == "" {
		searchPath = "."
	}

	// Construct rg command args
	args := []string{"--line-number", "--no-heading", "--color=never"}

	if ignoreCase, ok := input["case_insensitive"].(bool); ok && ignoreCase {
		args = append(args, "-i")
	}

	if glob, ok := input["glob"].(string); ok && glob != "" {
		args = append(args, "-g", glob)
	}

	if ctxLines, ok := input["context_lines"].(float64); ok && ctxLines > 0 {
		args = append(args, "-C", fmt.Sprintf("%d", int(ctxLines)))
	}

	// Pattern comes last (mostly), then path
	args = append(args, pattern, searchPath)

	cmd := exec.CommandContext(ctx, "rg", args...)

	// Limit output size
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	
	output := stdout.String()
	
	// Handle "no match" (rg returns 1) vs "error" (rg returns > 1)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				// Exit code 1 means no matches found, which is a valid result, not an error
				return "No matches found", nil
			}
		}
		// Real error
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("grep failed: %s", stderr.String())
		}
		return nil, err
	}

	// Truncate result
	const maxChars = 50000
	if len(output) > maxChars {
		output = output[:maxChars] + fmt.Sprintf("\n... (truncated, %d chars omitted)", len(output)-maxChars)
	}

	return output, nil
}

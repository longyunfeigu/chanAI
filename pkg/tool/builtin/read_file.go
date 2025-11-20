package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	_ "strings"

	"giai/pkg/tool"
)

type ReadFile struct {
	tool.BaseTool
}

func NewReadFile() *ReadFile {
	t := &ReadFile{
		BaseTool: tool.NewBaseTool(
			"read_file",
			"Read the contents of a file from the file system.",
		),
	}

	t.SchemaVal = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The absolute path to the file to read.",
			},
		},
		"required": []string{"path"},
	}

	return t
}

func (t *ReadFile) Execute(ctx context.Context, input map[string]any, tc *tool.ToolContext) (any, error) {
	path, ok := input["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path must be a string")
	}

	// Basic security check (expand as needed)
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("path must be absolute: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)

	// Optional: Truncate huge files to prevent context overflow
	const maxRunes = 50000
	if len(content) > maxRunes {
		content = content[:maxRunes] + fmt.Sprintf("\n... (truncated, %d chars omitted)", len(content)-maxRunes)
	}

	return content, nil
}

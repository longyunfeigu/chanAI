package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"giai/pkg/tool"
)

type ReadFile struct {
	tool.BaseTool
}

// NewReadFile 创建 read_file 工具。
// 为了和 agentsdk 的 builtin 工厂风格保持一致，这里接受 config 参数并返回 (tool.Tool, error)，
// 当前实现暂时不使用 config。
func NewReadFile(config map[string]any) (tool.Tool, error) {
	t := &ReadFile{
		BaseTool: tool.NewBaseTool(
			"read_file",
			"Read the contents of a file from the local filesystem.",
		),
	}

	t.SchemaVal = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The absolute path to the file to read.",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "Optional line offset to start reading from (0-based).",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Optional maximum number of lines to read. 0 means read until EOF or truncation.",
			},
		},
		"required": []any{"path"},
	}

	t.PromptVal = `Use this tool to inspect files on the local filesystem.

Usage:
- Always pass an absolute path in "path".
- You may optionally provide "offset" and "limit" to read only a window of lines.
- Large files are truncated to keep responses compact; request additional ranges if needed.

Returns:
- A JSON object with:
  - ok: boolean indicating success
  - path: the file path
  - content: the text content slice
  - offset/limit: the effective range used
  - truncated: whether the result was truncated
  - totalLines: total number of lines in the file
  - readLines: number of lines returned.`

	return t, nil
}

func (t *ReadFile) Execute(ctx context.Context, input map[string]any, tc *tool.ToolContext) (any, error) {
	path, ok := input["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path must be a string")
	}

	// Basic security check (expand as needed)
	if !filepath.IsAbs(path) {
		return map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("path must be absolute: %s", path),
			"path":  path,
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("failed to read file: %v", err),
			"path":  path,
		}, nil
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// Parse offset / limit (tolerant to int/float64/json.Number via helper).
	offset, _ := normalizeInt(input["offset"])
	limit, _ := normalizeInt(input["limit"])

	startLine := offset
	if startLine < 0 {
		startLine = 0
	}
	if startLine >= totalLines {
		return map[string]any{
			"ok":         true,
			"path":       path,
			"content":    "",
			"offset":     offset,
			"limit":      limit,
			"truncated":  false,
			"totalLines": totalLines,
			"readLines":  0,
		}, nil
	}

	endLine := totalLines
	truncated := false
	if limit > 0 {
		endLine = startLine + limit
		if endLine > totalLines {
			endLine = totalLines
		} else {
			truncated = true
		}
	}

	selectedLines := lines[startLine:endLine]
	resultContent := strings.Join(selectedLines, "\n")

	// Optional: Truncate huge files to prevent context overflow
	const maxRunes = 50000
	if len(resultContent) > maxRunes {
		resultContent = resultContent[:maxRunes] + fmt.Sprintf("\n... (truncated, %d chars omitted)", len(resultContent)-maxRunes)
		truncated = true
	}

	return map[string]any{
		"ok":         true,
		"path":       path,
		"content":    resultContent,
		"offset":     offset,
		"limit":      limit,
		"truncated":  truncated,
		"totalLines": totalLines,
		"readLines":  len(selectedLines),
	}, nil
}

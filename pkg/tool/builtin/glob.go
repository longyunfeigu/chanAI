package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"giai/pkg/tool"
	"github.com/bmatcuk/doublestar/v4"
)

type Glob struct {
	tool.BaseTool
}

// GlobResult keeps output shape stable even when truncating results.
type GlobResult struct {
	Matches      []string `json:"matches"`
	TotalMatches int      `json:"total_matches"`
	Truncated    bool     `json:"truncated,omitempty"`
	Warning      string   `json:"warning,omitempty"`
}

func NewGlob() *Glob {
	t := &Glob{
		BaseTool: tool.NewBaseTool(
			"glob",
			"Find files matching glob patterns. Supports wildcards like **/*.go.",
		),
	}

	t.TimeoutVal = 30 * time.Second

	t.SchemaVal = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "The glob pattern to match (e.g., 'src/**/*.ts').",
			},
			"root_dir": map[string]any{
				"type":        "string",
				"description": "The root directory to start searching from (defaults to current dir).",
			},
			"exclude": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
				"description": "List of patterns to exclude.",
			},
		},
		"required": []string{"pattern"},
	}

	return t
}

func (t *Glob) Execute(ctx context.Context, input map[string]any, tc *tool.ToolContext) (any, error) {
	pattern, ok := input["pattern"].(string)
	if !ok {
		return nil, fmt.Errorf("pattern must be a string")
	}

	rootDir, _ := input["root_dir"].(string)
	if rootDir == "" {
		rootDir = "."
	}

	// Handle exclusion
	var excludePatterns []string
	if excludes, ok := input["exclude"].([]any); ok {
		for _, e := range excludes {
			if s, ok := e.(string); ok {
				excludePatterns = append(excludePatterns, s)
			}
		}
	}

	// We use doublestar library for advanced matching (including **)
	// Ensure it is installed: go get github.com/bmatcuk/doublestar/v4
	fsys := os.DirFS(rootDir)

	matches, err := doublestar.Glob(fsys, pattern)
	if err != nil {
		return nil, fmt.Errorf("glob failed: %w", err)
	}

	// Filter exclusions
	var finalMatches []string
	for _, m := range matches {
		excluded := false
		for _, excl := range excludePatterns {
			if matched, _ := doublestar.Match(excl, m); matched {
				excluded = true
				break
			}
		}
		if !excluded {
			// Return absolute path if possible for clarity, or relative to root
			if absRoot, err := filepath.Abs(rootDir); err == nil {
				finalMatches = append(finalMatches, filepath.Join(absRoot, m))
			} else {
				finalMatches = append(finalMatches, m)
			}
		}
	}

	// Limit results to avoid context overflow
	const maxResults = 1000
	result := &GlobResult{
		TotalMatches: len(finalMatches),
		Matches:      finalMatches,
	}

	if len(finalMatches) > maxResults {
		result.Matches = finalMatches[:maxResults]
		result.Truncated = true
		result.Warning = fmt.Sprintf("Too many matches, truncated to %d", maxResults)
	}

	return result, nil
}

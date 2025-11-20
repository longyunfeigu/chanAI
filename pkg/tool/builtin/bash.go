package builtin

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"giai/pkg/tool"
)

type Bash struct {
	tool.BaseTool
}

func NewBash() *Bash {
	t := &Bash{
		BaseTool: tool.NewBaseTool(
			"bash",
			"Execute a bash command on the system. Use with caution.",
		),
	}
	
	// Set a default timeout for safety
	t.TimeoutVal = 2 * time.Minute

	t.SchemaVal = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The bash command to execute.",
			},
			"work_dir": map[string]any{
				"type":        "string",
				"description": "The working directory for the command (optional).",
			},
		},
		"required": []string{"command"},
	}
	
	return t
}

func (t *Bash) Execute(ctx context.Context, input map[string]any, tc *tool.ToolContext) (any, error) {
	cmdStr, ok := input["command"].(string)
	if !ok {
		return nil, fmt.Errorf("command must be a string")
	}

	workDir, _ := input["work_dir"].(string)

	// Create the command
	// We use "bash -c" to allow pipes and complex commands
	cmd := exec.CommandContext(ctx, "bash", "-c", cmdStr)
	
	if workDir != "" {
		cmd.Dir = workDir
	}

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	
	// Prepare output
	result := map[string]any{
		"stdout": stdout.String(),
		"stderr": stderr.String(),
		"code":   0,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result["code"] = exitErr.ExitCode()
		} else {
			result["code"] = -1 // Unknown error or signal
			result["error"] = err.Error()
		}
	}

	return result, nil
}

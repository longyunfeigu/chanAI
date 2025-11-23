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

// NewBash 创建 bash 工具。
// 签名与 agentsdk 风格保持一致：接受 config 并返回 (tool.Tool, error)，当前实现忽略 config。
func NewBash(config map[string]any) (tool.Tool, error) {
	t := &Bash{
		BaseTool: tool.NewBaseTool(
			"bash",
			"Execute bash commands on the host system. Use with caution.",
		),
	}
	
	// Set a default timeout for safety
	t.TimeoutVal = 2 * time.Minute

	t.SchemaVal = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"cmd": map[string]any{
				"type":        "string",
				"description": "Command to execute (runs via 'bash -c').",
			},
			"work_dir": map[string]any{
				"type":        "string",
				"description": "The working directory for the command (optional).",
			},
			"timeout_ms": map[string]any{
				"type":        "integer",
				"description": "Optional per-call timeout in milliseconds (if the executor respects it).",
			},
		},
		"required": []any{"cmd"},
	}

	t.PromptVal = `Execute bash commands on the host system.

Guidelines:
- Use this tool when you need to inspect files, run CLIs, or gather diagnostics.
- Always pass a single string in "cmd"; it is executed via "bash -c".
- You may set "work_dir" to change the working directory.
- Long-running or destructive commands are discouraged; prefer short, read-oriented commands.

Safety/Limitations:
- Commands are subject to an overall timeout (default 2 minutes here, or executor-level timeout).
- Non-zero exit codes do not raise Go errors; instead, the result includes ok=false and the exit code.
- Output combines stdout and stderr for easier inspection.`
	
	return t, nil
}

func (t *Bash) Execute(ctx context.Context, input map[string]any, tc *tool.ToolContext) (any, error) {
	// Prefer "cmd" (aligned with agentsdk's BashRunTool), but keep "command" as a backward-compatible alias.
	cmdStr, ok := input["cmd"].(string)
	if !ok || cmdStr == "" {
		if legacy, okLegacy := input["command"].(string); okLegacy {
			cmdStr = legacy
			ok = true
		}
	}
	if !ok || cmdStr == "" {
		return nil, fmt.Errorf("cmd must be a non-empty string")
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

	// Merge output similar to agentsdk's BashRunTool
	outStr := stdout.String()
	if stderr.Len() > 0 {
		if outStr != "" {
			outStr += "\n"
		}
		outStr += stderr.String()
	}
	if outStr == "" {
		outStr = "(no output)"
	}

	exitCode := 0
	okExec := true
	if err != nil {
		if exitErr, isExit := err.(*exec.ExitError); isExit {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
		okExec = exitCode == 0
	}

	resp := map[string]any{
		"ok":     okExec,
		"code":   exitCode,
		"output": outStr,
	}

	if err != nil {
		// Provide a structured error message and some generic recommendations,
		// mirroring the agentsdk style while working on the real host.
		resp["error"] = fmt.Sprintf("command failed: %v", err)
		resp["recommendations"] = []string{
			"检查命令语法是否正确",
			"确认命令在当前环境中可执行",
			"验证是否有足够的执行权限",
		}
	}

	return resp, nil
}

// Package tools provides built-in tool implementations for the Gemini CLI.
// Copyright 2025 linkalls
// SPDX-License-Identifier: Apache-2.0
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// shellPath is the global shell path used for executing commands
var shellPath string = ""

// SetShellPath sets the shell path for command execution
func SetShellPath(path string) {
	shellPath = path
}

// GetShellPath returns the current shell path
func GetShellPath() string {
	return shellPath
}

// =============================================================================
// ShellTool - Execute shell commands
// =============================================================================

// ShellTool executes shell commands
type ShellTool struct {
	rootDir string
}

func (t *ShellTool) Name() string        { return "shell" }
func (t *ShellTool) DisplayName() string { return "Shell" }
func (t *ShellTool) Description() string {
	return "Execute a shell command and return its output. Use this for running system commands, scripts, or CLI tools. Be cautious with destructive commands."
}

func (t *ShellTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "The shell command to execute"
			},
			"timeout": {
				"type": "integer",
				"description": "Timeout in seconds (default: 60, max: 300)"
			}
		},
		"required": ["command"]
	}`)
}

func (t *ShellTool) RequiresConfirmation() bool { return true }
func (t *ShellTool) ConfirmationType() string   { return "shell" }

func (t *ShellTool) Execute(args map[string]interface{}) (map[string]interface{}, error) {
	command, ok := args["command"].(string)
	if !ok || strings.TrimSpace(command) == "" {
		return map[string]interface{}{"error": "command is required and cannot be empty"}, nil
	}

	// Get timeout (default 60 seconds, max 300 seconds)
	timeout := 60
	if t, ok := args["timeout"].(float64); ok {
		timeout = int(t)
		if timeout <= 0 {
			timeout = 60
		}
		if timeout > 300 {
			timeout = 300
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	// Use custom shell path if set, otherwise use defaults
	if shellPath != "" {
		if strings.Contains(shellPath, "bash") {
			cmd = exec.CommandContext(ctx, shellPath, "-c", command)
		} else if strings.Contains(shellPath, "powershell") || shellPath == "powershell" {
			cmd = exec.CommandContext(ctx, shellPath, "-NoProfile", "-NonInteractive", "-Command", command)
		} else {
			// Generic shell
			cmd = exec.CommandContext(ctx, shellPath, "-c", command)
		}
	} else if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", command)
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	}

	// Set working directory
	if t.rootDir != "" {
		cmd.Dir = t.rootDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	result := map[string]interface{}{
		"command":     command,
		"duration_ms": duration.Milliseconds(),
	}

	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	// Truncate output if too long
	const maxOutput = 50000
	if len(stdoutStr) > maxOutput {
		stdoutStr = stdoutStr[:maxOutput] + "\n[Output truncated...]"
	}
	if len(stderrStr) > maxOutput {
		stderrStr = stderrStr[:maxOutput] + "\n[Output truncated...]"
	}

	result["stdout"] = stdoutStr
	result["stderr"] = stderrStr

	if ctx.Err() == context.DeadlineExceeded {
		result["error"] = fmt.Sprintf("command timed out after %d seconds", timeout)
		result["exit_code"] = -1
		return result, nil
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result["exit_code"] = exitErr.ExitCode()
		} else {
			result["error"] = err.Error()
			result["exit_code"] = -1
		}
	} else {
		result["exit_code"] = 0
	}

	return result, nil
}

func (t *ShellTool) SetRootDir(dir string) {
	t.rootDir = dir
}

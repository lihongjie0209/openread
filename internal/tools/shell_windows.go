//go:build windows

package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func runShell(command, cwd string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "cmd", "/C", command)
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		if ctx.Err() != nil {
			return "Error: command timed out (30 s limit)"
		}
		output += fmt.Sprintf("\n[error]: %v", err)
	}
	if len(output) > 10000 {
		output = output[:10000] + "\n...(truncated)"
	}
	result := strings.TrimSpace(output)
	if result == "" {
		return "(no output)"
	}
	return result
}

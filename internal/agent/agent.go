// Package agent implements a generic ReAct (tool-calling) loop compatible
// with any OpenAI-compatible API (DeepSeek, OpenAI, etc.).
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

const maxIterations = 50

// Config holds everything needed to run one agent invocation.
type Config struct {
	Client       *openai.Client
	Model        string
	SystemPrompt string
	UserPrompt   string
	Tools        []openai.Tool
	Handlers     map[string]func(args map[string]any) string
	// Verbose enables default fmt.Printf output for tool calls.
	Verbose bool
	// OnToolCall is called instead of (or in addition to) Verbose printing.
	// If set, Verbose fmt.Printf is suppressed.
	OnToolCall func(name, args, result string)
	// OnStatus is called with real-time status strings like "[requesting]",
	// "[tool: list_dir]", "[answering]" for display in the TUI.
	OnStatus func(status string)
}

// Run executes the multi-turn tool-calling ReAct loop and returns the final
// text content from the last AI message.
func Run(ctx context.Context, cfg Config) (string, error) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: cfg.SystemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: cfg.UserPrompt},
	}

	for i := 0; i < maxIterations; i++ {
		req := openai.ChatCompletionRequest{
			Model:       cfg.Model,
			Messages:    messages,
			Temperature: 0,
		}
		if len(cfg.Tools) > 0 {
			req.Tools = cfg.Tools
		}

		if cfg.OnStatus != nil {
			cfg.OnStatus("[requesting]")
		}
		resp, err := cfg.Client.CreateChatCompletion(ctx, req)
		if err != nil {
			return "", fmt.Errorf("API call failed (turn %d): %w", i+1, err)
		}
		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("no choices in API response (turn %d)", i+1)
		}

		choice := resp.Choices[0]
		msg := choice.Message
		messages = append(messages, msg)

		// No tool calls → final answer
		if len(msg.ToolCalls) == 0 {
			if cfg.OnStatus != nil {
				cfg.OnStatus("[answering]")
			}
			return msg.Content, nil
		}

		if cfg.OnStatus != nil {
			cfg.OnStatus("[thinking]")
		}

		// Execute all tool calls
		for _, tc := range msg.ToolCalls {
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = map[string]any{}
			}

			if cfg.OnStatus != nil {
				cfg.OnStatus("[tool: " + tc.Function.Name + "]")
			}

			var result string
			if handler, ok := cfg.Handlers[tc.Function.Name]; ok {
				result = handler(args)
			} else {
				result = fmt.Sprintf("Error: unknown tool '%s'", tc.Function.Name)
			}

			if cfg.OnToolCall != nil {
				cfg.OnToolCall(tc.Function.Name, tc.Function.Arguments, result)
			} else if cfg.Verbose {
				fmt.Printf("  \033[36m[tool]\033[0m %s(%s)\n",
					tc.Function.Name, abbrev(tc.Function.Arguments, 120))
				fmt.Printf("  \033[2m→ %s\033[0m\n", abbrev(result, 200))
			}

			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	return "", fmt.Errorf("agent exceeded maximum iterations (%d)", maxIterations)
}

func abbrev(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", "↵")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

package agent

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	"github.com/lihongjie0209/openread/internal/models"
	"github.com/lihongjie0209/openread/internal/prompts"
	"github.com/lihongjie0209/openread/internal/tools"
)

// RunCatalog explores the repository and returns a structured Wiki catalog.
func RunCatalog(
	ctx context.Context,
	client *openai.Client,
	model string,
	workDir string,
	language string,
	verbose bool,
	onToolCall func(name, args, result string),
	onStatus func(status string),
) (*models.Wiki, error) {
	osName := "linux"
	if runtime.GOOS == "windows" {
		osName = "windows"
	} else if runtime.GOOS == "darwin" {
		osName = "macos"
	}

	ts := tools.New(workDir)

	// Get top-level structure for the user prompt
	structure := ts.Handle["get_dir_structure"](map[string]any{"dir_path": ".", "max_depth": float64(2)})

	systemPrompt := fmt.Sprintf(prompts.CatalogSystem, workDir, osName)
	userPrompt := fmt.Sprintf(prompts.CatalogUser, workDir, osName, language, structure)

	raw, err := Run(ctx, Config{
		Client:       client,
		Model:        model,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Tools:        ts.Tools,
		Handlers:     ts.Handle,
		Verbose:      verbose,
		OnToolCall:   onToolCall,
		OnStatus:     onStatus,
	})
	if err != nil {
		return nil, fmt.Errorf("catalog agent failed: %w", err)
	}

	if !strings.Contains(raw, "<section>") {
		preview := raw
		if len(preview) > 500 {
			preview = preview[:500]
		}
		return nil, fmt.Errorf("catalog agent: no <section> found in LLM output.\n\n%s", preview)
	}

	wiki := models.ParseCatalogXML(raw)
	if len(wiki.Pages) == 0 {
		return nil, fmt.Errorf("parsed catalog has no pages. Raw output:\n%s", raw[:min(500, len(raw))])
	}

	return wiki, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

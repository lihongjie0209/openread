package agent

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	"zread-go/internal/models"
	"zread-go/internal/prompts"
	"zread-go/internal/tools"
)

var blogPattern = regexp.MustCompile(`(?s)<blog>(.*?)</blog>`)

// RunPage generates markdown documentation for a single wiki page.
// Returns the extracted blog content (without <blog> tags).
func RunPage(
	ctx context.Context,
	client *openai.Client,
	model string,
	workDir string,
	language string,
	page *models.WikiPage,
	wiki *models.Wiki,
	verbose bool,
	onToolCall func(name, args, result string),
	onStatus func(status string),
) (string, error) {
	osName := "linux"
	if runtime.GOOS == "windows" {
		osName = "windows"
	} else if runtime.GOOS == "darwin" {
		osName = "macos"
	}

	ts := tools.New(workDir)
	structure := ts.Handle["get_dir_structure"](map[string]any{"dir_path": ".", "max_depth": float64(2)})
	catalog := wiki.FormatCatalog(page.Slug)

	systemPrompt := fmt.Sprintf(prompts.PageSystem, workDir, osName)
	userPrompt := fmt.Sprintf(prompts.PageUser,
		workDir, osName,
		page.Title, page.Level, language,
		structure, catalog,
		page.Title, page.Title, page.Title,
	)

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
		return "", err
	}

	return extractBlog(raw), nil
}

func extractBlog(text string) string {
	if m := blogPattern.FindStringSubmatch(text); m != nil {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(text)
}

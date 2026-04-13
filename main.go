// zread-go — openread, open-source Go reimplementation of zread_cli
package main

import (
"fmt"
"os"
"path/filepath"
"runtime"

"github.com/spf13/cobra"
"github.com/lihongjie0209/openread/internal/browse"
"github.com/lihongjie0209/openread/internal/config"
"github.com/lihongjie0209/openread/internal/runner"
)

var appVersion = "0.1.0"

func main() {
root := &cobra.Command{
Use:          "openread",
Short:        "Turn any codebase into a beautiful wiki",
Long:         "openread — open-source reimplementation of zread_cli.\nGenerates structured wiki documentation from your local codebase using an AI agent.\nSee https://github.com/lihongjie0209/openread for docs.",
SilenceUsage: true,
}
root.AddCommand(
newGenerateCmd(),
newConfigCmd(),
newBrowseCmd(),
newVersionCmd(),
)
if err := root.Execute(); err != nil {
os.Exit(1)
}
}

// ── generate ──────────────────────────────────────────────────────────────────

func newGenerateCmd() *cobra.Command {
var (
dir            string
draft          string
skipFailed     bool
yes            bool
lang           string
workers        int
verboseCatalog bool // show catalog tool calls (disables TUI)
verbosePages   bool // show page tool calls (disables TUI)
retries        int
)

cmd := &cobra.Command{
Use:          "generate",
Short:        "Generate wiki documentation for the current workspace",
SilenceUsage: true,
RunE: func(cmd *cobra.Command, args []string) error {
cfg, err := config.Load()
if err != nil {
return err
}
if cfg.APIKey == "" {
return fmt.Errorf("API key not configured — run: zread config")
}

if cmd.Flags().Changed("lang") {
cfg.Language = lang
}
if cmd.Flags().Changed("workers") {
cfg.Workers = workers
}

targetDir := dir
if targetDir == "" {
targetDir, _ = os.Getwd()
}
absDir, err := filepath.Abs(targetDir)
if err != nil {
return err
}

return runner.Run(runner.Config{
APIKey:         cfg.APIKey,
BaseURL:        cfg.BaseURL,
Model:          cfg.Model,
WorkDir:        absDir,
Language:       cfg.Language,
Workers:        cfg.Workers,
Draft:          draft,
SkipFailed:     skipFailed,
AutoYes:        yes,
VerboseCatalog: verboseCatalog,
VerbosePages:   verbosePages,
MaxRetries:     retries,
})
},
}

f := cmd.Flags()
f.StringVar(&draft, "draft", "", "Action for existing draft: resume, clear, or cancel")
f.BoolVar(&skipFailed, "skip-failed", false, "Automatically skip failed pages and commit remaining wiki")
f.BoolVarP(&yes, "yes", "y", false, "Skip all confirmations and generate immediately")
f.BoolVar(&verbosePages, "verbose-pages", false, "Show page agent tool calls (disables TUI)")
f.BoolVar(&verboseCatalog, "verbose-catalog", false, "Show catalog agent tool calls (disables TUI)")
f.IntVar(&retries, "retries", 1, "Max retries per page on failure")
f.StringVar(&dir, "dir", "", "Target directory (default: current working directory)")
f.StringVar(&lang, "lang", "", "Override documentation language (e.g. Chinese, English)")
f.IntVar(&workers, "workers", 0, "Override concurrent worker count")

return cmd
}

// ── config ────────────────────────────────────────────────────────────────────

func newConfigCmd() *cobra.Command {
var (
apiKey   string
baseURL  string
model    string
language string
workers  int
retries  int
)

cmd := &cobra.Command{
Use:          "config",
Short:        "View or modify openread configuration",
Long:         "View or modify settings stored in ~/.zread/config.yaml.",
SilenceUsage: true,
RunE: func(cmd *cobra.Command, args []string) error {
raw := config.LoadRaw()

changed := false
if cmd.Flags().Changed("api-key") {
raw.LLM.APIKey = apiKey
changed = true
}
if cmd.Flags().Changed("base-url") {
raw.LLM.BaseURL = baseURL
changed = true
}
if cmd.Flags().Changed("model") {
raw.LLM.Model = model
changed = true
}
if cmd.Flags().Changed("lang") {
raw.Language = language
raw.DocLanguage = language
changed = true
}
if cmd.Flags().Changed("workers") {
raw.Concurrency.MaxConcurrent = workers
changed = true
}
if cmd.Flags().Changed("retries") {
raw.Concurrency.MaxRetries = retries
changed = true
}

if changed {
if err := config.Save(raw); err != nil {
return fmt.Errorf("failed to save config: %w", err)
}
fmt.Println("✓ Config saved →", config.ConfigPath())
} else {
printConfig(raw)
}
return nil
},
}

f := cmd.Flags()
f.StringVar(&apiKey, "api-key", "", "Set API key (llm.api_key)")
f.StringVar(&baseURL, "base-url", "", "Set API base URL (llm.base_url, default: https://api.deepseek.com/v1)")
f.StringVar(&model, "model", "", "Set model name (llm.model, default: deepseek-chat)")
f.StringVar(&language, "lang", "", "Set language code (language / doc_language, e.g. zh / en)")
f.IntVar(&workers, "workers", 0, "Set max concurrency (concurrency.max_concurrent, default: 1)")
f.IntVar(&retries, "retries", 0, "Set max retries (concurrency.max_retries, default: 1)")

return cmd
}

func printConfig(raw *config.Config) {
masked := raw.LLM.APIKey
if len(masked) > 8 {
masked = masked[:4] + "****" + masked[len(masked)-4:]
} else if len(masked) > 0 {
masked = "****"
} else {
masked = "(not set)"
}
fmt.Printf("Config (%s):\n", config.ConfigPath())
fmt.Printf("  language:                   %s\n", raw.Language)
fmt.Printf("  doc_language:               %s\n", raw.DocLanguage)
fmt.Printf("  llm.provider:               %s\n", raw.LLM.Provider)
fmt.Printf("  llm.model:                  %s\n", raw.LLM.Model)
fmt.Printf("  llm.api_key:                %s\n", masked)
fmt.Printf("  llm.base_url:               %s\n", raw.LLM.BaseURL)
fmt.Printf("  concurrency.max_concurrent: %d\n", raw.Concurrency.MaxConcurrent)
fmt.Printf("  concurrency.max_retries:    %d\n", raw.Concurrency.MaxRetries)
}

// ── browse ────────────────────────────────────────────────────────────────────

func newBrowseCmd() *cobra.Command {
var (
dir  string
port int
open bool
bld  bool
)

cmd := &cobra.Command{
Use:          "browse",
Short:        "Browse generated wiki docs in your browser",
Long:         "Start a local HTTP server to browse the wiki generated by openread generate.\nNo Node.js required. Use --build to export a static HTML site.",
SilenceUsage: true,
RunE: func(cmd *cobra.Command, args []string) error {
targetDir := dir
if targetDir == "" {
targetDir, _ = os.Getwd()
}
absDir, _ := filepath.Abs(targetDir)
siteDir := filepath.Join(absDir, ".zread", "site")
return browse.Run(absDir, siteDir, port, open, bld)
},
}

f := cmd.Flags()
f.StringVar(&dir, "dir", "", "Project directory (default: current directory)")
f.IntVar(&port, "port", 3000, "Local server port")
f.BoolVar(&open, "open", true, "Auto-open in browser")
f.BoolVar(&bld, "build", false, "Build static site (no server)")

return cmd
}

// ── version ───────────────────────────────────────────────────────────────────

func newVersionCmd() *cobra.Command {
return &cobra.Command{
Use:   "version",
Short: "Print version information",
Run: func(cmd *cobra.Command, args []string) {
fmt.Print(`
  ██████╗ ██████╗ ███████╗███╗   ██╗██████╗ ███████╗ █████╗ ██████╗
 ██╔═══██╗██╔══██╗██╔════╝████╗  ██║██╔══██╗██╔════╝██╔══██╗██╔══██╗
 ██║   ██║██████╔╝█████╗  ██╔██╗ ██║██████╔╝█████╗  ███████║██║  ██║
 ██║   ██║██╔═══╝ ██╔══╝  ██║╚██╗██║██╔══██╗██╔══╝  ██╔══██║██║  ██║
 ╚██████╔╝██║     ███████╗██║ ╚████║██║  ██║███████╗██║  ██║██████╔╝
  ╚═════╝ ╚═╝     ╚══════╝╚═╝  ╚═══╝╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝╚═════╝
         Turn any codebase into a beautiful wiki
──────────────────────────────────────────────────────────────────────
`)
fmt.Printf("Version:    %s\n", appVersion)
fmt.Printf("Go:         %s\n", runtime.Version())
fmt.Printf("OS:         %s\n", runtime.GOOS)
fmt.Printf("Arch:       %s\n", runtime.GOARCH)
},
}
}

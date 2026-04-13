// Package runner orchestrates the two-phase documentation pipeline.
package runner

import (
"bufio"
"context"
"encoding/json"
"fmt"
"os"
"path/filepath"
"strings"
"sync"
"sync/atomic"
"time"

tea "github.com/charmbracelet/bubbletea"
openai "github.com/sashabaranov/go-openai"
"github.com/lihongjie0209/openread/internal/agent"
"github.com/lihongjie0209/openread/internal/models"
"github.com/lihongjie0209/openread/internal/tui"
)

// Config holds all settings for a single documentation run.
type Config struct {
APIKey  string
BaseURL string
Model   string

WorkDir  string
Language string
Workers  int

OutputDir string

Draft      string // "resume" | "clear" | "cancel" | "" (auto-detect)
SkipFailed bool
AutoYes    bool // -y flag: skip interactive prompts

VerboseCatalog bool
VerbosePages   bool
MaxRetries     int
}

// draftsDir returns the path to the drafts directory.
func draftsDir(workDir string) string {
return filepath.Join(workDir, ".zread", "wiki", "drafts")
}

// versionsDir returns the path to the versions directory.
func versionsDir(workDir string) string {
return filepath.Join(workDir, ".zread", "wiki", "versions")
}

// currentFile returns the path to the current-version pointer file.
func currentFile(workDir string) string {
return filepath.Join(workDir, ".zread", "wiki", "current")
}

// hasDraft returns true when wiki.json exists in drafts.
func hasDraft(workDir string) bool {
_, err := os.Stat(filepath.Join(draftsDir(workDir), "wiki.json"))
return err == nil
}

// Run executes the full documentation generation pipeline.
func Run(cfg Config) error {
if cfg.APIKey == "" {
return fmt.Errorf("API key not configured, run: zread config")
}

oaiCfg := openai.DefaultConfig(cfg.APIKey)
oaiCfg.BaseURL = cfg.BaseURL
client := openai.NewClientWithConfig(oaiCfg)

outPath := cfg.OutputDir
if outPath == "" {
outPath = draftsDir(cfg.WorkDir)
}

// ── Draft state detection & interactive prompt ──────────────────────
action := cfg.Draft
if hasDraft(cfg.WorkDir) && action == "" {
if cfg.AutoYes {
action = "resume"
fmt.Println("Draft found, resuming (-y)")
} else {
var err error
action, err = promptDraftAction(cfg.WorkDir)
if err != nil {
return err
}
}
}

switch action {
case "cancel":
fmt.Println("Generation cancelled.")
return nil
case "clear":
fmt.Printf("Clearing draft: %s\n", outPath)
if err := clearDir(outPath); err != nil {
return err
}
action = ""
}

if err := os.MkdirAll(outPath, 0o755); err != nil {
return fmt.Errorf("cannot create output dir: %w", err)
}

// Choose TUI or plain output
useTUI := !cfg.VerboseCatalog && !cfg.VerbosePages
if useTUI {
return tuiRun(client, cfg, action, outPath)
}
return plainRun(client, cfg, action, outPath)
}

// ── TUI run ────────────────────────────────────────────────────────────────────

func tuiRun(client *openai.Client, cfg Config, action, outPath string) error {
ctx := context.Background()

// Channels for in-session retry / skip communication from TUI → runner.
retryCh := make(chan string, numWorkers(cfg.Workers)+4)
skipCh := make(chan struct{}, 1)

model := tui.NewWithChannels(retryCh, skipCh)
prog := tea.NewProgram(model, tea.WithAltScreen())

var runErr error

go func() {
// ── Phase 1: Catalog ──────────────────────────────────────────────
prog.Send(tui.CatalogStartMsg{})

var wiki *models.Wiki

if action == "resume" {
loaded, err := loadDraftWiki(outPath)
if err != nil {
runErr = fmt.Errorf("cannot load draft wiki.json: %w", err)
prog.Quit()
return
}
wiki = loaded
total := len(wiki.Pages)
done := countDonePages(outPath, wiki)
prog.Send(tui.CatalogStatusMsg{Status: fmt.Sprintf("[resumed: %d/%d pages done]", done, total)})
prog.Send(tui.CatalogDoneMsg{Pages: total, Sections: 0})
} else {
onStatus := func(s string) { prog.Send(tui.CatalogStatusMsg{Status: s}) }
onToolCall := func(name, args, result string) {
// Suppress individual tool-call lines in TUI mode; status updates suffice
_ = name; _ = args; _ = result
}

var err error
wiki, err = agent.RunCatalog(ctx, client, cfg.Model, cfg.WorkDir, cfg.Language, false, onToolCall, onStatus)
if err != nil {
runErr = err
prog.Quit()
return
}
if err2 := saveWiki(wiki, outPath); err2 != nil {
runErr = err2
prog.Quit()
return
}
sections := map[string]bool{}
for _, p := range wiki.Pages {
sections[p.Section] = true
}
prog.Send(tui.CatalogDoneMsg{Pages: len(wiki.Pages), Sections: len(sections)})
}

// ── Phase 2: Pages ────────────────────────────────────────────────
pages := wiki.Pages
if action == "resume" {
pages = missingPages(outPath, wiki)
}

rows := make([]tui.PageRow, len(pages))
for i, p := range pages {
rows[i] = tui.PageRow{Idx: i + 1, Title: p.Title, Slug: p.Slug}
}
prog.Send(tui.PagesInitMsg{Pages: rows})

// Map slug → WikiPage for lookup during retries.
pageBySlug := make(map[string]models.WikiPage, len(wiki.Pages))
for _, p := range wiki.Pages {
pageBySlug[p.Slug] = p
}

// Run initial batch of pages.
failedSet := make(map[string]bool)
var failedMu sync.Mutex
runPagesTracked(ctx, client, cfg, pages, wiki, outPath, prog, &failedSet, &failedMu)

// ── Phase 3: Retry loop / Commit ─────────────────────────────────
failedMu.Lock()
hasFailed := len(failedSet) > 0
failedMu.Unlock()

if !hasFailed {
// All succeeded — commit and finish.
versionID, err := commitDraft(cfg.WorkDir, outPath, wiki)
if err != nil {
runErr = err
prog.Quit()
return
}
prog.Send(tui.GenerationDoneMsg{VersionID: versionID, TotalPages: len(wiki.Pages)})
return
}

// Has failures — stay in retry loop until user skips or all retried.
var activeRetries int32
retryAllDone := make(chan struct{}, 1)

checkAllDone := func() {
failedMu.Lock()
f := len(failedSet)
failedMu.Unlock()
if f == 0 && atomic.LoadInt32(&activeRetries) == 0 {
select {
case retryAllDone <- struct{}{}:
default:
}
}
}

for {
select {
case slug := <-retryCh:
// User pressed r on a specific failed page — retry it.
failedMu.Lock()
delete(failedSet, slug)
failedMu.Unlock()

atomic.AddInt32(&activeRetries, 1)
go func(s string) {
defer func() {
atomic.AddInt32(&activeRetries, -1)
checkAllDone()
}()

prog.Send(tui.PageStartMsg{Slug: s})
p := pageBySlug[s]
onStatus := func(status string) {
prog.Send(tui.PageStatusMsg{Slug: s, Status: status})
}
content, err := agent.RunPage(ctx, client, cfg.Model, cfg.WorkDir, cfg.Language,
&p, wiki, false, nil, onStatus)
if err != nil {
failedMu.Lock()
failedSet[s] = true
failedMu.Unlock()
prog.Send(tui.PageFailedMsg{Slug: s, Err: err.Error()})
return
}
if err2 := savePage(s, content, outPath); err2 != nil {
failedMu.Lock()
failedSet[s] = true
failedMu.Unlock()
prog.Send(tui.PageFailedMsg{Slug: s, Err: err2.Error()})
return
}
prog.Send(tui.PageDoneMsg{Slug: s})
}(slug)

case <-skipCh:
// User pressed s — skip remaining failures and commit.
versionID, err := commitDraft(cfg.WorkDir, outPath, wiki)
if err != nil {
runErr = err
prog.Quit()
return
}
prog.Send(tui.GenerationDoneMsg{VersionID: versionID, TotalPages: len(wiki.Pages)})
return

case <-retryAllDone:
// All failures have been retried and succeeded.
versionID, err := commitDraft(cfg.WorkDir, outPath, wiki)
if err != nil {
runErr = err
prog.Quit()
return
}
prog.Send(tui.GenerationDoneMsg{VersionID: versionID, TotalPages: len(wiki.Pages)})
return
}
}
}()

if _, err := prog.Run(); err != nil {
return err
}
if runErr != nil {
return runErr
}
fmt.Printf("\n\033[32m✓ Wiki generation complete\033[0m\n")
return nil
}

// runPagesTracked runs pages in parallel, recording failures into failedSet.
func runPagesTracked(ctx context.Context, client *openai.Client, cfg Config, pages []models.WikiPage, wiki *models.Wiki, outPath string, prog *tea.Program, failedSet *map[string]bool, failedMu *sync.Mutex) {
w := numWorkers(cfg.Workers)
sem := make(chan struct{}, w)
var wg sync.WaitGroup

for _, page := range pages {
p := page
wg.Add(1)
sem <- struct{}{}
go func() {
defer wg.Done()
defer func() { <-sem }()

prog.Send(tui.PageStartMsg{Slug: p.Slug})
onStatus := func(status string) {
prog.Send(tui.PageStatusMsg{Slug: p.Slug, Status: status})
}

maxAttempts := cfg.MaxRetries + 1
var lastErr error
for attempt := 1; attempt <= maxAttempts; attempt++ {
if attempt > 1 {
prog.Send(tui.PageRetryingMsg{Slug: p.Slug})
}
content, err := agent.RunPage(ctx, client, cfg.Model, cfg.WorkDir, cfg.Language,
&p, wiki, false, nil, onStatus)
if err != nil {
lastErr = err
continue
}
if err2 := savePage(p.Slug, content, outPath); err2 != nil {
lastErr = err2
continue
}
prog.Send(tui.PageDoneMsg{Slug: p.Slug})
return
}
failedMu.Lock()
(*failedSet)[p.Slug] = true
failedMu.Unlock()
prog.Send(tui.PageFailedMsg{Slug: p.Slug, Err: lastErr.Error()})
}()
}
wg.Wait()
}

// ── Plain run (--quiet / --verbose-pages) ──────────────────────────────────────

func plainRun(client *openai.Client, cfg Config, action, outPath string) error {
ctx := context.Background()

var wiki *models.Wiki

if action == "resume" {
loaded, err := loadDraftWiki(outPath)
if err != nil {
return fmt.Errorf("cannot load draft wiki.json, try --draft clear: %w", err)
}
wiki = loaded
total := len(wiki.Pages)
done := countDonePages(outPath, wiki)
fmt.Printf("Resuming draft: %d/%d pages done, %d remaining\n", done, total, total-done)
} else {
var verboseFn func(string, string, string)
if cfg.VerboseCatalog {
verboseFn = tui.PlainLogFunc()
}
fmt.Printf("\033[34mPhase 1 — Generate Catalog\033[0m\n")
var err error
wiki, err = agent.RunCatalog(ctx, client, cfg.Model, cfg.WorkDir, cfg.Language, false, verboseFn, nil)
if err != nil {
return err
}
if err := saveWiki(wiki, outPath); err != nil {
return err
}
sections := map[string]bool{}
for _, p := range wiki.Pages {
sections[p.Section] = true
}
fmt.Printf("\033[32m  [done] — %d pages across %d sections\033[0m\n",
len(wiki.Pages), len(sections))
}

fmt.Printf("\n\033[34mPhase 2 — Generate Pages (%d total)\033[0m\n", len(wiki.Pages))

pages := wiki.Pages
if action == "resume" {
pages = missingPages(outPath, wiki)
}

w := numWorkers(cfg.Workers)
fmt.Printf("\033[2m%d pages, %d workers\033[0m\n", len(pages), w)

var verbosePageFn func(string, string, string)
if cfg.VerbosePages {
verbosePageFn = tui.PlainLogFunc()
}

if w <= 1 {
runSerialPlain(ctx, client, cfg, pages, wiki, outPath, verbosePageFn)
} else {
runParallelPlain(ctx, client, cfg, pages, wiki, outPath, verbosePageFn)
}

versionID, err := commitDraft(cfg.WorkDir, outPath, wiki)
if err != nil {
fmt.Printf("\033[33m⚠ commit failed: %v\033[0m\n", err)
}

fmt.Printf("\n\033[32m✓ Wiki generation complete! %d pages total\033[0m\n", len(wiki.Pages))
fmt.Printf("  Version: \033[36m%s\033[0m\n", versionID)
fmt.Printf("  Run zread browse to view your docs\n")
return nil
}

// ── page runners ───────────────────────────────────────────────────────────────

func runSerialPlain(ctx context.Context, client *openai.Client, cfg Config, pages []models.WikiPage, wiki *models.Wiki, outPath string, onToolCall func(string, string, string)) {
total := len(pages)
for i, page := range pages {
p := page
fmt.Printf("[%d/%d] %s (%s)\n", i+1, total, p.Title, p.Slug)
generateWithRetryPlain(ctx, client, cfg, wiki, &p, outPath, onToolCall)
}
}

func runParallelPlain(ctx context.Context, client *openai.Client, cfg Config, pages []models.WikiPage, wiki *models.Wiki, outPath string, onToolCall func(string, string, string)) {
sem := make(chan struct{}, numWorkers(cfg.Workers))
var wg sync.WaitGroup
for _, page := range pages {
p := page
wg.Add(1)
sem <- struct{}{}
go func() {
defer wg.Done()
defer func() { <-sem }()
generateWithRetryPlain(ctx, client, cfg, wiki, &p, outPath, onToolCall)
}()
}
wg.Wait()
}

func generateWithRetryPlain(ctx context.Context, client *openai.Client, cfg Config, wiki *models.Wiki, page *models.WikiPage, outPath string, onToolCall func(string, string, string)) {
maxAttempts := cfg.MaxRetries + 1
for attempt := 1; attempt <= maxAttempts; attempt++ {
content, err := agent.RunPage(ctx, client, cfg.Model, cfg.WorkDir, cfg.Language, page, wiki, cfg.VerbosePages, onToolCall, nil)
if err != nil {
if attempt < maxAttempts {
fmt.Printf("  \033[33m⚠ auto-retry %d/%d: %v\033[0m\n", attempt, cfg.MaxRetries, err)
continue
}
if cfg.SkipFailed {
fmt.Printf("  \033[33m⚠ skipped: %s (%v)\033[0m\n", page.Slug, err)
} else {
fmt.Printf("  \033[31m✗ failed: %v\033[0m\n", err)
}
return
}
if err2 := savePage(page.Slug, content, outPath); err2 != nil {
fmt.Printf("  \033[31m✗ save failed: %v\033[0m\n", err2)
return
}
fmt.Printf("  \033[32m✓\033[0m %s.md\n", page.Slug)
return
}
}

// ── interactive prompt ─────────────────────────────────────────────────────────────

func promptDraftAction(workDir string) (string, error) {
dDir := draftsDir(workDir)
data, _ := os.ReadFile(filepath.Join(dDir, "wiki.json"))
var w struct {
Pages []struct{ Slug string `json:"slug"` } `json:"pages"`
}
json.Unmarshal(data, &w)
total := len(w.Pages)
done := 0
for _, p := range w.Pages {
if _, e := os.Stat(filepath.Join(dDir, p.Slug+".md")); e == nil {
done++
}
}

partial := done < total
if partial {
fmt.Printf("Unfinished generation task found (Completed: %d/%d)\n", done, total)
} else {
fmt.Printf("Documentation already exists (%d pages).\n", total)
}
fmt.Println("  [r] Resume")
fmt.Println("  [c] Clear and restart")
fmt.Println("  [x] Cancel")
fmt.Print("Select [r/c/x]: ")

scanner := bufio.NewScanner(os.Stdin)
scanner.Scan()
switch strings.TrimSpace(strings.ToLower(scanner.Text())) {
case "r", "resume", "":
return "resume", nil
case "c", "clear":
return "clear", nil
default:
return "cancel", nil
}
}

// ── commit ─────────────────────────────────────────────────────────────────────

func commitDraft(workDir, outPath string, wiki *models.Wiki) (string, error) {
vDir := versionsDir(workDir)
versionID := time.Now().Format("20060102-150405")
dest := filepath.Join(vDir, versionID)
if err := os.MkdirAll(dest, 0o755); err != nil {
return "", err
}

entries, err := os.ReadDir(outPath)
if err != nil {
return "", err
}
for _, e := range entries {
if e.IsDir() {
continue
}
src := filepath.Join(outPath, e.Name())
dstFile := filepath.Join(dest, e.Name())
data, err := os.ReadFile(src)
if err != nil {
return "", err
}
if err := os.WriteFile(dstFile, data, 0o644); err != nil {
return "", err
}
}

if err := os.WriteFile(currentFile(workDir), []byte(versionID), 0o644); err != nil {
return "", err
}

_ = clearDir(outPath)
return versionID, nil
}

// ── helpers ────────────────────────────────────────────────────────────────────

func clearDir(dir string) error {
entries, err := os.ReadDir(dir)
if err != nil {
if os.IsNotExist(err) {
return nil
}
return err
}
for _, e := range entries {
_ = os.Remove(filepath.Join(dir, e.Name()))
}
return nil
}

func loadDraftWiki(outPath string) (*models.Wiki, error) {
data, err := os.ReadFile(filepath.Join(outPath, "wiki.json"))
if err != nil {
return nil, err
}
var w struct {
Pages []struct {
Title   string `json:"title"`
Slug    string `json:"slug"`
Level   string `json:"level"`
Section string `json:"section"`
Group   string `json:"group"`
} `json:"pages"`
}
if err := json.Unmarshal(data, &w); err != nil {
return nil, err
}
wiki := &models.Wiki{}
for _, p := range w.Pages {
wiki.Pages = append(wiki.Pages, models.WikiPage{
Title: p.Title, Slug: p.Slug, Level: p.Level,
Section: p.Section, Group: p.Group,
})
}
return wiki, nil
}

func missingPages(outPath string, wiki *models.Wiki) []models.WikiPage {
var out []models.WikiPage
skipped := 0
for _, p := range wiki.Pages {
if _, err := os.Stat(filepath.Join(outPath, p.Slug+".md")); err == nil {
skipped++
} else {
out = append(out, p)
}
}
if skipped > 0 {
fmt.Printf("\033[2mResuming: skipping %d already-generated pages, %d remaining\033[0m\n", skipped, len(out))
}
return out
}

func countDonePages(outPath string, wiki *models.Wiki) int {
n := 0
for _, p := range wiki.Pages {
if _, err := os.Stat(filepath.Join(outPath, p.Slug+".md")); err == nil {
n++
}
}
return n
}

func numWorkers(n int) int {
if n < 1 {
return 1
}
return n
}

func saveWiki(wiki *models.Wiki, outPath string) error {
type pageJSON struct {
Title   string `json:"title"`
Slug    string `json:"slug"`
Level   string `json:"level"`
Section string `json:"section"`
Group   string `json:"group,omitempty"`
}
type wikiJSON struct {
Pages []pageJSON `json:"pages"`
}
wj := wikiJSON{}
for _, p := range wiki.Pages {
wj.Pages = append(wj.Pages, pageJSON{p.Title, p.Slug, p.Level, p.Section, p.Group})
}
b, _ := json.MarshalIndent(wj, "", "  ")
dest := filepath.Join(outPath, "wiki.json")
return os.WriteFile(dest, b, 0o644)
}

func savePage(slug, content, outPath string) error {
return os.WriteFile(filepath.Join(outPath, slug+".md"), []byte(content), 0o644)
}

func abbrev(s string, n int) string {
s = strings.ReplaceAll(s, "\n", "←")
if len(s) > n {
return s[:n] + "…"
}
return s
}

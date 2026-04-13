// Package tools provides the three read-only repository exploration tools.
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
	openai "github.com/sashabaranov/go-openai"
)

// Directories always excluded from tree traversal.
var excludedDirs = map[string]bool{
	"node_modules": true, "vendor": true, ".git": true, "__pycache__": true,
	".venv": true, "venv": true, "dist": true, "build": true, "target": true,
	".gradle": true, ".idea": true, ".vscode": true, "coverage": true,
	".nyc_output": true, "tmp": true, ".tmp": true, ".pytest_cache": true,
	".mypy_cache": true, ".ruff_cache": true, "htmlcov": true, ".cache": true,
	".eggs": true, "site-packages": true, ".tox": true, "bower_components": true,
	".yarn": true, ".pnpm": true,
}

// Substrings that indicate a blocked (write/destructive) shell command.
var blockedTokens = []string{
	"del ", "erase ", " rd ", "rmdir ", " move ", " copy ", " ren ",
	"attrib ", "icacls ", "taskkill ", "format ", " reg ", " sc ",
	" net ", "mkdir ", " md ", "xcopy ", "robocopy ",
	"rm ", " mv ", " cp ", "chmod ", "chown ", "touch ",
	"wget ", "curl ", "pip install", "pip uninstall",
	"npm install", "npm uninstall", "yarn add", "apt ", "brew install",
	" > ", " >> ", "| tee ",
}

// ToolSet holds all tools bound to a specific repository root.
type ToolSet struct {
	Root   string
	spec   *gitignore.GitIgnore
	Tools  []openai.Tool
	Handle map[string]func(args map[string]any) string
}

// New creates a ToolSet bound to workDir.
func New(workDir string) *ToolSet {
	root, _ := filepath.Abs(workDir)

	var spec *gitignore.GitIgnore
	giPath := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(giPath); err == nil {
		if gi, err2 := gitignore.CompileIgnoreFile(giPath); err2 == nil {
			spec = gi
		}
	}

	ts := &ToolSet{Root: root, spec: spec}
	ts.Tools = ts.buildTools()
	ts.Handle = ts.buildHandlers()
	return ts
}

// ---------------------------------------------------------------------------
// Tool definitions (OpenAI function schema)
// ---------------------------------------------------------------------------

func (ts *ToolSet) buildTools() []openai.Tool {
	return []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "get_dir_structure",
				Description: "Get the directory structure of the repository as an ASCII tree. Filters .gitignore entries and common dependency directories. Use dir_path='.' for repository root. max_depth controls recursion depth (default 3).",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"dir_path":  map[string]any{"type": "string", "description": "Relative path from repo root, or '.' for root"},
						"max_depth": map[string]any{"type": "integer", "description": "Maximum recursion depth (default 3)", "default": 3},
					},
					"required": []string{"dir_path"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "view_file_in_detail",
				Description: "View the content of a file in the repository. Reads lines start_line..end_line (1-indexed, inclusive). If end_line is 0 reads up to 200 lines. Set show_line_numbers=true to prefix each line with its number.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"file_path":         map[string]any{"type": "string", "description": "Relative file path from repo root"},
						"start_line":        map[string]any{"type": "integer", "description": "First line to read (1-indexed, default 1)", "default": 1},
						"end_line":          map[string]any{"type": "integer", "description": "Last line to read (0 = start_line+199)", "default": 0},
						"show_line_numbers": map[string]any{"type": "boolean", "description": "Prefix each line with its number", "default": false},
					},
					"required": []string{"file_path"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "run_bash",
				Description: "Run a read-only shell command in the repository directory. Only informational commands are allowed (dir, type, find, git log, git show, etc.). Commands that write, delete, or modify files are blocked. Timeout: 30 seconds.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{"type": "string", "description": "Shell command to execute"},
					},
					"required": []string{"command"},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Tool handler implementations
// ---------------------------------------------------------------------------

func (ts *ToolSet) buildHandlers() map[string]func(args map[string]any) string {
	return map[string]func(args map[string]any) string{
		"get_dir_structure":  ts.getDirStructure,
		"view_file_in_detail": ts.viewFileInDetail,
		"run_bash":           ts.runBash,
	}
}

func (ts *ToolSet) getDirStructure(args map[string]any) string {
	dirPath, _ := args["dir_path"].(string)
	maxDepth := 3
	if v, ok := args["max_depth"]; ok {
		switch d := v.(type) {
		case float64:
			maxDepth = int(d)
		case int:
			maxDepth = d
		}
	}

	var target string
	if dirPath == "" || dirPath == "." {
		target = ts.Root
	} else {
		target = filepath.Join(ts.Root, dirPath)
	}
	target = filepath.Clean(target)

	if !strings.HasPrefix(target, ts.Root) {
		return "Error: path is outside the repository root"
	}
	info, err := os.Stat(target)
	if err != nil {
		return fmt.Sprintf("Error: path '%s' does not exist", dirPath)
	}
	if !info.IsDir() {
		return fmt.Sprintf("Error: '%s' is not a directory", dirPath)
	}

	header := dirPath
	if header == "" || header == "." {
		header = "."
	}
	lines := []string{header + "/"}
	lines = append(lines, buildTree(ts.Root, target, ts.spec, maxDepth, "", 0)...)
	return strings.Join(lines, "\n")
}

func buildTree(root, current string, spec *gitignore.GitIgnore, maxDepth int, prefix string, depth int) []string {
	if depth > maxDepth {
		return nil
	}
	entries, err := os.ReadDir(current)
	if err != nil {
		return []string{prefix + "(permission denied)"}
	}

	// Sort: dirs first, then files, both alphabetically
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	var filtered []os.DirEntry
	for _, e := range entries {
		name := e.Name()
		if excludedDirs[name] {
			continue
		}
		if strings.HasSuffix(name, ".egg-info") || strings.HasSuffix(name, ".dist-info") {
			continue
		}
		if spec != nil {
			rel, _ := filepath.Rel(root, filepath.Join(current, name))
			rel = filepath.ToSlash(rel)
			if e.IsDir() {
				rel += "/"
			}
			if spec.MatchesPath(rel) {
				continue
			}
		}
		filtered = append(filtered, e)
	}

	var lines []string
	for i, e := range filtered {
		isLast := i == len(filtered)-1
		connector := "├── "
		ext := "│   "
		if isLast {
			connector = "└── "
			ext = "    "
		}
		lines = append(lines, prefix+connector+e.Name())
		if e.IsDir() && depth < maxDepth {
			lines = append(lines, buildTree(root, filepath.Join(current, e.Name()), spec, maxDepth, prefix+ext, depth+1)...)
		}
	}
	return lines
}

func (ts *ToolSet) viewFileInDetail(args map[string]any) string {
	filePath, _ := args["file_path"].(string)
	startLine := 1
	endLine := 0
	showNums := false

	if v, ok := args["start_line"]; ok {
		if d, ok2 := v.(float64); ok2 {
			startLine = int(d)
		}
	}
	if v, ok := args["end_line"]; ok {
		if d, ok2 := v.(float64); ok2 {
			endLine = int(d)
		}
	}
	if v, ok := args["show_line_numbers"]; ok {
		if b, ok2 := v.(bool); ok2 {
			showNums = b
		}
	}

	target := filepath.Clean(filepath.Join(ts.Root, filePath))
	if !strings.HasPrefix(target, ts.Root) {
		return "Error: file is outside the repository root"
	}
	info, err := os.Stat(target)
	if err != nil {
		return fmt.Sprintf("Error: file '%s' does not exist", filePath)
	}
	if !info.Mode().IsRegular() {
		return fmt.Sprintf("Error: '%s' is not a regular file", filePath)
	}
	if info.Size() > 5*1024*1024 {
		return "Error: file is too large (> 5 MB). Use line range parameters."
	}

	data, err := os.ReadFile(target)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err)
	}

	allLines := strings.SplitAfter(string(data), "\n")
	total := len(allLines)

	startIdx := startLine - 1
	if startIdx < 0 {
		startIdx = 0
	}
	var endIdx int
	if endLine <= 0 {
		endIdx = startIdx + 200
	} else {
		endIdx = endLine
	}
	if endIdx > total {
		endIdx = total
	}

	selected := allLines[startIdx:endIdx]
	var body strings.Builder
	for i, ln := range selected {
		if showNums {
			body.WriteString(fmt.Sprintf("%4d | %s", startIdx+i+1, ln))
		} else {
			body.WriteString(ln)
		}
	}

	return fmt.Sprintf("File: %s (lines %d-%d of %d)\n%s", filePath, startIdx+1, endIdx, total, body.String())
}

func (ts *ToolSet) runBash(args map[string]any) string {
	command, _ := args["command"].(string)
	cmdLower := strings.ToLower(command)
	for _, token := range blockedTokens {
		if strings.Contains(cmdLower, token) {
			return fmt.Sprintf("Error: blocked operation '%s' detected in command", strings.TrimSpace(token))
		}
	}
	return runShell(command, ts.Root)
}

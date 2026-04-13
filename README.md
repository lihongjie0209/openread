<div align="center">

```
   ██████╗ ██████╗ ███████╗███╗   ██╗██████╗ ███████╗ █████╗ ██████╗
  ██╔═══██╗██╔══██╗██╔════╝████╗  ██║██╔══██╗██╔════╝██╔══██╗██╔══██╗
  ██║   ██║██████╔╝█████╗  ██╔██╗ ██║██████╔╝█████╗  ███████║██║  ██║
  ██║   ██║██╔═══╝ ██╔══╝  ██║╚██╗██║██╔══██╗██╔══╝  ██╔══██║██║  ██║
  ╚██████╔╝██║     ███████╗██║ ╚████║██║  ██║███████╗██║  ██║██████╔╝
   ╚═════╝ ╚═╝     ╚══════╝╚═╝  ╚═══╝╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝╚═════╝
```

**Turn any codebase into a beautiful wiki — in seconds.**

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey?style=flat-square)]()

*An open-source reimplementation of [zread_cli](https://www.npmjs.com/package/zread_cli), written in Go — no Node.js required.*

</div>

---

## ✨ What is openread?

**openread** is a command-line tool that uses an AI agent to deeply understand your codebase and generate structured, human-readable wiki documentation — automatically.

It is a fully open-source Go reimplementation of the `zread_cli` npm package, with the same configuration format and workflow, but:

- **Single binary** — no Node.js, no npm, no runtime dependencies
- **Cross-platform** — Linux, macOS, and Windows
- **Live TUI** — real-time progress display with per-page retry
- **Compatible** — reads the same `~/.zread/config.yaml` as the original

---

## 📸 Demo

```
Phase 2 — Generate Pages (12/28)

    #   Page                              Status
────────────────────────────────────────────────────────
❯   1   概述                              ✓  done
    2   快速开始                           ✓  done
    3   环境要求与安装                      ✓  done
    4   API密钥配置                        ⟳  requesting
    5   命令行工具使用                      ⚙  read_file
    6   架构设计理念                        ·  waiting
    7   双阶段文档生成流程                   ·  waiting
  ↓ 21 more

↑/↓: navigate  |  r: retry  |  s: skip failed & commit  |  ctrl+c: quit
```

> Statuses are color-coded: **green** = done · **cyan** = in progress · **magenta** = tool call · **red** = failed

---

## 🚀 Quick Start

### Installation

**Option 1: Download binary** (recommended)

Grab the latest release from [Releases](../../releases) and place it in your `PATH`.

**Option 2: Build from source**

```bash
git clone https://github.com/lihongjie0209/openread.git
cd openread
go build -o openread .
```

### 1. Configure your LLM

```bash
openread config --api-key sk-xxxxxxxx
```

This writes to `~/.zread/config.yaml` — compatible with the original `zread_cli`.

### 2. Generate docs for your project

```bash
cd /path/to/your/project
openread generate
```

That's it. openread will:
1. **Phase 1** — Analyze your codebase and generate a documentation catalog
2. **Phase 2** — Concurrently write each wiki page using an AI agent with file-reading tools

### 3. Browse the docs

```bash
openread browse
```

Opens a local web server at `http://localhost:3000` with your generated wiki.

---

## ⚙️ Configuration

Configuration is stored in `~/.zread/config.yaml` and is **identical to the original zread_cli format**:

```yaml
language: zh           # UI language
doc_language: zh       # Documentation output language (zh / en)

llm:
  provider: custom
  model: deepseek-chat
  api_key: sk-xxxxxxxxxxxxxxxx
  base_url: https://api.deepseek.com/v1  # Any OpenAI-compatible endpoint

concurrency:
  max_concurrent: 3   # Parallel page generation workers
  max_retries: 2      # Auto-retries per page on failure
```

Manage it via CLI:

```bash
openread config                           # Show current config
openread config --api-key sk-xxx          # Set API key
openread config --model gpt-4o            # Change model
openread config --base-url https://...    # Use custom endpoint
openread config --workers 5               # Set concurrency
openread config --lang en                 # Switch language
```

### Environment variable overrides

```bash
ZREAD_API_KEY=sk-xxx   openread generate
ZREAD_BASE_URL=https://your-proxy/v1   openread generate
ZREAD_MODEL=gpt-4o   openread generate
```

---

## 📖 Commands

### `openread generate`

Generate wiki documentation for the current workspace.

```
Flags:
  -y, --yes               Skip all confirmations and start immediately
      --draft string      Action for existing draft: resume | clear | cancel
      --skip-failed       Auto-skip failed pages and commit remaining wiki
      --lang string       Override documentation language
      --dir string        Target directory (default: current directory)
      --retries int       Max retries per page on failure (default: 1)
      --workers int       Override concurrent worker count
      --verbose-catalog   Show catalog agent tool calls (disables TUI)
      --verbose-pages     Show page agent tool calls (disables TUI)
```

**In-session TUI controls:**

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate page list |
| `r` | Retry the selected failed page |
| `s` | Skip all failed pages and commit |
| `ctrl+c` | Quit |

### `openread browse`

Serve the generated wiki locally.

```
Flags:
      --port int    Local server port (default: 3000)
      --open        Auto-open in browser (default: true)
      --build       Export static HTML site (no server)
      --dir string  Project directory (default: current directory)
```

### `openread config`

View or modify `~/.zread/config.yaml`.

### `openread version`

Show version, Go runtime, and platform info.

---

## 🏗️ How It Works

openread uses a **two-phase ReAct agent** pipeline:

```
┌─────────────────────────────────────────────────┐
│  Phase 1 — Catalog Agent                        │
│                                                 │
│  Reads your project structure → generates a     │
│  structured table of contents (sections +       │
│  pages) as a JSON catalog.                      │
└──────────────────────┬──────────────────────────┘
                       │ catalog.json
┌──────────────────────▼──────────────────────────┐
│  Phase 2 — Page Agents (parallel)               │
│                                                 │
│  For each page in the catalog, an agent:        │
│  1. Reads relevant source files via tools       │
│  2. Synthesizes content into markdown           │
│  3. Saves the page to .zread/wiki/              │
└─────────────────────────────────────────────────┘
```

**Agent tools available during generation:**
- `list_dir` — list directory contents
- `read_file` — read a source file
- `search_text` — grep across the project

**Output layout:**
```
.zread/
├── config.yaml          # project-level overrides (optional)
├── wiki/                # generated markdown pages
│   ├── catalog.json     # page index
│   └── *.md             # wiki pages
└── site/                # built HTML site (after browse --build)
```

---

## 🔄 Draft & Resume

If generation is interrupted, a draft is saved automatically. On next run:

```
An existing draft was found (15/28 pages complete).
? What would you like to do?
  > resume   — continue from where it left off
    clear    — discard the draft and start fresh
    cancel   — exit without changes
```

Or pass a flag to skip the prompt:

```bash
openread generate --draft resume
openread generate --draft clear
```

---

## 🌐 LLM Provider Compatibility

openread uses the **OpenAI-compatible chat completions API** and works with any provider that supports it:

| Provider | base_url |
|----------|----------|
| DeepSeek (default) | `https://api.deepseek.com/v1` |
| OpenAI | `https://api.openai.com/v1` |
| Ollama | `http://localhost:11434/v1` |
| Azure OpenAI | `https://{resource}.openai.azure.com/openai/deployments/{deploy}/` |
| Any proxy | Your custom endpoint |

---

## 🆚 Comparison with `zread_cli`

| Feature | `zread_cli` (npm) | **openread** (Go) |
|---------|-------------------|-------------------|
| Runtime required | Node.js 18+ | None (single binary) |
| Config format | `~/.zread/config.yaml` | ✅ Same |
| Two-phase generation | ✅ | ✅ |
| Real-time TUI | ✅ | ✅ |
| Per-page retry (`r`) | ✅ | ✅ |
| Skip failed (`s`) | ✅ | ✅ |
| Draft / resume | ✅ | ✅ |
| Browse command | ✅ | ✅ |
| Source available | ❌ Closed source | ✅ MIT |

---

## 🛠️ Development

**Requirements:** Go 1.21+

```bash
# Build
go build -o openread .

# Run directly
go run . generate

# Vet
go vet ./...
```

**Project structure:**

```
internal/
├── agent/      # ReAct LLM agent (catalog + page runners)
├── browse/     # Local HTTP wiki server
├── config/     # Config file loading (~/.zread/config.yaml)
├── models/     # OpenAI client wrapper
├── prompts/    # LLM prompt templates
├── runner/     # Orchestration: TUI + plain mode
├── tools/      # Agent tools: list_dir, read_file, search_text
└── tui/        # Bubbletea TUI model
```

---

## 📄 License

MIT © openread contributors

---

<div align="center">

*openread is an independent open-source project and is not affiliated with zread.ai.*

</div>

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
[![Release](https://img.shields.io/github/v/release/lihongjie0209/openread?style=flat-square)](https://github.com/lihongjie0209/openread/releases)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey?style=flat-square)]()

**Language:** [中文](README.md) | English

*Open-source reimplementation of [zread_cli](https://www.npmjs.com/package/zread_cli) — same features and config format, fully open source.*

</div>

---

## ✨ What is openread?

**openread** is a CLI tool that uses an AI agent to deeply understand your codebase and generate structured, human-readable wiki documentation — automatically.

`zread_cli` is a Go program distributed via npm (`npm install -g zread_cli`). **openread** is its open-source reimplementation — same configuration format, same workflow, downloadable as a single binary without npm.

Key highlights:

- 📦 **No npm required** — download a single binary directly from [Releases](https://github.com/lihongjie0209/openread/releases)
- 🖥️ **Live TUI** — color-coded progress table with per-page retry
- 🔁 **Draft & resume** — pick up where you left off after interruption
- 🔌 **Config-compatible** — reads the same `~/.zread/config.yaml` as the original
- 🌐 **Any OpenAI-compatible API** — DeepSeek, OpenAI, Ollama, etc.

---

## 📸 Demo

```
Phase 2 — Generate Pages (12/28)

    #   Page                              Status
────────────────────────────────────────────────────────
❯   1   Overview                          ✓  done
    2   Quick Start                        ✓  done
    3   Installation                       ✓  done
    4   API Key Setup                      ⟳  requesting
    5   CLI Usage                          ⚙  read_file
    6   Architecture                       ·  waiting
  ↓ 22 more

↑/↓: navigate  |  r: retry  |  s: skip failed & commit  |  ctrl+c: quit
```

> Status colors: **green** = done · **cyan** = requesting · **magenta** = tool call · **red** = failed

---

## 🚀 Quick Start

### Installation

**Download binary (recommended)**

Go to [Releases](https://github.com/lihongjie0209/openread/releases) and download the archive for your platform:

```bash
# Linux amd64
curl -L https://github.com/lihongjie0209/openread/releases/latest/download/openread_v0.1.0_linux_amd64.tar.gz | tar xz
sudo mv openread-linux-amd64 /usr/local/bin/openread

# macOS Apple Silicon
curl -L https://github.com/lihongjie0209/openread/releases/latest/download/openread_v0.1.0_darwin_arm64.tar.gz | tar xz
sudo mv openread-darwin-arm64 /usr/local/bin/openread
```

**Build from source**

```bash
git clone https://github.com/lihongjie0209/openread.git
cd openread
go build -o openread .
```

### 1. Configure your LLM

```bash
openread config --api-key sk-xxxxxxxx
```

Writes to `~/.zread/config.yaml` — fully compatible with the original `zread_cli`.

### 2. Generate docs

```bash
cd /path/to/your/project
openread generate
```

### 3. Browse

```bash
openread browse   # opens http://localhost:3000
```

---

## ⚙️ Configuration (`~/.zread/config.yaml`)

```yaml
language: zh           # UI language
doc_language: zh       # Documentation language (zh / en)

llm:
  provider: custom
  model: deepseek-chat
  api_key: sk-xxxxxxxxxxxxxxxx
  base_url: https://api.deepseek.com/v1  # Any OpenAI-compatible endpoint

concurrency:
  max_concurrent: 3   # Parallel page workers
  max_retries: 2      # Auto-retries per page on failure
```

```bash
openread config                        # Show current config
openread config --api-key sk-xxx       # Set API key
openread config --model gpt-4o         # Change model
openread config --base-url https://... # Custom endpoint
openread config --workers 5            # Set concurrency
openread config --lang en              # Switch language
```

Environment variable overrides:

```bash
ZREAD_API_KEY=sk-xxx        openread generate
ZREAD_BASE_URL=https://...  openread generate
ZREAD_MODEL=gpt-4o          openread generate
```

---

## 📖 Commands

### `openread generate`

```
Flags:
  -y, --yes               Skip confirmations and start immediately
      --draft string      Draft action: resume | clear | cancel
      --skip-failed       Auto-skip failed pages and commit
      --lang string       Override documentation language
      --dir string        Target directory (default: cwd)
      --retries int       Max retries per page (default: 1)
      --workers int       Override concurrent worker count
      --verbose-catalog   Show catalog agent tool calls (disables TUI)
      --verbose-pages     Show page agent tool calls (disables TUI)
```

**TUI controls:**

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate page list |
| `r` | Retry selected failed page |
| `s` | Skip all failures and commit |
| `ctrl+c` | Quit |

### `openread browse`

```
Flags:
      --port int    Local server port (default: 3000)
      --open        Auto-open in browser (default: true)
      --build       Export static HTML site (no server)
      --dir string  Project directory (default: cwd)
```

### `openread config` / `openread version`

View/edit `~/.zread/config.yaml` or print version info.

---

## 🏗️ How It Works

```
┌─────────────────────────────────────────────┐
│  Phase 1 — Catalog Agent                   │
│  Reads project structure → generates JSON   │
│  table of contents (sections + pages)       │
└──────────────────┬──────────────────────────┘
                   │ catalog.json
┌──────────────────▼──────────────────────────┐
│  Phase 2 — Page Agents (parallel)           │
│  Each page: read files → synthesize → save  │
│  Tools: list_dir · read_file · search_text  │
└─────────────────────────────────────────────┘
```

Output layout:
```
.zread/
├── wiki/            # Generated markdown pages
│   ├── catalog.json
│   └── *.md
└── site/            # Static HTML (after browse --build)
```

---

## 🌐 LLM Provider Compatibility

| Provider | base_url |
|----------|----------|
| DeepSeek (default) | `https://api.deepseek.com/v1` |
| OpenAI | `https://api.openai.com/v1` |
| Ollama (local) | `http://localhost:11434/v1` |
| Azure OpenAI | `https://{resource}.openai.azure.com/openai/deployments/{deploy}/` |
| Custom proxy | Your endpoint |

---

## 🆚 vs `zread_cli`

> `zread_cli` is a Go program distributed via npm. openread is its open-source equivalent — install directly without npm.

| Feature | `zread_cli` | **openread** |
|---------|-------------|--------------|
| Installation | `npm install -g zread_cli` | Download binary directly |
| Source code | ❌ Closed source | ✅ MIT open source |
| Config format | `~/.zread/config.yaml` | ✅ Identical |
| Two-phase generation | ✅ | ✅ |
| Live TUI | ✅ | ✅ |
| Per-page retry (`r`) | ✅ | ✅ |
| Skip failed (`s`) | ✅ | ✅ |
| Draft / resume | ✅ | ✅ |
| Browse command | ✅ | ✅ |

---

## 🛠️ Development

Requirements: Go 1.21+

```bash
go build -o openread .
go vet ./...
```

```
internal/
├── agent/    # ReAct LLM agent (catalog + page runners)
├── browse/   # Local wiki HTTP server
├── config/   # Config file (~/.zread/config.yaml)
├── models/   # OpenAI client wrapper
├── prompts/  # LLM prompt templates
├── runner/   # Orchestration: TUI + plain mode
├── tools/    # Agent tools: list_dir, read_file, search_text
└── tui/      # Bubbletea TUI model
```

---

## 📄 License

MIT © openread contributors

*openread is an independent open-source project and is not affiliated with zread.ai.*
<div align="center">

```
  ██████╗ ██████╗ ███████╗███╗   ██╗██████╗ ███████╗ █████╗ ██████╗
 ██╔═══██╗██╔══██╗██╔════╝████╗  ██║██╔══██╗██╔════╝██╔══██╗██╔══██╗
 ██║   ██║██████╔╝█████╗  ██╔██╗ ██║██████╔╝█████╗  ███████║██║  ██║
 ██║   ██║██╔═══╝ ██╔══╝  ██║╚██╗██║██╔══██╗██╔══╝  ██╔══██║██║  ██║
 ╚██████╔╝██║     ███████╗██║ ╚████║██║  ██║███████╗██║  ██║██████╔╝
  ╚═════╝ ╚═╝     ╚══════╝╚═╝  ╚═══╝╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝╚═════╝
```

**将任意代码库一键转化为结构化 Wiki 文档**

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)
[![Release](https://img.shields.io/github/v/release/lihongjie0209/openread?style=flat-square)](https://github.com/lihongjie0209/openread/releases)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey?style=flat-square)]()

**语言 / Language：** 中文 | [English](#english)

*[zread_cli](https://www.npmjs.com/package/zread_cli) 的开源复刻版本，使用 Go 编写 — 无需 Node.js。*

</div>

---

## ✨ 简介

**openread** 是一款命令行工具，通过 AI 智能体深度理解你的代码库，自动生成结构化、可读性强的 Wiki 文档。

本项目是 `zread_cli` npm 包的完全开源 Go 复刻版，保持相同的配置格式和工作流，同时带来：

- 🚀 **单一二进制** — 无需 Node.js、npm 或任何运行时依赖
- 🖥️ **实时 TUI** — 彩色进度表格，支持单页重试
- 🔁 **草稿恢复** — 中断后可从断点继续生成
- 🔌 **兼容原版** — 读取相同的 `~/.zread/config.yaml` 配置文件
- 🌐 **支持任意 OpenAI 兼容接口** — DeepSeek、OpenAI、Ollama 等

---

## 📸 效果展示

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
  ↓ 22 more

↑/↓: navigate  |  r: retry  |  s: skip failed & commit  |  ctrl+c: quit
```

> 状态颜色说明：**绿色** = 完成 · **青色** = 请求中 · **品红** = 工具调用 · **红色** = 失败

---

## 🚀 快速开始

### 安装

**方式一：下载二进制（推荐）**

前往 [Releases](https://github.com/lihongjie0209/openread/releases) 下载对应平台的压缩包，解压后放入 `PATH`。

```bash
# Linux amd64
curl -L https://github.com/lihongjie0209/openread/releases/latest/download/openread_v0.1.0_linux_amd64.tar.gz | tar xz
sudo mv openread-linux-amd64 /usr/local/bin/openread

# macOS Apple Silicon
curl -L https://github.com/lihongjie0209/openread/releases/latest/download/openread_v0.1.0_darwin_arm64.tar.gz | tar xz
sudo mv openread-darwin-arm64 /usr/local/bin/openread
```

**方式二：从源码构建**

```bash
git clone https://github.com/lihongjie0209/openread.git
cd openread
go build -o openread .
```

### 1. 配置 LLM

```bash
openread config --api-key sk-xxxxxxxx
```

配置写入 `~/.zread/config.yaml`，与原版 `zread_cli` 格式完全兼容。

### 2. 生成文档

```bash
cd /path/to/your/project
openread generate
```

openread 将分两阶段工作：
1. **第一阶段** — 分析代码结构，生成文档目录（Catalog）
2. **第二阶段** — 并发调用 AI 智能体，逐页编写 Wiki 内容

### 3. 浏览文档

```bash
openread browse
```

自动在浏览器打开 `http://localhost:3000`，展示生成的 Wiki。

---

## ⚙️ 配置文件

配置存储于 `~/.zread/config.yaml`，**与原版 zread_cli 格式完全兼容**：

```yaml
language: zh           # 界面语言
doc_language: zh       # 文档输出语言（zh / en）

llm:
  provider: custom
  model: deepseek-chat
  api_key: sk-xxxxxxxxxxxxxxxx
  base_url: https://api.deepseek.com/v1  # 任意 OpenAI 兼容接口

concurrency:
  max_concurrent: 3   # 并发页面生成数
  max_retries: 2      # 单页失败后自动重试次数
```

通过命令行管理：

```bash
openread config                           # 查看当前配置
openread config --api-key sk-xxx          # 设置 API Key
openread config --model gpt-4o            # 更换模型
openread config --base-url https://...    # 自定义接口地址
openread config --workers 5               # 设置并发数
openread config --lang en                 # 切换语言
```

### 环境变量覆盖

```bash
ZREAD_API_KEY=sk-xxx         openread generate
ZREAD_BASE_URL=https://...   openread generate
ZREAD_MODEL=gpt-4o           openread generate
```

---

## 📖 命令参考

### `openread generate`

为当前工作区生成 Wiki 文档。

```
参数：
  -y, --yes               跳过所有确认，立即开始生成
      --draft string      草稿处理方式：resume（继续）| clear（清除）| cancel（取消）
      --skip-failed       自动跳过失败页面并提交剩余内容
      --lang string       覆盖文档输出语言
      --dir string        目标目录（默认：当前目录）
      --retries int       单页最大重试次数（默认：1）
      --workers int       覆盖并发数
      --verbose-catalog   显示目录智能体工具调用（关闭 TUI）
      --verbose-pages     显示页面智能体工具调用（关闭 TUI）
```

**TUI 实时交互快捷键：**

| 按键 | 功能 |
|------|------|
| `↑` / `↓` | 导航页面列表 |
| `r` | 重试选中的失败页面 |
| `s` | 跳过所有失败页面并提交 |
| `ctrl+c` | 退出 |

### `openread browse`

在浏览器中浏览生成的 Wiki。

```
参数：
      --port int    本地服务端口（默认：3000）
      --open        自动在浏览器中打开（默认：true）
      --build       导出静态 HTML 站点（不启动服务）
      --dir string  项目目录（默认：当前目录）
```

### `openread config`

查看或修改 `~/.zread/config.yaml`。

### `openread version`

显示版本、Go 运行时及平台信息。

---

## 🏗️ 工作原理

openread 使用**两阶段 ReAct 智能体**流水线：

```
┌─────────────────────────────────────────────────┐
│  第一阶段 — 目录智能体（Catalog Agent）           │
│                                                 │
│  读取项目结构 → 生成结构化目录                    │
│  （章节 + 页面列表，JSON 格式）                   │
└──────────────────────┬──────────────────────────┘
                       │ catalog.json
┌──────────────────────▼──────────────────────────┐
│  第二阶段 — 页面智能体（并发执行）                │
│                                                 │
│  每个页面独立运行一个智能体：                     │
│  1. 通过工具读取相关源文件                        │
│  2. 将内容综合为 Markdown                        │
│  3. 保存至 .zread/wiki/                          │
└─────────────────────────────────────────────────┘
```

**智能体可用工具：**
- `list_dir` — 列出目录内容
- `read_file` — 读取源文件
- `search_text` — 在项目中全文搜索

**输出目录结构：**
```
.zread/
├── wiki/                # 生成的 Markdown 文档
│   ├── catalog.json     # 页面索引
│   └── *.md             # Wiki 页面
└── site/                # 静态 HTML 站点（browse --build 后生成）
```

---

## 🔄 草稿与恢复

生成中断后草稿会自动保存，下次运行时提示：

```
发现未完成的草稿（已完成 15/28 页）
? 请选择操作：
  > resume   — 从断点继续
    clear    — 清除草稿重新开始
    cancel   — 退出
```

也可通过参数跳过提示：

```bash
openread generate --draft resume
openread generate --draft clear
```

---

## 🌐 LLM 提供商兼容性

openread 使用 **OpenAI 兼容的 Chat Completions API**，支持任意兼容提供商：

| 提供商 | base_url |
|--------|----------|
| DeepSeek（默认） | `https://api.deepseek.com/v1` |
| OpenAI | `https://api.openai.com/v1` |
| Ollama（本地） | `http://localhost:11434/v1` |
| Azure OpenAI | `https://{resource}.openai.azure.com/openai/deployments/{deploy}/` |
| 自定义代理 | 你的接口地址 |

---

## 🆚 与 `zread_cli` 对比

| 功能 | `zread_cli` (npm) | **openread** (Go) |
|------|-------------------|-------------------|
| 运行时依赖 | Node.js 18+ | 无（单一二进制） |
| 配置格式 | `~/.zread/config.yaml` | ✅ 完全兼容 |
| 两阶段生成 | ✅ | ✅ |
| 实时 TUI | ✅ | ✅ |
| 单页重试（`r`） | ✅ | ✅ |
| 跳过失败（`s`） | ✅ | ✅ |
| 草稿 / 恢复 | ✅ | ✅ |
| 文档浏览 | ✅ | ✅ |
| 开放源码 | ❌ 闭源 | ✅ MIT |

---

## 🛠️ 参与开发

**环境要求：** Go 1.21+

```bash
# 构建
go build -o openread .

# 直接运行
go run . generate

# 代码检查
go vet ./...
```

**项目结构：**

```
internal/
├── agent/      # ReAct LLM 智能体（目录 + 页面）
├── browse/     # 本地 Wiki HTTP 服务
├── config/     # 配置文件读写（~/.zread/config.yaml）
├── models/     # OpenAI 客户端封装
├── prompts/    # LLM 提示词模板
├── runner/     # 编排层：TUI 模式 + 纯文本模式
├── tools/      # 智能体工具：list_dir、read_file、search_text
└── tui/        # Bubbletea TUI 模型
```

---

## 📄 许可证

MIT © openread contributors

*openread 是独立的开源项目，与 zread.ai 官方无关。*

---

<br/>

---

<h2 id="english">English</h2>

<div align="center">

**Turn any codebase into a beautiful wiki — in seconds.**

*An open-source reimplementation of [zread_cli](https://www.npmjs.com/package/zread_cli), written in Go — no Node.js required.*

**语言 / Language：** [中文](#) | English

</div>

### ✨ What is openread?

**openread** is a CLI tool that uses an AI agent to deeply understand your codebase and generate structured, human-readable wiki documentation — automatically.

It is a fully open-source Go reimplementation of the `zread_cli` npm package, with the same configuration format and workflow, but ships as a **single binary** with no runtime dependencies.

### 🚀 Quick Start

```bash
# 1. Configure your LLM
openread config --api-key sk-xxxxxxxx

# 2. Generate docs
cd /path/to/your/project
openread generate

# 3. Browse
openread browse
```

### ⚙️ Configuration (`~/.zread/config.yaml`)

```yaml
language: zh
doc_language: zh       # zh / en
llm:
  model: deepseek-chat
  api_key: sk-xxxxxxxx
  base_url: https://api.deepseek.com/v1
concurrency:
  max_concurrent: 3
  max_retries: 2
```

### 📖 Commands

| Command | Description |
|---------|-------------|
| `openread generate` | Generate wiki docs for the current workspace |
| `openread browse` | Serve the wiki locally in your browser |
| `openread config` | View or edit `~/.zread/config.yaml` |
| `openread version` | Print version info |

**TUI controls during generation:**

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate page list |
| `r` | Retry selected failed page |
| `s` | Skip all failures and commit |
| `ctrl+c` | Quit |

### 🌐 LLM Providers

Any OpenAI-compatible endpoint: DeepSeek (default), OpenAI, Ollama, Azure OpenAI, or any custom proxy.

### 📄 License

MIT © openread contributors

*openread is an independent open-source project and is not affiliated with zread.ai.*


---


## 🛠️ 参与开发

**环境要求：** Go 1.21+

```bash
# 构建
go build -o openread .

# 直接运行
go run . generate

# 代码检查
go vet ./...
```

**项目结构：**

```
internal/
├── agent/      # ReAct LLM 智能体（目录 + 页面）
├── browse/     # 本地 Wiki HTTP 服务
├── config/     # 配置文件读写（~/.zread/config.yaml）
├── models/     # OpenAI 客户端封装
├── prompts/    # LLM 提示词模板
├── runner/     # 编排层：TUI 模式 + 纯文本模式
├── tools/      # 智能体工具：list_dir、read_file、search_text
└── tui/        # Bubbletea TUI 模型
```

---

## 📄 许可证

MIT © openread contributors

*openread 是独立的开源项目，与 zread.ai 官方无关。*

---

<br/>

---

<h2 id="english">English</h2>

<div align="center">

**Turn any codebase into a beautiful wiki — in seconds.**

*An open-source reimplementation of [zread_cli](https://www.npmjs.com/package/zread_cli), written in Go — no Node.js required.*

**语言 / Language：** [中文](#) | English

</div>

### ✨ What is openread?

**openread** is a CLI tool that uses an AI agent to deeply understand your codebase and generate structured, human-readable wiki documentation — automatically.

It is a fully open-source Go reimplementation of the `zread_cli` npm package, with the same configuration format and workflow, but ships as a **single binary** with no runtime dependencies.

### 🚀 Quick Start

```bash
# 1. Configure your LLM
openread config --api-key sk-xxxxxxxx

# 2. Generate docs
cd /path/to/your/project
openread generate

# 3. Browse
openread browse
```

### ⚙️ Configuration (`~/.zread/config.yaml`)

```yaml
language: zh
doc_language: zh       # zh / en
llm:
  model: deepseek-chat
  api_key: sk-xxxxxxxx
  base_url: https://api.deepseek.com/v1
concurrency:
  max_concurrent: 3
  max_retries: 2
```

### 📖 Commands

| Command | Description |
|---------|-------------|
| `openread generate` | Generate wiki docs for the current workspace |
| `openread browse` | Serve the wiki locally in your browser |
| `openread config` | View or edit `~/.zread/config.yaml` |
| `openread version` | Print version info |

**TUI controls during generation:**

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate page list |
| `r` | Retry selected failed page |
| `s` | Skip all failures and commit |
| `ctrl+c` | Quit |

### 🌐 LLM Providers

Any OpenAI-compatible endpoint: DeepSeek (default), OpenAI, Ollama, Azure OpenAI, or any custom proxy.

### 🆚 vs `zread_cli`

| Feature | `zread_cli` (npm) | **openread** (Go) |
|---------|-------------------|-------------------|
| Runtime | Node.js 18+ | None (single binary) |
| Config format | `~/.zread/config.yaml` | ✅ Same |
| Two-phase generation | ✅ | ✅ |
| Live TUI | ✅ | ✅ |
| Per-page retry (`r`) | ✅ | ✅ |
| Skip failed (`s`) | ✅ | ✅ |
| Draft / resume | ✅ | ✅ |
| Source available | ❌ Closed source | ✅ MIT |

### 📄 License

MIT © openread contributors

*openread is an independent open-source project and is not affiliated with zread.ai.*

<p align="center">
  <img src="gmn.png" alt="gmn logo" width="150">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/github/v/release/linkalls/gmn?style=for-the-badge" alt="Release">
  <img src="https://img.shields.io/github/license/linkalls/gmn?style=for-the-badge" alt="License">
  <img src="https://img.shields.io/github/actions/workflow/status/linkalls/gmn/ci.yml?style=for-the-badge&label=CI" alt="CI">
</p>

<p align="center">
  <strong>A lightweight Gemini CLI written in Go</strong><br>
  <em>A love letter to <a href="https://github.com/google-gemini/gemini-cli">Google's Gemini CLI</a></em>
</p>

<p align="center">
  <a href="#-why-gmn">Why gmn?</a> •
  <a href="#-installation">Installation</a> •
  <a href="#-quick-start">Quick Start</a> •
  <a href="#-interactive-chat">Chat Mode</a> •
  <a href="#-built-in-tools">Tools</a> •
  <a href="#-mcp-support">MCP</a> •
  <a href="#-benchmarks">Benchmarks</a>
</p>

---

## ⚡ Why gmn?

The official Gemini CLI is an **amazing tool** with excellent MCP support and seamless Google authentication. However, for scripting and automation, its Node.js runtime adds startup overhead.

**gmn** reimplements the core functionality in Go, achieving **~40x faster startup** while maintaining full compatibility with the official CLI's authentication.

```
$ time gmn "hi" > /dev/null
0.02s user 0.01s system

$ time gemini -p "hi" > /dev/null
0.94s user 0.20s system
```

### ✨ Features

- **Fast startup** — Native Go binary, no runtime overhead
- **Interactive chat mode** — Rich TUI with multi-turn conversations
- **Built-in tools** — File operations, web search, shell commands
- **YOLO mode** — Skip confirmations for automated workflows (`--yolo`)
- **Session stats** — Token usage tracking with Ctrl+C graceful exit
- **Gemini 3 Pro support** — Full compatibility with `gemini-3-pro-preview`
- **MCP support** — Connect to Model Context Protocol servers
- **Credential reuse** — Uses existing Gemini CLI authentication

## 📦 Installation

### ⚠️ Prerequisites (Required)

**gmn does not have its own authentication.** You must authenticate once using the official Gemini CLI first:

```bash
npm install -g @google/gemini-cli
gemini  # Choose "Login with Google"
```

gmn reuses these credentials automatically from `~/.gemini/`. Your free tier quota or Workspace Code Assist quota applies.

### Go

```bash
go install github.com/linkalls/gmn@latest
```

### Binary

Download from [Releases](https://github.com/linkalls/gmn/releases)

## 🚀 Quick Start

```bash
# Simple prompt (one-shot)
gmn "Explain quantum computing"

# Interactive chat mode
gmn chat

# Chat with specific model (tab completion available)
gmn chat -m gemini-3-pro-preview

# Chat with initial prompt
gmn chat -p "Review this codebase"

# With file context
gmn "Review this code" -f main.go

# Pipe input
cat error.log | gmn "What's wrong?"

# JSON output
gmn "List 3 colors" -o json
```

## 💬 Interactive Chat

Start an interactive session with a rich TUI and tool execution support:

```bash
gmn chat                              # Default model (gemini-2.5-flash)
gmn chat -m gemini-3-pro-preview      # Use Gemini 3 Pro
gmn chat -p "explain this codebase"   # Start with a prompt
gmn chat -r last                      # Resume the last session
gmn chat -r my-project                # Resume a named session
gmn chat --yolo                       # Skip all confirmations (dangerous!)
gmn chat --shell /bin/zsh             # Use custom shell
```

### TUI Features

```
╭──────────────────────────────────────╮
│  ✨ gmn   gemini-3-pro-preview       │
│  📁 /path/to/your/project            │
╰──────────────────────────────────────╯
Type /help for commands, /exit to quit

❯ _
```

- **Rich header** — Model badge, working directory, YOLO indicator
- **Thinking indicator** — Spinner while waiting for response
- **Tool notifications** — Visual feedback for tool calls
- **Session persistence** — Auto-save conversations, resume anytime
- **Session stats** — Token usage on exit (including Ctrl+C)
- **Tab completion** — Auto-complete models and commands
- **Command history** — Navigate with Up/Down arrows

### Chat Commands

| Command         | Description                                    |
| --------------- | ---------------------------------------------- |
| `/help`, `/h`   | Show available commands                        |
| `/exit`, `/q`   | Exit with session stats                        |
| `/clear`        | Clear conversation history                     |
| `/stats`        | Show current token usage                       |
| `/model`        | Show current model and available models        |
| `/model <name>` | Switch model (e.g., `/model gemini-2.5-flash`) |
| `/sessions`     | List all saved sessions                        |
| `/save [name]`  | Save current session (optional name)           |
| `/load <id>`    | Load a saved session                           |
| `Ctrl+C`        | Exit gracefully with session stats             |

## 🔧 Built-in Tools

In chat mode, Gemini can automatically call these tools:

| Tool                  | Description                    | Confirmation |
| --------------------- | ------------------------------ | ------------ |
| `list_directory`      | List contents of a directory   | No           |
| `read_file`           | Read file contents             | No           |
| `write_file`          | Write content to a file        | **Yes**      |
| `edit_file`           | Edit file by replacing text    | **Yes**      |
| `glob`                | Find files matching a pattern  | No           |
| `search_file_content` | Search for text/regex in files | No           |
| `web_search`          | Search the web (DuckDuckGo)    | No           |
| `web_fetch`           | Fetch and parse web pages      | **Yes**      |
| `shell`               | Execute shell commands         | **Yes**      |

### Confirmation Prompt

For dangerous operations, gmn shows a rich confirmation dialog:

```
╭─ Allow Shell Command? ────────────────────╮
│  Command: rm -rf ./build                  │
│                                           │
│  [y] Yes  [n] No  [a] Always allow shell  │
╰───────────────────────────────────────────╯
```

Use `--yolo` to skip all confirmations (be careful!).

## 📋 Usage

```
gmn [prompt] [flags]
gmn chat [flags]
gmn mcp <command>

Commands:
  chat                         Start interactive chat session
  mcp list                     List MCP servers and tools
  mcp call <server> <tool>     Call an MCP tool

Global Flags:
  -p, --prompt string          Prompt (alternative to positional arg)
  -m, --model string           Model (default "gemini-2.5-flash")
  -f, --file strings           Files to include
  -o, --output-format string   text, json, stream-json (default "text")
  -t, --timeout duration       Timeout (default 5m)
      --debug                  Debug output
  -v, --version                Version

Chat Flags:
  -p, --prompt string          Initial prompt to send
  -m, --model string           Model (default based on tier)
  -f, --file strings           Files to include in context
  -r, --resume string          Resume a session (ID, name, or 'last')
      --yolo                   Skip all confirmation prompts
      --shell string           Custom shell path (default: auto-detect)
```

### Supported Models

| Model                    | Tier            | Notes                   |
| ------------------------ | --------------- | ----------------------- |
| `gemini-2.5-flash`       | Free / Standard | Default, fast responses |
| `gemini-2.5-pro`         | Free / Standard | More capable            |
| `gemini-3-pro-preview`   | Standard        | Latest, best for coding |
| `gemini-3-flash-preview` | Standard        | Fast Gemini 3           |

## 🔌 MCP Support

gmn supports [Model Context Protocol](https://modelcontextprotocol.io/) servers.

Configure in `~/.gemini/settings.json`:

```json
{
  "mcpServers": {
    "my-server": {
      "command": "/path/to/mcp-server"
    }
  }
}
```

```bash
# List available tools
gmn mcp list

# Call a tool
gmn mcp call my-server tool-name arg=value
```

## 📊 Benchmarks

| Metric  | gmn       | Official CLI | Improvement |
| ------- | --------- | ------------ | ----------- |
| Startup | **~20ms** | ~850ms       | **~40x**    |
| Binary  | ~11MB     | ~200MB       | **~18x**    |
| Runtime | None      | Node.js      | -           |

_Measured on macOS/Linux. Windows startup may vary._

## 🏗️ Build

```bash
git clone https://github.com/linkalls/gmn.git
cd gmn
make build          # Current platform
make cross-compile  # All platforms
```

## 🚫 What's NOT Included

- OAuth flow → authenticate with official CLI first
- API Key / Vertex AI auth
- Some advanced official CLI features

## 📄 License

Apache License 2.0 — See [LICENSE](LICENSE)

This project is a derivative work based on [Gemini CLI](https://github.com/google-gemini/gemini-cli) by Google LLC.

## 🙏 Acknowledgments

- [Google Gemini CLI](https://github.com/google-gemini/gemini-cli) — The incredible original
- [OpenAI Codex CLI](https://github.com/openai/codex) — TUI design inspiration
- [Google Gemini API](https://ai.google.dev/) — The underlying API

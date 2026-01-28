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
- **Interactive chat mode** — Multi-turn conversations with tool execution
- **Gemini 3 Pro support** — Full compatibility with latest models including `gemini-3-pro-preview`
- **Built-in tools** — File operations, search, and more (in chat mode)
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

# Chat with specific model
gmn chat -m gemini-3-pro-preview

# With file context
gmn "Review this code" -f main.go

# Pipe input
cat error.log | gmn "What's wrong?"

# JSON output
gmn "List 3 colors" -o json
```

## 💬 Interactive Chat

Start an interactive session with tool execution support:

```bash
gmn chat                           # Default model (gemini-2.5-flash)
gmn chat -m gemini-3-pro-preview   # Use Gemini 3 Pro
```

### Built-in Tools (Chat Mode)

| Tool                  | Description                    |
| --------------------- | ------------------------------ |
| `list_directory`      | List contents of a directory   |
| `read_file`           | Read file contents             |
| `write_file`          | Write content to a file        |
| `edit_file`           | Edit file by replacing text    |
| `glob`                | Find files matching a pattern  |
| `search_file_content` | Search for text/regex in files |

Tools are automatically called by Gemini when needed. You'll be prompted for confirmation before file modifications.

## 📋 Usage

```
gmn [prompt] [flags]
gmn chat [flags]
gmn mcp <command>

Commands:
  chat                         Start interactive chat session

Flags:
  -p, --prompt string          Prompt (alternative to positional arg)
  -m, --model string           Model (default "gemini-2.5-flash")
  -f, --file strings           Files to include
  -o, --output-format string   text, json, stream-json (default "text")
  -t, --timeout duration       Timeout (default 5m)
      --debug                  Debug output
  -v, --version                Version

Chat Flags:
  -m, --model string           Model (default "gemini-2.5-flash",
                               recommended for Code Assist: "gemini-3-pro-preview")

MCP Commands:
  gmn mcp list                 List MCP servers and tools
  gmn mcp call <server> <tool> Call an MCP tool
```

### Supported Models

| Model                  | Tier            | Notes                   |
| ---------------------- | --------------- | ----------------------- |
| `gemini-2.5-flash`     | Free / Standard | Default, fast responses |
| `gemini-2.5-pro`       | Free / Standard | More capable            |
| `gemini-3-pro-preview` | Standard        | Latest, best for coding |

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
- [Google Gemini API](https://ai.google.dev/) — The underlying API

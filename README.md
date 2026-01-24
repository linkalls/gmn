# gmn

A lightweight, non-interactive Gemini CLI written in Go — a love letter to [Google's Gemini CLI](https://github.com/google-gemini/gemini-cli).

## Why gmn?

The official Gemini CLI is an amazing tool with excellent MCP support and seamless Google authentication. However, for scripting and automation, its Node.js runtime adds startup overhead.

**gmn** reimplements the core functionality in Go, achieving **37x faster startup** while maintaining full compatibility with the official CLI's authentication.

| Metric | gmn | Official CLI |
|--------|-----|--------------|
| Startup time | **23ms** | 847ms |
| Binary size | 5.6MB | ~200MB (with node_modules) |
| Runtime | None | Node.js |

## Features

- **Blazing Fast** — Single binary, 23ms startup
- **Zero Config** — Reuses `~/.gemini/` from official CLI
- **Streaming** — Real-time text output by default
- **MCP Support** — Model Context Protocol client (stdio transport)
- **Cross-platform** — macOS, Linux, Windows

## Prerequisites

Authenticate once using the official Gemini CLI:

```bash
npm install -g @google/gemini-cli
gemini  # Choose "Login with Google"
```

gmn will reuse these credentials automatically.

## Installation

```bash
go install github.com/tomohiro-owada/gmn@latest
```

Or download from [Releases](https://github.com/tomohiro-owada/gmn/releases).

## Quick Start

```bash
# Simple prompt
gmn -p "Explain quantum computing in one sentence"

# With file context
gmn -f main.go -p "Review this code"

# Pipe input
cat error.log | gmn -p "What's wrong here?"

# JSON output
gmn -o json -p "List 3 colors"
```

## Usage

```
gmn [flags]

Flags:
  -p, --prompt string          Prompt to send (required)
  -m, --model string           Model to use (default "gemini-2.5-flash")
  -f, --file stringArray       Files to include in context
  -o, --output-format string   Output format: text, json, stream-json (default "text")
  -t, --timeout duration       API timeout (default 5m)
      --debug                  Enable debug output
  -h, --help                   Help for gmn
  -v, --version                Version for gmn
```

## MCP (Model Context Protocol)

gmn supports MCP servers configured in `~/.gemini/settings.json`:

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
# List available MCP servers and tools
gmn mcp list

# Call an MCP tool
gmn mcp call my-server tool-name arg1=value1
```

## Output Formats

| Format | Description | Use Case |
|--------|-------------|----------|
| `text` | Streaming plain text | Interactive use (default) |
| `json` | Structured JSON | Parsing responses |
| `stream-json` | NDJSON streaming | Real-time processing |

## Build from Source

```bash
git clone https://github.com/tomohiro-owada/gmn.git
cd gmn
make build          # Current platform
make cross-compile  # All platforms
```

## What's NOT Included

- Interactive/TUI mode (use official CLI)
- OAuth flow (authenticate with official CLI first)
- API Key / Vertex AI auth

## License

Apache License 2.0

This project is a derivative work based on [Gemini CLI](https://github.com/google-gemini/gemini-cli) by Google LLC. See [NOTICE](./NOTICE) for details.

## Acknowledgments

- [Google Gemini CLI](https://github.com/google-gemini/gemini-cli) — The incredible original that inspired this project
- [Google Gemini API](https://ai.google.dev/) — The underlying API

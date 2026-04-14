<p align="center">
  <img src="https://img.shields.io/github/v/release/edouard-claude/snip?style=flat-square" alt="Release">
  <img src="https://img.shields.io/github/actions/workflow/status/edouard-claude/snip/ci.yaml?branch=master&style=flat-square&label=CI" alt="CI">
  <img src="https://img.shields.io/github/license/edouard-claude/snip?style=flat-square" alt="License">
  <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go" alt="Go">
</p>

# snip - Reduce LLM Token Usage by 60-90%

**CLI proxy that filters shell output before it reaches your AI coding assistant's context window.** Works with Claude Code, Cursor, Copilot, Gemini CLI, Windsurf, Cline, Codex, Aider, and any tool that runs shell commands.

AI coding agents burn tokens on verbose shell output that adds zero signal. A passing `go test` produces hundreds of lines the LLM will never use. `git log` dumps full commit metadata when a one-liner per commit suffices.

snip sits between your AI tool and the shell, filtering output through **declarative YAML pipelines** -- no compiled filters, no code changes. Write a YAML file, drop it in a folder, done. An extensible [rtk](https://github.com/rtk-ai/rtk) alternative built in Go.

```
  snip — Token Savings Report
  ══════════════════════════════

  Commands filtered     128
  Tokens saved          2.3M
  Avg savings           99.8%
  Efficiency            Elite
  Total time            725.9s

  ███████████████████░ 100%

  14-day trend  ▁█▇

  Top commands by tokens saved

  Command                    Runs  Saved   Savings  Impact
  ─────────────────────────  ────  ──────  ───────  ────────────
  go test ./...              8     806.2K  99.8%    ████████████
  go test ./pkg/...          3     482.9K  99.8%    ███████░░░░░
  go test ./... -count=1     3     482.0K  99.8%    ███████░░░░░
```

> Measured on a real Claude Code session — 128 commands, 2.3M tokens saved.

## Quick Start

```bash
# Homebrew (macOS/Linux)
brew install edouard-claude/tap/snip

# Or with Go
go install github.com/edouard-claude/snip/cmd/snip@latest

# Then hook into Claude Code
snip init
# That's it. Every shell command Claude runs now goes through snip.
```

## How It Works

**Before** — Claude Code sees this (689 tokens):
```
$ go test ./...
ok  	github.com/edouard-claude/snip/internal/cli	3.728s	coverage: 14.4% of statements
ok  	github.com/edouard-claude/snip/internal/config	2.359s	coverage: 65.0% of statements
ok  	github.com/edouard-claude/snip/internal/display	1.221s	coverage: 72.6% of statements
ok  	github.com/edouard-claude/snip/internal/engine	1.816s	coverage: 47.9% of statements
ok  	github.com/edouard-claude/snip/internal/filter	4.306s	coverage: 72.3% of statements
ok  	github.com/edouard-claude/snip/internal/initcmd	2.981s	coverage: 59.1% of statements
ok  	github.com/edouard-claude/snip/internal/tee	0.614s	coverage: 70.6% of statements
ok  	github.com/edouard-claude/snip/internal/tracking	5.355s	coverage: 75.0% of statements
ok  	github.com/edouard-claude/snip/internal/utils	5.515s	coverage: 100.0% of statements
```

**After** — snip returns this (16 tokens):
```
10 passed, 0 failed
```

That's **97.7% fewer tokens**. The LLM gets the same signal — all tests pass — without the noise.

```
┌─────────────┐     ┌─────────────────┐     ┌──────────────┐     ┌────────────┐
│ Claude Code │────>│ snip intercept  │────>│ run command  │────>│   filter   │
│  runs git   │     │  match filter   │     │  capture I/O │     │  pipeline  │
└─────────────┘     └─────────────────┘     └──────────────┘     └─────┬──────┘
                                                                       │
                    ┌─────────────────┐     ┌──────────────┐           │
                    │   Claude Code   │<────│ track savings│<──────────┘
                    │  sees filtered  │     │  in SQLite   │
                    └─────────────────┘     └──────────────┘
```

No filter match? The command passes through unchanged — zero overhead.

### Token Savings by Command

| Command | Before | After | Reduction |
|---------|-------:|------:|----------:|
| `cargo test` | 591 tokens | 5 tokens | **99.2%** |
| `go test ./...` | 689 tokens | 16 tokens | **97.7%** |
| `git log` | 371 tokens | 53 tokens | **85.7%** |
| `git status` | 112 tokens | 16 tokens | **85.7%** |
| `git diff` | 355 tokens | 66 tokens | **81.4%** |

Stop wasting tokens on noise. snip gives the LLM the same signal in a fraction of the context window.

## Installation

### Homebrew (recommended)

```bash
brew install edouard-claude/tap/snip
```

### From GitHub Releases

Download the latest binary for your platform from [Releases](https://github.com/edouard-claude/snip/releases).

```bash
# macOS (Apple Silicon)
curl -Lo snip.tar.gz https://github.com/edouard-claude/snip/releases/latest/download/snip_$(curl -s https://api.github.com/repos/edouard-claude/snip/releases/latest | grep tag_name | cut -d'"' -f4 | tr -d v)_darwin_arm64.tar.gz
tar xzf snip.tar.gz && mv snip /usr/local/bin/
```

### From source

```bash
go install github.com/edouard-claude/snip/cmd/snip@latest
```

Or build locally:

```bash
git clone https://github.com/edouard-claude/snip.git
cd snip && make install
```

Requires Go 1.24+.

## Supported AI Tools

snip integrates with every major AI coding assistant. One binary, universal compatibility.

| Tool | Install | Method |
|------|---------|--------|
| **Claude Code** | `snip init` | PreToolUse hook |
| **Cursor** | hooks.json config | beforeShellExecution hook |
| **OpenCode** | [opencode-snip](https://github.com/VincentHardouin/opencode-snip) plugin | tool.execute.before hook |
| **GitHub Copilot** | shell aliases | prefix commands with snip |
| **Gemini CLI** | shell aliases | prefix commands with snip |
| **Codex (OpenAI)** | shell aliases | prefix commands with snip |
| **Windsurf** | shell aliases | prefix commands with snip |
| **Cline / Roo Code** | shell aliases | prefix commands with snip |
| **Aider** | shell aliases | prefix commands with snip |

### Claude Code

```bash
snip init
```

This installs a `PreToolUse` hook that transparently rewrites supported commands. Claude Code never sees the substitution -- it receives compressed output as if the original command produced it.

Supported commands: `git`, `go`, `cargo`, `npm`, `npx`, `yarn`, `pnpm`, `docker`, `kubectl`, `make`, `pip`, `pytest`, `jest`, `tsc`, `eslint`, `rustc`.

```bash
snip init --uninstall   # remove the hook
```

### Cursor

Cursor supports hooks since v1.7 via `~/.cursor/hooks.json`:

```json
{
  "version": 1,
  "hooks": {
    "beforeShellExecution": [
      { "command": "snip hook" }
    ]
  }
}
```

### OpenCode

Install the [opencode-snip](https://github.com/VincentHardouin/opencode-snip) plugin by adding it to your OpenCode config (`~/.config/opencode/opencode.json`):

```json
{
  "$schema": "https://opencode.ai/config.json",
  "plugin": ["opencode-snip@latest"]
}
```

The plugin uses the `tool.execute.before` hook to automatically prefix all commands with `snip`. Commands not supported by snip pass through unchanged.

### Copilot / Gemini / Codex / Windsurf / Cline / Aider

Use shell aliases to route commands through snip:

```bash
# Add to ~/.bashrc or ~/.zshrc
alias git="snip git"
alias go="snip go"
alias cargo="snip cargo"
```

Or instruct the LLM via system prompt to prefix commands with `snip`.

### Standalone

snip works without any AI tool:

```bash
snip git log -10
snip go test ./...
snip gain             # token savings report
```

## Usage

```bash
snip <command> [args]       # filter a command
snip gain                   # full dashboard (summary + sparkline + top commands)
snip gain --daily           # daily breakdown
snip gain --weekly          # weekly breakdown
snip gain --monthly         # monthly breakdown
snip gain --top 10          # top N commands by tokens saved
snip gain --history 20      # last 20 commands
snip gain --json            # machine-readable output
snip gain --csv             # CSV export
snip -v <command>           # verbose mode (show filter details)
snip proxy <command>        # force passthrough (no filtering)
snip config                 # show config
snip init                   # install Claude Code hook
snip init --uninstall       # remove hook
```

## Filters

Filters are declarative YAML files. The binary is the engine, filters are data — the two evolve independently.

```yaml
name: "git-log"
version: 1
description: "Condense git log to hash + message"

match:
  command: "git"
  subcommand: "log"
  exclude_flags: ["--format", "--pretty", "--oneline"]

inject:
  args: ["--pretty=format:%h %s (%ar) <%an>", "--no-merges"]
  defaults:
    "-n": "10"

pipeline:
  - action: "keep_lines"
    pattern: "\\S"
  - action: "truncate_lines"
    max: 80
  - action: "format_template"
    template: "{{.count}} commits:\n{{.lines}}"

on_error: "passthrough"
```

### Built-in Filters

| Filter | What it does |
|--------|-------------|
| `git-status` | Categorized status with file counts |
| `git-diff` | Stat summary, truncated to 30 files |
| `git-log` | One-line per commit: hash + message + author + date |
| `go-test` | Pass/fail summary with failure details |
| `cargo-test` | Pass/fail summary with failure details |

### 16 Pipeline Actions

| Action | Description |
|--------|-------------|
| `keep_lines` | Keep lines matching regex |
| `remove_lines` | Remove lines matching regex |
| `truncate_lines` | Truncate lines to max length |
| `strip_ansi` | Remove ANSI escape codes |
| `head` / `tail` | Keep first/last N lines |
| `group_by` | Group lines by regex capture |
| `dedup` | Deduplicate with optional normalization |
| `json_extract` | Extract fields from JSON |
| `json_schema` | Infer schema from JSON |
| `ndjson_stream` | Process newline-delimited JSON |
| `regex_extract` | Extract regex captures |
| `state_machine` | Multi-state line processing |
| `aggregate` | Count pattern matches |
| `format_template` | Go template formatting |
| `compact_path` | Shorten file paths |

### Custom Filters

```bash
snip init                                    # creates ~/.config/snip/filters/
vim ~/.config/snip/filters/my-tool.yaml      # add your filter
```

User filters take priority over built-in ones. Later directories in the list override earlier ones.

## Configuration

Optional TOML config at `~/.config/snip/config.toml`:

```toml
[tracking]
db_path = "~/.local/share/snip/tracking.db"

[display]
color = true
emoji = true
quiet_no_filter = false  # suppress "no filter" stderr messages

[filters]
dir = "~/.config/snip/filters"

[filters.enable]
# git-diff = false       # disable a specific built-in filter

[tee]
enabled = true
mode = "failures"    # "failures" | "always" | "never"
max_files = 20
max_file_size = 1048576
```

### Multiple Filter Directories

`filters.dir` accepts a single string or an array of directories. This enables per-project filter rules alongside global ones:

```toml
[filters]
dir = [
    "~/.config/snip/filters",
    "${env.PWD}/.snip",
]
```

Later directories take priority: a filter in `.snip/` overrides one with the same name in `~/.config/snip/filters/`.

### Environment Variable Expansion

All path values support `${env.VAR}` syntax to reference environment variables:

```toml
[filters]
dir = "${env.HOME}/.config/snip/filters"

[tracking]
db_path = "${env.XDG_DATA_HOME}/snip/tracking.db"
```

Tilde expansion (`~/`) is also supported and applied after env var expansion.

## Design

- **Startup < 10ms** — snip intercepts every shell command; latency is critical
- **Graceful degradation** — if a filter fails, fall back to raw output
- **Exit code preservation** — always propagate the underlying tool's exit code
- **Lazy regex compilation** — `sync.Once` per pattern, reused across invocations
- **Zero CGO** — pure Go SQLite driver, static binaries, trivial cross-compilation
- **Goroutine concurrency** — stdout/stderr captured in parallel without thread pools

## snip vs rtk (Rust Token Killer)

Looking for an rtk alternative? snip takes a fundamentally different approach to LLM token reduction: **filters are data, not code**.

| | **[rtk](https://github.com/rtk-ai/rtk)** (Rust) | **snip** (Go) |
|---|---|---|
| Filter authoring | Write Rust, recompile, wait for release | Write YAML, drop in a folder, done |
| Filter format | Compiled into the binary | Declarative YAML -- engine and filters evolve independently |
| Custom filters | Fork the repo, add Rust code | Create a `.yaml` file in `~/.config/snip/filters/` |
| Concurrency | 2 OS threads | Goroutines (lightweight, no thread pool) |
| SQLite | Requires CGO + C compiler | Pure Go driver -- static binary, no dependencies |
| Cross-compilation | Per-target C toolchain | `GOOS=linux GOARCH=arm64 go build` |
| Pipeline actions | Built-in strategies | 16 composable actions (keep, remove, regex, JSON, state machine...) |
| Contributing | Rust knowledge required | YAML knowledge sufficient |

Both tools solve the same problem -- reducing AI token costs from verbose CLI output. snip's bet is that **extensibility wins**: when anyone can write a filter in 5 minutes without touching Go or Rust, the filter ecosystem grows faster.

## Development

```bash
make build        # static binary (CGO_ENABLED=0)
make test         # all tests with coverage
make test-race    # race detector
make lint         # go vet + golangci-lint
make install      # install to $GOPATH/bin
```

## Documentation

Full documentation is available on the **[Wiki](https://github.com/edouard-claude/snip/wiki)**:

- [Installation](https://github.com/edouard-claude/snip/wiki/Installation) — Homebrew, Go, binaries (macOS/Linux/Windows), from source
- [Integration](https://github.com/edouard-claude/snip/wiki/Integration) — Claude Code, Cursor, Aider, standalone
- [Gain Dashboard](https://github.com/edouard-claude/snip/wiki/Gain-Dashboard) — Token savings reports and analytics
- [Filters](https://github.com/edouard-claude/snip/wiki/Filters) — Built-in filters, custom filters
- [Filter DSL Reference](https://github.com/edouard-claude/snip/wiki/Filter-DSL-Reference) — All 16 pipeline actions
- [Configuration](https://github.com/edouard-claude/snip/wiki/Configuration) — TOML config, environment variables
- [Architecture](https://github.com/edouard-claude/snip/wiki/Architecture) — Design decisions, internals
- [Contributing](https://github.com/edouard-claude/snip/wiki/Contributing) — Dev setup, adding filters, conventions

## Credits

Inspired by [rtk](https://github.com/rtk-ai/rtk) (Rust Token Killer) by the rtk-ai team. rtk proved that filtering shell output before it reaches the LLM context window is a powerful idea for cutting AI coding costs. snip rebuilds the concept in Go with a focus on extensibility -- declarative YAML filters that anyone can write without touching the codebase.

## License

MIT

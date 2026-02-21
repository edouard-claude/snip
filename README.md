<p align="center">
  <img src="https://img.shields.io/github/v/release/edouard-claude/snip?style=flat-square" alt="Release">
  <img src="https://img.shields.io/github/actions/workflow/status/edouard-claude/snip/ci.yaml?branch=master&style=flat-square&label=CI" alt="CI">
  <img src="https://img.shields.io/github/license/edouard-claude/snip?style=flat-square" alt="License">
  <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go" alt="Go">
</p>

# snip

**CLI proxy that cuts LLM token waste from shell output.**

AI coding agents burn tokens on verbose shell output that adds zero signal. A passing `go test` produces hundreds of lines the LLM will never use. `git log` dumps full commit metadata when a one-liner per commit suffices.

snip sits between your AI tool and the shell, filtering output through declarative YAML pipelines before it reaches the context window.

```
  snip — Token Savings Report
  ══════════════════════════════

  Commands filtered     102
  Tokens saved          935.9K
  Avg savings           99.8%
  Total time            626.1s

  ████████████████████ 100%
```

> Measured on a real Claude Code session — 102 commands, 935K tokens saved.

## Quick Start

```bash
# Install
go install github.com/edouard-claude/snip/cmd/snip@latest

# Hook into Claude Code
snip init

# That's it. Every shell command Claude runs now goes through snip.
```

## How It Works

```
Claude Code → PreToolUse hook → snip intercepts command
                                   ↓
                          Match declarative filter (O(1) lookup)
                                   ↓
                    Inject optimized args (e.g. --stat, --pretty=format:...)
                                   ↓
                  Execute command, capture stdout/stderr (goroutines)
                                   ↓
                    Apply filter pipeline (keep/remove/reformat)
                                   ↓
                  Return filtered output → track savings in SQLite
```

No filter match? The command passes through unchanged.

### Measured Savings

| Command | Before | After | Savings |
|---------|-------:|------:|--------:|
| `go test ./...` | 689 tokens | 16 tokens | **97.7%** |
| `cargo test` | 591 tokens | 5 tokens | **99.2%** |
| `git log` | 371 tokens | 53 tokens | **85.7%** |
| `git status` | 112 tokens | 16 tokens | **85.7%** |
| `git diff` | 355 tokens | 66 tokens | **81.4%** |

## Installation

### From GitHub Releases (recommended)

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

Requires Go 1.24+ and `jq` (for the hook script).

## Integration

### Claude Code

```bash
snip init
```

This installs a `PreToolUse` hook that transparently rewrites supported commands. Claude Code never sees the substitution — it receives compressed output as if the original command produced it.

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
      { "command": "~/.claude/hooks/snip-rewrite.sh" }
    ]
  }
}
```

### Aider / Windsurf / Other Tools

Use shell aliases:

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
snip gain                   # savings report
snip gain --daily           # daily breakdown
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

User filters take priority over built-in ones.

## Configuration

Optional TOML config at `~/.config/snip/config.toml`:

```toml
[tracking]
db_path = "~/.local/share/snip/tracking.db"

[display]
color = true
emoji = true

[filters]
dir = "~/.config/snip/filters"

[tee]
enabled = true
mode = "failures"    # "failures" | "always" | "never"
max_files = 20
max_file_size = 1048576
```

## Design

- **Startup < 10ms** — snip intercepts every shell command; latency is critical
- **Graceful degradation** — if a filter fails, fall back to raw output
- **Exit code preservation** — always propagate the underlying tool's exit code
- **Lazy regex compilation** — `sync.Once` per pattern, reused across invocations
- **Zero CGO** — pure Go SQLite driver, static binaries, trivial cross-compilation
- **Goroutine concurrency** — stdout/stderr captured in parallel without thread pools

## Why Go over Rust?

| | **rtk** (Rust) | **snip** (Go) |
|---|---|---|
| Filters | Compiled into the binary | Declarative YAML — no code needed |
| Concurrency | 2 OS threads | Goroutines |
| SQLite | Requires CGO + C compiler | Pure Go driver — static binary |
| Cross-compilation | Per-target C toolchain | `GOOS=linux GOARCH=arm64 go build` |
| Contributing a filter | Write Rust, wait for release | Write YAML, drop in a folder |

## Development

```bash
make build        # static binary (CGO_ENABLED=0)
make test         # all tests with coverage
make test-race    # race detector
make lint         # go vet + golangci-lint
make install      # install to $GOPATH/bin
```

## Credits

Inspired by [rtk](https://github.com/rtk-ai/rtk) by the rtk-ai team. rtk proved that filtering shell output before it reaches the LLM context is a powerful idea. snip rebuilds it in Go with a focus on extensibility — declarative YAML filters that anyone can write without touching the codebase.

## License

MIT

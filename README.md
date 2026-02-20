# snip

**CLI proxy that cuts 60-90% of LLM token waste from shell output.**

Inspired by [rtk](https://github.com/rtk-ai/rtk) (Rust Token Killer) — rebuilt in Go with a declarative filter DSL, goroutine-based concurrency, and zero CGO dependencies.

---

## The Problem

AI coding agents (Claude Code, Cursor, Aider) burn tokens proportional to shell output verbosity — not usefulness. A `git push` generates 15 lines to convey one bit: success or failure. A passing `go test` produces hundreds of lines the LLM will never use.

The result: sessions that exhaust prematurely, inflated API costs, and an agent drowning in noise instead of reasoning on signal.

## Why snip over rtk?

| | **rtk** (Rust) | **snip** (Go) |
|---|---|---|
| Filters | Compiled Rust code baked into the binary | Declarative YAML files — add a filter without writing Go |
| Concurrency | 2 OS threads for stdout/stderr | Goroutines — lightweight, no thread pool overhead |
| SQLite | Requires CGO + C compiler | Pure Go driver (`modernc.org/sqlite`) — true static binary |
| Cross-compilation | Needs per-target C toolchain for SQLite | `GOOS=linux GOARCH=arm64 go build` — done |
| Contributing a filter | Write Rust, open PR, wait for release | Write YAML, drop in `~/.config/snip/filters/` |
| Binary size | ~4MB | ~9MB (includes SQLite in pure Go) |

snip is a clean-room reimplementation. Same concept, different trade-offs: **community extensibility** over compiled performance.

## How It Works

```
Claude Code → PreToolUse hook → snip intercepts command
                                   ↓
                          Match declarative filter
                                   ↓
                    Inject optimized args (e.g. --pretty=format:...)
                                   ↓
                  Execute command, capture stdout/stderr (goroutines)
                                   ↓
                    Apply filter pipeline (keep/remove/reformat)
                                   ↓
                  Output filtered result → track savings in SQLite
```

### Token Savings (measured on test fixtures)

| Command | Before | After | Savings |
|---------|--------|-------|---------|
| `git log` | 371 tokens | 53 tokens | **85.7%** |
| `git status` | 112 tokens | 16 tokens | **85.7%** |
| `git diff` | 355 tokens | 66 tokens | **81.4%** |
| `go test` | 689 tokens | 16 tokens | **97.7%** |
| `cargo test` | 591 tokens | 5 tokens | **99.2%** |

## Installation

### From source

```bash
git clone https://github.com/YOUR_USERNAME/snip.git
cd snip
make build
make install
```

### Setup Claude Code integration

```bash
snip init
```

This installs a PreToolUse hook that transparently rewrites supported commands through snip. Supported commands: `git`, `go`, `cargo`, `npm`, `npx`, `yarn`, `pnpm`, `docker`, `kubectl`, `make`, `pip`, `pytest`, `jest`, `tsc`, `eslint`, `rustc`.

## Usage

```bash
# Filter any supported command
snip git log -10
snip go test ./...
snip git status
snip cargo test

# Token savings report
snip gain
snip gain --daily
snip gain --json

# Verbose mode (see what snip does)
snip -v git log -5

# Direct passthrough (no filtering)
snip proxy ls -la

# Show config
snip config
```

## Filter DSL

Filters are YAML files with a simple structure:

```yaml
name: "git-log"
version: 1
description: "Condense git log to hash + message + author + date"

match:
  command: "git"
  subcommand: "log"
  exclude_flags: ["--format", "--pretty", "--graph", "--oneline"]

inject:
  args: ["--pretty=format:%h %s (%ar) <%an>", "--no-merges"]
  defaults:
    "-n": "10"
  skip_if_present: ["--merges", "--format", "--pretty", "--oneline"]

pipeline:
  - action: "keep_lines"
    pattern: "\\S"
  - action: "truncate_lines"
    max: 80
  - action: "format_template"
    template: "{{.count}} commits:\n{{.lines}}"

on_error: "passthrough"
```

### 16 Built-in Actions

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

### Adding Custom Filters

Drop a YAML file in `~/.config/snip/filters/`:

```bash
snip init  # creates the directory
vim ~/.config/snip/filters/my-tool.yaml
```

User filters take priority over built-in ones.

## Architecture

```
cmd/snip/main.go           Entry point
internal/
  cli/                     CLI routing, flag parsing (no cobra — raw os.Args for <10ms startup)
  config/                  TOML config (~/.config/snip/config.toml)
  engine/
    executor.go            Goroutine-based stdout/stderr capture
    pipeline.go            Filter matching → arg injection → execute → filter → tee → track
  filter/
    types.go               DSL types (Filter, Match, Inject, Pipeline, Action)
    actions.go             16 built-in actions
    parser.go              YAML parsing + validation
    registry.go            O(1) command→filter lookup
    loader.go              Embedded FS + user directory loading
  tracking/                SQLite token tracking (90-day retention, pure Go)
  tee/                     Raw output recovery on failure
  display/                 Lipgloss terminal styling + gain reports
  utils/                   Truncate, StripANSI, EstimateTokens, LazyRegex
filters/*.yaml             5 MVP filters (embedded via go:embed)
```

## Development

```bash
make build        # Static binary (CGO_ENABLED=0)
make test         # All tests with coverage
make test-race    # Race detector
make lint         # go vet + golangci-lint
```

Requires Go 1.24+.

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
mode = "failures"    # "failures", "always", "never"
max_files = 20
max_file_size = 1048576
```

## Credits

snip is directly inspired by [rtk](https://github.com/rtk-ai/rtk) by the rtk-ai team. rtk proved that intercepting and filtering shell output before it reaches the LLM context is a powerful way to reduce token consumption. snip takes that idea and rebuilds it in Go with a focus on extensibility — declarative YAML filters that anyone can write, goroutine-based concurrency, and a pure Go stack with zero CGO.

## License

MIT

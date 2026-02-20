# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**snip** is a CLI proxy written in Go that reduces LLM token consumption by 60-90% by filtering shell output before it reaches the LLM context. Inspired by [rtk](https://github.com/rtk-ai/rtk) (Rust Token Killer), snip improves on the concept with a **declarative filter DSL** — filters are YAML config files, not compiled code.

## Key Concept

The binary (snip) is the engine. Filters are data files. The two evolve independently. Anyone can contribute a filter without knowing Go.

## Repository Structure

```
cmd/snip/main.go        # Entry point
embed.go                # Embedded default filters (go:embed)
filters/*.yaml          # Declarative filter definitions (MVP: 5 filters)
internal/
  cli/                  # CLI routing, flag parsing
  config/               # TOML config loading (~/.config/snip/config.toml)
  display/              # Lipgloss terminal styling, gain report
  engine/               # Command execution (goroutines), pipeline orchestration
  filter/               # DSL types, 16 built-in actions, YAML parser, registry
  initcmd/              # Claude Code hook installation
  tracking/             # SQLite token tracking (pure Go, no CGO)
  tee/                  # Raw output recovery on failure
  utils/                # Truncate, StripANSI, EstimateTokens, LazyRegex
tests/fixtures/         # Test fixtures for integration tests
```

## Architecture

### Core Loop

1. Intercept command via Claude Code PreToolUse hook
2. Route to matching filter (O(1) registry lookup)
3. Execute original command, capture stdout/stderr via goroutines
4. Apply declarative filter pipeline (regex-based: keep/remove lines, reformat, template)
5. Output filtered result, track token savings in SQLite

### Why Go over Rust

- Static binaries, no runtime dependencies, trivial cross-compilation
- Goroutines naturally solve the stdout/stderr concurrent read problem (vs 2 OS threads in rtk)
- Lower barrier to entry for community contributions
- Pure Go SQLite driver (no CGO needed)

## Development Commands

```bash
make build               # Build static binary (CGO_ENABLED=0)
make test                # Run all tests with coverage
make test-race           # Run tests with race detector
make lint                # go vet + golangci-lint
make install             # Install to $GOPATH/bin
go test -run TestName ./internal/filter/...   # Single test
```

## Design Constraints

- **Startup < 10ms** — snip intercepts every shell command; latency is critical
- **Graceful degradation** — if a filter fails, fall back to raw command output
- **Exit code preservation** — always propagate the underlying tool's exit code
- **No async runtime** — goroutines are sufficient; avoid heavy dependencies
- **Lazy compilation** — compile regex once (sync.Once), reuse across invocations
- **Minimal memory** — stream and filter line-by-line, don't buffer entire output

## Filter DSL

Filters are declarative YAML files with 16 built-in actions:
`keep_lines`, `remove_lines`, `truncate_lines`, `strip_ansi`, `head`, `tail`,
`group_by`, `dedup`, `json_extract`, `json_schema`, `ndjson_stream`,
`regex_extract`, `state_machine`, `aggregate`, `format_template`, `compact_path`

## Conventions

- Respond in French, but all code, comments, variable names, commits, and documentation files must be in English
- Direct communication style — no hedging, state facts and solutions
- TDD workflow: write test first, implement, refactor
- Use context wrapping on errors: `fmt.Errorf("operation: %w", err)`

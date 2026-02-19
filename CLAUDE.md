# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**snip** is a CLI proxy written in Go that reduces LLM token consumption by 60-90% by filtering shell output before it reaches the LLM context. It is the Go successor to [rtk](https://github.com/rtk-ai/rtk) (Rust Token Killer), adding a **decentralized filter registry** where filters are declarative config files rather than compiled code.

**Current state**: Pre-development (vision phase). See `VISION.md` for the full design document.

## Key Concept

The binary (snip) is the engine. Filters are data files hosted in a separate versioned repository. The two evolve independently. Anyone can contribute a filter without knowing Go.

## Repository Structure

```
go.mod              # Module: snip, Go 1.24.3
VISION.md           # Architecture vision and design decisions
source_rtk/         # rtk Rust source (reference implementation, read-only)
```

### Reference Implementation: source_rtk/

`source_rtk/` is a clone of the rtk Rust project. Use it as architectural reference — do not modify it. Key files to study:

- `source_rtk/src/main.rs` — CLI entry point, Clap command routing, global flags (-v, -u)
- `source_rtk/src/git.rs` — Git command proxy (largest module, status/diff/log/add/commit/push)
- `source_rtk/src/filter.rs` — Language-aware code filtering (none/minimal/aggressive levels)
- `source_rtk/src/tracking.rs` — SQLite token tracking (~4 chars/token heuristic, 90-day retention)
- `source_rtk/src/utils.rs` — Shared utilities (truncate, strip_ansi, execute_command, pkg manager detection)
- `source_rtk/src/tee.rs` — Raw output recovery on failure (saves to file, prints hint for LLM)
- `source_rtk/CLAUDE.md` — Detailed rtk developer documentation
- `source_rtk/ARCHITECTURE.md` — Full architectural overview with diagrams

## Architecture (from VISION.md)

### Core Loop

1. Intercept command via Claude Code PreToolUse hook
2. Route to matching filter (from local cache of registry)
3. Execute original command, capture stdout/stderr via goroutines
4. Apply declarative filter rules (regex-based: keep/remove lines, reformat, template summary)
5. Output filtered result, track token savings in SQLite

### Two Repositories

1. **snip** (this repo) — Go binary: command dispatch, filter cache, registry updates, token tracking
2. **Filter registry** (separate repo) — Declarative filter files per tool, versioned, signed, community-maintained

### Why Go over Rust

- Static binaries, no runtime dependencies, trivial cross-compilation
- Goroutines naturally solve the stdout/stderr concurrent read problem (vs 2 OS threads in rtk)
- Lower barrier to entry for community contributions
- Pure Go SQLite driver (no CGO needed)

## Development Commands

```bash
go build -o snip .                    # Build binary
go test ./...                         # Run all tests
go test -run TestName ./path/...      # Run single test
go test -race ./...                   # Race detector
go vet ./...                          # Static analysis
golangci-lint run                     # Lint (if installed)
```

## Design Constraints (carried from rtk)

- **Startup < 10ms** — snip intercepts every shell command; latency is critical
- **Graceful degradation** — if a filter fails, fall back to raw command output
- **Exit code preservation** — always propagate the underlying tool's exit code for CI/CD
- **No async runtime** — goroutines are sufficient; avoid heavy dependencies
- **Lazy compilation** — compile regex once (sync.Once or init), reuse across invocations
- **Minimal memory** — stream and filter line-by-line, don't buffer entire output

## Filter Format (planned)

Filters are declarative YAML/TOML files describing how to process a command's output:
- Which lines to remove (regex patterns)
- Which lines to keep
- How to reformat the result
- Summary template to apply

Filters are downloaded from the registry, cached locally, and verified by digital signature. Unsigned filters are rejected unless the user opts in for local/third-party filters.

## Conventions

- Respond in French, but all code, comments, variable names, commits, and documentation files must be in English
- Direct communication style — no hedging, state facts and solutions
- TDD workflow: write test first, implement, refactor
- Use `context` wrapping on errors (like anyhow in Rust): `fmt.Errorf("operation: %w", err)`

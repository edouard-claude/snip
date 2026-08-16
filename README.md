<p align="center">
  <img src="https://img.shields.io/github/v/release/edouard-claude/snip?style=flat-square" alt="Release">
  <img src="https://img.shields.io/github/actions/workflow/status/edouard-claude/snip/ci.yaml?branch=master&style=flat-square&label=CI" alt="CI">
  <img src="https://img.shields.io/github/license/edouard-claude/snip?style=flat-square" alt="License">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go" alt="Go">
</p>

# snip - Reduce LLM Token Usage by 60-90%

**CLI proxy that filters shell output before it reaches your AI coding assistant's context window.** Works with Claude Code, Cursor, Copilot, Gemini CLI, Windsurf, Cline, Codex, Pi, Grok Build, Kilo Code, Antigravity, OpenCode, OpenClaw, Aider, and any tool that runs shell commands.

AI coding agents burn tokens on verbose shell output that adds zero signal. A passing `go test` produces hundreds of lines the LLM will never use. `git log` dumps full commit metadata when a one-liner per commit suffices.

snip sits between your AI tool and the shell, filtering output through **declarative YAML pipelines**. Write a YAML file, drop it in a folder, done. The extensible LLM token optimizer: filters are YAML data files, not compiled code.

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
# Quick install (macOS/Linux)
curl -fsSL https://raw.githubusercontent.com/edouard-claude/snip/master/install.sh | sh

# Or via Homebrew
brew install edouard-claude/tap/snip

# Or with Go
go install github.com/edouard-claude/snip/cmd/snip@latest

# Then hook into Claude Code
snip init
# That's it. Every shell command Claude runs now goes through snip.
```

## How It Works

**Before** — Claude Code sees this (275 tokens):
```
$ go test ./...
ok  	github.com/edouard-claude/snip	13.697s
?   	github.com/edouard-claude/snip/cmd/snip	[no test files]
ok  	github.com/edouard-claude/snip/internal/cli	1.357s
ok  	github.com/edouard-claude/snip/internal/config	1.898s
ok  	github.com/edouard-claude/snip/internal/discover	4.271s
ok  	github.com/edouard-claude/snip/internal/display	2.516s
ok  	github.com/edouard-claude/snip/internal/economics	3.073s
ok  	github.com/edouard-claude/snip/internal/engine	5.262s
ok  	github.com/edouard-claude/snip/internal/filter	3.687s
ok  	github.com/edouard-claude/snip/internal/hook	4.555s
ok  	github.com/edouard-claude/snip/internal/hookaudit	4.560s
ok  	github.com/edouard-claude/snip/internal/initcmd	4.534s
ok  	github.com/edouard-claude/snip/internal/inspect	4.598s
ok  	github.com/edouard-claude/snip/internal/learn	4.474s
ok  	github.com/edouard-claude/snip/internal/tee	4.430s
ok  	github.com/edouard-claude/snip/internal/tracking	4.055s
ok  	github.com/edouard-claude/snip/internal/trust	4.544s
ok  	github.com/edouard-claude/snip/internal/utils	4.528s
ok  	github.com/edouard-claude/snip/internal/verify	4.517s
```

**After** — snip returns this (8 tokens):
```
1125 passed, 0 failed
```

That's **97% fewer tokens**, measured on this very repository. The filter injects `-json` and counts individual test results, so the LLM gets more signal — 1125 tests passed, not just 18 packages — in a fraction of the space.

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
| `go test ./...` | 275 tokens | 8 tokens | **97.1%** |
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

`make install` and `make install-lite` use the first available destination:
an explicit `GOBIN`, `go env GOBIN`, or the first `go env GOPATH` entry plus
`/bin`. Use `make upgrade` or `make upgrade-lite` to replace the resolved
`snip` on `PATH` instead. An explicit `GOBIN` overrides the upgrade destination.

Requires Go 1.25+.

## Supported AI Tools

snip integrates with every major AI coding assistant. One binary, universal compatibility.

| Tool | Install | Method |
|------|---------|--------|
| **Claude Code** | `snip init` | PreToolUse hook (native) |
| **Cursor** | `snip init --agent cursor` | beforeShellExecution hook (native) |
| **GitHub Copilot** | `snip init --agent copilot` | preToolUse hook (native) |
| **Gemini CLI** | `snip init --agent gemini` | GEMINI.md prompt injection |
| **Codex (OpenAI)** | `snip init --agent codex` | PreToolUse hook (native) |
| **Pi (pi.dev)** | `snip init --agent pi` | PreToolUse hook (via [pi-hooks](https://github.com/hsingjui/pi-hooks)) |
| **Grok Build (xAI)** | `snip init --agent grok` | PreToolUse hook (deny + re-run suggestion) |
| **Windsurf** | `snip init --agent windsurf` | .windsurfrules prompt injection |
| **Cline / Roo Code** | `snip init --agent cline` | .clinerules prompt injection |
| **Kilo Code** | `snip init --agent kilocode` | .kilocode/rules/ prompt injection |
| **Antigravity** | `snip init --agent antigravity` | .agents/rules/ prompt injection |
| **OpenCode** | [opencode-snip](https://github.com/VincentHardouin/opencode-snip) plugin | tool.execute.before hook |
| **OpenClaw** | `openclaw plugins install openclaw-snip` | plugin |
| **Aider** | shell aliases | prefix commands with snip |

### Claude Code

```bash
snip init
```

This installs a `PreToolUse` hook that transparently rewrites supported commands. Claude Code never sees the substitution -- it receives compressed output as if the original command produced it.

Supported commands: 132 filters covering 100 distinct commands: git, go, cargo, npm, yarn, pnpm, docker, kubectl, terraform, aws, gh, dotnet, and many more.

```bash
snip init --uninstall   # remove the hook
```

### Cursor

```bash
snip init --agent cursor
```

This patches `~/.cursor/hooks.json` with a `beforeShellExecution` hook. Works the same way as Claude Code.

```bash
snip init --agent cursor --uninstall   # remove the hook
```

### Pi (pi.dev)

```bash
snip init --agent pi
```

This patches `~/.pi/agent/settings.json` with a `PreToolUse` entry matching the `bash` tool. The runtime hook is interpreted by the community extension [`@hsingjui/pi-hooks`](https://github.com/hsingjui/pi-hooks), which mirrors Claude Code's `hookSpecificOutput` format (including command rewriting via `updatedInput`). Install it once:

```bash
pi install npm:@hsingjui/pi-hooks
```

Then run `/reload` (or restart Pi). Once active, snip rewrites supported commands transparently.

```bash
snip init --agent pi --uninstall   # remove the hook
```

### Grok Build (xAI)

```bash
snip init --agent grok
```

This writes `~/.grok/hooks/snip.json` with a `PreToolUse` hook matching the shell tool. Grok Build hooks cannot rewrite commands in place (the hook contract is allow/deny only), so snip denies matched commands with a re-run suggestion (`"…/snip" run -- <command>`). Non-matching commands pass through untouched, and the hook is fail-open: if snip breaks, commands simply run unfiltered.

Prefer prompt injection instead? Grok Build reads `AGENTS.md` natively:

```bash
snip init --agent grok --mode prompt   # creates AGENTS.md
snip init --agent grok --uninstall     # remove the hook
```

`AGENTS.md` is shared with Codex, so `--uninstall` never deletes it; it prints a reminder instead.

### Codex

```bash
snip init --agent codex
```

This patches `~/.codex/hooks.json` with a native `PreToolUse` hook. Supported shell commands are transparently rewritten through snip with Codex's `updatedInput` contract, so the model runs commands normally and sees only the filtered output.

For safety, mixed commands containing unsupported segments, pipelines with uninspected tails, and command substitutions pass through unchanged so Codex keeps its native permission flow.

Use `snip proxy -- <command>` when full unfiltered output is required.

```bash
snip init --agent codex --uninstall
```

Codex CLI 0.131.0 or later is required for `PreToolUse` input rewriting. For older releases, use the legacy project-scoped prompt integration:

```bash
snip init --agent codex --mode prompt
```

### GitHub Copilot

```bash
snip init --agent copilot
```

This patches `~/.copilot/hooks/snip.json` with a native `preToolUse` hook that rewrites supported commands transparently, like Claude Code.

Prefer prompt injection instead? Use the legacy project-scoped mode:

```bash
snip init --agent copilot --mode prompt   # creates .github/copilot-instructions.md
snip init --agent copilot --uninstall     # remove the hook
```

### Gemini / Windsurf / Cline / Kilo Code / Antigravity

```bash
snip init --agent gemini       # creates GEMINI.md
snip init --agent windsurf     # creates .windsurfrules
snip init --agent cline        # creates .clinerules
snip init --agent kilocode     # creates .kilocode/rules/snip-rules.md
snip init --agent antigravity  # creates .agents/rules/snip-rules.md
```

These agents use prompt injection: a markdown file instructs the LLM to prefix shell commands with snip. Project-scoped (created in the current directory).

### OpenCode

Install the [opencode-snip](https://github.com/VincentHardouin/opencode-snip) plugin by adding it to your OpenCode config (`~/.config/opencode/opencode.json`):

```json
{
  "$schema": "https://opencode.ai/config.json",
  "plugin": ["opencode-snip@latest"]
}
```

The plugin uses the `tool.execute.before` hook to automatically prefix all commands with `snip`. Commands not supported by snip pass through unchanged.

### OpenClaw

```bash
openclaw plugins install openclaw-snip
```

### Aider

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
snip <command> [args]       # filter a command (implicit)
snip run -- <command>       # same, with explicit separator
snip check -- <command>     # check if a command would be filtered
snip proxy <command>        # force passthrough (no filtering)
snip proxy -- <command>     # same, with explicit separator
snip gain                   # full dashboard (summary + sparkline + top commands)
snip gain --daily           # daily breakdown
snip gain --weekly          # weekly breakdown
snip gain --monthly         # monthly breakdown
snip gain --top 10          # top N commands by tokens saved
snip gain --history 20      # last 20 commands
snip gain --quota           # savings against plan quotas
snip gain --unfiltered      # opt-in report on unfiltered commands
snip gain --no-truncate     # disable command truncation
snip gain --json            # machine-readable output
snip gain --csv             # CSV export
snip cc-economics           # financial impact by API tier (haiku/sonnet/opus)
snip discover               # find missed savings in Claude Code history
snip discover --since 30    # scan last 30 days
snip discover --all         # scan all projects
snip learn                  # detect CLI error-correction patterns in sessions
snip verify                 # run the filters' inline tests
snip config                 # show config
snip trust [path]           # trust project-local filter file(s) by SHA-256
snip untrust [path]         # remove file(s) from the trust store
snip hook                   # agent PreToolUse handler (used by the hooks)
snip hook-audit             # show recent hook activity (SNIP_HOOK_AUDIT=1)
snip init                       # install Claude Code hook
snip init --agent cursor        # install Cursor hook
snip init --agent pi            # install Pi (pi.dev) hook
snip init --agent copilot       # install Copilot hook
snip init --agent gemini        # install Gemini CLI integration
snip init --agent kilocode      # install Kilo Code integration
snip init --agent antigravity   # install Antigravity integration
snip init --uninstall           # remove hook
```

Global flags: `-v`/`-vv` (verbose, stackable), `-u` (ultra-compact), `--skip-env`, `--version`, `--help`.

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

`match.subcommand` can be a scalar string (as above) or a list of exact subcommands:

```yaml
match:
  command: "npm"
  subcommand: ["install", "add", "i"]
```

If `subcommand` is omitted, the filter matches every subcommand for that command. To match only a bare command invocation, include an explicit empty string, for example `subcommand: ["", "install"]` to match `yarn` and `yarn install` without matching `yarn why`.

### 132 Built-in Filters

snip ships with **132 declarative YAML filters** covering all major developer tools:

| Category | Filters |
|----------|---------|
| **Git** (12) | status, log, diff, show, add, commit, push, pull, branch, fetch, stash, worktree |
| **GitHub CLI** (3) | gh pr, gh issue, gh run |
| **Go** (4) | go test, go build, go vet, golangci-lint |
| **Rust** (7) | cargo test/build/check/clippy/install/nextest, rustc |
| **Python** (11) | pytest, ruff, mypy, basedpyright, ty, pip, poetry, uv add/lock/remove/sync |
| **JavaScript/TypeScript** (17) | jest, vitest, eslint, tsc, biome, oxlint, prettier, next, playwright, nx, turbo, npm, npx, yarn, pnpm install/list, prisma |
| **Ruby** (6) | rspec, rubocop, rake, bundle, rails migrate, rails routes |
| **.NET** (3) | dotnet build/test/format |
| **Elixir** (2) | mix compile, mix format |
| **Docker/K8s** (7) | docker build/ps/images/logs/compose, kubectl get/logs |
| **Cloud/Infra** (6) | terraform, tofu, helm, ansible-playbook, gcloud, aws |
| **Build tools** (14) | make, gcc, g++, gradle, gradlew, gradlew.bat, mvn, swift, xcodebuild, just, task, pio, trunk, mise |
| **Files/Search** (7) | ls, find, grep, rg, diff, wc, tree |
| **Linting** (6) | shellcheck, hadolint, markdownlint, markdownlint-cli2, yamllint, pre-commit |
| **Package managers** (2) | brew, composer |
| **System/Network** (14) | curl, wget, psql, jq, ping, ssh, rsync, df, du, ps, systemctl, iptables, stat, fail2ban |
| **Other** (11) | jira, jj, yadm, gt, ollama, sops, skopeo, shopify, quarto, liquibase, spring-boot |

Run `snip discover` to see which of your commands already have filters.

### 20 Pipeline Actions

| Action | Description |
|--------|-------------|
| `keep_lines` | Keep lines matching regex |
| `remove_lines` | Remove lines matching regex |
| `truncate_lines` | Truncate lines to max length |
| `truncate_bytes` | Hard cap on output size in bytes |
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
| `compact_path` | Shorten file paths (see caveat below) |
| `replace` | Regex find and replace |
| `match_output` | Conditional short-circuit (return message if pattern matches) |
| `on_empty` | Return message if output is empty |

> **`compact_path` emits paths that may not resolve.** It strips a leading
> `src/`, `lib/`, `internal/`, `pkg/` or `vendor/` segment unconditionally and
> with no marker, so `internal/soak/report.go` becomes `soak/report.go` — which
> `ENOENT`s from the directory the command ran in. No bundled filter uses it.
> Reach for it only when the path is display-only and will never be opened.

### Custom Filters

```bash
snip init                                    # creates ~/.config/snip/filters/
vim ~/.config/snip/filters/my-tool.yaml      # add your filter
```

User filters take priority over built-in ones. Later directories in the list override earlier ones.

Filters under `~/.config/snip/` are always loaded. Filters anywhere else (for example a project's `.snip/` directory) must be approved once with `snip trust`, otherwise they are skipped:

```bash
snip trust .snip/filters     # approve the project's filters (SHA-256 pinned)
```

Editing a trusted file invalidates its hash; run `snip trust` again after changes.

## Configuration

Optional TOML config at `~/.config/snip/config.toml` (override the path with `SNIP_CONFIG`):

```toml
# mode = "user"          # "project" in a .snip/config.toml lets that file
                         # override user settings (see Project Configuration)

[tracking]
db_path = "~/.local/share/snip/tracking.db"
track_unfiltered = false # opt-in: also record commands that had no filter

[display]
color = true
emoji = true
quiet_no_filter = false  # suppress "no filter" stderr messages
summary = false          # prepend a "[snip: ...]" line showing which filter/args were applied

[filters]
dir = "~/.config/snip/filters"

[filters.enable]
# git-diff = false       # disable a specific built-in filter

[filters.global]         # safety caps appended to every filter's pipeline (0 = unlimited)
# max_lines = 0          # cap the number of output lines
# max_line_length = 0    # cap each line's length
# max_output_bytes = 0   # hard cap on the bytes any filter emits.
                         # Applied last, cutting on a UTF-8 rune boundary and
                         # appending a "... truncated at N bytes" marker that is
                         # counted inside the cap.

[filters.override.dotnet-test]  # tune a single filter without rewriting it
# head = 200             # raise dotnet-test's cap from 40 to 200 lines
# stream_mode = "full"   # or skip this filter's pipeline entirely
# Other overridable keys: tail, truncate_lines, keep_lines, remove_lines

[filters.bypass]
# commands = ["dotnet publish"]  # always run these unfiltered

[tee]
enabled = true
mode = "failures"    # "failures" | "always" | "never"
max_files = 20
max_file_size = 1048576
# project_marker = ".git"  # write tee files to <repo-root>/.snip/tee/ instead
                           # of the global dir when the marker is found by
                           # walking up from the working directory; a
                           # .gitignore is created there so logs are never
                           # committed, and snip falls back to the global dir
                           # if the repo root is not writable
```

> Override values must be positive: `head = 0` does not remove a filter's
> truncation, it is ignored. To get unlimited output from one filter, use
> `stream_mode = "full"` in its override block.

Full reference for every key, default and merge rule: [Configuration wiki page](https://github.com/edouard-claude/snip/wiki/Configuration).

### Verify Your Configuration

Every setting above is reproducible. To prove one is active without touching your real config, point `SNIP_CONFIG` at a scratch file:

```bash
cd "$(mktemp -d)" && touch a.go b.md c.txt
export SNIP_CONFIG=$PWD/test.toml

printf '[filters.override.ls]\nhead = 2\n' > test.toml
snip ls    # 2 entries + "... +more entries (truncated by snip)"

printf '[filters.bypass]\ncommands = ["ls"]\n' > test.toml
snip ls    # raw ls output, no filtering

unset SNIP_CONFIG
```

`snip config` prints the user config snip loads, `snip check -- <command>` names the filter that would handle a command, and `snip -v <command>` shows the filter and injected args as it runs.

### Project Configuration

A repo can ship its own `.snip/config.toml`; snip walks up from the working directory and uses the first one it finds. The file must be trusted once (`snip trust .snip/config.toml`) and declare `mode = "project"` to override the user config. Project keys that participate in the merge: `filters.enable`, `filters.global`, `filters.override` (project wins), and `filters.bypass.commands` (concatenated, regardless of mode). Typical corporate setup: defaults for every developer live in the repo, personal tweaks stay in `~/.config/snip/config.toml`.

### Plugin Configuration

An agent plugin (for example a Claude Code plugin) can ship a snip config that applies on every machine as the **base** of the cascade `plugin < user < project`. The plugin's hook exports the path:

```bash
SNIP_PLUGIN_CONFIG=${CLAUDE_PLUGIN_ROOT}/snip/config.toml
```

The file is a regular snip TOML restricted to the filter sections (`filters.enable`, `filters.global`, `filters.override`, `filters.bypass`, `transparent_prefixes`, and `filters.dir` so the plugin can ship its own filter YAMLs; plugin filters load first, so user filters win by name). A relative `filters.dir` resolves against the TOML's own directory, so `dir = "filters"` just works. The variable only needs to exist in the hook's environment: rewritten commands carry the path along as a `--plugin-config` flag, so the agent's shell needs no special environment. Like a project config, it must be trusted once:

```bash
snip trust "$SNIP_PLUGIN_CONFIG"               # the config itself
snip trust "${SNIP_PLUGIN_CONFIG%/*}/filters"  # the plugin's filters
```

A plugin typically runs both commands from a session-start hook, so plugin updates re-trust automatically. Anything the user or the repo sets overrides the plugin layer. Full details on the [Configuration wiki page](https://github.com/edouard-claude/snip/wiki/Configuration).

```toml
# .snip/config.toml — checked into the repo
mode = "project"

[filters.override.dotnet-test]
head = 500               # this monorepo runs thousands of tests

[filters.bypass]
commands = ["dotnet publish"]
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

Directories outside `~/.config/snip/` go through the trust store: run `snip trust ${PWD}/.snip` once per project (and after each edit), or the files in it are silently skipped.

### Environment Variable Expansion

`tracking.db_path` and `filters.dir` support `${env.VAR}` syntax to reference environment variables:

```toml
[filters]
dir = "${env.HOME}/.config/snip/filters"

[tracking]
db_path = "${env.XDG_DATA_HOME}/snip/tracking.db"
```

Tilde expansion (`~/`) is also supported and applied after env var expansion.

### Runner Prefixes

When a command is run through a runner wrapper, snip strips the wrapper, applies the inner command's filter, and leaves the wrapper in place. So `uv run pytest` is filtered by the `pytest` filter with no extra configuration -- no need to copy or duplicate the filter.

```bash
uv run pytest -v        # filtered by the pytest filter
uv run --python 3.12 pytest   # runner flags before the command are skipped
poetry run ruff check .       # filtered by the ruff filter
```

Built-in prefixes: `uv run`, `poetry run`, `pdm run`, `pipenv run`, `rye run`, `hatch run`, and the shell wrappers `noglob`, `nocorrect`, `command`, `exec`. Add your own (e.g. a container or env wrapper) with `transparent_prefixes`:

```toml
[filters]
transparent_prefixes = ["docker exec mycontainer", "direnv exec ."]
```

Detection is fail-closed: the first non-flag token after the prefix must be a known snip command, otherwise the command passes through untouched. A runner executing an unknown program (e.g. `uv run bash -c ...`) is never rewritten and never auto-allowed, preserving snip's confirmation-prompt guarantee.

## Design

- **Startup < 10ms** — snip intercepts every shell command; latency is critical
- **Graceful degradation** — if a filter fails, fall back to raw output
- **Exit code preservation** — always propagate the underlying tool's exit code
- **Lazy regex compilation** — `sync.Once` per pattern, reused across invocations
- **Zero CGO** — pure Go SQLite driver, static binaries, trivial cross-compilation
- **Goroutine concurrency** — stdout/stderr captured in parallel without thread pools

## Design Philosophy

snip chose a fundamentally different approach to LLM token reduction: **filters are data, not code**. The binary is the engine, filters are YAML data files, and the two evolve independently.

| | **[rtk](https://github.com/rtk-ai/rtk)** (Rust) | **snip** (Go) |
|---|---|---|
| Filter authoring | Write Rust, recompile, wait for release | Write YAML, drop in a folder, done |
| Filter format | Compiled into the binary | Declarative YAML, engine and filters evolve independently |
| Custom filters | Fork the repo, add Rust code | Create a `.yaml` file in `~/.config/snip/filters/` |
| Concurrency | 2 OS threads | Goroutines (lightweight, no thread pool) |
| SQLite | Requires CGO + C compiler | Pure Go driver, static binary, no dependencies |
| Cross-compilation | Per-target C toolchain | `GOOS=linux GOARCH=arm64 go build` |
| Pipeline actions | Built-in strategies | 20 composable actions (keep, remove, regex, JSON, state machine...) |
| Contributing | Rust knowledge required | YAML knowledge sufficient |

Both tools solve the same problem: reducing AI token costs from verbose CLI output. snip's bet is that **extensibility wins**. When anyone can write a filter in 5 minutes without touching Go or Rust, the filter ecosystem grows faster.

## Development

```bash
make build        # static binary (CGO_ENABLED=0)
make build-lite   # build without SQLite tracking (-tags lite, ~5MB smaller)
make test         # all tests with coverage
make test-race    # race detector
make verify       # run the filters' inline tests
make lint         # go vet + golangci-lint (pinned version)
make vulncheck    # govulncheck ./...
make ci           # pre-PR gate: test-race + verify + lint + vulncheck
make install      # install using GOBIN or the Go environment
make upgrade      # replace the active snip binary (GOBIN overrides)
```

## Documentation

Full documentation is available on the **[Wiki](https://github.com/edouard-claude/snip/wiki)**:

- [Installation](https://github.com/edouard-claude/snip/wiki/Installation) — Homebrew, Go, binaries (macOS/Linux/Windows), from source
- [Integration](https://github.com/edouard-claude/snip/wiki/Integration) — Claude Code, Cursor, Copilot, Gemini, Kilo Code, Antigravity, and more
- [Gain Dashboard](https://github.com/edouard-claude/snip/wiki/Gain-Dashboard) — Token savings reports and analytics
- [Filters](https://github.com/edouard-claude/snip/wiki/Filters) — Built-in filters, custom filters
- [Filter DSL Reference](https://github.com/edouard-claude/snip/wiki/Filter-DSL-Reference) — All 20 pipeline actions
- [Configuration](https://github.com/edouard-claude/snip/wiki/Configuration) — TOML config, environment variables
- [Architecture](https://github.com/edouard-claude/snip/wiki/Architecture) — Design decisions, internals
- [Contributing](https://github.com/edouard-claude/snip/wiki/Contributing) — Dev setup, adding filters, conventions

## Credits

Inspired by [rtk](https://github.com/rtk-ai/rtk) (Rust Token Killer) by the rtk-ai team. rtk proved that filtering shell output before it reaches the LLM context window is a powerful idea for cutting AI coding costs. snip rebuilds the concept in Go with a focus on extensibility -- declarative YAML filters that anyone can write without touching the codebase.

## License

MIT

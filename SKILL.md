# Creating snip Filters

You are an expert at writing declarative YAML filters for **snip**, a CLI proxy that reduces LLM token consumption by filtering shell output.

## Filter File Location

- **Built-in filters**: `filters/*.yaml` (embedded in the binary at build time)
- **User filters**: `~/.config/snip/filters/*.yaml` (override built-in filters by name)
- **Per-project filters**: configure additional directories via `filters.dir` array in `~/.config/snip/config.toml` (e.g. `dir = ["~/.config/snip/filters", "${env.PWD}/.snip"]`). Later directories take priority.

**Trust store**: filters in any directory outside `~/.config/snip/` are ignored (with a stderr warning) until approved once with `snip trust <dir>`, which pins each file's SHA-256. Re-run `snip trust` after editing a trusted file.

## Filter Structure

Every filter is a YAML file with this structure:

```yaml
name: "tool-subcommand"          # Required. Unique identifier, used for registry lookup.
version: 1                       # Informational revision number of the filter (bump on behavior change).
description: "What this filter does"  # Human-readable purpose.

match:                           # Required. When to apply this filter.
  command: "tool"                # Required. The CLI tool name (e.g., "git", "go", "npm").
  subcommand: "sub"             # Optional. First non-flag argument (e.g., "test", "log").
                                # Also accepts a list: ["install", "add", "i"]; include ""
                                # in the list to match the bare command invocation too.
  exclude_flags: ["-v", "--json"]  # Optional. Skip filter if user passes any of these.
  require_flags: ["--all"]      # Optional. Only apply if user passes ALL of these.

inject:                          # Optional. Modify command args before execution.
  args: ["--json"]              # Arguments to append to the command.
  defaults:                     # Flag defaults, only added if flag not already present.
    "-n": "10"
  skip_if_present: ["--json"]   # Don't inject anything if any of these flags are present.

streams: ["stdout", "stderr"]    # Optional. Which streams to filter. Default: ["stdout"].
                                 # Use ["stderr"] for tools that output to stderr (e.g., bun test).
                                 # Use ["stdout", "stderr"] to filter both streams merged together.

pipeline:                        # Required. Ordered list of transformation actions.
  - action: "keep_lines"
    pattern: "\\S"
  - action: "head"
    n: 20

on_error: "passthrough"          # Descriptive convention: the engine ALWAYS falls back to raw
                                 # output when a pipeline fails; no other value is implemented.
```

## Match Rules

- `command` is matched exactly against the first token of the shell command.
- `subcommand` is matched against the first non-flag argument.
- Flag matching uses **prefix matching**: `"-v"` matches both `-v` and `-verbose`.
- Registry lookup is O(1) by key `"command"` or `"command:subcommand"`.

## Inject Behavior

- Injected `args` are inserted before any `--` separator, otherwise appended.
- `defaults` only apply if their flag key is not already present in the user's args.
- If any flag in `skip_if_present` is found, the entire inject block is skipped.

## The 20 Pipeline Actions

### Line Filtering

| Action | Params | Description |
|--------|--------|-------------|
| `keep_lines` | `pattern` (regex) | Keep only lines matching the pattern |
| `remove_lines` | `pattern` (regex) | Remove lines matching the pattern |
| `head` | `n` (int, default 10), `overflow_msg` (string, default "+{remaining} more lines") | Keep first N lines |
| `tail` | `n` (int, default 10), `overflow_msg` (string, default "+{dropped} earlier lines") | Keep last N lines |
| `dedup` | `normalize` ([]string of regexes to strip before comparing), `top` (int, 0=all) | Deduplicate lines, output "text (xN)" for repeats |

### Line Transformation

| Action | Params | Description |
|--------|--------|-------------|
| `truncate_lines` | `max` (int, default 80), `ellipsis` (string, default "...") | Truncate long lines |
| `replace` | `pattern` (regex), `replacement` (string, supports $1, $2...) | Regex find and replace on each line |
| `truncate_bytes` | `max` (int, 0=disabled), `overflow_msg` (string, default "... truncated at {max} bytes") | Cap the whole output at `max` bytes, cutting on a UTF-8 rune boundary. The marker is paid for out of `max`, and is dropped when it alone would not fit |
| `strip_ansi` | (none) | Remove ANSI escape codes |
| `compact_path` | (none) | Strips a leading `src/`/`lib/`/`internal/`/`pkg/`/`vendor/` segment. The result may not resolve from the cwd, and carries no marker saying so — no bundled filter uses it. Display-only paths only. |

### Extraction & Grouping

| Action | Params | Description |
|--------|--------|-------------|
| `regex_extract` | `pattern` (regex with capture groups), `format` (string using $0, $1, $2...) | Extract data via regex capture groups |
| `group_by` | `pattern` (regex with capture group), `format` (template, default "{{.Key}}: {{.Count}}"), `top` (int) | Group lines by capture group, count occurrences |
| `aggregate` | `patterns` (map of name->regex), `format` (Go template), `append` (bool) | Count lines matching named patterns. **Replaces** the input lines with the summary unless `append: true` (forgetting it caused bugs #134/#136: a correct count and no content) |
| `state_machine` | `states` (map of state definitions with `keep`, `until`, `next`) | Stateful line filtering with transitions |

### JSON Processing

| Action | Params | Description |
|--------|--------|-------------|
| `json_extract` | `fields` ([]string), `format` (template, optional) | Extract fields from JSON input |
| `json_schema` | `max_depth` (int, default 3) | Output JSON type schema |
| `ndjson_stream` | `group_by` (string field name), `format` (template with .Key, .Count, .Events) | Process newline-delimited JSON |

### Formatting & Conditionals

| Action | Params | Description |
|--------|--------|-------------|
| `format_template` | `template` (Go text/template, required) | Format output using Go template |
| `match_output` | `pattern` (regex), `message` (string, default "ok"), `unless` (regex) | If the whole output matches `pattern`, replace it with `message`; `unless` takes priority and passes through |
| `on_empty` | `message` (string) | Return `message` when the output is empty or whitespace-only |

### Template Data for `format_template`

The template receives:
- `{{.lines}}` - all current lines joined with newlines
- `{{.count}}` - number of lines
- `{{.groups}}` - map from `group_by` action (if used earlier in pipeline)
- `{{.stats}}` - map from `aggregate` action (if used earlier in pipeline)

**`{{.count}}` trap**: it counts the lines *reaching the template*, not entities. After any stage that emits a summary, an overflow marker or a cap, the number is wrong (caused bug #125). Prefer the tool's own count over recomputing one.

### Metadata Flow Between Actions

- `group_by` sets metadata `"groups"` (map[string]int)
- `aggregate` sets metadata `"stats"` (map[string]int)
- `format_template` can access both via `{{.groups}}` and `{{.stats}}`
- All other actions pass metadata through unchanged

## Design Principles

1. **Start with `keep_lines` pattern `"\\S"`** to strip blank lines early.
2. **Use `inject` to request machine-readable output** (e.g., `--json`, `--porcelain`) then filter that structured data.
3. **Respect user intent**: use `exclude_flags` to skip filtering when the user explicitly requests a different format.
4. **Keep the conventional `on_error: "passthrough"` line**: it documents the engine's actual fallback behavior (raw output on any pipeline failure).
5. **Chain actions from broad to specific**: filter noise first, then extract, then format.
6. **Keep output minimal but useful**: the goal is 60-90% token reduction while preserving actionable information.

## Examples

### Simple: remove noise lines

```yaml
name: "npm-install"
version: 1
description: "Condensed npm install output"
match:
  command: "npm"
  subcommand: "install"
pipeline:
  - action: "remove_lines"
    pattern: "^(npm warn|npm notice)"
  - action: "keep_lines"
    pattern: "\\S"
  - action: "aggregate"
    patterns:
      added: "^added "
      removed: "^removed "
      up_to_date: "up to date"
    format: "{{if gt .up_to_date 0}}up to date{{else}}{{.added}} added, {{.removed}} removed{{end}}"
on_error: "passthrough"
```

### Intermediate: inject flags + extract structured data

```yaml
name: "go-test"
version: 1
description: "Condensed go test output with pass/fail summary"
match:
  command: "go"
  subcommand: "test"
  exclude_flags: ["-json", "-v", "-bench", "-run"]
inject:
  args: ["-json"]
  skip_if_present: ["-json", "-v", "-bench"]
pipeline:
  - action: "keep_lines"
    pattern: "\\S"
  - action: "keep_lines"
    pattern: "\"Test\":\""
  - action: "keep_lines"
    pattern: "\"Action\":\"(pass|fail)\""
  - action: "aggregate"
    patterns:
      passed: '"Action":"pass"'
      failed: '"Action":"fail"'
    format: "{{if and (eq .passed 0) (eq .failed 0)}}No tests found{{else}}{{.passed}} passed, {{.failed}} failed{{end}}"
on_error: "passthrough"
```

### Advanced: state machine for multi-section output

```yaml
name: "cargo-test"
version: 2
description: "cargo test summary lines, plus the failure report when a test fails"
match:
  command: "cargo"
  subcommand: "test"
pipeline:
  - action: "remove_lines"
    pattern: "^\\s*(Compiling|Downloading|Downloaded|Updating|Running|Executable)"
  - action: "keep_lines"
    pattern: "\\S"
  - action: "state_machine"
    states:
      start:
        keep: "^test result"
        until: "^failures"
        next: "failures"
      failures:
        keep: "."
  - action: "head"
    n: 150
  - action: "format_template"
    template: "{{.lines}}"
on_error: "passthrough"
```

The `until` on `start` is what makes this safe. Everything after the first
`failures:` line is the failure report, and a panic message can hold any text at
all — including a line that looks exactly like a test result. So every stage that
drops or counts lines has to sit on the `start` side of that boundary, which
`keep` does. A `remove_lines` placed after the state machine would instead run
over the panic messages and delete them.

## Workflow to Create a New Filter

1. **Identify the command** and its typical verbose output.
2. **Run the command** and capture raw output to understand the structure.
3. **Decide what to keep**: what information does the LLM actually need?
4. **Check if the tool has a machine-readable flag** (--json, --porcelain, etc.) that would make filtering easier -- use `inject` if so.
5. **Write the pipeline**: strip blanks, filter/extract, aggregate, format.
6. **Test the filter** by placing it in `~/.config/snip/filters/` and running the command through snip.
7. **To contribute**: add the YAML to `filters/` in the repo and submit a PR.

# Creating snip Filters

You are an expert at writing declarative YAML filters for **snip**, a CLI proxy that reduces LLM token consumption by filtering shell output.

## Filter File Location

- **Built-in filters**: `filters/*.yaml` (embedded in the binary at build time)
- **User filters**: `~/.config/snip/filters/*.yaml` (override built-in filters by name)

## Filter Structure

Every filter is a YAML file with this structure:

```yaml
name: "tool-subcommand"          # Required. Unique identifier, used for registry lookup.
version: 1                       # Schema version (always 1 for now).
description: "What this filter does"  # Human-readable purpose.

match:                           # Required. When to apply this filter.
  command: "tool"                # Required. The CLI tool name (e.g., "git", "go", "npm").
  subcommand: "sub"             # Optional. First non-flag argument (e.g., "test", "log").
  exclude_flags: ["-v", "--json"]  # Optional. Skip filter if user passes any of these.
  require_flags: ["--all"]      # Optional. Only apply if user passes ALL of these.

inject:                          # Optional. Modify command args before execution.
  args: ["--json"]              # Arguments to append to the command.
  defaults:                     # Flag defaults, only added if flag not already present.
    "-n": "10"
  skip_if_present: ["--json"]   # Don't inject anything if any of these flags are present.

pipeline:                        # Required. Ordered list of transformation actions.
  - action: "keep_lines"
    pattern: "\\S"
  - action: "head"
    n: 20

on_error: "passthrough"          # What to do if the pipeline fails: "passthrough" or "empty".
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

## The 16 Pipeline Actions

### Line Filtering

| Action | Params | Description |
|--------|--------|-------------|
| `keep_lines` | `pattern` (regex) | Keep only lines matching the pattern |
| `remove_lines` | `pattern` (regex) | Remove lines matching the pattern |
| `head` | `n` (int, default 10), `overflow_msg` (string, default "+{remaining} more lines") | Keep first N lines |
| `tail` | `n` (int, default 10) | Keep last N lines |
| `dedup` | `normalize` ([]string of regexes to strip before comparing), `top` (int, 0=all) | Deduplicate lines, output "text (xN)" for repeats |

### Line Transformation

| Action | Params | Description |
|--------|--------|-------------|
| `truncate_lines` | `max` (int, default 80), `ellipsis` (string, default "...") | Truncate long lines |
| `strip_ansi` | (none) | Remove ANSI escape codes |
| `compact_path` | (none) | Remove directory prefixes from file paths |

### Extraction & Grouping

| Action | Params | Description |
|--------|--------|-------------|
| `regex_extract` | `pattern` (regex with capture groups), `format` (string using $0, $1, $2...) | Extract data via regex capture groups |
| `group_by` | `pattern` (regex with capture group), `format` (template, default "{{.Key}}: {{.Count}}"), `top` (int) | Group lines by capture group, count occurrences |
| `aggregate` | `patterns` (map of name->regex), `format` (Go template) | Count matches for named patterns across all input |
| `state_machine` | `states` (map of state definitions with `keep`, `until`, `next`) | Stateful line filtering with transitions |

### JSON Processing

| Action | Params | Description |
|--------|--------|-------------|
| `json_extract` | `fields` ([]string), `format` (template, optional) | Extract fields from JSON input |
| `json_schema` | `max_depth` (int, default 3) | Output JSON type schema |
| `ndjson_stream` | `group_by` (string field name), `format` (template with .Key, .Count, .Events) | Process newline-delimited JSON |

### Formatting

| Action | Params | Description |
|--------|--------|-------------|
| `format_template` | `template` (Go text/template, required) | Format output using Go template |

### Template Data for `format_template`

The template receives:
- `{{.lines}}` - all current lines joined with newlines
- `{{.count}}` - number of lines
- `{{.groups}}` - map from `group_by` action (if used earlier in pipeline)
- `{{.stats}}` - map from `aggregate` action (if used earlier in pipeline)

### Metadata Flow Between Actions

- `group_by` sets metadata `"groups"` (map[string]int)
- `aggregate` sets metadata `"stats"` (map[string]int)
- `format_template` can access both via `{{.groups}}` and `{{.stats}}`
- All other actions pass metadata through unchanged

## Design Principles

1. **Start with `keep_lines` pattern `"\\S"`** to strip blank lines early.
2. **Use `inject` to request machine-readable output** (e.g., `--json`, `--porcelain`) then filter that structured data.
3. **Respect user intent**: use `exclude_flags` to skip filtering when the user explicitly requests a different format.
4. **Always set `on_error: "passthrough"`** so raw output is returned if filtering fails.
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
version: 1
description: "Condensed cargo test output"
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
        keep: "^(test |running |test result)"
        until: "^failures"
        next: "failures"
      failures:
        keep: "."
        until: "^$"
        next: "done"
  - action: "aggregate"
    patterns:
      pass: "\\.\\.\\. ok$"
      fail: "\\.\\.\\. FAILED$"
      ignored: "\\.\\.\\. ignored$"
  - action: "format_template"
    template: "{{.lines}}"
on_error: "passthrough"
```

## Workflow to Create a New Filter

1. **Identify the command** and its typical verbose output.
2. **Run the command** and capture raw output to understand the structure.
3. **Decide what to keep**: what information does the LLM actually need?
4. **Check if the tool has a machine-readable flag** (--json, --porcelain, etc.) that would make filtering easier -- use `inject` if so.
5. **Write the pipeline**: strip blanks, filter/extract, aggregate, format.
6. **Test the filter** by placing it in `~/.config/snip/filters/` and running the command through snip.
7. **To contribute**: add the YAML to `filters/` in the repo and submit a PR.

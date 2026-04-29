package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestUnproxyableCommands(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{"cd", true},
		{"chdir", true},
		{"pushd", true},
		{"popd", true},
		{"source", true},
		{".", true},
		{"export", true},
		{"unset", true},
		{"alias", true},
		{"unalias", true},
		{"readonly", true},
		{"declare", true},
		{"typeset", true},
		{"local", true},
		{"shift", true},
		{"read", true},
		{"mapfile", true},
		{"readarray", true},
		{"let", true},
		{"getopts", true},
		{"set", true},
		{"shopt", true},
		{"setopt", true},
		{"unsetopt", true},
		{"emulate", true},
		{"eval", true},
		{"exec", true},
		{"exit", true},
		{"logout", true},
		{"return", true},
		{"break", true},
		{"continue", true},
		{"wait", true},
		{"bg", true},
		{"fg", true},
		{"disown", true},
		{"jobs", true},
		{"suspend", true},
		{"bindkey", true},
		{"bind", true},
		{"complete", true},
		{"compopt", true},
		{"compinit", true},
		{"zstyle", true},
		{"autoload", true},
		{"zmodload", true},
		{"enable", true},
		{"disable", true},
		{"abbr", true},
		{"functions", true},
		{"hash", true},
		{"trap", true},
		{"umask", true},
		{"ulimit", true},
		{"git", false},
		{"go", false},
		{"docker", false},
		{"echo", false},
		{"printf", false},
		{"pwd", false},
		{"test", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := unproxyableReason(tt.command) != ""
			if got != tt.want {
				t.Errorf("unproxyableReason(%q) returned %q, wantBlocked=%v", tt.command, unproxyableReason(tt.command), tt.want)
			}
		})
	}
}

func TestRunRejectsCd(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	code := Run([]string{"snip", "cd", "/tmp"})
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 1 {
		t.Errorf("Run(cd) = %d, want 1", code)
	}
	output := buf.String()
	if !strings.Contains(output, "cd") {
		t.Errorf("expected stderr to contain 'cd', got %q", output)
	}
}

func TestRunSubcommandMissingSeparator(t *testing.T) {
	code := Run([]string{"snip", "run", "git", "log"})
	if code != 1 {
		t.Errorf("Run(run without --) = %d, want 1", code)
	}
}

func TestRunSubcommandEmptyAfterSeparator(t *testing.T) {
	code := Run([]string{"snip", "run", "--"})
	if code != 1 {
		t.Errorf("Run(run --) = %d, want 1", code)
	}
}

func TestRunSubcommandRejectsUnproxyable(t *testing.T) {
	code := Run([]string{"snip", "run", "--", "cd", "/tmp"})
	if code != 1 {
		t.Errorf("Run(run -- cd) = %d, want 1", code)
	}
}

func TestRunSubcommandRejectsArgsBeforeSeparator(t *testing.T) {
	code := Run([]string{"snip", "run", "foo", "--", "bar"})
	if code != 1 {
		t.Errorf("Run(run foo -- bar) = %d, want 1", code)
	}
}

func TestRunGlobalHelpBeforeSeparator(t *testing.T) {
	code := Run([]string{"snip", "run", "--help", "--", "foo", "bar"})
	if code != 0 {
		t.Errorf("Run(run --help -- foo bar) = %d, want 0", code)
	}
}

func TestRunSubcommandWithFlags(t *testing.T) {
	flags, remaining := ParseFlags([]string{"-v", "run", "--", "git", "log", "-10"})
	if flags.Verbose != 1 {
		t.Errorf("flags.Verbose = %d, want 1", flags.Verbose)
	}
	wantRemaining := []string{"run", "--", "git", "log", "-10"}
	if !reflect.DeepEqual(remaining, wantRemaining) {
		t.Errorf("remaining = %v, want %v", remaining, wantRemaining)
	}
}

func TestCheckMissingSeparator(t *testing.T) {
	code := Run([]string{"snip", "check", "git", "log"})
	if code != 1 {
		t.Errorf("Run(check without --) = %d, want 1", code)
	}
}

func TestCheckEmptyAfterSeparator(t *testing.T) {
	code := Run([]string{"snip", "check", "--"})
	if code != 1 {
		t.Errorf("Run(check --) = %d, want 1", code)
	}
}

func TestCheckShellBuiltin(t *testing.T) {
	code := Run([]string{"snip", "check", "--", "cd", "/tmp"})
	if code != 1 {
		t.Errorf("Run(check -- cd) = %d, want 1", code)
	}
}

func TestCheckShellBuiltinExport(t *testing.T) {
	code := Run([]string{"snip", "check", "--", "export", "FOO=bar"})
	if code != 1 {
		t.Errorf("Run(check -- export) = %d, want 1", code)
	}
}

func TestCheckShellBuiltinSet(t *testing.T) {
	code := Run([]string{"snip", "check", "--", "set", "-e"})
	if code != 1 {
		t.Errorf("Run(check -- set) = %d, want 1", code)
	}
}

func TestCheckShellBuiltinExit(t *testing.T) {
	code := Run([]string{"snip", "check", "--", "exit"})
	if code != 1 {
		t.Errorf("Run(check -- exit) = %d, want 1", code)
	}
}

func TestRunRejectsSource(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	code := Run([]string{"snip", "source", "script.sh"})
	_ = w.Close()
	os.Stderr = old
var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if code != 1 {
		t.Errorf("Run(source) = %d, want 1", code)
	}
	output := buf.String()
	if !strings.Contains(output, "source") {
		t.Errorf("expected stderr to contain 'source', got %q", output)
	}
	if !strings.Contains(output, "cannot be proxied") {
		t.Errorf("expected stderr to contain 'cannot be proxied', got %q", output)
	}
}

func TestRunRejectsDot(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	code := Run([]string{"snip", ".", "script.sh"})
	_ = w.Close()
	os.Stderr = old
var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if code != 1 {
		t.Errorf("Run(.) = %d, want 1", code)
	}
	output := buf.String()
	if !strings.Contains(output, ".") {
		t.Errorf("expected stderr to contain '.', got %q", output)
	}
	if !strings.Contains(output, "cannot be proxied") {
		t.Errorf("expected stderr to contain 'cannot be proxied', got %q", output)
	}
}

func TestRunSubcommandRejectsUnproxyableErrorMessage(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	code := Run([]string{"snip", "run", "--", "cd", "/tmp"})
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 1 {
		t.Errorf("Run(run -- cd) = %d, want 1", code)
	}
	output := buf.String()
	if !strings.Contains(output, "cd") {
		t.Errorf("expected stderr to contain 'cd', got %q", output)
	}
	if !strings.Contains(output, "cannot be proxied") {
		t.Errorf("expected stderr to contain 'cannot be proxied', got %q", output)
	}
}

func TestRunGlobalUnproxyableErrorMessage(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	code := Run([]string{"snip", "cd", "/tmp"})
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 1 {
		t.Errorf("Run(cd) = %d, want 1", code)
	}
	output := buf.String()
	if !strings.Contains(output, "cd") {
		t.Errorf("expected stderr to contain 'cd', got %q", output)
	}
	if !strings.Contains(output, "cannot be proxied") {
		t.Errorf("expected stderr to contain 'cannot be proxied', got %q", output)
	}
}

func TestCheckRejectsSource(t *testing.T) {
	code := Run([]string{"snip", "check", "--", "source", "script.sh"})
	if code != 1 {
		t.Errorf("Run(check -- source) = %d, want 1", code)
	}
}

func TestCheckRejectsDot(t *testing.T) {
	code := Run([]string{"snip", "check", "--", ".", "script.sh"})
	if code != 1 {
		t.Errorf("Run(check -- .) = %d, want 1", code)
	}
}

func TestCheckRejectsArgsBeforeSeparator(t *testing.T) {
	code := Run([]string{"snip", "check", "foo", "--", "bar"})
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
}

func TestCheckNoFilter(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := os.MkdirAll(filterDir, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", filepath.Join(home, ".config", "snip", "config.toml"))

	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := Run([]string{"snip", "check", "--", "ls", "-la"})
	_ = w.Close()
	os.Stdout = old
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(buf.String(), "no filter") {
		t.Errorf("expected output to contain 'no filter', got %q", buf.String())
	}
}

func TestCheckExcludedByFlags(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := os.MkdirAll(filterDir, 0o755); err != nil {
		t.Fatal(err)
	}

	filterYAML := `name: "git-log"
version: 1
description: "Test filter"
match:
  command: "git"
  subcommand: "log"
  exclude_flags: ["--format", "--pretty", "--graph", "--oneline"]
inject:
  args: ["--pretty=format:%h %s", "--no-merges"]
  defaults:
    "-n": "10"
  skip_if_present: ["--merges", "--format", "--pretty", "--oneline"]
pipeline:
  - action: "keep_lines"
    pattern: "\\S"
on_error: "passthrough"
`
	if err := os.WriteFile(filepath.Join(filterDir, "git-log.yaml"), []byte(filterYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", filepath.Join(home, ".config", "snip", "config.toml"))

	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := Run([]string{"snip", "check", "--", "git", "log", "--pretty"})
	_ = w.Close()
	os.Stdout = old
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(buf.String(), "excluded by flags") {
		t.Errorf("expected output to contain 'excluded by flags', got %q", buf.String())
	}
}

func TestCheckShellBuiltinOutputIncludesCommand(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	code := Run([]string{"snip", "check", "--", "cd", "/tmp"})
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	output := buf.String()
	if !strings.Contains(output, "cd") {
		t.Errorf("expected output to contain 'cd', got %q", output)
	}
	if !strings.HasPrefix(strings.TrimSpace(output), "snip: cd is") {
		t.Errorf("expected output to start with 'snip: cd is', got %q", output)
	}
}

func TestCheckShellBuiltinSourceOutput(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	code := Run([]string{"snip", "check", "--", "source", "script.sh"})
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	output := buf.String()
	if !strings.Contains(output, "source") {
		t.Errorf("expected output to contain 'source', got %q", output)
	}
	if !strings.Contains(output, "shell builtin") {
		t.Errorf("expected output to contain 'shell builtin', got %q", output)
	}
}

func TestCheckShellBuiltinDotOutput(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	code := Run([]string{"snip", "check", "--", ".", "script.sh"})
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	output := buf.String()
	if !strings.Contains(output, ".") {
		t.Errorf("expected output to contain '.', got %q", output)
	}
	if !strings.Contains(output, "shell builtin") {
		t.Errorf("expected output to contain 'shell builtin', got %q", output)
	}
}

func TestCheckFilterFoundOutput(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := os.MkdirAll(filterDir, 0o755); err != nil {
		t.Fatal(err)
	}

	filterYAML := `name: "git-log"
version: 1
description: "Test filter"
match:
  command: "git"
  subcommand: "log"
  exclude_flags: ["--format", "--pretty", "--graph", "--oneline"]
inject:
  args: ["--pretty=format:%h %s", "--no-merges"]
  defaults:
    "-n": "10"
  skip_if_present: ["--merges", "--format", "--pretty", "--oneline"]
pipeline:
  - action: "keep_lines"
    pattern: "\\S"
on_error: "passthrough"
`
	if err := os.WriteFile(filepath.Join(filterDir, "git-log.yaml"), []byte(filterYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", filepath.Join(home, ".config", "snip", "config.toml"))

	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := Run([]string{"snip", "check", "--", "git", "log"})
	_ = w.Close()
	os.Stdout = old
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(buf.String(), "filter: git-log") {
		t.Errorf("expected output to contain 'filter: git-log', got %q", buf.String())
	}
}

func TestParseSeparatorArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		cmdName   string
		wantCmd   string
		wantArgs  []string
		wantErr   string
	}{
		{
			name:     "normal case",
			args:     []string{"--", "git", "log", "-10"},
			cmdName:  "run",
			wantCmd:  "git",
			wantArgs: []string{"log", "-10"},
			wantErr:  "",
		},
		{
			name:     "single command after separator",
			args:     []string{"--", "docker"},
			cmdName:  "run",
			wantCmd:  "docker",
			wantArgs: []string{},
			wantErr:  "",
		},
		{
			name:     "no separator",
			args:     []string{"git", "log"},
			cmdName:  "run",
			wantCmd:  "",
			wantArgs: nil,
			wantErr:  "run requires -- separator: snip run -- <command> [args...]",
		},
		{
			name:     "empty after separator",
			args:     []string{"--"},
			cmdName:  "run",
			wantCmd:  "",
			wantArgs: nil,
			wantErr:  "run requires a command after --",
		},
		{
			name:     "args before separator",
			args:     []string{"foo", "--", "bar"},
			cmdName:  "run",
			wantCmd:  "",
			wantArgs: nil,
			wantErr:  "run: unexpected arguments before -- (foo)",
		},
		{
			name:     "command with embedded double dash",
			args:     []string{"--", "git", "--", "log"},
			cmdName:  "run",
			wantCmd:  "git",
			wantArgs: []string{"--", "log"},
			wantErr:  "",
		},
		{
			name:     "check command name in error",
			args:     []string{"git"},
			cmdName:  "check",
			wantCmd:  "",
			wantArgs: nil,
			wantErr:  "check requires -- separator: snip check -- <command> [args...]",
		},
		{
			name:     "check empty after separator",
			args:     []string{"--"},
			cmdName:  "check",
			wantCmd:  "",
			wantArgs: nil,
			wantErr:  "check requires a command after --",
		},
		{
			name:     "multiple args before separator",
			args:     []string{"-v", "extra", "--", "git", "log"},
			cmdName:  "run",
			wantCmd:  "",
			wantArgs: nil,
			wantErr:  "run: unexpected arguments before -- (-v extra)",
		},
		{
			name:     "separator is second arg",
			args:     []string{"-v", "--", "git", "log"},
			cmdName:  "run",
			wantCmd:  "",
			wantArgs: nil,
			wantErr:  "run: unexpected arguments before -- (-v)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, args, errMsg := parseSeparatorArgs(tt.args, tt.cmdName)
			if cmd != tt.wantCmd {
				t.Errorf("cmd = %q, want %q", cmd, tt.wantCmd)
			}
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
			}
			if errMsg != tt.wantErr {
				t.Errorf("errMsg = %q, want %q", errMsg, tt.wantErr)
			}
		})
	}
}

func TestCheckWithCommandOnlyFilter(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := os.MkdirAll(filterDir, 0o755); err != nil {
		t.Fatal(err)
	}

	filterYAML := `name: "docker-all"
version: 1
description: "Docker filter"
match:
  command: "docker"
pipeline:
  - action: "keep_lines"
    pattern: "\\S"
on_error: "passthrough"
`
	if err := os.WriteFile(filepath.Join(filterDir, "docker-all.yaml"), []byte(filterYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", filepath.Join(home, ".config", "snip", "config.toml"))

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := Run([]string{"snip", "check", "--", "docker", "build", "-t", "app", "."})
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(buf.String(), "filter: docker-all") {
		t.Errorf("expected output to contain 'filter: docker-all', got %q", buf.String())
	}
}

func TestCheckWithRequireFlagsMatches(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := os.MkdirAll(filterDir, 0o755); err != nil {
		t.Fatal(err)
	}

	filterYAML := `name: "go-test-json"
version: 1
description: "Go test JSON filter"
match:
  command: "go"
  subcommand: "test"
  require_flags: ["-json"]
pipeline:
  - action: "keep_lines"
    pattern: "\\S"
on_error: "passthrough"
`
	if err := os.WriteFile(filepath.Join(filterDir, "go-test-json.yaml"), []byte(filterYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", filepath.Join(home, ".config", "snip", "config.toml"))

	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := Run([]string{"snip", "check", "--", "go", "test", "-json"})
	_ = w.Close()
	os.Stdout = old
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(buf.String(), "filter: go-test-json") {
		t.Errorf("expected output to contain 'filter: go-test-json', got %q", buf.String())
	}
}

func TestCheckWithRequireFlagsExcluded(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := os.MkdirAll(filterDir, 0o755); err != nil {
		t.Fatal(err)
	}

	filterYAML := `name: "go-test-json"
version: 1
description: "Go test JSON filter"
match:
  command: "go"
  subcommand: "test"
  require_flags: ["-json"]
pipeline:
  - action: "keep_lines"
    pattern: "\\S"
on_error: "passthrough"
`
	if err := os.WriteFile(filepath.Join(filterDir, "go-test-json.yaml"), []byte(filterYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", filepath.Join(home, ".config", "snip", "config.toml"))

	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := Run([]string{"snip", "check", "--", "go", "test", "-v"})
	_ = w.Close()
	os.Stdout = old
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(buf.String(), "no filter: excluded by flags") {
		t.Errorf("expected output to contain 'no filter: excluded by flags', got %q", buf.String())
	}
}

func TestCheckBareCommandNoFilter(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := os.MkdirAll(filterDir, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", filepath.Join(home, ".config", "snip", "config.toml"))

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := Run([]string{"snip", "check", "--", "git"})
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(buf.String(), "no filter") {
		t.Errorf("expected output to contain 'no filter', got %q", buf.String())
	}
}

func TestCheckUnproxyableEval(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	code := Run([]string{"snip", "check", "--", "eval", "echo", "hi"})
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(buf.String(), "eval") {
		t.Errorf("expected stderr to contain 'eval', got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "shell builtin") {
		t.Errorf("expected stderr to contain 'shell builtin', got %q", buf.String())
	}
}

func TestCheckUnproxyableWait(t *testing.T) {
	code := Run([]string{"snip", "check", "--", "wait"})
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
}

func TestRunUnproxyableExec(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	code := Run([]string{"snip", "run", "--", "exec", "ls"})
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(buf.String(), "exec") {
		t.Errorf("expected stderr to contain 'exec', got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "cannot be proxied") {
		t.Errorf("expected stderr to contain 'cannot be proxied', got %q", buf.String())
	}
}

func TestCheckFilterExplicitlyEnabled(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := os.MkdirAll(filterDir, 0o755); err != nil {
		t.Fatal(err)
	}

	filterYAML := `name: "git-log"
version: 1
description: "Test filter"
match:
  command: "git"
  subcommand: "log"
pipeline:
  - action: "keep_lines"
    pattern: "\\S"
on_error: "passthrough"
`
	if err := os.WriteFile(filepath.Join(filterDir, "git-log.yaml"), []byte(filterYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	configContent := `[filters]
[filters.enable]
git-log = true
`
	if err := os.WriteFile(filepath.Join(home, ".config", "snip", "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", filepath.Join(home, ".config", "snip", "config.toml"))

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := Run([]string{"snip", "check", "--", "git", "log"})
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(buf.String(), "filter: git-log") {
		t.Errorf("expected output to contain 'filter: git-log', got %q", buf.String())
	}
}

func TestCheckFilterDisabled(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := os.MkdirAll(filterDir, 0o755); err != nil {
		t.Fatal(err)
	}

	filterYAML := `name: "git-log"
version: 1
description: "Test filter"
match:
  command: "git"
  subcommand: "log"
  exclude_flags: ["--format", "--pretty", "--graph", "--oneline"]
inject:
  args: ["--pretty=format:%h %s", "--no-merges"]
  defaults:
    "-n": "10"
  skip_if_present: ["--merges", "--format", "--pretty", "--oneline"]
pipeline:
  - action: "keep_lines"
    pattern: "\\S"
on_error: "passthrough"
`
	if err := os.WriteFile(filepath.Join(filterDir, "git-log.yaml"), []byte(filterYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	configContent := `[filters]
[filters.enable]
git-log = false
`
	if err := os.WriteFile(filepath.Join(home, ".config", "snip", "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", filepath.Join(home, ".config", "snip", "config.toml"))

	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := Run([]string{"snip", "check", "--", "git", "log"})
	_ = w.Close()
	os.Stdout = old
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(buf.String(), "filter disabled: git-log") {
		t.Errorf("expected output to contain 'filter disabled: git-log', got %q", buf.String())
	}
}

func TestCheckAndRunUnproxyableFormatConsistent(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantSub string
	}{
		{"check cd", []string{"snip", "check", "--", "cd", "/tmp"}, "is a shell builtin"},
		{"run cd", []string{"snip", "run", "--", "cd", "/tmp"}, "cannot be proxied"},
		{"global cd", []string{"snip", "cd", "/tmp"}, "cannot be proxied"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w
			code := Run(tt.args)
			_ = w.Close()
			os.Stderr = old
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

			if code != 1 {
				t.Errorf("expected exit code 1, got %d", code)
			}
			output := buf.String()
			if !strings.Contains(output, tt.wantSub) {
				t.Errorf("expected stderr to contain %q, got %q", tt.wantSub, output)
			}
		})
	}
}

func TestRunSubcommandRejectsArgsBeforeSeparatorErrorMessage(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	code := Run([]string{"snip", "run", "-v", "--", "git", "log"})
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	output := buf.String()
	if !strings.Contains(output, "unexpected arguments before --") {
		t.Errorf("expected stderr to contain 'unexpected arguments before --', got %q", output)
	}
}

func TestCheckSubcommandRejectsArgsBeforeSeparatorErrorMessage(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	code := Run([]string{"snip", "check", "-v", "--", "git", "log"})
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	output := buf.String()
	if !strings.Contains(output, "unexpected arguments before --") {
		t.Errorf("expected stderr to contain 'unexpected arguments before --', got %q", output)
	}
}

func TestRunSubcommandNoArgs(t *testing.T) {
	code := Run([]string{"snip", "run"})
	if code != 1 {
		t.Errorf("Run(run with no args) = %d, want 1", code)
	}
}

func TestCheckSubcommandNoArgs(t *testing.T) {
	code := Run([]string{"snip", "check"})
	if code != 1 {
		t.Errorf("Run(check with no args) = %d, want 1", code)
	}
}

func TestParseFlagsHelpNotConsumedAfterSeparator(t *testing.T) {
	flags, remaining := ParseFlags([]string{"run", "--", "git", "--help"})
	if flags.Help {
		t.Errorf("flags.Help = true, want false (--help after -- should be a command arg)")
	}
	wantRemaining := []string{"run", "--", "git", "--help"}
	if !reflect.DeepEqual(remaining, wantRemaining) {
		t.Errorf("remaining = %v, want %v", remaining, wantRemaining)
	}
}

func TestParseFlagsVersionNotConsumedAfterSeparator(t *testing.T) {
	flags, remaining := ParseFlags([]string{"run", "--", "git", "--version"})
	if flags.Version {
		t.Errorf("flags.Version = true, want false (--version after -- should be a command arg)")
	}
	wantRemaining := []string{"run", "--", "git", "--version"}
	if !reflect.DeepEqual(remaining, wantRemaining) {
		t.Errorf("remaining = %v, want %v", remaining, wantRemaining)
	}
}

func TestParseFlagsHelpConsumedBeforeSeparatorForRun(t *testing.T) {
	flags, remaining := ParseFlags([]string{"run", "--help", "--", "git", "log"})
	if !flags.Help {
		t.Errorf("flags.Help = false, want true (--help before -- should be snip flag)")
	}
	wantRemaining := []string{"run", "git", "log"}
	if !reflect.DeepEqual(remaining, wantRemaining) {
		t.Errorf("remaining = %v, want %v", remaining, wantRemaining)
	}
}

func TestParseFlagsVersionConsumedBeforeSeparatorForRun(t *testing.T) {
	flags, remaining := ParseFlags([]string{"run", "--version", "--", "git", "log"})
	if !flags.Version {
		t.Errorf("flags.Version = false, want true (--version before -- should be snip flag)")
	}
	wantRemaining := []string{"run", "git", "log"}
	if !reflect.DeepEqual(remaining, wantRemaining) {
		t.Errorf("remaining = %v, want %v", remaining, wantRemaining)
	}
}

func TestParseFlagsHelpConsumedBeforeSeparatorForCheck(t *testing.T) {
	flags, remaining := ParseFlags([]string{"check", "--help", "--", "git", "log"})
	if !flags.Help {
		t.Errorf("flags.Help = false, want true")
	}
	wantRemaining := []string{"check", "git", "log"}
	if !reflect.DeepEqual(remaining, wantRemaining) {
		t.Errorf("remaining = %v, want %v", remaining, wantRemaining)
	}
}

func TestParseFlagsHelpNotConsumedAfterSeparatorForCheck(t *testing.T) {
	flags, remaining := ParseFlags([]string{"check", "--", "git", "--help"})
	if flags.Help {
		t.Errorf("flags.Help = true, want false (--help after -- should be a command arg)")
	}
	wantRemaining := []string{"check", "--", "git", "--help"}
	if !reflect.DeepEqual(remaining, wantRemaining) {
		t.Errorf("remaining = %v, want %v", remaining, wantRemaining)
	}
}

func TestParseSeparatorArgsFindsFirstSeparator(t *testing.T) {
	cmd, args, errMsg := parseSeparatorArgs([]string{"--", "git", "--", "log"}, "run")
	if errMsg != "" {
		t.Errorf("unexpected error: %q", errMsg)
	}
	if cmd != "git" {
		t.Errorf("cmd = %q, want %q", cmd, "git")
	}
	if !reflect.DeepEqual(args, []string{"--", "log"}) {
		t.Errorf("args = %v, want [--, log]", args)
	}
}

func TestRunSubcommandPreservesDoubleDash(t *testing.T) {
	flags, remaining := ParseFlags([]string{"run", "--", "git", "--", "log"})
	if flags.Help {
		t.Errorf("flags.Help = true, want false")
	}
	wantRemaining := []string{"run", "--", "git", "--", "log"}
	if !reflect.DeepEqual(remaining, wantRemaining) {
		t.Errorf("remaining = %v, want %v", remaining, wantRemaining)
	}
}

func TestCheckSubcommandPreservesDoubleDash(t *testing.T) {
	flags, remaining := ParseFlags([]string{"check", "--", "git", "--", "log"})
	if flags.Help {
		t.Errorf("flags.Help = true, want false")
	}
	wantRemaining := []string{"check", "--", "git", "--", "log"}
	if !reflect.DeepEqual(remaining, wantRemaining) {
		t.Errorf("remaining = %v, want %v", remaining, wantRemaining)
	}
}

func TestRunSubcommandUnproxyableErrorMessageFormat(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{"source", []string{"snip", "run", "--", "source"}, "cannot be proxied"},
		{"dot", []string{"snip", "run", "--", "."}, "cannot be proxied"},
		{"export", []string{"snip", "run", "--", "export"}, "cannot be proxied"},
		{"eval", []string{"snip", "run", "--", "eval"}, "cannot be proxied"},
		{"set", []string{"snip", "run", "--", "set"}, "cannot be proxied"},
		{"exit", []string{"snip", "run", "--", "exit"}, "cannot be proxied"},
		{"exec", []string{"snip", "run", "--", "exec"}, "cannot be proxied"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w
			code := Run(tt.args)
			_ = w.Close()
			os.Stderr = old
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

			if code != 1 {
				t.Errorf("expected exit code 1, got %d", code)
			}
			output := buf.String()
			if !strings.Contains(output, tt.wantMsg) {
				t.Errorf("expected stderr to contain %q, got %q", tt.wantMsg, output)
			}
		})
	}
}

func TestCheckSubcommandUnproxyableErrorMessageFormat(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{"source", []string{"snip", "check", "--", "source"}, "is a shell builtin"},
		{"dot", []string{"snip", "check", "--", "."}, "is a shell builtin"},
		{"export", []string{"snip", "check", "--", "export"}, "is a shell builtin"},
		{"eval", []string{"snip", "check", "--", "eval"}, "is a shell builtin"},
		{"set", []string{"snip", "check", "--", "set"}, "is a shell builtin"},
		{"exit", []string{"snip", "check", "--", "exit"}, "is a shell builtin"},
		{"exec", []string{"snip", "check", "--", "exec"}, "is a shell builtin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w
			code := Run(tt.args)
			_ = w.Close()
			os.Stderr = old
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

			if code != 1 {
				t.Errorf("expected exit code 1, got %d", code)
			}
			output := buf.String()
			if !strings.Contains(output, tt.wantMsg) {
				t.Errorf("expected stderr to contain %q, got %q", tt.wantMsg, output)
			}
		})
	}
}

func TestGlobalUnproxyableErrorMessageFormat(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{"cd", []string{"snip", "cd"}, "cannot be proxied"},
		{"source", []string{"snip", "source"}, "cannot be proxied"},
		{"eval", []string{"snip", "eval"}, "cannot be proxied"},
		{"exec", []string{"snip", "exec"}, "cannot be proxied"},
		{"set", []string{"snip", "set"}, "cannot be proxied"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w
			code := Run(tt.args)
			_ = w.Close()
			os.Stderr = old
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

			if code != 1 {
				t.Errorf("expected exit code 1, got %d", code)
			}
			output := buf.String()
			if !strings.Contains(output, tt.wantMsg) {
				t.Errorf("expected stderr to contain %q, got %q", tt.wantMsg, output)
			}
		})
	}
}



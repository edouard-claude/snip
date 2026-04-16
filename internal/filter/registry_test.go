package filter

import (
	"testing"
)

func makeFilter(name, cmd, subcmd string) Filter {
	return Filter{
		Name:    name,
		Version: 1,
		Match:   Match{Command: cmd, Subcommand: subcmd},
		OnError: "passthrough",
	}
}

func TestRegistryMatch(t *testing.T) {
	filters := []Filter{
		makeFilter("git-log", "git", "log"),
		makeFilter("git-status", "git", "status"),
		makeFilter("go-test", "go", "test"),
	}
	reg := NewRegistry(filters)

	tests := []struct {
		cmd     string
		subcmd  string
		args    []string
		want    string
		wantNil bool
	}{
		{"git", "log", nil, "git-log", false},
		{"git", "status", nil, "git-status", false},
		{"go", "test", nil, "go-test", false},
		{"git", "push", nil, "", true},
		{"npm", "install", nil, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.cmd+" "+tt.subcmd, func(t *testing.T) {
			f := reg.Match(tt.cmd, tt.subcmd, tt.args)
			if tt.wantNil {
				if f != nil {
					t.Errorf("expected nil, got %q", f.Name)
				}
				return
			}
			if f == nil {
				t.Fatal("expected match, got nil")
			}
			if f.Name != tt.want {
				t.Errorf("got %q, want %q", f.Name, tt.want)
			}
		})
	}
}

func TestRegistryMatchExcludeFlags(t *testing.T) {
	f := Filter{
		Name:    "git-log",
		Version: 1,
		Match:   Match{Command: "git", Subcommand: "log", ExcludeFlags: []string{"--format", "--pretty"}},
		OnError: "passthrough",
	}
	reg := NewRegistry([]Filter{f})

	// Should match without excluded flags
	if reg.Match("git", "log", []string{"-10"}) == nil {
		t.Error("expected match without excluded flags")
	}

	// Should NOT match with excluded flag
	if reg.Match("git", "log", []string{"--format=oneline"}) != nil {
		t.Error("expected no match with --format flag")
	}
	if reg.Match("git", "log", []string{"--pretty=short"}) != nil {
		t.Error("expected no match with --pretty flag")
	}
}

func TestRegistryMatchRequireFlags(t *testing.T) {
	f := Filter{
		Name:    "special",
		Version: 1,
		Match:   Match{Command: "cmd", RequireFlags: []string{"--json"}},
		OnError: "passthrough",
	}
	reg := NewRegistry([]Filter{f})

	if reg.Match("cmd", "", []string{"--json"}) == nil {
		t.Error("expected match with required flag")
	}
	if reg.Match("cmd", "", []string{"--text"}) != nil {
		t.Error("expected no match without required flag")
	}
}

func TestRegistryCommands(t *testing.T) {
	filters := []Filter{
		makeFilter("git-log", "git", "log"),
		makeFilter("git-status", "git", "status"),
		makeFilter("go-test", "go", "test"),
		makeFilter("npm-install", "npm", ""),
		makeFilter("docker-build", "docker", "build"),
	}
	reg := NewRegistry(filters)

	got := reg.Commands()
	want := []string{"docker", "git", "go", "npm"}

	if len(got) != len(want) {
		t.Fatalf("Commands() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Commands()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRegistryCommandsEmpty(t *testing.T) {
	reg := NewRegistry(nil)
	got := reg.Commands()
	if len(got) != 0 {
		t.Errorf("Commands() on empty registry = %v, want empty", got)
	}
}

func TestHasAnyFilter(t *testing.T) {
	f := Filter{
		Name:    "go-test",
		Version: 1,
		Match:   Match{Command: "go", Subcommand: "test", ExcludeFlags: []string{"-v"}},
		OnError: "passthrough",
	}
	reg := NewRegistry([]Filter{f})

	// Key scenario: Match returns nil due to -v, but HasAnyFilter must still return true.
	// This is what suppresses the misleading "no filter for go" message.
	if reg.Match("go", "test", []string{"-v"}) != nil {
		t.Error("Match should return nil when -v is excluded")
	}
	if !reg.HasAnyFilter("go", "test") {
		t.Error("HasAnyFilter should return true even when flags exclude the filter")
	}

	// Known command, unknown subcommand — no command-only entry
	if reg.HasAnyFilter("go", "build") {
		t.Error("expected HasAnyFilter=false for go build (no filter)")
	}
	// Completely unknown command
	if reg.HasAnyFilter("python", "") {
		t.Error("expected HasAnyFilter=false for python")
	}
}

func TestShouldInject(t *testing.T) {
	f := Filter{
		Name: "git-log",
		Inject: &Inject{
			Args:          []string{"--oneline"},
			Defaults:      map[string]string{"-n": "10"},
			SkipIfPresent: []string{"--format"},
		},
	}
	reg := NewRegistry(nil)

	// Normal injection
	args, injected := reg.ShouldInject(&f, []string{"log"})
	if !injected {
		t.Fatal("expected injection")
	}
	hasOneline := false
	hasN := false
	for _, a := range args {
		if a == "--oneline" {
			hasOneline = true
		}
		if a == "-n" {
			hasN = true
		}
	}
	if !hasOneline {
		t.Error("missing --oneline")
	}
	if !hasN {
		t.Error("missing -n default")
	}

	// Skip injection when --format present
	args2, injected2 := reg.ShouldInject(&f, []string{"log", "--format=short"})
	if injected2 {
		t.Error("expected skip injection with --format")
	}
	if len(args2) != 2 {
		t.Errorf("args modified: %v", args2)
	}
}

func TestShouldInjectNoInject(t *testing.T) {
	f := Filter{Name: "test"}
	reg := NewRegistry(nil)
	args, injected := reg.ShouldInject(&f, []string{"test"})
	if injected {
		t.Error("expected no injection")
	}
	if len(args) != 1 {
		t.Errorf("args modified: %v", args)
	}
}

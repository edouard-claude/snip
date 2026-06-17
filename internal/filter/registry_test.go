package filter

import (
	"os"
	"testing"
)

// TestDockerBuildScopedToBuild guards against the docker-build filter matching
// unrelated docker subcommands (e.g. docker run/exec), which would strip their
// output to nothing. See issue #85. Parses the real shipped filter file.
func TestDockerBuildScopedToBuild(t *testing.T) {
	data, err := os.ReadFile("../../filters/docker-build.yaml")
	if err != nil {
		t.Fatalf("read docker-build.yaml: %v", err)
	}
	f, err := ParseFilter(data)
	if err != nil {
		t.Fatalf("parse docker-build.yaml: %v", err)
	}
	reg := NewRegistry([]Filter{*f})

	if got := reg.Match("docker", "build", []string{"-t", "app", "."}); got == nil || got.Name != "docker-build" {
		t.Errorf("docker build should match docker-build, got %v", got)
	}

	for _, sub := range []string{"run", "exec", "cp", "inspect", "logs"} {
		if got := reg.Match("docker", sub, []string{"--rm", "ubuntu", "ls"}); got != nil {
			t.Errorf("docker %s must not match any filter, got %q", sub, got.Name)
		}
	}
}

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
		{"./go", "test", nil, "go-test", false},
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

func TestHasAnyFilterCommandOnly(t *testing.T) {
	// Command-only filter: no Subcommand field — should match any subcommand.
	f := Filter{
		Name:    "npm-filter",
		Version: 1,
		Match:   Match{Command: "npm"},
		OnError: "passthrough",
	}
	reg := NewRegistry([]Filter{f})

	if !reg.HasAnyFilter("npm", "install") {
		t.Error("HasAnyFilter should return true for npm+install via command-only filter")
	}
	if !reg.HasAnyFilter("npm", "run") {
		t.Error("HasAnyFilter should return true for npm+run via command-only filter")
	}
	if !reg.HasAnyFilter("npm", "") {
		t.Error("HasAnyFilter should return true for npm (no subcommand) via command-only filter")
	}
	if reg.HasAnyFilter("yarn", "install") {
		t.Error("HasAnyFilter should return false for yarn (no filter registered)")
	}
}

func TestHasAnyFilterForCommand(t *testing.T) {
	// Reproduces issue #56: snip ships filters for git-add, git-commit, etc.
	// but not for git checkout. HasAnyFilterForCommand("git") must return true
	// so that running "git checkout" doesn't print the misleading
	// "no filter for git" hint.
	filters := []Filter{
		{Name: "git-add", Version: 1, Match: Match{Command: "git", Subcommand: "add"}, OnError: "passthrough"},
		{Name: "git-commit", Version: 1, Match: Match{Command: "git", Subcommand: "commit"}, OnError: "passthrough"},
	}
	reg := NewRegistry(filters)

	if !reg.HasAnyFilterForCommand("git") {
		t.Error("HasAnyFilterForCommand should return true for git (subcommand filters registered)")
	}
	if reg.HasAnyFilterForCommand("python") {
		t.Error("HasAnyFilterForCommand should return false for python (no filter registered)")
	}

	// Command-only filter must also be recognized.
	regCmdOnly := NewRegistry([]Filter{
		{Name: "npm", Version: 1, Match: Match{Command: "npm"}, OnError: "passthrough"},
	})
	if !regCmdOnly.HasAnyFilterForCommand("npm") {
		t.Error("HasAnyFilterForCommand should return true for npm (command-only filter)")
	}
}

func TestRegistryDotSlashWrapper(t *testing.T) {
	// Wrapper scripts invoked from the project root (./gradlew, ./mvnw) must
	// match filters keyed on the bare name.
	filters := []Filter{
		{Name: "gradlew", Version: 1, Match: Match{Command: "gradlew"}, OnError: "passthrough"},
	}
	reg := NewRegistry(filters)

	if f := reg.Match("./gradlew", "", nil); f == nil || f.Name != "gradlew" {
		t.Errorf("Match(./gradlew) should resolve to gradlew filter, got %v", f)
	}
	if !reg.HasAnyFilter("./gradlew", "") {
		t.Error("HasAnyFilter(./gradlew) should return true")
	}
	if !reg.HasAnyFilterForCommand("./gradlew") {
		t.Error("HasAnyFilterForCommand(./gradlew) should return true")
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

// TestShouldInjectUserFlagWinsOverDefault is the regression test for #79:
// when the user already provided a flag that has a default, the default
// must not be injected (otherwise git's last-flag-wins picks the wrong one).
func TestShouldInjectUserFlagWinsOverDefault(t *testing.T) {
	f := Filter{
		Name: "git-log",
		Inject: &Inject{
			Args:     []string{"--no-merges"},
			Defaults: map[string]string{"-n": "10"},
		},
	}
	reg := NewRegistry(nil)

	args, injected := reg.ShouldInject(&f, []string{"log", "-n", "50"})
	if !injected {
		t.Fatal("expected injection")
	}
	// Count occurrences of -n. Must be exactly one (the user's).
	nCount := 0
	for _, a := range args {
		if a == "-n" {
			nCount++
		}
	}
	if nCount != 1 {
		t.Errorf("-n appears %d times, want 1: %v", nCount, args)
	}
	// The user's "50" must survive somewhere after the user's -n.
	hasFifty := false
	for _, a := range args {
		if a == "50" {
			hasFifty = true
		}
	}
	if !hasFifty {
		t.Errorf("user value 50 was dropped: %v", args)
	}
}

// TestShouldInjectDefaultsPrecedeUserArgs ensures defaults end up between
// inject.args and user args (not appended at the end), so user flags
// always have the final word under last-flag-wins.
func TestShouldInjectDefaultsPrecedeUserArgs(t *testing.T) {
	f := Filter{
		Name: "git-log",
		Inject: &Inject{
			Args:     []string{"--no-merges"},
			Defaults: map[string]string{"-n": "10"},
		},
	}
	reg := NewRegistry(nil)

	args, injected := reg.ShouldInject(&f, []string{"log", "HEAD"})
	if !injected {
		t.Fatal("expected injection")
	}
	// Find positions
	nIdx, headIdx := -1, -1
	for i, a := range args {
		if a == "-n" {
			nIdx = i
		}
		if a == "HEAD" {
			headIdx = i
		}
	}
	if nIdx < 0 || headIdx < 0 {
		t.Fatalf("missing tokens in %v", args)
	}
	if nIdx > headIdx {
		t.Errorf("default -n (idx %d) must precede user HEAD (idx %d): %v", nIdx, headIdx, args)
	}
}

// TestMatchPathPrefixNormalization covers the filepath.Base normalization
// for invocation styles like ./gradlew, /usr/bin/git, and bare names.
func TestMatchPathPrefixNormalization(t *testing.T) {
	filters := []Filter{
		makeFilter("git-log", "git", "log"),
		makeFilter("gradlew", "gradlew", ""),
	}
	reg := NewRegistry(filters)

	cases := []struct {
		name    string
		cmd     string
		sub     string
		args    []string
		wantHit bool
	}{
		{"bare git", "git", "log", []string{"log"}, true},
		{"relative ./gradlew", "./gradlew", "", []string{}, true},
		{"absolute /usr/bin/git", "/usr/bin/git", "log", []string{"log"}, true},
		{"empty command", "", "log", []string{"log"}, false},
		{"root slash", "/", "log", []string{"log"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := reg.Match(tc.cmd, tc.sub, tc.args)
			if tc.wantHit && got == nil {
				t.Errorf("expected match for %q", tc.cmd)
			}
			if !tc.wantHit && got != nil {
				t.Errorf("unexpected match %q for %q", got.Name, tc.cmd)
			}
		})
	}
}

func TestHasAnyFilterPathPrefixNormalization(t *testing.T) {
	filters := []Filter{makeFilter("git-log", "git", "log")}
	reg := NewRegistry(filters)

	if !reg.HasAnyFilter("/usr/bin/git", "log") {
		t.Error("HasAnyFilter should normalize absolute path to bare name")
	}
	if !reg.HasAnyFilterForCommand("./git") {
		t.Error("HasAnyFilterForCommand should strip ./ prefix")
	}
	if reg.HasAnyFilter("", "log") {
		t.Error("empty command must not match anything")
	}
}

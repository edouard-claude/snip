package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/edouard-claude/snip/internal/filter"
)

// scriptTestPipeline knows the JS tools that repos front with npm scripts,
// plus an npm subcommand filter so npm itself is a known command, as with the
// shipped npm-install filter. The jest filter injects a flag, like the shipped
// git-status or pytest filters do.
func scriptTestPipeline() (*Pipeline, *[]string) {
	filters := []filter.Filter{
		{Name: "npm-install", Match: filter.Match{Command: "npm", Subcommand: filter.NewSubcommand("install")}},
		{
			Name:  "vitest",
			Match: filter.Match{Command: "vitest"},
			Pipeline: filter.Pipeline{
				{ActionName: "keep_lines", Params: map[string]any{"pattern": `^vitest:`}},
			},
		},
		{Name: "tsc", Match: filter.Match{Command: "tsc"}},
		{Name: "playwright", Match: filter.Match{Command: "playwright"}},
		{
			Name:   "jest",
			Match:  filter.Match{Command: "jest"},
			Inject: &filter.Inject{Args: []string{"--json"}},
			Pipeline: filter.Pipeline{
				{ActionName: "keep_lines", Params: map[string]any{"pattern": `^\{`}},
			},
		},
	}
	var executed []string
	p := &Pipeline{
		Registry: filter.NewRegistry(filters),
		execute: func(command string, args []string) (*Result, error) {
			executed = append([]string{command}, args...)
			return &Result{Stdout: "npm notice something\nvitest: 3 passed\n"}, nil
		},
	}
	return p, &executed
}

// chdirWithPackageJSON moves the test into a temp dir holding the given
// package.json (none when content is empty).
func chdirWithPackageJSON(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	if content != "" {
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
}

const testPackageJSON = `{
  "name": "app",
  "scripts": {
    "test": "vitest run",
    "typecheck": "tsc --noEmit",
    "e2e": "NODE_ENV=test node_modules/.bin/playwright test --project=chromium",
    "lint": "eslint . && prettier --check .",
    "dyn": "$(cat tool) run",
    "serve": "node server.js",
    "chain": "npm run test",
    "unit": "jest --ci"
  }
}`

// npm test and npm run <script> resolve to the command the package.json script
// runs, so that command's filter can apply (issue #200).
func TestUnwrapScript(t *testing.T) {
	chdirWithPackageJSON(t, testPackageJSON)
	p, _ := scriptTestPipeline()

	cases := []struct {
		name          string
		command       string
		args          []string
		wantOK        bool
		wantInner     string
		wantInnerArgs []string
	}{
		{
			name: "npm test runs the test script", command: "npm", args: []string{"test"},
			wantOK: true, wantInner: "vitest", wantInnerArgs: []string{"run"},
		},
		{
			name: "npm t is an alias of npm test", command: "npm", args: []string{"t"},
			wantOK: true, wantInner: "vitest", wantInnerArgs: []string{"run"},
		},
		{
			name: "npm run <script>", command: "npm", args: []string{"run", "typecheck"},
			wantOK: true, wantInner: "tsc", wantInnerArgs: []string{"--noEmit"},
		},
		{
			name: "npm run-script is an alias of npm run", command: "npm", args: []string{"run-script", "typecheck"},
			wantOK: true, wantInner: "tsc", wantInnerArgs: []string{"--noEmit"},
		},
		{
			name: "arguments after -- reach the inner command", command: "npm", args: []string{"test", "--", "--coverage"},
			wantOK: true, wantInner: "vitest", wantInnerArgs: []string{"run", "--coverage"},
		},
		{
			name: "env assignments are skipped and a path-qualified tool is kept", command: "npm", args: []string{"run", "e2e"},
			wantOK: true, wantInner: "node_modules/.bin/playwright", wantInnerArgs: []string{"test", "--project=chromium"},
		},
		{
			name: "path-qualified npm matches by base name", command: "/usr/local/bin/npm", args: []string{"test"},
			wantOK: true, wantInner: "vitest", wantInnerArgs: []string{"run"},
		},
		{
			// npm is a known command, so the script resolves to it; the match
			// then fails on `npm run`, and scripts are never followed further.
			name: "a script delegating to npm is not followed", command: "npm", args: []string{"run", "chain"},
			wantOK: true, wantInner: "npm", wantInnerArgs: []string{"run", "test"},
		},
		{
			// Two tools' outputs interleave: one filter would mangle the other's lines.
			name: "compound script fails closed", command: "npm", args: []string{"run", "lint"},
			wantOK: false,
		},
		{
			name: "command substitution fails closed", command: "npm", args: []string{"run", "dyn"},
			wantOK: false,
		},
		{
			name: "script running an unknown tool fails closed", command: "npm", args: []string{"run", "serve"},
			wantOK: false,
		},
		{
			name: "missing script fails closed", command: "npm", args: []string{"run", "nope"},
			wantOK: false,
		},
		{
			name: "npm run without a script name lists scripts", command: "npm", args: []string{"run"},
			wantOK: false,
		},
		{
			// --workspace and --prefix change which package.json applies.
			name: "runner flag before the script name fails closed", command: "npm", args: []string{"run", "--workspace=a", "test"},
			wantOK: false,
		},
		{
			name: "runner flag after the script name fails closed", command: "npm", args: []string{"test", "--silent"},
			wantOK: false,
		},
		{
			name: "other npm subcommands are not scripts", command: "npm", args: []string{"install"},
			wantOK: false,
		},
		{
			name: "npm alone", command: "npm", args: nil,
			wantOK: false,
		},
		{
			name: "not a script runner", command: "make", args: []string{"test"},
			wantOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inner, innerArgs, ok := p.unwrapScript(tc.command, tc.args)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (inner=%q innerArgs=%q)", ok, tc.wantOK, inner, innerArgs)
			}
			if !ok {
				return
			}
			if inner != tc.wantInner {
				t.Errorf("inner = %q, want %q", inner, tc.wantInner)
			}
			if !sliceEq(innerArgs, tc.wantInnerArgs) {
				t.Errorf("innerArgs = %q, want %q", innerArgs, tc.wantInnerArgs)
			}
		})
	}
}

func TestUnwrapScriptWithoutPackageJSON(t *testing.T) {
	p, _ := scriptTestPipeline()

	chdirWithPackageJSON(t, "")
	if _, _, ok := p.unwrapScript("npm", []string{"test"}); ok {
		t.Error("resolved a script with no package.json in the working directory")
	}

	chdirWithPackageJSON(t, `{"scripts": `)
	if _, _, ok := p.unwrapScript("npm", []string{"test"}); ok {
		t.Error("resolved a script from an invalid package.json")
	}
}

// The script's command gets its filter, while npm runs exactly as typed.
func TestPipelineRunFiltersNpmScriptByItsCommand(t *testing.T) {
	chdirWithPackageJSON(t, testPackageJSON)
	p, executed := scriptTestPipeline()

	out := captureStdout(t, func() { p.Run("npm", []string{"test"}) })

	if want := []string{"npm", "test"}; !sliceEq(*executed, want) {
		t.Fatalf("executed %q, want %q (nil means npm test passed through unfiltered)", *executed, want)
	}
	if out != "vitest: 3 passed\n" {
		t.Errorf("output = %q, want the vitest filter applied", out)
	}
}

// The inner command lives in package.json, so snip cannot inject into it: a
// filter whose pipeline expects its injected flags is skipped, and the script
// passes through rather than receive output in the wrong shape.
func TestPipelineRunNpmScriptSkipsInjectingFilter(t *testing.T) {
	chdirWithPackageJSON(t, testPackageJSON)
	p, executed := scriptTestPipeline()
	p.QuietNoFilter = true
	// An empty PATH makes the passthrough of the real npm fail fast with exit 1
	// instead of running the fixture script.
	t.Setenv("PATH", "")

	code := p.Run("npm", []string{"run", "unit"})

	if *executed != nil {
		t.Fatalf("executed %q through the filtered path, want a plain passthrough", *executed)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1 from the failed passthrough with an empty PATH", code)
	}
}

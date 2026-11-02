package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/edouard-claude/snip/internal/hook"
)

// scriptRunner describes how a package manager runs the scripts declared in
// package.json: the subcommands that take the script name as their next
// argument, and the lifecycle shortcuts that name a script directly.
type scriptRunner struct {
	run      map[string]bool
	shortcut map[string]string
}

// scriptRunners lists the package managers whose `run <script>` and `test`
// invocations resolve to the script's command (issue #200). Repos front
// vitest, tsc or playwright with `npm test` and `npm run typecheck`, and a
// filter keyed on `npm run` would have to fit every script.
var scriptRunners = map[string]scriptRunner{
	"npm": {
		run:      map[string]bool{"run": true, "run-script": true, "rum": true, "urn": true},
		shortcut: map[string]string{"test": "test", "t": "test", "tst": "test"},
	},
}

// unwrapScript resolves `npm test` and `npm run <script>` to the command the
// package.json script runs, so that command's filter applies to the output.
// It returns the script's command and arguments (plus anything the caller
// passed after `--`, which npm forwards to the script) for matching only: the
// runner executes with its original arguments, and nothing is injected into a
// script snip does not own.
//
// It fails closed (ok=false) when the runner is unknown, a runner flag
// surrounds the script name (--workspace and --prefix change which
// package.json applies), package.json or the script is missing, the script
// is more than one plain command (an env assignment prefix is fine; `&&`,
// `;`, `|` or command substitution are not, since the output then mixes
// tools), or the script's command is not a known snip command.
func (p *Pipeline) unwrapScript(command string, args []string) (inner string, innerArgs []string, ok bool) {
	runner, known := scriptRunners[filepath.Base(command)]
	if !known || len(args) == 0 {
		return "", nil, false
	}

	var script string
	var forwarded []string
	switch sub := args[0]; {
	case runner.run[sub]:
		if len(args) < 2 || strings.HasPrefix(args[1], "-") {
			return "", nil, false
		}
		script, forwarded = args[1], args[2:]
	case runner.shortcut[sub] != "":
		script, forwarded = runner.shortcut[sub], args[1:]
	default:
		return "", nil, false
	}

	// Only what follows "--" reaches the script; anything before it is a
	// runner option, and a runner option is where the fail-closed rule applies.
	var extra []string
	if len(forwarded) > 0 {
		if forwarded[0] != "--" {
			return "", nil, false
		}
		extra = forwarded[1:]
	}

	body, found := lookupScript(script)
	if !found || hook.HasUnverifiableConstruct(body) || hook.ExtractFirstSegment(body) != body {
		return "", nil, false
	}
	_, _, bare := hook.ParseSegment(body)
	// Fields is enough here: the tokens only feed the filter match (flag
	// prefixes), never an exec, so a quoted argument split in two is harmless.
	toks := strings.Fields(bare)
	if len(toks) == 0 || !p.Registry.HasAnyFilterForCommand(toks[0]) {
		return "", nil, false
	}
	innerArgs = make([]string, 0, len(toks)-1+len(extra))
	innerArgs = append(innerArgs, toks[1:]...)
	innerArgs = append(innerArgs, extra...)
	return toks[0], innerArgs, true
}

// lookupScript reads the named script from package.json in the working
// directory, which is where npm resolves it from.
func lookupScript(name string) (body string, ok bool) {
	data, err := os.ReadFile("package.json")
	if err != nil {
		return "", false
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", false
	}
	body, ok = pkg.Scripts[name]
	return body, ok
}

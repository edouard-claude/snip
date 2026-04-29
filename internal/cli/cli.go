package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"path/filepath"

	"github.com/edouard-claude/snip/internal/config"
	"github.com/edouard-claude/snip/internal/discover"
	"github.com/edouard-claude/snip/internal/display"
	"github.com/edouard-claude/snip/internal/economics"
	"github.com/edouard-claude/snip/internal/engine"
	"github.com/edouard-claude/snip/internal/filter"
	"github.com/edouard-claude/snip/internal/hook"
	"github.com/edouard-claude/snip/internal/hookaudit"
	"github.com/edouard-claude/snip/internal/initcmd"
	"github.com/edouard-claude/snip/internal/learn"
	"github.com/edouard-claude/snip/internal/tee"
	"github.com/edouard-claude/snip/internal/tracking"
	"github.com/edouard-claude/snip/internal/trust"
	"github.com/edouard-claude/snip/internal/verify"
)

// version is set at build time via -ldflags "-X ...". Do not reassign.
var version = "dev"

// Run is the main entry point. Returns exit code.
func Run(args []string) int {
	if len(args) < 2 {
		printUsage()
		return 0
	}

	flags, remaining := ParseFlags(args[1:])

	if flags.Version {
		fmt.Printf("snip v%s\n", version)
		return 0
	}
	if flags.Help || len(remaining) == 0 {
		printUsage()
		return 0
	}

	command := remaining[0]
	cmdArgs := remaining[1:]

	// Commands that cannot be proxied: they must run in the parent shell
	// to have any effect. Running them in a subprocess is a silent no-op.
	if reason := unproxyableReason(command); reason != "" {
		fmt.Fprintf(os.Stderr, "snip: %s cannot be proxied (%s)\n", command, reason)
		return 1
	}

	// Built-in commands
	switch command {
	case "hook":
		return runHook()

	case "hook-audit":
		if err := hookaudit.Run(cmdArgs); err != nil {
			display.PrintError(err.Error())
			return 1
		}
		return 0

	case "init":
		if err := initcmd.Run(cmdArgs); err != nil {
			display.PrintError(err.Error())
			return 1
		}
		return 0

	case "gain":
		if !tracking.DriverAvailable {
			display.PrintError("gain requires full build (this binary was built with -tags lite)")
			return 1
		}
		cfg, cfgErr := config.Load()
		if cfgErr != nil {
			cfg = config.DefaultConfig()
		}
		dbPath := tracking.DBPath(cfg.Tracking.DBPath)
		tracker, err := tracking.NewTracker(dbPath)
		if err != nil {
			display.PrintError(err.Error())
			return 1
		}
		defer func() { _ = tracker.Close() }()
		if err := display.RunGain(tracker, cmdArgs); err != nil {
			display.PrintError(err.Error())
			return 1
		}
		return 0

	case "cc-economics":
		if !tracking.DriverAvailable {
			display.PrintError("cc-economics requires full build (this binary was built with -tags lite)")
			return 1
		}
		cfg, cfgErr := config.Load()
		if cfgErr != nil {
			cfg = config.DefaultConfig()
		}
		dbPath := tracking.DBPath(cfg.Tracking.DBPath)
		tracker, err := tracking.NewTracker(dbPath)
		if err != nil {
			display.PrintError(err.Error())
			return 1
		}
		defer func() { _ = tracker.Close() }()
		if err := economics.Run(tracker, cmdArgs); err != nil {
			display.PrintError(err.Error())
			return 1
		}
		return 0

	case "config":
		cfg, err := config.Load()
		if err != nil {
			display.PrintError(err.Error())
			return 1
		}
		fmt.Printf("tracking.db_path: %s\n", cfg.Tracking.DBPath)
		fmt.Printf("filters.dir: %s\n", strings.Join(cfg.Filters.Dirs(), ", "))
		fmt.Printf("tee.mode: %s\n", cfg.Tee.Mode)
		fmt.Printf("tee.max_files: %d\n", cfg.Tee.MaxFiles)
		fmt.Printf("display.color: %v\n", cfg.Display.Color)
		fmt.Printf("display.emoji: %v\n", cfg.Display.Emoji)
		fmt.Printf("display.quiet_no_filter: %v\n", cfg.Display.QuietNoFilter)
		if len(cfg.Filters.Enable) == 0 {
			fmt.Println("filters.enable: (all enabled)")
		} else {
			names := make([]string, 0, len(cfg.Filters.Enable))
			for k := range cfg.Filters.Enable {
				names = append(names, k)
			}
			sort.Strings(names)
			for _, name := range names {
				fmt.Printf("filters.enable.%s: %v\n", name, cfg.Filters.Enable[name])
			}
		}
		return 0

	case "discover":
		if err := discover.Run(cmdArgs); err != nil {
			display.PrintError(err.Error())
			return 1
		}
		return 0

	case "learn":
		if err := learn.Run(cmdArgs); err != nil {
			display.PrintError(err.Error())
			return 1
		}
		return 0

	case "verify":
		return verify.Run(cmdArgs)

	case "trust":
		return runTrust(cmdArgs)

	case "untrust":
		return runUntrust(cmdArgs)

	case "run":
		sepIdx := -1
		for i, a := range cmdArgs {
			if a == "--" {
				sepIdx = i
				break
			}
		}
		if sepIdx < 0 {
			display.PrintError("run requires -- separator: snip run -- <command> [args...]")
			return 1
		}
		if sepIdx > 0 {
			display.PrintError(fmt.Sprintf("run: unexpected arguments before -- (%s)", strings.Join(cmdArgs[:sepIdx], " ")))
			return 1
		}
		runArgs := cmdArgs[1:]
		if len(runArgs) == 0 {
			display.PrintError("run requires a command after --")
			return 1
		}
		runCmd := runArgs[0]
		runCmdArgs := runArgs[1:]
		if reason := unproxyableReason(runCmd); reason != "" {
			display.PrintError(fmt.Sprintf("%s cannot be proxied (%s)", runCmd, reason))
			return 1
		}
		return runPipeline(runCmd, runCmdArgs, flags)

	case "proxy":
		// Direct passthrough without filtering
		if len(cmdArgs) == 0 {
			display.PrintError("proxy requires a command argument")
			return 1
		}
		p := &engine.Pipeline{}
		return p.Passthrough(cmdArgs[0], cmdArgs[1:])
	}

	// Filter pipeline
	return runPipeline(command, cmdArgs, flags)
}

// runHook handles the "snip hook" subcommand for Claude Code PreToolUse.
// Always returns 0 (graceful degradation).
func runHook() int {
	snipBin, err := os.Executable()
	if err != nil {
		return 0
	}
	snipBin, err = filepath.EvalSymlinks(snipBin)
	if err != nil {
		return 0
	}
	snipBin, err = filepath.Abs(snipBin)
	if err != nil {
		return 0
	}

	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	filters, err := filter.LoadAll(cfg.Filters.Dirs())
	if err != nil {
		return 0
	}

	registry := filter.NewRegistry(filters)
	commands := registry.Commands()

	_ = hook.Run(os.Stdin, os.Stdout, commands, snipBin)
	return 0
}

func runPipeline(command string, args []string, flags Flags) int {
	cfg, err := config.Load()
	if err != nil {
		if flags.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "snip: config error: %v, using defaults\n", err)
		}
		cfg = config.DefaultConfig()
	}

	filters, err := filter.LoadAll(cfg.Filters.Dirs())
	if err != nil {
		display.PrintError(fmt.Sprintf("load filters: %v", err))
		return 1
	}

	registry := filter.NewRegistry(filters)

	// Lazy tracker: DB opens on first use (concurrently with command execution)
	var tracker *tracking.Tracker
	if tracking.DriverAvailable {
		dbPath := tracking.DBPath(cfg.Tracking.DBPath)
		tracker = tracking.NewLazyTracker(dbPath)
		defer func() { _ = tracker.Close() }()
	}

	teeCfg := tee.DefaultConfig()
	teeCfg.Enabled = cfg.Tee.Enabled
	teeCfg.Mode = cfg.Tee.Mode
	teeCfg.MaxFiles = cfg.Tee.MaxFiles
	teeCfg.MaxFileSize = cfg.Tee.MaxFileSize

	pipeline := &engine.Pipeline{
		Registry:      registry,
		Tracker:       tracker,
		TeeConfig:     teeCfg,
		Verbose:       flags.Verbose,
		UltraCompact:  flags.UltraCompact,
		QuietNoFilter: cfg.Display.QuietNoFilter,
		FilterEnabled: cfg.Filters.Enable,
	}

	return pipeline.Run(command, args)
}

func printUsage() {
	usage := `snip v%s — CLI Token Killer

Usage: snip [flags] <command> [args...]

Commands:
  run             Run command through snip filter pipeline (use -- to separate)
  <command>       Run command through snip filter pipeline (implicit)
  init            Install agent integration (default: claude-code)
  hook            Handle agent PreToolUse/shell hook
  hook-audit      Show recent hook activity (set SNIP_HOOK_AUDIT=1 to log)
  gain            Show token savings report
  cc-economics    Show financial impact of token savings by API tier
  discover        Scan sessions for missed filter opportunities
  learn           Detect CLI error-correction patterns in sessions
  verify          Run inline filter tests (--require-all to enforce coverage)
  config          Show current configuration
  trust           Trust project-local filter file(s) by SHA-256 hash
  untrust         Remove filter file(s) from the trust store
  proxy           Passthrough without filtering

Init flags:
  --agent <name>  Agent to configure:
                  claude-code (default), cursor, codex, windsurf, cline,
                  copilot, gemini, kilocode, antigravity
  --uninstall     Remove snip integration for the agent

Flags:
  -v, -vv      Verbose output (stackable)
  -u            Ultra-compact mode
  --skip-env    Skip environment loading
  --version     Show version
  --help        Show this help

Examples:
  snip run -- git log -10
  snip run -- docker build -t app .
  snip git log -10
  snip go test ./...
  snip gain --daily
  snip gain --weekly
  snip gain --monthly
  snip gain --top 10
  snip gain --history 20
  snip gain --no-truncate
  snip gain --quota
  snip cc-economics
  snip cc-economics --tier sonnet
  snip init
  snip init --agent cursor
  snip init --agent copilot
  snip init --agent gemini
  snip init --agent kilocode
`
	fmt.Printf(usage, version)
}

// unproxyableReason returns a human-readable reason if the command cannot be
// proxied through an external process, or "" if it can.
func unproxyableReason(command string) string {
	switch command {
	case "cd":
		return "it must run in the parent shell to change directory"
	case "source", ".":
		return "it must run in the parent shell to modify the environment"
	}
	return ""
}

// runTrust handles the "snip trust [path]" subcommand.
func runTrust(args []string) int {
	var paths []string

	if len(args) == 0 {
		// Default: trust all YAML files in .snip/filters/ relative to cwd
		cwd, err := os.Getwd()
		if err != nil {
			display.PrintError(fmt.Sprintf("get working directory: %v", err))
			return 1
		}
		dir := filepath.Join(cwd, ".snip", "filters")
		found, err := trust.FindFilterFiles(dir)
		if err != nil {
			display.PrintError(fmt.Sprintf("find filters in %s: %v", dir, err))
			return 1
		}
		if len(found) == 0 {
			display.PrintError(fmt.Sprintf("no YAML filter files found in %s", dir))
			return 1
		}
		paths = found
	} else {
		for _, arg := range args {
			info, err := os.Stat(arg)
			if err != nil {
				display.PrintError(fmt.Sprintf("stat %s: %v", arg, err))
				return 1
			}
			if info.IsDir() {
				found, err := trust.FindFilterFiles(arg)
				if err != nil {
					display.PrintError(fmt.Sprintf("find filters in %s: %v", arg, err))
					return 1
				}
				paths = append(paths, found...)
			} else {
				paths = append(paths, arg)
			}
		}
	}

	if len(paths) == 0 {
		display.PrintError("no filter files to trust")
		return 1
	}

	store, err := trust.Load()
	if err != nil {
		display.PrintError(err.Error())
		return 1
	}

	results, err := trust.Trust(store, paths)
	if err != nil {
		display.PrintError(err.Error())
		return 1
	}

	if err := trust.Save(store); err != nil {
		display.PrintError(err.Error())
		return 1
	}

	for _, r := range results {
		fmt.Printf("trusted: %s (sha256:%s)\n", r.Path, r.Hash)
	}
	return 0
}

// runUntrust handles the "snip untrust [path]" subcommand.
func runUntrust(args []string) int {
	var paths []string

	if len(args) == 0 {
		// Default: untrust all YAML files in .snip/filters/ relative to cwd
		cwd, err := os.Getwd()
		if err != nil {
			display.PrintError(fmt.Sprintf("get working directory: %v", err))
			return 1
		}
		dir := filepath.Join(cwd, ".snip", "filters")
		found, err := trust.FindFilterFiles(dir)
		if err != nil {
			display.PrintError(fmt.Sprintf("find filters in %s: %v", dir, err))
			return 1
		}
		if len(found) == 0 {
			display.PrintError(fmt.Sprintf("no YAML filter files found in %s", dir))
			return 1
		}
		paths = found
	} else {
		for _, arg := range args {
			info, err := os.Stat(arg)
			if err != nil {
				// File might not exist on disk but could still be in the trust store
				abs, absErr := filepath.Abs(arg)
				if absErr == nil {
					paths = append(paths, abs)
				}
				continue
			}
			if info.IsDir() {
				found, err := trust.FindFilterFiles(arg)
				if err != nil {
					display.PrintError(fmt.Sprintf("find filters in %s: %v", arg, err))
					return 1
				}
				paths = append(paths, found...)
			} else {
				paths = append(paths, arg)
			}
		}
	}

	if len(paths) == 0 {
		display.PrintError("no filter files to untrust")
		return 1
	}

	store, err := trust.Load()
	if err != nil {
		display.PrintError(err.Error())
		return 1
	}

	removed, err := trust.Untrust(store, paths)
	if err != nil {
		display.PrintError(err.Error())
		return 1
	}

	if err := trust.Save(store); err != nil {
		display.PrintError(err.Error())
		return 1
	}

	if len(removed) == 0 {
		fmt.Println("no matching entries found in trust store")
		return 0
	}

	for _, p := range removed {
		fmt.Printf("untrusted: %s\n", p)
	}
	return 0
}

// Version returns the current version string.
func Version() string {
	return version
}

// BuildCommandString joins command and args for display.
func BuildCommandString(command string, args []string) string {
	if len(args) == 0 {
		return command
	}
	return command + " " + strings.Join(args, " ")
}

package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"path/filepath"

	"github.com/edouard-claude/snip/internal/config"
	"github.com/edouard-claude/snip/internal/display"
	"github.com/edouard-claude/snip/internal/engine"
	"github.com/edouard-claude/snip/internal/filter"
	"github.com/edouard-claude/snip/internal/hook"
	"github.com/edouard-claude/snip/internal/initcmd"
	"github.com/edouard-claude/snip/internal/tee"
	"github.com/edouard-claude/snip/internal/tracking"
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
	if unproxyableReason(command) != "" {
		fmt.Fprintf(os.Stderr, "snip: %s cannot be proxied (%s)\n", command, unproxyableReason(command))
		return 1
	}

	// Built-in commands
	switch command {
	case "hook":
		return runHook()

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
  <command>    Run command through snip filter pipeline
  init         Install Claude Code hook
  hook         Handle Claude Code PreToolUse hook
  gain         Show token savings report
  config       Show current configuration
  proxy        Passthrough without filtering

Flags:
  -v, -vv      Verbose output (stackable)
  -u            Ultra-compact mode
  --skip-env    Skip environment loading
  --version     Show version
  --help        Show this help

Examples:
  snip git log -10
  snip go test ./...
  snip gain --daily
  snip gain --weekly
  snip gain --monthly
  snip gain --top 10
  snip gain --history 20
  snip gain --nocommand-truncate
  snip init
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

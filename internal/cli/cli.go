package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/edouard-claude/snip/internal/config"
	"github.com/edouard-claude/snip/internal/display"
	"github.com/edouard-claude/snip/internal/engine"
	"github.com/edouard-claude/snip/internal/filter"
	"github.com/edouard-claude/snip/internal/initcmd"
	"github.com/edouard-claude/snip/internal/tee"
	"github.com/edouard-claude/snip/internal/tracking"
)

const version = "0.1.0"

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

	// Built-in commands
	switch command {
	case "init":
		if err := initcmd.Run(cmdArgs); err != nil {
			display.PrintError(err.Error())
			return 1
		}
		return 0

	case "gain":
		tracker, err := lazyTracker()
		if err != nil {
			display.PrintError(err.Error())
			return 1
		}
		if tracker != nil {
			defer tracker.Close()
		}
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
		fmt.Printf("filters.dir: %s\n", cfg.Filters.Dir)
		fmt.Printf("tee.mode: %s\n", cfg.Tee.Mode)
		fmt.Printf("tee.max_files: %d\n", cfg.Tee.MaxFiles)
		fmt.Printf("display.color: %v\n", cfg.Display.Color)
		fmt.Printf("display.emoji: %v\n", cfg.Display.Emoji)
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

func runPipeline(command string, args []string, flags Flags) int {
	cfg, err := config.Load()
	if err != nil {
		if flags.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "snip: config error: %v, using defaults\n", err)
		}
		cfg = config.DefaultConfig()
	}

	filters, err := filter.LoadAll(cfg.Filters.Dir)
	if err != nil {
		display.PrintError(fmt.Sprintf("load filters: %v", err))
		return 1
	}

	registry := filter.NewRegistry(filters)

	tracker, err := lazyTracker()
	if err != nil && flags.Verbose > 0 {
		fmt.Fprintf(os.Stderr, "snip: tracking disabled: %v\n", err)
	}
	if tracker != nil {
		defer tracker.Close()
	}

	teeCfg := tee.DefaultConfig()
	teeCfg.Enabled = cfg.Tee.Enabled
	teeCfg.Mode = cfg.Tee.Mode
	teeCfg.MaxFiles = cfg.Tee.MaxFiles
	teeCfg.MaxFileSize = cfg.Tee.MaxFileSize

	pipeline := &engine.Pipeline{
		Registry:     registry,
		Tracker:      tracker,
		TeeConfig:    teeCfg,
		Verbose:      flags.Verbose,
		UltraCompact: flags.UltraCompact,
	}

	return pipeline.Run(command, args)
}

func lazyTracker() (*tracking.Tracker, error) {
	cfg, _ := config.Load()
	dbPath := tracking.DBPath("")
	if cfg != nil {
		dbPath = tracking.DBPath(cfg.Tracking.DBPath)
	}
	return tracking.NewTracker(dbPath)
}

func printUsage() {
	usage := `snip v%s — CLI Token Killer

Usage: snip [flags] <command> [args...]

Commands:
  <command>    Run command through snip filter pipeline
  init         Install Claude Code hook
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
  snip init
`
	fmt.Printf(usage, version)
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

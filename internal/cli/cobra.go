package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/edouard-claude/snip/internal/config"
	"github.com/edouard-claude/snip/internal/discover"
	"github.com/edouard-claude/snip/internal/display"
	"github.com/edouard-claude/snip/internal/economics"
	"github.com/edouard-claude/snip/internal/engine"
	"github.com/edouard-claude/snip/internal/hookaudit"
	"github.com/edouard-claude/snip/internal/initcmd"
	"github.com/edouard-claude/snip/internal/learn"
	"github.com/edouard-claude/snip/internal/tracking"
	"github.com/edouard-claude/snip/internal/verify"
	"github.com/spf13/cobra"
)

func newRootCommand(flags *Flags, exitCode *int) *cobra.Command {
	root := &cobra.Command{
		Use:     "snip",
		Short:   "CLI Token Killer",
		Long:    "snip proxies shell commands through a filter pipeline to reduce token usage.",
		Example: "  snip git log -10\n  snip gain --daily\n  snip init --agent cursor",
		Version: version,
		Args:    cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				_ = cmd.Help()
				*exitCode = 0
				return
			}

			command := args[0]
			cmdArgs := args[1:]

			if reason := unproxyableReason(command); reason != "" {
				fmt.Fprintf(os.Stderr, "snip: %s cannot be proxied (%s)\n", command, reason)
				*exitCode = 1
				return
			}

			*exitCode = runPipeline(command, cmdArgs, *flags)
		},
	}

	root.SetVersionTemplate("snip v{{.Version}}\n")
	root.Flags().SetInterspersed(false)
	root.PersistentFlags().SetInterspersed(false)

	root.PersistentFlags().CountVarP(&flags.Verbose, "verbose", "v", "Verbose output (stackable)")
	root.PersistentFlags().BoolVarP(&flags.UltraCompact, "ultra-compact", "u", false, "Ultra-compact mode")
	root.PersistentFlags().BoolVar(&flags.SkipEnv, "skip-env", false, "Skip environment loading")

	root.AddCommand(newHookCommand(exitCode))
	root.AddCommand(newHookAuditCommand(exitCode))
	root.AddCommand(newInitCommand(exitCode))
	root.AddCommand(newGainCommand(exitCode))
	root.AddCommand(newCCEconomicsCommand(exitCode))
	root.AddCommand(newConfigCommand(exitCode))
	root.AddCommand(newDiscoverCommand(exitCode))
	root.AddCommand(newLearnCommand(exitCode))
	root.AddCommand(newVerifyCommand(exitCode))
	root.AddCommand(newTrustCommand(exitCode))
	root.AddCommand(newUntrustCommand(exitCode))
	root.AddCommand(newProxyCommand(exitCode))

	return root
}

func newHookCommand(exitCode *int) *cobra.Command {
	return &cobra.Command{
		Use:     "hook",
		Short:   "Handle agent PreToolUse/shell hook",
		Example: "  snip hook",
		Run: func(cmd *cobra.Command, args []string) {
			*exitCode = runHook()
		},
	}
}

func newHookAuditCommand(exitCode *int) *cobra.Command {
	tail := hookaudit.DefaultTail
	clear := false

	cmd := &cobra.Command{
		Use:     "hook-audit",
		Short:   "Show recent hook activity",
		Example: "  snip hook-audit --tail 50\n  snip hook-audit --clear",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if clear {
				if err := hookaudit.Clear(); err != nil {
					display.PrintError(err.Error())
					*exitCode = 1
					return
				}
				fmt.Println("Audit log cleared.")
				*exitCode = 0
				return
			}

			events, err := hookaudit.ReadEvents()
			if err != nil {
				display.PrintError(err.Error())
				*exitCode = 1
				return
			}

			hookaudit.FormatTable(os.Stdout, events, tail)
			*exitCode = 0
		},
	}

	cmd.Flags().IntVar(&tail, "tail", hookaudit.DefaultTail, "Number of events to display")
	cmd.Flags().BoolVar(&clear, "clear", false, "Clear the audit log")

	return cmd
}

func newInitCommand(exitCode *int) *cobra.Command {
	agent := "claude-code"
	uninstall := false

	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Install agent integration",
		Example: "  snip init\n  snip init --agent codex\n  snip init --agent cursor --uninstall",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			initArgs := []string{"--agent", agent}
			if uninstall {
				initArgs = append(initArgs, "--uninstall")
			}
			if err := initcmd.Run(initArgs); err != nil {
				display.PrintError(err.Error())
				*exitCode = 1
				return
			}
			*exitCode = 0
		},
	}

	cmd.Flags().StringVar(&agent, "agent", "claude-code", "Agent to configure")
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "Remove snip integration for the selected agent")

	return cmd
}

func newGainCommand(exitCode *int) *cobra.Command {
	showDaily := false
	showWeekly := false
	showMonthly := false
	showJSON := false
	showCSV := false
	showQuota := false
	noTruncate := false
	historyN := 0
	topN := 0

	cmd := &cobra.Command{
		Use:     "gain",
		Short:   "Show token savings report",
		Example: "  snip gain\n  snip gain --daily\n  snip gain --top 20\n  snip gain --history 10",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if !tracking.DriverAvailable {
				display.PrintError("gain requires full build (this binary was built with -tags lite)")
				*exitCode = 1
				return
			}
			cfg, cfgErr := config.Load()
			if cfgErr != nil {
				cfg = config.DefaultConfig()
			}
			dbPath := tracking.DBPath(cfg.Tracking.DBPath)
			tracker, err := tracking.NewTracker(dbPath)
			if err != nil {
				display.PrintError(err.Error())
				*exitCode = 1
				return
			}
			defer func() { _ = tracker.Close() }()

			showTop := cmd.Flags().Changed("top")

			opts := display.GainOptions{
				ShowDaily:   showDaily,
				ShowWeekly:  showWeekly,
				ShowMonthly: showMonthly,
				ShowJSON:    showJSON,
				ShowCSV:     showCSV,
				ShowTop:     showTop,
				ShowQuota:   showQuota,
				NoTruncate:  noTruncate,
				HistoryN:    historyN,
				TopN:        topN,
				Days:        7,
			}
			if err := display.RunGainWithOptions(tracker, opts); err != nil {
				display.PrintError(err.Error())
				*exitCode = 1
				return
			}
			*exitCode = 0
		},
	}

	cmd.Flags().BoolVar(&showDaily, "daily", false, "Show daily savings breakdown")
	cmd.Flags().BoolVar(&showWeekly, "weekly", false, "Show weekly savings breakdown")
	cmd.Flags().BoolVar(&showMonthly, "monthly", false, "Show monthly savings breakdown")
	cmd.Flags().BoolVar(&showJSON, "json", false, "Export report as JSON")
	cmd.Flags().BoolVar(&showCSV, "csv", false, "Export report as CSV")
	cmd.Flags().IntVar(&topN, "top", 0, "Show top commands by savings")
	cmd.Flags().BoolVar(&showQuota, "quota", false, "Show monthly quota projection")
	cmd.Flags().BoolVar(&noTruncate, "no-truncate", false, "Do not truncate long command names")
	cmd.Flags().IntVar(&historyN, "history", 0, "Show recent command history entries")

	if f := cmd.Flags().Lookup("top"); f != nil {
		f.NoOptDefVal = "10"
	}
	if f := cmd.Flags().Lookup("history"); f != nil {
		f.NoOptDefVal = "10"
	}

	return cmd
}

func newCCEconomicsCommand(exitCode *int) *cobra.Command {
	tier := ""

	cmd := &cobra.Command{
		Use:     "cc-economics",
		Short:   "Show financial impact of token savings by API tier",
		Example: "  snip cc-economics\n  snip cc-economics --tier sonnet",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if !tracking.DriverAvailable {
				display.PrintError("cc-economics requires full build (this binary was built with -tags lite)")
				*exitCode = 1
				return
			}
			cfg, cfgErr := config.Load()
			if cfgErr != nil {
				cfg = config.DefaultConfig()
			}
			dbPath := tracking.DBPath(cfg.Tracking.DBPath)
			tracker, err := tracking.NewTracker(dbPath)
			if err != nil {
				display.PrintError(err.Error())
				*exitCode = 1
				return
			}
			defer func() { _ = tracker.Close() }()
			if err := economics.RunWithOptions(tracker, economics.Options{Tier: tier}); err != nil {
				display.PrintError(err.Error())
				*exitCode = 1
				return
			}
			*exitCode = 0
		},
	}

	cmd.Flags().StringVar(&tier, "tier", "", "Model tier to report (haiku, sonnet, opus)")

	return cmd
}

func newConfigCommand(exitCode *int) *cobra.Command {
	return &cobra.Command{
		Use:     "config",
		Short:   "Show current configuration",
		Example: "  snip config",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				display.PrintError(err.Error())
				*exitCode = 1
				return
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
				*exitCode = 0
				return
			}

			names := make([]string, 0, len(cfg.Filters.Enable))
			for k := range cfg.Filters.Enable {
				names = append(names, k)
			}
			sort.Strings(names)
			for _, name := range names {
				fmt.Printf("filters.enable.%s: %v\n", name, cfg.Filters.Enable[name])
			}

			*exitCode = 0
		},
	}
}

func newDiscoverCommand(exitCode *int) *cobra.Command {
	all := false
	since := 7

	cmd := &cobra.Command{
		Use:     "discover",
		Short:   "Scan sessions for missed filter opportunities",
		Example: "  snip discover\n  snip discover --all --since 30",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			opts := discover.Options{All: all, Since: since}
			if err := discover.RunWithOptions(opts); err != nil {
				display.PrintError(err.Error())
				*exitCode = 1
				return
			}
			*exitCode = 0
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Scan all projects instead of only the current project")
	cmd.Flags().IntVar(&since, "since", 7, "Look back N days")

	return cmd
}

func newLearnCommand(exitCode *int) *cobra.Command {
	all := false
	generate := false
	since := 30

	cmd := &cobra.Command{
		Use:     "learn",
		Short:   "Detect CLI error-correction patterns in sessions",
		Example: "  snip learn\n  snip learn --since 14 --generate",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			opts := learn.Options{All: all, Generate: generate, Since: since}
			if err := learn.RunWithOptions(opts); err != nil {
				display.PrintError(err.Error())
				*exitCode = 1
				return
			}
			*exitCode = 0
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Scan all projects instead of only the current project")
	cmd.Flags().BoolVar(&generate, "generate", false, "Generate correction rules from detected patterns")
	cmd.Flags().IntVar(&since, "since", 30, "Look back N days")

	return cmd
}

func newVerifyCommand(exitCode *int) *cobra.Command {
	requireAll := false

	cmd := &cobra.Command{
		Use:     "verify",
		Short:   "Run inline filter tests",
		Example: "  snip verify\n  snip verify --require-all",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			verifyArgs := []string{}
			if requireAll {
				verifyArgs = append(verifyArgs, "--require-all")
			}
			*exitCode = verify.Run(verifyArgs)
		},
	}

	cmd.Flags().BoolVar(&requireAll, "require-all", false, "Fail if any filter has no tests")

	return cmd
}

func newTrustCommand(exitCode *int) *cobra.Command {
	return &cobra.Command{
		Use:     "trust [path]",
		Short:   "Trust project-local filter files by hash",
		Example: "  snip trust\n  snip trust .snip/filters\n  snip trust path/to/filter.yaml",
		Args:    cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			*exitCode = runTrust(args)
		},
	}
}

func newUntrustCommand(exitCode *int) *cobra.Command {
	return &cobra.Command{
		Use:     "untrust [path]",
		Short:   "Remove filter file entries from trust store",
		Example: "  snip untrust\n  snip untrust .snip/filters\n  snip untrust path/to/filter.yaml",
		Args:    cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			*exitCode = runUntrust(args)
		},
	}
}

func newProxyCommand(exitCode *int) *cobra.Command {
	return &cobra.Command{
		Use:                "proxy <command> [args...]",
		Short:              "Passthrough without filtering",
		Example:            "  snip proxy git status\n  snip proxy go test ./...",
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			if wantsHelp(args) {
				_ = cmd.Help()
				*exitCode = 0
				return
			}
			if len(args) == 0 {
				display.PrintError("proxy requires a command argument")
				*exitCode = 1
				return
			}
			p := &engine.Pipeline{}
			*exitCode = p.Passthrough(args[0], args[1:])
		},
	}
}

func wantsHelp(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

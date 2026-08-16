package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/edouard-claude/snip/internal/trust"
)

var envVarRe = regexp.MustCompile(`\$\{env\.(\w+)\}`)

type Config struct {
	Mode      string          `toml:"mode"` // "user" (default) or "project"
	Tracking  TrackingConfig  `toml:"tracking"`
	Display   DisplayConfig   `toml:"display"`
	Filters   FiltersConfig   `toml:"filters"`
	Tee       TeeConfig       `toml:"tee"`
	Economics EconomicsConfig `toml:"economics"`
}

// EconomicsConfig holds the pricing tiers used by cc-economics. Keys are
// tier names, values are $ per 1M input tokens. Empty means the built-in
// defaults (current Anthropic list prices) apply.
type EconomicsConfig struct {
	Tiers map[string]float64 `toml:"tiers"`
}

type TrackingConfig struct {
	DBPath string `toml:"db_path"`
	// TrackUnfiltered, when true, records commands that ran with no matching
	// filter at all so `snip gain --unfiltered` can surface filter-coverage
	// gaps. Off by default to keep the passthrough path free of extra work.
	// See issue #96.
	TrackUnfiltered bool `toml:"track_unfiltered"`
}

type DisplayConfig struct {
	Color         bool `toml:"color"`
	Emoji         bool `toml:"emoji"`
	QuietNoFilter bool `toml:"quiet_no_filter"`
	Summary       bool `toml:"summary"`
}

type FiltersConfig struct {
	Dir      any                       `toml:"dir"`
	Enable   map[string]bool           `toml:"enable"`
	Global   FilterGlobalConfig        `toml:"global"`
	Override map[string]FilterOverride `toml:"override"`
	Bypass   FilterBypassConfig        `toml:"bypass"`
	// TransparentPrefixes are wrapper commands (e.g. "poetry run",
	// "docker exec ctr") that snip strips before routing so the inner command
	// matches its filter. Built-in prefixes (uv run, ...) always apply too.
	TransparentPrefixes []string `toml:"transparent_prefixes"`
}

// FilterGlobalConfig applies to all filters in the pipeline.
type FilterGlobalConfig struct {
	MaxLines       int    `toml:"max_lines"`        // 0 = unlimited
	MaxLineLength  int    `toml:"max_line_length"`  // 0 = unlimited
	MaxOutputBytes int    `toml:"max_output_bytes"` // 0 = unlimited
	StreamMode     string `toml:"stream_mode"`      // "filter" | "full"
}

// FilterOverride overrides specific pipeline action parameters for a named filter.
type FilterOverride struct {
	Head          int    `toml:"head"`
	Tail          int    `toml:"tail"`
	TruncateLines int    `toml:"truncate_lines"`
	KeepLines     string `toml:"keep_lines"`
	RemoveLines   string `toml:"remove_lines"`
	StreamMode    string `toml:"stream_mode"` // "full" = skip the entire pipeline
}

// FilterBypassConfig contains commands that should always bypass filtering.
type FilterBypassConfig struct {
	Commands []string `toml:"commands"`
}

// Dirs returns the filter directories as a normalized string slice.
// Dir can be a single string or an array of strings in TOML.
func (f *FiltersConfig) Dirs() []string {
	switch v := f.Dir.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []any:
		dirs := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				dirs = append(dirs, s)
			}
		}
		return dirs
	case []string:
		return v
	default:
		return nil
	}
}

type TeeConfig struct {
	Enabled     bool   `toml:"enabled"`
	Mode        string `toml:"mode"` // "failures", "always", "never"
	MaxFiles    int    `toml:"max_files"`
	MaxFileSize int64  `toml:"max_file_size"`
	// ProjectMarker, when set (e.g. ".git"), makes tee walk upward from the
	// current working directory looking for that file or directory. If found
	// in directory D, tee files are written to D/.snip/tee/ instead of the
	// global tee directory (falling back to the global directory when the
	// project directory is not writable). Empty (default) keeps the global
	// directory. See issue #107.
	ProjectMarker string `toml:"project_marker"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return &Config{
		Tracking: TrackingConfig{
			DBPath: filepath.Join(home, ".local", "share", "snip", "tracking.db"),
		},
		Display: DisplayConfig{
			Color:   true,
			Emoji:   true,
			Summary: false,
		},
		Filters: FiltersConfig{
			Dir: filepath.Join(home, ".config", "snip", "filters"),
		},
		Tee: TeeConfig{
			Enabled:     true,
			Mode:        "failures",
			MaxFiles:    20,
			MaxFileSize: 1 << 20, // 1MB
		},
	}
}

// Load reads config from file, merging with defaults. Returns defaults if file missing.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		// go-toml/v2 cannot decode a TOML array into interface{}.
		// Retry with an alternative struct that accepts dir as []string.
		cfg = DefaultConfig()
		if !tryUnmarshalArrayDir(data, cfg) {
			return nil, err
		}
	}

	cfg.expandPaths()

	return cfg, nil
}

// tryUnmarshalArrayDir handles the case where filters.dir is a TOML array.
// The alternative structs must mirror Config and FiltersConfig field for
// field (only Dir changes type): any section missing here is silently lost
// whenever filters.dir is an array (#158).
func tryUnmarshalArrayDir(data []byte, cfg *Config) bool {
	type filtersArray struct {
		Dir                 []string                  `toml:"dir"`
		Enable              map[string]bool           `toml:"enable"`
		TransparentPrefixes []string                  `toml:"transparent_prefixes"`
		Global              FilterGlobalConfig        `toml:"global"`
		Override            map[string]FilterOverride `toml:"override"`
		Bypass              FilterBypassConfig        `toml:"bypass"`
	}
	type configArray struct {
		Mode      string          `toml:"mode"`
		Tracking  TrackingConfig  `toml:"tracking"`
		Display   DisplayConfig   `toml:"display"`
		Filters   filtersArray    `toml:"filters"`
		Tee       TeeConfig       `toml:"tee"`
		Economics EconomicsConfig `toml:"economics"`
	}

	def := DefaultConfig()
	alt := configArray{
		Mode:     def.Mode,
		Tracking: def.Tracking,
		Display:  def.Display,
		Filters:  filtersArray{Dir: def.Filters.Dirs()},
		Tee:      def.Tee,
	}

	if err := toml.Unmarshal(data, &alt); err != nil {
		return false
	}

	cfg.Mode = alt.Mode
	cfg.Tracking = alt.Tracking
	cfg.Display = alt.Display
	cfg.Filters.Dir = alt.Filters.Dir
	cfg.Filters.Enable = alt.Filters.Enable
	cfg.Filters.TransparentPrefixes = alt.Filters.TransparentPrefixes
	cfg.Filters.Global = alt.Filters.Global
	cfg.Filters.Override = alt.Filters.Override
	cfg.Filters.Bypass = alt.Filters.Bypass
	cfg.Tee = alt.Tee
	cfg.Economics = alt.Economics
	return true
}

// Path returns the user config file location: SNIP_CONFIG when set,
// otherwise ~/.config/snip/config.toml.
func Path() string {
	return configPath()
}

// expandPaths expands ${env.VAR} references and leading "~/" in all path fields.
func (c *Config) expandPaths() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	c.Tracking.DBPath = expandPath(expandEnvVars(c.Tracking.DBPath), home)

	dirs := c.Filters.Dirs()
	expanded := make([]string, len(dirs))
	for i, d := range dirs {
		expanded[i] = expandPath(expandEnvVars(d), home)
	}
	c.Filters.Dir = expanded
}

func expandPath(p, home string) string {
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

// expandEnvVars replaces ${env.VAR} patterns with the corresponding
// environment variable value.
func expandEnvVars(s string) string {
	return envVarRe.ReplaceAllStringFunc(s, func(match string) string {
		// "${env.VAR}" -> extract "VAR"
		name := match[6 : len(match)-1]
		return os.Getenv(name)
	})
}

// projectConfigPath walks upward from the current working directory looking
// for a .snip/config.toml file. Returns the first match found (closest to
// CWD takes priority). Returns an empty string if no project config exists.
func projectConfigPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		cfg := filepath.Join(dir, ".snip", "config.toml")
		if _, err := os.Stat(cfg); err == nil {
			return cfg
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached root on this platform
		}
	}
	return ""
}

// pluginConfigPath returns the config path provided by an agent plugin
// through SNIP_PLUGIN_CONFIG (e.g. a Claude Code plugin hook exporting
// SNIP_PLUGIN_CONFIG=${CLAUDE_PLUGIN_ROOT}/snip/config.toml), or "" when
// no plugin layer is present.
func pluginConfigPath() string {
	return os.Getenv("SNIP_PLUGIN_CONFIG")
}

// LoadWithPlugin returns the user config with the plugin layer, if any,
// merged underneath it: plugin < user. Only the filter sections participate
// (enable, global, override, bypass, transparent_prefixes, dir), matching
// what a project config can influence. filters.dir concatenates plugin
// directories before the user's, so user filters override plugin filters
// by name. The plugin file must be trusted (`snip trust <path>`); an
// untrusted or unreadable plugin layer is skipped with a stderr warning.
func LoadWithPlugin() (*Config, error) {
	user, err := Load()
	if err != nil {
		return nil, err
	}
	applyPluginLayer(user)
	return user, nil
}

// applyPluginLayer merges the SNIP_PLUGIN_CONFIG file underneath the user
// config in place. The user's explicit settings win over the plugin's.
func applyPluginLayer(user *Config) {
	path := pluginConfigPath()
	if path == "" {
		return
	}

	store, err := trust.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "snip: ignoring untrusted plugin config %s (trust store unreadable: %v)\n", path, err)
		return
	}
	if !trust.IsTrusted(store, path) {
		fmt.Fprintf(os.Stderr, "snip: ignoring untrusted plugin config %s (run 'snip trust %s' to trust)\n", path, path)
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "snip: ignoring unreadable plugin config %s: %v\n", path, err)
		return
	}
	plugin := DefaultConfig()
	if err := toml.Unmarshal(data, plugin); err != nil {
		plugin = DefaultConfig()
		if !tryUnmarshalArrayDir(data, plugin) {
			fmt.Fprintf(os.Stderr, "snip: ignoring invalid plugin config %s: %v\n", path, err)
			return
		}
	}
	plugin.expandPaths()

	// Relative filter dirs resolve against the plugin config's own directory,
	// so a plugin ships `dir = "filters"` with no env dependency (#169).
	// DefaultConfig pre-fills Dir with the absolute user filters dir, so only
	// paths the plugin actually wrote can be relative here.
	pluginBase := filepath.Dir(path)
	pluginDirs := plugin.Filters.Dirs()
	for i, d := range pluginDirs {
		if !filepath.IsAbs(d) {
			pluginDirs[i] = filepath.Join(pluginBase, d)
		}
	}
	plugin.Filters.Dir = pluginDirs

	// Enable and Override: plugin provides the base, user keys win.
	if len(plugin.Filters.Enable) > 0 {
		merged := make(map[string]bool, len(plugin.Filters.Enable)+len(user.Filters.Enable))
		for k, v := range plugin.Filters.Enable {
			merged[k] = v
		}
		for k, v := range user.Filters.Enable {
			merged[k] = v
		}
		user.Filters.Enable = merged
	}
	if len(plugin.Filters.Override) > 0 {
		merged := make(map[string]FilterOverride, len(plugin.Filters.Override)+len(user.Filters.Override))
		for k, v := range plugin.Filters.Override {
			merged[k] = v
		}
		for k, v := range user.Filters.Override {
			merged[k] = v
		}
		user.Filters.Override = merged
	}

	// Global caps: the user's own block wins entirely when set at all.
	if user.Filters.Global == (FilterGlobalConfig{}) {
		user.Filters.Global = plugin.Filters.Global
	}

	// Bypass and transparent prefixes accumulate from both layers.
	user.Filters.Bypass.Commands = append(
		append([]string{}, plugin.Filters.Bypass.Commands...),
		user.Filters.Bypass.Commands...)
	user.Filters.TransparentPrefixes = append(
		append([]string{}, plugin.Filters.TransparentPrefixes...),
		user.Filters.TransparentPrefixes...)

	// Filter directories: plugin dirs first, user dirs last (later dirs
	// win by filter name in the loader). Skip duplicates.
	seen := make(map[string]bool)
	var dirs []string
	for _, d := range append(plugin.Filters.Dirs(), user.Filters.Dirs()...) {
		if d != "" && !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	user.Filters.Dir = dirs
}

// LoadMerged loads the user config (with the plugin layer underneath, see
// LoadWithPlugin), then layers the project config on top. The resulting
// precedence is plugin < user < project. When mode == "project", the project
// config's filter settings override the user's. When mode == "user"
// (default), user settings take priority.
func LoadMerged() (*Config, error) {
	user, err := LoadWithPlugin()
	if err != nil {
		// If user config file is missing, use defaults (normal for new installs).
		// Other errors (permission, corrupt TOML) propagate to the caller so the
		// user knows something is wrong with their config.
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("load user config: %w", err)
	}

	projectPath := projectConfigPath()
	if projectPath == "" {
		return user, nil // no project config — user only
	}

	// Trust gate: project configs must be explicitly trusted via `snip trust`.
	// Without this guard, any cloned repo could ship a .snip/config.toml that
	// disables filtering, injects ReDoS regex, or adds bypass commands.
	store, err := trust.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "snip: ignoring untrusted project config %s (trust store unreadable: %v)\n", projectPath, err)
		return user, nil // no trust store = no project configs trusted
	}
	if !trust.IsTrusted(store, projectPath) {
		fmt.Fprintf(os.Stderr, "snip: ignoring untrusted project config %s (run 'snip trust %s' to trust)\n", projectPath, projectPath)
		return user, nil // untrusted: fall back to user config only
	}

	project := DefaultConfig()
	data, err := os.ReadFile(projectPath)
	if err != nil {
		return user, nil
	}
	if err := toml.Unmarshal(data, project); err != nil {
		return nil, fmt.Errorf("parse project config %s: %w", projectPath, err)
	}

	// Default mode is "user" — developer's personal config wins conflicts
	merged := user
	merged.Mode = project.Mode

	// When project mode is active, project overrides user for filter sections
	if project.Mode == "project" {
		// Enable/disable: project keys win for shared names
		if merged.Filters.Enable == nil {
			merged.Filters.Enable = make(map[string]bool)
		}
		for k, v := range project.Filters.Enable {
			merged.Filters.Enable[k] = v
		}
		// Global limits: project wins entirely
		if project.Filters.Global.MaxLines > 0 || project.Filters.Global.MaxLineLength > 0 || project.Filters.Global.MaxOutputBytes > 0 || project.Filters.Global.StreamMode != "" {
			merged.Filters.Global = project.Filters.Global
		}
		// Per-filter overrides: project wins
		if project.Filters.Override != nil {
			if merged.Filters.Override == nil {
				merged.Filters.Override = make(map[string]FilterOverride)
			}
			for k, v := range project.Filters.Override {
				merged.Filters.Override[k] = v
			}
		}
	}

	// Bypass list merges from both sides (no override).
	// Force fresh slice to avoid reusing user's backing array on double-call.
	merged.Filters.Bypass.Commands = append([]string{}, user.Filters.Bypass.Commands...)
	merged.Filters.Bypass.Commands = append(merged.Filters.Bypass.Commands,
		project.Filters.Bypass.Commands...)

	return merged, nil
}

func configPath() string {
	if p := os.Getenv("SNIP_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "snip", "config.toml")
}

// ClaudeBaseDir returns the base directory for Claude Code's `.claude` files.
// It honors the CLAUDE_CONFIG_DIR environment variable (which Claude Code itself
// respects), falling back to ~/.claude.
func ClaudeBaseDir() (string, error) {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

// ClaudeProjectsDir returns the base Claude Code projects directory,
// honoring CLAUDE_CONFIG_DIR. Returns "" when the home directory cannot be
// resolved; callers treat that as "no sessions found".
func ClaudeProjectsDir() string {
	base, err := ClaudeBaseDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "projects")
}

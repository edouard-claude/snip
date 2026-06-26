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
	Mode     string         `toml:"mode"` // "user" (default) or "project"
	Tracking TrackingConfig `toml:"tracking"`
	Display  DisplayConfig  `toml:"display"`
	Filters  FiltersConfig  `toml:"filters"`
	Tee      TeeConfig      `toml:"tee"`
}

type TrackingConfig struct {
	DBPath string `toml:"db_path"`
}

type DisplayConfig struct {
	Color         bool `toml:"color"`
	Emoji         bool `toml:"emoji"`
	QuietNoFilter bool `toml:"quiet_no_filter"`
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
			Color: true,
			Emoji: true,
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
func tryUnmarshalArrayDir(data []byte, cfg *Config) bool {
	type filtersArray struct {
		Dir    []string        `toml:"dir"`
		Enable map[string]bool `toml:"enable"`
	}
	type configArray struct {
		Tracking TrackingConfig `toml:"tracking"`
		Display  DisplayConfig  `toml:"display"`
		Filters  filtersArray   `toml:"filters"`
		Tee      TeeConfig      `toml:"tee"`
	}

	def := DefaultConfig()
	alt := configArray{
		Tracking: def.Tracking,
		Display:  def.Display,
		Filters:  filtersArray{Dir: def.Filters.Dirs()},
		Tee:      def.Tee,
	}

	if err := toml.Unmarshal(data, &alt); err != nil {
		return false
	}

	cfg.Tracking = alt.Tracking
	cfg.Display = alt.Display
	cfg.Filters.Dir = alt.Filters.Dir
	cfg.Filters.Enable = alt.Filters.Enable
	cfg.Tee = alt.Tee
	return true
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

// LoadMerged loads the user config, then layers the project config on top.
// When mode == "project", the project config's filter settings override the
// user's. When mode == "user" (default), user settings take priority.
func LoadMerged() (*Config, error) {
	user, err := Load()
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
		return user, nil // no trust store = no project configs trusted
	}
	if !trust.IsTrusted(store, projectPath) {
		return user, nil // untrusted: silently fall back to user config only
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

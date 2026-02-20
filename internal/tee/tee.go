package tee

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Config for tee behavior.
type Config struct {
	Enabled     bool
	Mode        string // "failures", "always", "never"
	MaxFiles    int
	MaxFileSize int64
	Dir         string
}

// DefaultConfig returns tee defaults.
func DefaultConfig() Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return Config{
		Enabled:     true,
		Mode:        "failures",
		MaxFiles:    20,
		MaxFileSize: 1 << 20, // 1MB
		Dir:         filepath.Join(home, ".local", "share", "snip", "tee"),
	}
}

// MaybeSave saves raw output if conditions are met. Returns hint string if saved.
func MaybeSave(raw string, exitCode int, cmd string, cfg Config) string {
	if !cfg.Enabled || cfg.Mode == "never" {
		return ""
	}

	// Check SNIP_TEE env override
	if os.Getenv("SNIP_TEE") == "0" {
		return ""
	}

	shouldSave := cfg.Mode == "always" || (cfg.Mode == "failures" && exitCode != 0)
	if !shouldSave {
		return ""
	}

	// Skip if output is too small
	if len(raw) < 500 {
		return ""
	}

	dir := cfg.Dir
	if envDir := os.Getenv("SNIP_TEE_DIR"); envDir != "" {
		dir = envDir
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "" // Silent failure
	}

	// Truncate if too large (rune-safe)
	data := raw
	if int64(len(data)) > cfg.MaxFileSize {
		runes := []rune(data)
		// Find the rune boundary within MaxFileSize bytes
		byteCount := 0
		for i, r := range runes {
			byteCount += len(string(r))
			if int64(byteCount) > cfg.MaxFileSize {
				data = string(runes[:i])
				break
			}
		}
	}

	// Sanitize command name for filename
	safeName := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, cmd)

	filename := fmt.Sprintf("%d-%s.log", time.Now().Unix(), safeName)
	path := filepath.Join(dir, filename)

	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		return "" // Silent failure
	}

	// Rotate
	rotateFiles(dir, cfg.MaxFiles)

	return fmt.Sprintf("[full output: %s]", path)
}

func rotateFiles(dir string, maxFiles int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var logFiles []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			logFiles = append(logFiles, e)
		}
	}

	if len(logFiles) <= maxFiles {
		return
	}

	// Sort by name (timestamp prefix = chronological)
	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].Name() < logFiles[j].Name()
	})

	// Remove oldest
	toRemove := len(logFiles) - maxFiles
	for i := 0; i < toRemove; i++ {
		os.Remove(filepath.Join(dir, logFiles[i].Name()))
	}
}

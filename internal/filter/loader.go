package filter

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EmbeddedFS is set by the main package to provide embedded filter files.
// This avoids go:embed constraints on internal packages.
var EmbeddedFS *embed.FS

// LoadEmbedded loads all embedded YAML filter files.
func LoadEmbedded() ([]Filter, error) {
	if EmbeddedFS == nil {
		return nil, nil
	}

	// Try "filters" subdir first (when embedded from root), then "." (flat)
	dir := "filters"
	entries, err := EmbeddedFS.ReadDir(dir)
	if err != nil {
		dir = "."
		entries, err = EmbeddedFS.ReadDir(dir)
		if err != nil {
			return nil, nil
		}
	}

	var filters []Filter
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		path := entry.Name()
		if dir != "." {
			path = dir + "/" + entry.Name()
		}
		data, err := EmbeddedFS.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read embedded filter %s: %w", entry.Name(), err)
		}
		f, err := ParseFilter(data)
		if err != nil {
			return nil, fmt.Errorf("parse embedded filter %s: %w", entry.Name(), err)
		}
		filters = append(filters, *f)
	}
	return filters, nil
}

// LoadUserFilters loads all YAML files from a directory.
func LoadUserFilters(dir string) ([]Filter, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read filter dir: %w", err)
	}

	var filters []Filter
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read user filter %s: %w", entry.Name(), err)
		}
		f, err := ParseFilter(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "snip: skipping invalid filter %s: %v\n", entry.Name(), err)
			continue
		}
		filters = append(filters, *f)
	}
	return filters, nil
}

// LoadAll loads user filters (priority) and embedded filters, merging by name.
func LoadAll(userDir string) ([]Filter, error) {
	user, err := LoadUserFilters(userDir)
	if err != nil {
		return nil, err
	}

	embedded, err := LoadEmbedded()
	if err != nil {
		return nil, err
	}

	byName := make(map[string]bool)
	var result []Filter
	for _, f := range user {
		byName[f.Name] = true
		result = append(result, f)
	}
	for _, f := range embedded {
		if !byName[f.Name] {
			result = append(result, f)
		}
	}
	return result, nil
}

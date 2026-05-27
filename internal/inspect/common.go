// Package inspect provides code quality checks for snip's own Go source.
// Commands: snip inspect --dead-fields, snip inspect --append-safety, snip inspect --all.
package inspect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func findGoFiles(dir string, skipTests bool) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "vendor" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".go") {
			return nil
		}
		if skipTests && strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}

func resolveRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found (walked up from current directory)")
		}
		dir = parent
	}
}

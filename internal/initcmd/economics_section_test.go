package initcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureEconomicsSectionCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("SNIP_CONFIG", path)

	if err := ensureEconomicsSection("claude-code"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "[economics.tiers]") || !strings.Contains(s, "# opus = 5.00") {
		t.Errorf("expected commented Anthropic tiers template, got:\n%s", s)
	}
	if !strings.Contains(s, "adjust") {
		t.Errorf("expected an adjust-your-rates note, got:\n%s", s)
	}
}

func TestEnsureEconomicsSectionAppendsWithoutTouchingExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	existing := "[display]\ncolor = true\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SNIP_CONFIG", path)

	if err := ensureEconomicsSection("gemini"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	s := string(data)
	if !strings.HasPrefix(s, existing) {
		t.Errorf("existing content must be preserved verbatim, got:\n%s", s)
	}
	if !strings.Contains(s, "[economics.tiers]") {
		t.Errorf("expected appended economics section, got:\n%s", s)
	}
	// Non-Anthropic agent: generic template, no invented provider rates.
	if strings.Contains(s, "fable") {
		t.Errorf("generic template must not carry Anthropic tier names, got:\n%s", s)
	}
}

func TestEnsureEconomicsSectionIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("SNIP_CONFIG", path)

	if err := ensureEconomicsSection("claude-code"); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)
	if err := ensureEconomicsSection("claude-code"); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Error("second call must be a no-op")
	}
}

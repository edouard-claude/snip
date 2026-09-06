package trust

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestHashFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	content := []byte("name: test\ncommands: [echo]\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := HashFile(path)
	if err != nil {
		t.Fatal(err)
	}

	want := sha256.Sum256(content)
	wantHex := hex.EncodeToString(want[:])
	if got != wantHex {
		t.Errorf("HashFile = %s, want %s", got, wantHex)
	}
}

func TestHashFile_NotFound(t *testing.T) {
	_, err := HashFile("/nonexistent/file.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestTrustAndIsTrusted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "filter.yaml")
	if err := os.WriteFile(path, []byte("name: foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := make(Store)

	// Not trusted initially
	if IsTrusted(store, path) {
		t.Error("expected file to be untrusted initially")
	}

	// Trust it
	results, err := Trust(store, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	abs, _ := filepath.Abs(path)
	if results[0].Path != abs {
		t.Errorf("result path = %s, want %s", results[0].Path, abs)
	}
	if results[0].Hash == "" {
		t.Error("expected non-empty hash")
	}

	// Now trusted
	if !IsTrusted(store, path) {
		t.Error("expected file to be trusted after Trust()")
	}
}

func TestIsTrusted_ModifiedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "filter.yaml")
	if err := os.WriteFile(path, []byte("name: foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := make(Store)
	if _, err := Trust(store, []string{path}); err != nil {
		t.Fatal(err)
	}

	// Modify the file
	if err := os.WriteFile(path, []byte("name: foo\ncommands: [malicious]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Hash mismatch should make it untrusted
	if IsTrusted(store, path) {
		t.Error("expected file to be untrusted after modification")
	}
}

func TestUntrust(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "filter.yaml")
	if err := os.WriteFile(path, []byte("name: foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := make(Store)
	if _, err := Trust(store, []string{path}); err != nil {
		t.Fatal(err)
	}
	if !IsTrusted(store, path) {
		t.Fatal("expected trusted before untrust")
	}

	removed, err := Untrust(store, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(removed))
	}
	if IsTrusted(store, path) {
		t.Error("expected untrusted after Untrust()")
	}
}

func TestUntrust_NotPresent(t *testing.T) {
	store := make(Store)
	removed, err := Untrust(store, []string{"/nonexistent/path.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 {
		t.Errorf("expected 0 removed, got %d", len(removed))
	}
}

func TestLoadSave(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "trusted.json")

	store := Store{
		"/some/path/filter.yaml": "abc123",
	}

	if err := SaveTo(store, storePath); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadFrom(storePath)
	if err != nil {
		t.Fatal(err)
	}

	if loaded["/some/path/filter.yaml"] != "abc123" {
		t.Errorf("loaded hash = %s, want abc123", loaded["/some/path/filter.yaml"])
	}
}

func TestLoadFrom_Missing(t *testing.T) {
	store, err := LoadFrom("/nonexistent/trusted.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(store) != 0 {
		t.Errorf("expected empty store, got %d entries", len(store))
	}
}

func TestLoadFrom_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trusted.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFindFilterFiles(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	for _, name := range []string{"a.yaml", "b.yml", "c.txt", "d.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Create a subdirectory (should be skipped)
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	files, err := FindFilterFiles(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 YAML files, got %d: %v", len(files), files)
	}

	// Check that both yaml/yml files are present
	names := map[string]bool{}
	for _, f := range files {
		names[filepath.Base(f)] = true
	}
	if !names["a.yaml"] || !names["b.yml"] {
		t.Errorf("expected a.yaml and b.yml, got %v", names)
	}
}

func TestFindFilterFiles_NotFound(t *testing.T) {
	_, err := FindFilterFiles("/nonexistent/dir")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestIsGlobalDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	tests := []struct {
		dir  string
		want bool
	}{
		{filepath.Join(home, ".config", "snip", "filters"), true},
		{filepath.Join(home, ".config", "snip"), true},
		{filepath.Join(home, ".config", "snip", "filters", "sub"), true},
		{"/some/project/.snip/filters", false},
		{filepath.Join(home, ".config", "other"), false},
		{filepath.Join(home, "projects", "myapp"), false},
	}

	for _, tt := range tests {
		got := IsGlobalDir(tt.dir)
		if got != tt.want {
			t.Errorf("IsGlobalDir(%s) = %v, want %v", tt.dir, got, tt.want)
		}
	}
}

func TestTrustMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		filepath.Join(dir, "a.yaml"),
		filepath.Join(dir, "b.yaml"),
	}
	for _, p := range paths {
		if err := os.WriteFile(p, []byte("name: "+filepath.Base(p)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	store := make(Store)
	results, err := Trust(store, paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for _, p := range paths {
		if !IsTrusted(store, p) {
			t.Errorf("expected %s to be trusted", p)
		}
	}
}

func TestTrustStorePath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	path := TrustStorePath()
	expected := filepath.Join(tmpDir, ".config", "snip", "trusted.json")
	if path != expected {
		t.Errorf("TrustStorePath() = %q, want %q", path, expected)
	}
}

func TestLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create necessary directories.
	configDir := filepath.Join(tmpDir, ".config", "snip")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	store := Store{
		"/a/filter.yaml": "abc123",
		"/b/filter.yaml": "def456",
	}

	if err := Save(store); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded["/a/filter.yaml"] != "abc123" {
		t.Errorf("loaded hash = %s, want abc123", loaded["/a/filter.yaml"])
	}
	if loaded["/b/filter.yaml"] != "def456" {
		t.Errorf("loaded hash = %s, want def456", loaded["/b/filter.yaml"])
	}
	if len(loaded) != 2 {
		t.Errorf("expected 2 entries, got %d", len(loaded))
	}
}

func TestLoadNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	store, err := Load()
	if err != nil {
		t.Fatalf("Load nonexistent: %v", err)
	}
	if len(store) != 0 {
		t.Errorf("expected empty store, got %d entries", len(store))
	}
}

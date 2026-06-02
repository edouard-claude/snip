package inspect

import (
	"go/ast"
	"testing"
)

func TestDeadFields_KnownCases(t *testing.T) {
	root := "../.."

	checker := DeadFieldChecker{}
	findings, err := checker.Run(root)
	if err != nil {
		t.Fatalf("DeadFieldChecker.Run: %v", err)
	}

	foundFields := make(map[string]bool)
	for _, f := range findings {
		foundFields[f.Field] = true
	}

	// Known: MaxOutputBytes is now wired — should NOT be dead
	if foundFields["MaxOutputBytes"] {
		t.Errorf("MaxOutputBytes should NOT be dead — it is now wired via truncate_bytes action in applyGlobalLimit")
	}

	// Known: Description tag removed — yaml:"description" was parsed but never read
	if foundFields["Description"] {
		t.Errorf("Description should NOT be flagged — yaml tag removed, field is just a struct comment")
	}

	// Known: Name is used everywhere (Filter.Name, Match.Name, etc.) — should NOT be dead
	if foundFields["Name"] {
		t.Errorf("Name should NOT be flagged — it is used extensively via Filter.Name")
	}

	// Known: Version appears in test files and via YAML parser — should NOT be dead
	if foundFields["Version"] {
		t.Errorf("Version should NOT be flagged — used in tests and config")
	}

	// Known: OnError tag removed — yaml:"on_error" was parsed but hardcoded to passthrough
	if foundFields["OnError"] {
		t.Errorf("OnError should NOT be flagged — yaml tag removed, field is just a struct comment")
	}
}

func TestTypeString(t *testing.T) {
	cases := []struct {
		name string
		expr func() ast.Expr
		want string
	}{
		{"ident", func() ast.Expr { return ast.NewIdent("string") }, "string"},
		{"selector", func() ast.Expr {
			return &ast.SelectorExpr{X: ast.NewIdent("time"), Sel: ast.NewIdent("Duration")}
		}, "time.Duration"},
		{"pointer", func() ast.Expr {
			return &ast.StarExpr{X: ast.NewIdent("Config")}
		}, "*Config"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := typeString(tc.expr())
			if got != tc.want {
				t.Errorf("typeString() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFilterFindings(t *testing.T) {
	findings := []Finding{
		{Category: "dead-field", Field: "A"},
		{Category: "append-safety", Field: "B"},
		{Category: "dead-field", Field: "C"},
		{Category: "append-safety", Field: "D"},
	}

	dead := filterFindings(findings, "dead-field")
	if len(dead) != 2 {
		t.Errorf("filterFindings(dead-field) = %d, want 2", len(dead))
	}

	safety := filterFindings(findings, "append-safety")
	if len(safety) != 2 {
		t.Errorf("filterFindings(append-safety) = %d, want 2", len(safety))
	}

	none := filterFindings(findings, "nonexistent")
	if len(none) != 0 {
		t.Errorf("filterFindings(nonexistent) = %d, want 0", len(none))
	}
}

func TestShortenPath(t *testing.T) {
	cases := []struct {
		root, full, want string
	}{
		{"/home/user/snip", "/home/user/snip/internal/config/config.go", "internal/config/config.go"},
		{"/home/user/snip", "/other/path/file.go", "/other/path/file.go"},
		{"/a", "/a/b", "b"},
		{"/a/", "/a/b", "b"},
	}

	for _, tc := range cases {
		got := shortenPath(tc.root, tc.full)
		if got != tc.want {
			t.Errorf("shortenPath(%q, %q) = %q, want %q", tc.root, tc.full, got, tc.want)
		}
	}
}

func TestCollectTaggedFields(t *testing.T) {
	root := "../.."
	fields, err := collectTaggedFields(root)
	if err != nil {
		t.Fatalf("collectTaggedFields: %v", err)
	}

	found := make(map[string]bool)
	for _, f := range fields {
		found[f.Name] = true
	}

	// Must find these known tagged fields
	must := []string{"Mode", "Color", "DBPath", "MaxLines", "MaxOutputBytes", "Name"}
	for _, name := range must {
		if !found[name] {
			t.Errorf("expected to find tagged field %q, but it was not collected", name)
		}
	}

	// Must NOT collect unexported fields
	for _, f := range fields {
		if len(f.Name) > 0 && f.Name[0] >= 'a' && f.Name[0] <= 'z' {
			t.Errorf("unexported field %q should not be collected", f.Name)
		}
	}

	t.Logf("Collected %d tagged fields", len(fields))
}

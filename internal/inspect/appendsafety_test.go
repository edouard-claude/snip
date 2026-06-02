package inspect

import (
	"go/ast"
	"testing"
)

func TestAppendSafety_KnownCases(t *testing.T) {
	root := "../.."

	checker := AppendSafetyChecker{}
	findings, err := checker.Run(root)
	if err != nil {
		t.Fatalf("AppendSafetyChecker.Run: %v", err)
	}

	type fileLine struct {
		file string
		line int
	}
	risky := make(map[fileLine]bool)
	safe := make(map[fileLine]bool)
	for _, f := range findings {
		fl := fileLine{f.File, f.Line}
		if f.Level == "risky" {
			risky[fl] = true
		} else {
			safe[fl] = true
		}
	}

	// Known: config.go:283+284 — was RISKY in PR #70, now uses fresh-slice guard → SAFE
	configFixed := false
	for _, f := range findings {
		if contains(f.File, "config.go") && f.Level == "safe" {
			configFixed = true
			break
		}
	}
	if !configFixed {
		t.Errorf("config.go should be SAFE — fresh-slice guard added to bypass merge")
	}

	// Known: actions.go:252 — was RISKY, now fixed: append(append([]string{}, input.Lines...), ...)
	// The inner append creates a fresh copy so the outer call is no longer shared-state.
	// The checker no longer reports this line at all (neither risky nor safe).
	actions252Gone := true
	for _, f := range findings {
		if contains(f.File, "actions.go") && f.Line == 252 {
			actions252Gone = false
			break
		}
	}
	if !actions252Gone {
		t.Errorf("actions.go:252 should no longer appear in findings — backing array aliasing fixed")
	}

	// Known: actions.go:640 — was RISKY, same fix applied → no longer detected
	actions640Gone := true
	for _, f := range findings {
		if contains(f.File, "actions.go") && f.Line == 640 {
			actions640Gone = false
			break
		}
	}
	if !actions640Gone {
		t.Errorf("actions.go:640 should no longer appear in findings — backing array aliasing fixed")
	}

	t.Logf("Found %d append safety issues (%d risky)", len(findings), len(risky))
}

func TestIsSharedState(t *testing.T) {
	cases := []struct {
		name string
		expr ast.Expr
		want bool
	}{
		{
			name: "selector on struct",
			expr: &ast.SelectorExpr{X: ast.NewIdent("user"), Sel: ast.NewIdent("Commands")},
			want: true,
		},
		{
			name: "selector via index",
			expr: &ast.IndexExpr{X: &ast.SelectorExpr{X: ast.NewIdent("r"), Sel: ast.NewIdent("byKey")}, Index: ast.NewIdent("key")},
			want: true,
		},
		{
			name: "bare ident (local)",
			expr: ast.NewIdent("x"),
			want: false,
		},
		{
			name: "bare ident (param)",
			expr: ast.NewIdent("input"),
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isSharedState(tc.expr)
			if got != tc.want {
				t.Errorf("isSharedState() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHasGuard(t *testing.T) {
	cases := []struct {
		name  string
		code  []string
		line  int
		guard bool
	}{
		{
			name:  "clone guard found",
			code:  []string{"", "", "f = f.Clone()", "", "", "f.Pipeline = append(f.Pipeline, action{"},
			line:  6,
			guard: true,
		},
		{
			name:  "fresh-slice comment",
			code:  []string{"// fresh slice", "", "", "out = append(input.Lines, ...)"},
			line:  4,
			guard: true,
		},
		{
			name:  "make guard",
			code:  []string{"dest := make([]string, 0, len(src))", "", "dest = append(dest, ...)"},
			line:  3,
			guard: true,
		},
		{
			name:  "no guard found",
			code:  []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "x = append(y, z...)"},
			line:  11,
			guard: false,
		},
		{
			name:  "guard too far (beyond window)",
			code:  []string{"f = f.Clone()", "", "", "", "", "", "", "", "", "", "", "append(f.Pipeline, ...)"},
			line:  12,
			guard: false,
		},
		{
			name:  "edge: line 1",
			code:  []string{"append(out, ...)"},
			line:  1,
			guard: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hasGuard(tc.code, tc.line)
			if got != tc.guard {
				t.Errorf("hasGuard() = %v, want %v", got, tc.guard)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

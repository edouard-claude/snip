package inspect

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

type AppendSafetyChecker struct{}

func (a AppendSafetyChecker) Name() string { return "append-safety" }

func (a AppendSafetyChecker) Run(dir string) ([]Finding, error) {
	files, err := findGoFiles(dir, true) // skip test files
	if err != nil {
		return nil, fmt.Errorf("find go files: %w", err)
	}

	var findings []Finding
	fset := token.NewFileSet()

	for _, path := range files {
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			continue
		}

		/* #nosec G304 — reading project source files, not user-controlled paths */
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			ident, ok := call.Fun.(*ast.Ident)
			if !ok || ident.Name != "append" {
				return true
			}

			if len(call.Args) < 2 {
				return true
			}

			firstArg := call.Args[0]
			isShared := isSharedState(firstArg)

			if !isShared {
				return true
			}

			line := fset.Position(call.Pos()).Line
			guarded := hasGuard(lines, line)

			level := "risky"
			if guarded {
				level = "safe"
			}

			code := strings.TrimSpace(snippet(lines, line-1))
			pos := fset.Position(call.Pos())

			findings = append(findings, Finding{
				File:     pos.Filename,
				Line:     pos.Line,
				Category: "append-safety",
				Level:    level,
				Message:  fmt.Sprintf("append() on shared state: %s", code),
				Context:  code,
			})

			return true
		})
	}

	return findings, nil
}

func isSharedState(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		return e.X != nil
	case *ast.Ident:
		return false
	case *ast.IndexExpr:
		return isSharedState(e.X)
	default:
		return false
	}
}

func hasGuard(lines []string, line int) bool {
	start := line - 10
	if start < 0 {
		start = 0
	}
	end := line + 5
	if end > len(lines) {
		end = len(lines)
	}

	for i := start; i < end; i++ {
		l := strings.ToLower(lines[i])
		if strings.Contains(l, "clone()") ||
			strings.Contains(l, "fresh slice") ||
			strings.Contains(l, "safe because") ||
			strings.Contains(l, "make([]") {
			return true
		}
	}
	return false
}

func snippet(lines []string, i int) string {
	if i < 0 || i >= len(lines) {
		return "<unknown>"
	}
	return lines[i]
}

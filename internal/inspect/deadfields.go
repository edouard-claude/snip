package inspect

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type DeadFieldChecker struct{}

func (d DeadFieldChecker) Name() string { return "dead-fields" }

func (d DeadFieldChecker) Run(dir string) ([]Finding, error) {
	structFields, err := collectTaggedFields(dir)
	if err != nil {
		return nil, fmt.Errorf("collect tagged fields: %w", err)
	}

	fieldNames := make(map[string]*structField)
	for i := range structFields {
		fieldNames[structFields[i].Name] = &structFields[i]
	}

	counts, testCounts := countAllFieldRefs(dir, fieldNames)

	var findings []Finding
	for name, sf := range fieldNames {
		behavioral := counts[name] - testCounts[name]
		if behavioral <= 0 {
			findings = append(findings, Finding{
				File:     sf.File,
				Line:     sf.Line,
				Field:    sf.Name,
				Category: "dead-field",
				Level:    "dead",
				Message:  fmt.Sprintf("%s: tagged but never read by behavior code", sf.Name),
			})
		}
	}

	return findings, nil
}

type structField struct {
	File     string
	Line     int
	Name     string
	TypeName string
}

func collectTaggedFields(dir string) ([]structField, error) {
	files, err := findGoFiles(dir, true) // skip test files
	if err != nil {
		return nil, err
	}

	var fields []structField
	fset := token.NewFileSet()

	for _, path := range files {
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			continue
		}

		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, field := range st.Fields.List {
					if field.Tag == nil {
						continue
					}
					tag := field.Tag.Value
					if !strings.Contains(tag, `toml:`) && !strings.Contains(tag, `yaml:`) && !strings.Contains(tag, `json:`) {
						continue
					}
					// Skip embedded/anonymous fields (no names)
					if len(field.Names) == 0 {
						continue
					}
					for _, name := range field.Names {
						if !ast.IsExported(name.Name) {
							continue
						}
						// Skip struct/interface/map/slice types (composition)
						if _, isStruct := field.Type.(*ast.StructType); isStruct {
							continue
						}
						if _, isMap := field.Type.(*ast.MapType); isMap {
							continue
						}
						if _, isArray := field.Type.(*ast.ArrayType); isArray {
							continue
						}
						fields = append(fields, structField{
							File:     path,
							Line:     fset.Position(field.Pos()).Line,
							Name:     name.Name,
							TypeName: typeString(field.Type),
						})
					}
				}
			}
		}
	}

	return fields, nil
}

func typeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return typeString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + typeString(t.X)
	default:
		return "unknown"
	}
}

func countAllFieldRefs(dir string, fieldNames map[string]*structField) (counts, testCounts map[string]int) {
	counts = make(map[string]int)
	testCounts = make(map[string]int)
	for name := range fieldNames {
		counts[name] = 0
		testCounts[name] = 0
	}

	patterns := make(map[string]*regexp.Regexp)
	for name := range fieldNames {
		patterns[name] = regexp.MustCompile(`\.` + regexp.QuoteMeta(name) + `\b`)
	}

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && (info.Name() == ".git" || info.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".go") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		isTest := strings.HasSuffix(info.Name(), "_test.go")
		for name, pat := range patterns {
			matches := pat.FindAll(data, -1)
			counts[name] += len(matches)
			if isTest {
				testCounts[name] += len(matches)
			}
		}
		return nil
	})

	return counts, testCounts
}

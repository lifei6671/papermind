package db

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGORMWhereCallsUseSingleCondition(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob go files: %v", err)
	}

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}

		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, source, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}

		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Where" {
				return true
			}

			start := fset.Position(call.Args[0].Pos()).Offset
			end := fset.Position(call.Args[0].End()).Offset
			if strings.Contains(string(source[start:end]), "AND") {
				t.Errorf("%s: GORM Where must contain only one condition", fset.Position(call.Pos()))
			}
			return true
		})
	}
}

func TestTenantUserRepositoryDoesNotUseJoinQueries(t *testing.T) {
	source, err := os.ReadFile("tenant_user_repository.go")
	if err != nil {
		t.Fatalf("read tenant user repository: %v", err)
	}
	if !strings.Contains(string(source), "tenant_user_memberships AS tum") {
		t.Fatalf("tenant user repository must join tenant_user_memberships for tenant-scoped user queries")
	}
}

func TestAPIListRepositoriesUseDatabasePagination(t *testing.T) {
	cases := []struct {
		file string
		name string
	}{
		{file: "exam_repository.go", name: "ListExams"},
		{file: "tenant_repository.go", name: "List"},
		{file: "space_repository.go", name: "ListSpaces"},
		{file: "tenant_user_repository.go", name: "ListUsers"},
		{file: "question_repository.go", name: "ListVisibleQuestions"},
	}

	for _, item := range cases {
		t.Run(item.file+"."+item.name, func(t *testing.T) {
			source, err := os.ReadFile(item.file)
			if err != nil {
				t.Fatalf("read %s: %v", item.file, err)
			}
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, item.file, source, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", item.file, err)
			}

			body := ""
			for _, decl := range parsed.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Name.Name != item.name || fn.Body == nil {
					continue
				}
				start := fset.Position(fn.Body.Pos()).Offset
				end := fset.Position(fn.Body.End()).Offset
				body = string(source[start:end])
				break
			}
			if body == "" {
				t.Fatalf("function %s not found in %s", item.name, item.file)
			}
			for _, required := range []string{"Count(", "Limit(", "Offset("} {
				if !strings.Contains(body, required) {
					t.Fatalf("%s must call %s before returning API list data", item.name, required)
				}
			}
		})
	}
}

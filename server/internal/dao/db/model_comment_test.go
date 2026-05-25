package db

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

func TestModelFieldsHaveChineseComments(t *testing.T) {
	root := "."
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		for _, decl := range parsed.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !isDatabaseModelStruct(typeSpec.Name.Name) {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, field := range structType.Fields.List {
					if !fieldNeedsComment(field) {
						continue
					}
					if !hasChineseComment(field) {
						position := fset.Position(field.Pos())
						t.Fatalf("%s.%s missing Chinese field comment at %s", typeSpec.Name.Name, fieldName(field), position)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir() error = %v", err)
	}
}

func isDatabaseModelStruct(name string) bool {
	switch name {
	case "BaseFields", "SoftDeleteFields", "RelationFields", "EventFields":
		return true
	default:
		return strings.HasSuffix(name, "DO")
	}
}

func fieldNeedsComment(field *ast.Field) bool {
	if len(field.Names) == 0 {
		return true
	}
	for _, name := range field.Names {
		if name.IsExported() {
			return true
		}
	}
	return false
}

func hasChineseComment(field *ast.Field) bool {
	comment := ""
	if field.Doc != nil {
		comment += field.Doc.Text()
	}
	if field.Comment != nil {
		comment += field.Comment.Text()
	}
	for _, r := range comment {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func fieldName(field *ast.Field) string {
	if len(field.Names) == 0 {
		return "embedded"
	}
	names := make([]string, 0, len(field.Names))
	for _, name := range field.Names {
		names = append(names, name.Name)
	}
	return strings.Join(names, ",")
}

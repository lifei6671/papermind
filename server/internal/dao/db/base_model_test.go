package db

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gorm.io/datatypes"
	"gorm.io/plugin/soft_delete"
)

func TestBaseFieldsUseExplicitColumnMapping(t *testing.T) {
	modelType := reflect.TypeOf(BaseFields{})
	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)
		if field.Tag.Get("gorm") == "" {
			t.Fatalf("%s missing gorm tag", field.Name)
		}
		if !strings.Contains(field.Tag.Get("gorm"), "column:") {
			t.Fatalf("%s gorm tag = %q, want explicit column mapping", field.Name, field.Tag.Get("gorm"))
		}
	}
}

func TestBaseFieldsUseDatatypesJSONForExtJSON(t *testing.T) {
	field, ok := reflect.TypeOf(BaseFields{}).FieldByName("ExtJSON")
	if !ok {
		t.Fatalf("ExtJSON field missing")
	}
	if field.Type != reflect.TypeOf(datatypes.JSON{}) {
		t.Fatalf("ExtJSON type = %s", field.Type.String())
	}
	if !strings.Contains(field.Tag.Get("gorm"), "column:ext_json") {
		t.Fatalf("ExtJSON gorm tag = %q", field.Tag.Get("gorm"))
	}
}

func TestSoftDeleteFieldsUseUnixSoftDeletePlugin(t *testing.T) {
	field, ok := reflect.TypeOf(SoftDeleteFields{}).FieldByName("DeletedAt")
	if !ok {
		t.Fatalf("DeletedAt field missing")
	}
	if field.Type != reflect.TypeOf(soft_delete.DeletedAt(0)) {
		t.Fatalf("DeletedAt type = %s", field.Type.String())
	}
	if !strings.Contains(field.Tag.Get("gorm"), "softDelete:milli") {
		t.Fatalf("DeletedAt gorm tag = %q", field.Tag.Get("gorm"))
	}
}

func TestRelationFieldsOnlyKeepCreateAuditAndExtJSON(t *testing.T) {
	modelType := reflect.TypeOf(RelationFields{})
	assertHasField(t, modelType, "ID")
	assertHasField(t, modelType, "CreatedAt")
	assertHasField(t, modelType, "CreatedBy")
	assertHasField(t, modelType, "ExtJSON")
	assertMissingField(t, modelType, "UpdatedAt")
	assertMissingField(t, modelType, "UpdatedBy")
	assertMissingField(t, modelType, "Version")
}

func TestEventFieldsOnlyKeepCreateAuditAndExtJSON(t *testing.T) {
	modelType := reflect.TypeOf(EventFields{})
	assertHasField(t, modelType, "ID")
	assertHasField(t, modelType, "CreatedAt")
	assertHasField(t, modelType, "CreatedBy")
	assertHasField(t, modelType, "ExtJSON")
	assertMissingField(t, modelType, "UpdatedAt")
	assertMissingField(t, modelType, "UpdatedBy")
	assertMissingField(t, modelType, "Version")
}

func TestBaseColumnMappingsStayWithBaseEntity(t *testing.T) {
	if BaseColumns.ID != "id" {
		t.Fatalf("BaseColumns.ID = %q", BaseColumns.ID)
	}
	if BaseColumns.ExtJSON != "ext_json" {
		t.Fatalf("BaseColumns.ExtJSON = %q", BaseColumns.ExtJSON)
	}
	if SoftDeleteColumns.DeletedAt != "deleted_at" {
		t.Fatalf("SoftDeleteColumns.DeletedAt = %q", SoftDeleteColumns.DeletedAt)
	}
	if RelationColumns.CreatedBy != "created_by" {
		t.Fatalf("RelationColumns.CreatedBy = %q", RelationColumns.CreatedBy)
	}
	if EventColumns.ExtJSON != "ext_json" {
		t.Fatalf("EventColumns.ExtJSON = %q", EventColumns.ExtJSON)
	}
}

func TestServiceLayerDoesNotImportGORM(t *testing.T) {
	root := filepath.Join("..", "..", "service")
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(file, ".go") {
			return nil
		}

		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, item := range parsed.Imports {
			importPath := strings.Trim(item.Path.Value, `"`)
			if strings.HasPrefix(importPath, "gorm.io/") {
				t.Fatalf("service file %s imports %s", file, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir() error = %v", err)
	}
}

func assertHasField(t *testing.T, modelType reflect.Type, fieldName string) {
	t.Helper()

	field, ok := modelType.FieldByName(fieldName)
	if !ok {
		t.Fatalf("%s field missing", fieldName)
	}
	if !strings.Contains(field.Tag.Get("gorm"), "column:") {
		t.Fatalf("%s gorm tag = %q, want explicit column mapping", fieldName, field.Tag.Get("gorm"))
	}
}

func assertMissingField(t *testing.T, modelType reflect.Type, fieldName string) {
	t.Helper()

	if _, ok := modelType.FieldByName(fieldName); ok {
		t.Fatalf("%s field should be omitted", fieldName)
	}
}

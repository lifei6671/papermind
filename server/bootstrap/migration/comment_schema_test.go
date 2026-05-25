package migration

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

func TestPostgreSQLSchemaTablesAndColumnsHaveChineseComments(t *testing.T) {
	path := filepath.Join("..", "..", "data", "migrations", "postgres", "001_tenant_space.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	sql := string(content)
	tableBlocks := regexp.MustCompile(`(?s)CREATE TABLE IF NOT EXISTS ([a-z_]+) \((.*?)\);`).FindAllStringSubmatch(sql, -1)
	if len(tableBlocks) == 0 {
		t.Fatalf("%s has no PostgreSQL table blocks", path)
	}

	for _, block := range tableBlocks {
		tableName := block[1]
		if !hasPostgresComment(sql, "TABLE", tableName) {
			t.Fatalf("%s missing Chinese COMMENT ON TABLE", tableName)
		}
		for _, column := range postgresColumns(block[2]) {
			if !hasPostgresComment(sql, "COLUMN", tableName+"."+column) {
				t.Fatalf("%s.%s missing Chinese COMMENT ON COLUMN", tableName, column)
			}
		}
	}
}

func TestSQLiteSchemaTablesHaveChineseComments(t *testing.T) {
	path := filepath.Join("..", "..", "data", "migrations", "sqlite", "001_tenant_space.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	sql := string(content)
	tableNames := regexp.MustCompile(`CREATE TABLE IF NOT EXISTS ([a-z_]+)`).FindAllStringSubmatch(sql, -1)
	if len(tableNames) == 0 {
		t.Fatalf("%s has no SQLite table blocks", path)
	}

	for _, match := range tableNames {
		tableName := match[1]
		commentPattern := regexp.MustCompile(`--\s*` + regexp.QuoteMeta(tableName) + `[:：][^\n]*`)
		comment := commentPattern.FindString(sql)
		if !containsChineseComment(comment) {
			t.Fatalf("%s missing Chinese -- table comment", tableName)
		}
	}
}

func postgresColumns(tableBody string) []string {
	columns := make([]string, 0)
	for _, line := range strings.Split(tableBody, "\n") {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, ","))
		if !isColumnDefinition(trimmed) {
			continue
		}
		name := strings.Fields(trimmed)[0]
		columns = append(columns, name)
	}
	return columns
}

func hasPostgresComment(sql string, commentType string, target string) bool {
	pattern := regexp.MustCompile(`COMMENT ON ` + commentType + ` ` + regexp.QuoteMeta(target) + ` IS '([^']+)'`)
	match := pattern.FindStringSubmatch(sql)
	return len(match) == 2 && containsChineseComment(match[1])
}

func containsChineseComment(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

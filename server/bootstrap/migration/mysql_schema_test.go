package migration

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

func TestMySQLSchemaUsesInnoDBAndUTF8MB4ForEveryTable(t *testing.T) {
	path := filepath.Join("..", "..", "data", "migrations", "mysql", "001_tenant_space.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	sql := string(content)
	tableNames := regexp.MustCompile(`CREATE TABLE IF NOT EXISTS ([a-z_]+)`).FindAllStringSubmatch(sql, -1)
	if len(tableNames) == 0 {
		t.Fatalf("%s has no CREATE TABLE statements", path)
	}

	for _, match := range tableNames {
		tableName := match[1]
		blockStart := strings.Index(sql, "CREATE TABLE IF NOT EXISTS "+tableName+" ")
		if blockStart < 0 {
			t.Fatalf("%s create table block missing", tableName)
		}
		blockEnd := strings.Index(sql[blockStart:], ";\n")
		if blockEnd < 0 {
			t.Fatalf("%s create table block terminator missing", tableName)
		}

		block := sql[blockStart : blockStart+blockEnd]
		if !strings.Contains(block, ") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='") {
			t.Fatalf("%s missing ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 table options", tableName)
		}
	}
}

func TestMySQLSchemaColumnsHaveChineseComments(t *testing.T) {
	path := filepath.Join("..", "..", "data", "migrations", "mysql", "001_tenant_space.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	sql := string(content)
	tableBlocks := regexp.MustCompile(`(?s)CREATE TABLE IF NOT EXISTS ([a-z_]+) \((.*?)\) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='([^']+)'`).FindAllStringSubmatch(sql, -1)
	if len(tableBlocks) == 0 {
		t.Fatalf("%s has no MySQL table blocks", path)
	}

	for _, block := range tableBlocks {
		tableName := block[1]
		tableComment := block[3]
		if !containsHan(tableComment) {
			t.Fatalf("%s table comment should contain Chinese", tableName)
		}

		for _, line := range strings.Split(block[2], "\n") {
			trimmed := strings.TrimSpace(line)
			if !isColumnDefinition(trimmed) {
				continue
			}
			commentStart := strings.Index(trimmed, "COMMENT '")
			if commentStart < 0 {
				t.Fatalf("%s column definition missing COMMENT: %s", tableName, trimmed)
			}
			commentText := trimmed[commentStart+len("COMMENT '"):]
			if !containsHan(commentText) {
				t.Fatalf("%s column comment should contain Chinese: %s", tableName, trimmed)
			}
		}
	}
}

func isColumnDefinition(line string) bool {
	if line == "" {
		return false
	}
	upper := strings.ToUpper(line)
	return !strings.HasPrefix(upper, "PRIMARY KEY") &&
		!strings.HasPrefix(upper, "UNIQUE KEY") &&
		!strings.HasPrefix(upper, "KEY ") &&
		!strings.HasPrefix(upper, "CONSTRAINT ")
}

func containsHan(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

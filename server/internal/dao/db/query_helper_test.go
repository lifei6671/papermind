package db

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLikeIgnoreCaseAdaptsToDialector(t *testing.T) {
	tests := []struct {
		name        string
		dialector   gorm.Dialector
		wantSQLPart string
		notSQLPart  string
	}{
		{
			name: "postgres uses ilike",
			dialector: postgres.New(postgres.Config{
				DSN: "host=127.0.0.1 user=papermind dbname=papermind sslmode=disable",
			}),
			wantSQLPart: "exams.name ILIKE",
			notSQLPart:  "LOWER(exams.name)",
		},
		{
			name: "mysql uses collation aware like",
			dialector: mysql.New(mysql.Config{
				DSN:                       "papermind:papermind@tcp(127.0.0.1:3306)/papermind?parseTime=true",
				SkipInitializeWithVersion: true,
			}),
			wantSQLPart: "exams.name LIKE",
			notSQLPart:  "LOWER(exams.name)",
		},
		{
			name:        "sqlite uses ascii case insensitive like",
			dialector:   sqlite.Open(":memory:"),
			wantSQLPart: "exams.name LIKE",
			notSQLPart:  "LOWER(exams.name)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gormDB, err := gorm.Open(tt.dialector, &gorm.Config{
				DryRun:               true,
				DisableAutomaticPing: true,
			})
			if err != nil {
				t.Fatalf("open dry-run db: %v", err)
			}

			var rows []ExamDO
			stmt := LikeIgnoreCase(gormDB.Model(&ExamDO{}), "exams.name", "Hello").Find(&rows).Statement
			sql := stmt.SQL.String()
			if !strings.Contains(sql, tt.wantSQLPart) {
				t.Fatalf("SQL = %q, want contains %q", sql, tt.wantSQLPart)
			}
			if strings.Contains(sql, tt.notSQLPart) {
				t.Fatalf("SQL = %q, want not contains %q", sql, tt.notSQLPart)
			}
			if len(stmt.Vars) == 0 || stmt.Vars[0] != "%Hello%" {
				t.Fatalf("Vars = %#v, want first var %%Hello%%", stmt.Vars)
			}
		})
	}
}

func TestRepositoriesUseLikeIgnoreCaseHelper(t *testing.T) {
	disallowed := regexp.MustCompile(`(?is)LOWER\s*\([^)]*\)\s+LIKE`)
	var offenders []string
	err := filepath.WalkDir(".", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		if path == "query_helper.go" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if disallowed.Match(content) {
			offenders = append(offenders, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan repository files: %v", err)
	}
	if len(offenders) > 0 {
		t.Fatalf("repositories should use LikeIgnoreCase helper instead of direct LOWER LIKE: %s", strings.Join(offenders, ", "))
	}
}

package v1

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseQuestionImportCSVSupportsQuotedMarkdownTitle(t *testing.T) {
	csvContent := "题型,题干,选项,正确答案,标准答案,参考答案,题目解析,难度,标签\n" +
		"简答题,\"### 材料\n\n    SELECT * FROM users;\n    WHERE id = 1;\",,,,说明 SQL 作用,Markdown 解析,中等,数据库"
	body, contentType := buildQuestionImportMultipart(t, csvContent)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/questions/import", body)
	request.Header.Set("Content-Type", contentType)
	if err := request.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("ParseMultipartForm returned error: %v", err)
	}

	fileHeaders := request.MultipartForm.File["file"]
	if len(fileHeaders) != 1 {
		t.Fatalf("expected one uploaded file, got %d", len(fileHeaders))
	}

	rows, rowErrors, err := parseQuestionImportCSV(fileHeaders[0], []string{"题型", "题干", "选项", "正确答案", "标准答案", "参考答案", "题目解析", "难度", "标签"})
	if err != nil {
		t.Fatalf("parseQuestionImportCSV returned error: %v", err)
	}
	if len(rowErrors) != 0 {
		t.Fatalf("expected no row errors, got %#v", rowErrors)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one parsed row, got %d", len(rows))
	}
	wantTitle := "### 材料\n\n    SELECT * FROM users;\n    WHERE id = 1;"
	if rows[0].Title != wantTitle {
		t.Fatalf("expected markdown title preserved, got %q", rows[0].Title)
	}
}

package response

import (
	"encoding/json"
	"testing"
)

func TestOKBuildsUnifiedResponse(t *testing.T) {
	resp := OK(map[string]string{"id": "1"})

	if resp.Code != 0 {
		t.Fatalf("Code = %d", resp.Code)
	}
	if resp.Message != "ok" {
		t.Fatalf("Message = %q", resp.Message)
	}
	if resp.Data == nil {
		t.Fatalf("Data = nil")
	}
}

func TestPageBuildsPaginationResponse(t *testing.T) {
	page := Page([]string{"a", "b"}, 2, 20, 41)

	if len(page.Items) != 2 {
		t.Fatalf("Items length = %d", len(page.Items))
	}
	if page.Page != 2 {
		t.Fatalf("Page = %d", page.Page)
	}
	if page.PageSize != 20 {
		t.Fatalf("PageSize = %d", page.PageSize)
	}
	if page.Total != 41 {
		t.Fatalf("Total = %d", page.Total)
	}
}

func TestResponseJSONFieldNamesAreStable(t *testing.T) {
	body, err := json.Marshal(OK("pong"))
	if err != nil {
		t.Fatalf("Marshal(OK) error = %v", err)
	}
	if string(body) != `{"code":0,"message":"ok","data":"pong"}` {
		t.Fatalf("OK JSON = %s", string(body))
	}

	page, err := json.Marshal(Page([]string{"a"}, 1, 10, 1))
	if err != nil {
		t.Fatalf("Marshal(Page) error = %v", err)
	}
	if string(page) != `{"items":["a"],"page":1,"page_size":10,"total":1}` {
		t.Fatalf("Page JSON = %s", string(page))
	}
}

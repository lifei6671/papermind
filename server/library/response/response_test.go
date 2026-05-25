package response

import "testing"

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

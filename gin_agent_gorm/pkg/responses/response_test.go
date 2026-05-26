package responses

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"gin-biz-web-api/pkg/paginator"
)

func TestToResponseUsesGlobalEnvelope(t *testing.T) {
	w := performResponse(func(r *Response) {
		r.ToResponse(gin.H{"id": 1})
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var payload struct {
		Code int            `json:"code"`
		Msg  string         `json:"msg"`
		Data map[string]int `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if payload.Code != http.StatusOK {
		t.Fatalf("code = %d, want %d", payload.Code, http.StatusOK)
	}
	if payload.Msg != "请求成功" {
		t.Fatalf("msg = %q, want %q", payload.Msg, "请求成功")
	}
	if payload.Data["id"] != 1 {
		t.Fatalf("data.id = %d, want %d", payload.Data["id"], 1)
	}
}

func TestToResponseWithPaginationUsesFlatPageData(t *testing.T) {
	w := performResponse(func(r *Response) {
		r.ToResponseWithPagination([]string{}, paginator.Pagination{
			Page:       1,
			PerPage:    10,
			TotalCount: 0,
			TotalPage:  0,
		})
	})

	var payload struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			List       []string `json:"list"`
			Page       int      `json:"page"`
			PageSize   int      `json:"page_size"`
			Total      int      `json:"total"`
			TotalPages int      `json:"total_pages"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if payload.Code != http.StatusOK {
		t.Fatalf("code = %d, want %d", payload.Code, http.StatusOK)
	}
	if payload.Msg != "查询成功" {
		t.Fatalf("msg = %q, want %q", payload.Msg, "查询成功")
	}
	if len(payload.Data.List) != 0 {
		t.Fatalf("list length = %d, want 0", len(payload.Data.List))
	}
	if payload.Data.Page != 1 || payload.Data.PageSize != 10 || payload.Data.Total != 0 || payload.Data.TotalPages != 0 {
		t.Fatalf("unexpected pagination data: %+v", payload.Data)
	}
}

func performResponse(fn func(*Response)) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	fn(New(c))
	return w
}

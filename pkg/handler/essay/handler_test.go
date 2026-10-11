package essay

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anzhiyu-c/anheyu-app/ent/enttest"
	ent_impl "github.com/anzhiyu-c/anheyu-app/internal/infra/persistence/ent"
	essay_service "github.com/anzhiyu-c/anheyu-app/pkg/service/essay"
	"github.com/gin-gonic/gin"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	client := enttest.Open(t, "sqlite3", "file:essay_handler_"+name+"?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { _ = client.Close() })

	h := NewHandler(essay_service.NewService(ent_impl.NewEssayRepo(client)))
	r := gin.New()
	r.GET("/api/public/essays", h.ListPublic)
	r.GET("/api/essays", h.ListAdmin)
	r.GET("/api/essays/:id", h.Get)
	r.POST("/api/essays", h.Create)
	r.PUT("/api/essays/:id", h.Update)
	r.DELETE("/api/essays/:id", h.Delete)
	return r
}

func do(t *testing.T, r *gin.Engine, method, path, body string) (int, apiResp) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp apiResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json %q: %v", w.Body.String(), err)
	}
	return w.Code, resp
}

func TestCreateValidationReturns400(t *testing.T) {
	r := newTestRouter(t)
	code, resp := do(t, r, http.MethodPost, "/api/essays", `{"content":"  "}`)
	if code != http.StatusBadRequest || !strings.Contains(resp.Message, "至少填写一项") {
		t.Fatalf("code=%d msg=%q", code, resp.Message)
	}
}

func TestCreateThenGet(t *testing.T) {
	r := newTestRouter(t)
	code, resp := do(t, r, http.MethodPost, "/api/essays", `{"content":"海边","images":["https://a/b.jpg"]}`)
	if code != http.StatusOK {
		t.Fatalf("create code=%d msg=%q", code, resp.Message)
	}
	var created struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(resp.Data, &created); err != nil || created.ID == 0 {
		t.Fatalf("bad create data %s: %v", resp.Data, err)
	}

	code, resp = do(t, r, http.MethodGet, "/api/essays/1", "")
	if code != http.StatusOK || !strings.Contains(string(resp.Data), "海边") {
		t.Fatalf("get code=%d data=%s", code, resp.Data)
	}
}

func TestPublicListHidesDrafts(t *testing.T) {
	r := newTestRouter(t)
	do(t, r, http.MethodPost, "/api/essays", `{"content":"公开"}`)
	do(t, r, http.MethodPost, "/api/essays", `{"content":"草稿","is_published":false}`)

	code, resp := do(t, r, http.MethodGet, "/api/public/essays?page=1&pageSize=10", "")
	if code != http.StatusOK {
		t.Fatalf("code=%d", code)
	}
	var list struct {
		Total int64 `json:"total"`
	}
	_ = json.Unmarshal(resp.Data, &list)
	if list.Total != 1 || strings.Contains(string(resp.Data), "草稿") {
		t.Fatalf("public list leaked drafts: %s", resp.Data)
	}
}

func TestStatusCodeMapping(t *testing.T) {
	r := newTestRouter(t)
	if code, _ := do(t, r, http.MethodPut, "/api/essays/999", `{"content":"x"}`); code != http.StatusNotFound {
		t.Fatalf("update missing: code=%d", code)
	}
	if code, _ := do(t, r, http.MethodDelete, "/api/essays/abc", ""); code != http.StatusBadRequest {
		t.Fatalf("bad id: code=%d", code)
	}
	if code, _ := do(t, r, http.MethodPost, "/api/essays", `not-json`); code != http.StatusBadRequest {
		t.Fatalf("bad json: code=%d", code)
	}

	do(t, r, http.MethodPost, "/api/essays", `{"content":"x"}`)
	if code, _ := do(t, r, http.MethodDelete, "/api/essays/1", ""); code != http.StatusOK {
		t.Fatalf("delete: code=%d", code)
	}
	if code, _ := do(t, r, http.MethodGet, "/api/essays/1", ""); code != http.StatusNotFound {
		t.Fatalf("get deleted: code=%d", code)
	}
}

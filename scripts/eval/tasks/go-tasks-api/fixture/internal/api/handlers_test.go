package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/tasks/internal/store"
)

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCreateAndGet(t *testing.T) {
	h := NewHandler(store.NewMemory())
	rec := do(t, h, "POST", "/tasks", `{"title":"  write tests ","tags":["dev"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var task store.Task
	json.Unmarshal(rec.Body.Bytes(), &task)
	if task.Title != "write tests" || task.ID != 1 {
		t.Fatalf("task %+v", task)
	}
	if rec := do(t, h, "GET", "/tasks/1", ""); rec.Code != http.StatusOK {
		t.Fatalf("get: %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/tasks/9", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("get missing: %d", rec.Code)
	}
}

func TestValidation(t *testing.T) {
	h := NewHandler(store.NewMemory())
	for _, body := range []string{`{"title":""}`, `{"title":"` + strings.Repeat("x", 201) + `"}`, `{"title":"a","extra":1}`, `nope`} {
		if rec := do(t, h, "POST", "/tasks", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", body, rec.Code)
		}
	}
	if rec := do(t, h, "GET", "/tasks/abc", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("bad id: %d", rec.Code)
	}
}

func TestPatchAndDelete(t *testing.T) {
	h := NewHandler(store.NewMemory())
	do(t, h, "POST", "/tasks", `{"title":"a"}`)
	rec := do(t, h, "PATCH", "/tasks/1", `{"done":true}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"done":true`) {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, h, "DELETE", "/tasks/1", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
}

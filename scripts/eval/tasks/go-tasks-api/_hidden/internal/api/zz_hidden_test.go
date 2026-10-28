package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"example.com/tasks/internal/api"
	"example.com/tasks/internal/store"
)

type page struct {
	Tasks      []store.Task `json:"tasks"`
	NextCursor string       `json:"next_cursor"`
}

func hreq(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func seed(t *testing.T, n int) http.Handler {
	t.Helper()
	h := api.NewHandler(store.NewMemory())
	for i := 1; i <= n; i++ {
		tags := `[]`
		if i%3 == 0 {
			tags = `["red"]`
		}
		if rec := hreq(t, h, "POST", "/tasks", fmt.Sprintf(`{"title":"t%d","tags":%s}`, i, tags)); rec.Code != http.StatusCreated {
			t.Fatalf("seed: %d %s", rec.Code, rec.Body)
		}
		if i%2 == 0 {
			hreq(t, h, "PATCH", fmt.Sprintf("/tasks/%d", i), `{"done":true}`)
		}
	}
	return h
}

func getPage(t *testing.T, h http.Handler, query string) page {
	t.Helper()
	rec := hreq(t, h, "GET", "/tasks"+query, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /tasks%s: %d %s", query, rec.Code, rec.Body)
	}
	var p page
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("GET /tasks%s: body is not {tasks, next_cursor}: %v: %s", query, err, rec.Body)
	}
	return p
}

func ids(p page) []int {
	out := make([]int, len(p.Tasks))
	for i, t := range p.Tasks {
		out[i] = t.ID
	}
	return out
}

func walk(t *testing.T, h http.Handler, query string, between func()) []int {
	t.Helper()
	var all []int
	cursor := ""
	for n := 0; n < 50; n++ {
		q := query
		if cursor != "" {
			sep := "?"
			if strings.Contains(q, "?") {
				sep = "&"
			}
			q += sep + "cursor=" + url.QueryEscape(cursor)
		}
		p := getPage(t, h, q)
		all = append(all, ids(p)...)
		if p.NextCursor == "" {
			return all
		}
		cursor = p.NextCursor
		if between != nil {
			between()
			between = nil
		}
	}
	t.Fatal("pagination never ended")
	return nil
}

func TestHiddenDefaultLimitAndLastPage(t *testing.T) {
	h := seed(t, 25)
	p := getPage(t, h, "")
	if len(p.Tasks) != 20 || p.NextCursor == "" {
		t.Fatalf("first page: %d tasks, next_cursor %q; want 20 and a cursor", len(p.Tasks), p.NextCursor)
	}
	if ids(p)[0] != 1 || ids(p)[19] != 20 {
		t.Fatalf("first page ids %v", ids(p))
	}
	last := getPage(t, h, "?cursor="+url.QueryEscape(p.NextCursor))
	if got := ids(last); len(got) != 5 || got[0] != 21 || last.NextCursor != "" {
		t.Fatalf("last page ids %v next %q", got, last.NextCursor)
	}
}

func TestHiddenWalkCoversEverythingOnce(t *testing.T) {
	h := seed(t, 23)
	got := walk(t, h, "?limit=7", nil)
	if len(got) != 23 {
		t.Fatalf("walk returned %d tasks: %v", len(got), got)
	}
	for i, id := range got {
		if id != i+1 {
			t.Fatalf("walk order %v", got)
		}
	}
}

func TestHiddenDeletesBetweenPagesSkipNothing(t *testing.T) {
	h := seed(t, 30)
	got := walk(t, h, "?limit=10", func() {
		// Delete tasks already seen; an offset cursor would now skip three.
		for _, id := range []int{2, 4, 6} {
			hreq(t, h, "DELETE", fmt.Sprintf("/tasks/%d", id), "")
		}
	})
	if len(got) != 30 {
		t.Fatalf("deleting seen tasks changed what later pages return: got %d ids %v", len(got), got)
	}
}

func TestHiddenFilters(t *testing.T) {
	h := seed(t, 12)
	for _, tc := range []struct {
		q    string
		want []int
	}{
		{"?done=true", []int{2, 4, 6, 8, 10, 12}},
		{"?done=false", []int{1, 3, 5, 7, 9, 11}},
		{"?tag=red", []int{3, 6, 9, 12}},
		{"?tag=red&done=true", []int{6, 12}},
	} {
		if got := walk(t, h, tc.q, nil); fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("%s: %v, want %v", tc.q, got, tc.want)
		}
	}
	if got := walk(t, h, "?tag=red&limit=1", nil); fmt.Sprint(got) != "[3 6 9 12]" {
		t.Errorf("filtered walk with limit 1: %v", got)
	}
}

func TestHiddenLimitBounds(t *testing.T) {
	h := seed(t, 120)
	if p := getPage(t, h, "?limit=100"); len(p.Tasks) != 100 {
		t.Fatalf("limit=100 gave %d", len(p.Tasks))
	}
	if p := getPage(t, h, "?limit=1"); len(p.Tasks) != 1 || p.NextCursor == "" {
		t.Fatalf("limit=1 gave %d, next %q", len(p.Tasks), p.NextCursor)
	}
}

func TestHiddenInvalidParameters(t *testing.T) {
	h := seed(t, 5)
	for _, q := range []string{"?limit=0", "?limit=101", "?limit=-3", "?limit=ten", "?done=maybe", "?cursor=zzzz"} {
		rec := hreq(t, h, "GET", "/tasks"+q, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", q, rec.Code)
			continue
		}
		var body map[string]string
		if json.Unmarshal(rec.Body.Bytes(), &body) != nil || body["error"] == "" {
			t.Errorf("%s: body %s, want {\"error\": ...}", q, rec.Body)
		}
	}
}

func TestHiddenEmptyList(t *testing.T) {
	h := api.NewHandler(store.NewMemory())
	p := getPage(t, h, "")
	if p.Tasks == nil {
		// "tasks": null is not a list; clients iterate over it.
		t.Fatalf("empty store should give \"tasks\": [], got null")
	}
	if len(p.Tasks) != 0 || p.NextCursor != "" {
		t.Fatalf("empty store: %+v", p)
	}
}

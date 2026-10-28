package api

// Reference solution for the eval's oracle check only; it is never shown to
// the agent. check-reference.sh replaces handler.list with this.

import (
	"encoding/base64"
	"net/http"
	"slices"
	"strconv"

	"example.com/tasks/internal/store"
)

type listPage struct {
	Tasks      []store.Task `json:"tasks"`
	NextCursor string       `json:"next_cursor"`
}

func (h *handler) listPaged(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 20
	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 100 {
			writeError(w, http.StatusBadRequest, "limit must be 1 to 100")
			return
		}
		limit = n
	}
	after := 0
	if s := q.Get("cursor"); s != "" {
		raw, err := base64.RawURLEncoding.DecodeString(s)
		n, err2 := strconv.Atoi(string(raw))
		if err != nil || err2 != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		after = n
	}
	var done *bool
	if s := q.Get("done"); s != "" {
		b, err := strconv.ParseBool(s)
		if err != nil || (s != "true" && s != "false") {
			writeError(w, http.StatusBadRequest, "done must be true or false")
			return
		}
		done = &b
	}
	tag := q.Get("tag")
	all, err := h.store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := listPage{Tasks: []store.Task{}}
	for _, t := range all {
		if t.ID <= after || (done != nil && t.Done != *done) || (tag != "" && !slices.Contains(t.Tags, tag)) {
			continue
		}
		if len(out.Tasks) == limit {
			out.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(out.Tasks[limit-1].ID)))
			break
		}
		out.Tasks = append(out.Tasks, t)
	}
	writeJSON(w, http.StatusOK, out)
}

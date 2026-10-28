// Package api is the HTTP interface to the store.
package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"example.com/tasks/internal/store"
)

const maxTitle = 200

type handler struct {
	store store.Store
}

// NewHandler routes the API onto s.
func NewHandler(s store.Store) http.Handler {
	h := &handler{store: s}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tasks", h.list)
	mux.HandleFunc("POST /tasks", h.create)
	mux.HandleFunc("GET /tasks/{id}", h.get)
	mux.HandleFunc("PATCH /tasks/{id}", h.update)
	mux.HandleFunc("DELETE /tasks/{id}", h.delete)
	return mux
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

type createRequest struct {
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if msg := validTitle(req.Title); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	t, err := h.store.Create(strings.TrimSpace(req.Title), req.Tags)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	t, err := h.store.Get(id)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var p store.Patch
	if err := decode(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if p.Title != nil {
		if msg := validTitle(*p.Title); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		title := strings.TrimSpace(*p.Title)
		p.Title = &title
	}
	t, err := h.store.Update(id, p)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.store.Delete(id); err != nil {
		storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "id must be a positive integer")
		return 0, false
	}
	return id, true
}

func validTitle(title string) string {
	n := utf8.RuneCountInString(strings.TrimSpace(title))
	if n == 0 || n > maxTitle {
		return "title must be 1 to 200 characters"
	}
	return ""
}

func storeError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

// Package store keeps tasks.
package store

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// ErrNotFound is returned for an id that does not exist.
var ErrNotFound = errors.New("task not found")

// Task is one to-do item.
type Task struct {
	ID      int       `json:"id"`
	Title   string    `json:"title"`
	Done    bool      `json:"done"`
	Tags    []string  `json:"tags"`
	Created time.Time `json:"created"`
}

// Patch is a partial update; nil fields are left as they are.
type Patch struct {
	Title *string   `json:"title"`
	Done  *bool     `json:"done"`
	Tags  *[]string `json:"tags"`
}

// Store is where tasks live.
type Store interface {
	Create(title string, tags []string) (Task, error)
	Get(id int) (Task, error)
	// List returns every task in ascending id order.
	List() ([]Task, error)
	Update(id int, p Patch) (Task, error)
	Delete(id int) error
}

// Memory is a Store held in memory, safe for concurrent use.
type Memory struct {
	mu    sync.Mutex
	next  int
	tasks map[int]Task
	now   func() time.Time
}

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{tasks: map[int]Task{}, now: time.Now}
}

func (m *Memory) Create(title string, tags []string) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.next++
	t := Task{ID: m.next, Title: title, Tags: cloneTags(tags), Created: m.now().UTC()}
	m.tasks[t.ID] = t
	return t, nil
}

func (m *Memory) Get(id int) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	return t, nil
}

func (m *Memory) List() ([]Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *Memory) Update(id int, p Patch) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	if p.Title != nil {
		t.Title = *p.Title
	}
	if p.Done != nil {
		t.Done = *p.Done
	}
	if p.Tags != nil {
		t.Tags = cloneTags(*p.Tags)
	}
	m.tasks[id] = t
	return t, nil
}

func (m *Memory) Delete(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tasks[id]; !ok {
		return ErrNotFound
	}
	delete(m.tasks, id)
	return nil
}

func cloneTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return append([]string(nil), tags...)
}

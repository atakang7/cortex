package store

import (
	"errors"
	"testing"
)

func TestMemoryLifecycle(t *testing.T) {
	m := NewMemory()
	a, _ := m.Create("a", nil)
	b, _ := m.Create("b", []string{"x"})
	if a.ID != 1 || b.ID != 2 {
		t.Fatalf("ids %d %d", a.ID, b.ID)
	}
	done := true
	if got, err := m.Update(a.ID, Patch{Done: &done}); err != nil || !got.Done {
		t.Fatalf("update: %+v %v", got, err)
	}
	if err := m.Delete(b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Get(b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get deleted: %v", err)
	}
	list, _ := m.List()
	if len(list) != 1 || list[0].ID != a.ID {
		t.Fatalf("list %+v", list)
	}
}

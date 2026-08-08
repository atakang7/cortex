package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/atakang7/axon"
)

// tracelog.go is the third event sink, alongside Bridge (the live TUI) and
// Plain (the non-interactive renderer). Where those two curate — deciding
// what a human should see mid-session — TraceLog does not: every axon.Event
// that reaches it is written, unfiltered, as one JSON line. It exists so
// "what did the runtime actually do" never depends on remembering to
// reproduce a bug with the right thing on screen at the right time.

// TraceLog appends every runtime event to a file as newline-delimited JSON.
// One event, one line, in emission order — safe to `tail -f` while cortex
// runs, or load wholesale after the fact.
type TraceLog struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
}

// traceLine is what actually gets written. axon.Event itself doesn't
// marshal cleanly — Err is an error interface, and Kind is just an int — so
// this reshapes it into the form a log reader wants: a kind name and a
// flattened error string.
type traceLine struct {
	Time    time.Time         `json:"time"`
	Kind    string            `json:"kind"`
	Turn    int               `json:"turn,omitempty"`
	Text    string            `json:"text,omitempty"`
	Tool    *axon.ToolEvent   `json:"tool,omitempty"`
	Prune   *axon.PruneInfo   `json:"prune,omitempty"`
	Session *axon.SessionInfo `json:"session,omitempty"`
	Err     string            `json:"err,omitempty"`
}

// NewTraceLog creates (or truncates) a trace file at path, making its parent
// directory as needed. Returns a nil *TraceLog, no error, when path is empty
// — the zero value's Emit is a no-op, so a disabled trace costs callers no
// special case.
func NewTraceLog(path string) (*TraceLog, error) {
	if path == "" {
		return nil, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("trace log: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("trace log: %w", err)
	}

	return &TraceLog{file: f, enc: json.NewEncoder(f)}, nil
}

// DefaultTracePath names where a session's trace goes: one file per process,
// so two cortex sessions running at once never interleave writes.
func DefaultTracePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}

	name := fmt.Sprintf("%s-%d.jsonl", time.Now().Format("20060102-150405"), os.Getpid())

	return filepath.Join(dir, "cortex", "trace", name)
}

// Emit satisfies axon's Config.OnEvent signature. Safe to call on a nil
// receiver — the pre-Attach state every other sink in this package treats as
// normal — and safe under concurrent calls, though axon's own contract (Step
// is never called concurrently with itself) means that should never happen
// in practice.
func (t *TraceLog) Emit(_ context.Context, e axon.Event) {
	if t == nil {
		return
	}

	line := traceLine{
		Time:    e.Time,
		Kind:    e.Kind.String(),
		Turn:    e.Turn,
		Text:    e.Text,
		Tool:    e.Tool,
		Prune:   e.Prune,
		Session: e.Session,
	}
	if e.Err != nil {
		line.Err = e.Err.Error()
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	_ = t.enc.Encode(line)
}

// Path returns the file this log writes to, or "" for a nil/disabled log.
func (t *TraceLog) Path() string {
	if t == nil {
		return ""
	}

	return t.file.Name()
}

// Close flushes and closes the underlying file. Safe on a nil receiver.
func (t *TraceLog) Close() error {
	if t == nil {
		return nil
	}

	return t.file.Close()
}

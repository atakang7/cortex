package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/atakang7/axon/v2"
)

type TraceLog struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
}

type traceLine struct {
	Time    time.Time    `json:"time"`
	Kind    string       `json:"kind"`
	Turn    int          `json:"turn,omitempty"`
	Text    string       `json:"text,omitempty"`
	Tool    *toolLine    `json:"tool,omitempty"`
	Prune   *pruneLine   `json:"prune,omitempty"`
	Session *sessionLine `json:"session,omitempty"`
	Wire    *wireLine    `json:"wire,omitempty"`
	Run     *RunInfo     `json:"run,omitempty"`
	Err     string       `json:"err,omitempty"`
}

type toolLine struct {
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Args      json.RawMessage `json:"args,omitempty"`
	ArgsDelta string          `json:"args_delta,omitempty"`
	Result    string          `json:"result,omitempty"`
	BlockID   string          `json:"block_id,omitempty"`
}

type pruneLine struct {
	Before   int      `json:"before"`
	After    int      `json:"after"`
	Rejected []string `json:"rejected,omitempty"`
}

type sessionLine struct {
	ID       string `json:"id,omitempty"`
	Cwd      string `json:"cwd,omitempty"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	Path     string `json:"path,omitempty"`
	PrunerOn bool   `json:"pruner_on"`
}

const (
	kindWireRequest = "wire_request"
	kindWireReply   = "wire_reply"
	kindRun         = "run"
)

type RunInfo struct {
	Version        string `json:"version,omitempty"`
	Provider       string `json:"provider,omitempty"`
	Model          string `json:"model,omitempty"`
	PrunerProvider string `json:"pruner_provider,omitempty"`
	PrunerModel    string `json:"pruner_model,omitempty"`
	Cwd            string `json:"cwd,omitempty"`
}

type wireLine struct {
	Role           string          `json:"role"`
	Call           int             `json:"call"`
	Messages       []axon.Msg      `json:"messages,omitempty"`
	Tools          []string        `json:"tools,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	ResponseFormat json.RawMessage `json:"response_format,omitempty"`
	Reply          *axon.Msg       `json:"reply,omitempty"`
	ElapsedMS      int64           `json:"elapsed_ms,omitempty"`
	Err            string          `json:"err,omitempty"`
}

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

func DefaultTracePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	name := fmt.Sprintf("%s-%d.jsonl", time.Now().Format("20060102-150405"), os.Getpid())
	return filepath.Join(dir, "cortex", "trace", name)
}

func (t *TraceLog) Emit(_ context.Context, e axon.Event) {
	if t == nil {
		return
	}
	line := traceLine{Time: e.Time, Kind: e.Kind.String(), Turn: e.Turn, Text: e.Text}
	if e.Err != nil {
		line.Err = e.Err.Error()
	}
	if tool := e.Tool; tool != nil {
		line.Tool = &toolLine{ID: tool.ID, Name: tool.Name, Args: tool.Args, ArgsDelta: tool.ArgsDelta, Result: tool.Result, BlockID: tool.BlockID}
	}
	if p := e.Prune; p != nil {
		line.Prune = &pruneLine{Before: p.Before, After: p.After, Rejected: p.Rejected}
	}
	if s := e.Session; s != nil {
		line.Session = &sessionLine{ID: s.ID, Cwd: s.Cwd, Provider: s.Provider, Model: s.Model, Path: s.Path, PrunerOn: s.PrunerOn}
	}
	t.write(line)
}

func (t *TraceLog) write(line traceLine) {
	if t == nil {
		return
	}
	if line.Time.IsZero() {
		line.Time = time.Now()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	_ = t.enc.Encode(line)
}

func (t *TraceLog) Describe(info RunInfo) {
	t.write(traceLine{Kind: kindRun, Run: &info})
}

func (t *TraceLog) Wiretap(role string, model axon.Model) axon.Model {
	if t == nil || model == nil {
		return model
	}
	return &wiretap{trace: t, role: role, inner: model}
}

type wiretap struct {
	trace *TraceLog
	role  string
	inner axon.Model
	calls atomic.Int64
}

func (w *wiretap) Complete(ctx context.Context, req axon.Request) (*axon.Msg, error) {
	call := int(w.calls.Add(1))
	names := make([]string, len(req.Tools))
	for i, spec := range req.Tools {
		names[i] = spec.Name
	}
	w.trace.write(traceLine{Kind: kindWireRequest, Wire: &wireLine{Role: w.role, Call: call, Messages: req.Messages, Tools: names, MaxTokens: req.MaxTokens, ResponseFormat: req.ResponseFormat}})
	start := time.Now()
	reply, err := w.inner.Complete(ctx, req)
	line := &wireLine{Role: w.role, Call: call, Reply: reply, ElapsedMS: time.Since(start).Milliseconds()}
	if err != nil {
		line.Err = err.Error()
	}
	w.trace.write(traceLine{Kind: kindWireReply, Wire: line})
	return reply, err
}

func (t *TraceLog) Path() string {
	if t == nil {
		return ""
	}
	return t.file.Name()
}

func (t *TraceLog) Close() error {
	if t == nil {
		return nil
	}
	return t.file.Close()
}

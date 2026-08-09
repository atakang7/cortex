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

// tracelog.go is the third event sink, alongside Bridge (the live TUI) and
// Plain (the non-interactive renderer). Where those two curate — deciding
// what a human should see mid-session — TraceLog does not: every axon.Event
// that reaches it is written, unfiltered, as one JSON line. It exists so
// "what did the runtime actually do" never depends on remembering to
// reproduce a bug with the right thing on screen at the right time.
//
// Events alone do not answer the question that matters when the agent
// misbehaves, which is almost never "what did the runtime do" and almost
// always "what did the model actually see". Events are emitted downstream of
// the request: they describe the reply, never the prompt that caused it. The
// composed system message, the windowed message array, the tool schemas and
// the pruner's own curator calls are all invisible in an event stream.
//
// TraceLog.Wiretap closes that gap. It decorates an axon.Model so the exact
// Request going out and the Msg coming back are written to this same file,
// interleaved in real time with the events they produced. Prompt engineering
// is guesswork without it.

// TraceLog appends every runtime event to a file as newline-delimited JSON.
// One event, one line, in emission order — safe to `tail -f` while cortex
// runs, or load wholesale after the fact.
type TraceLog struct {
	mu   sync.Mutex
	file *os.File
	enc  *json.Encoder
}

// traceLine is what actually gets written — one shape for every line in the
// file, so a reader switches on exactly one field. axon.Event does not
// marshal cleanly (Err is an error interface, Kind is an int), so this
// reshapes it into the form a log reader wants: a kind name and a flattened
// error string.
//
// Kind is either an axon.Kind name or one of the wire kinds below. Nothing
// else distinguishes the two, deliberately: a request is just another thing
// that happened, at a point in time, in the same ordered stream.
type traceLine struct {
	Time    time.Time    `json:"time"`
	Kind    string       `json:"kind"`
	Turn    int          `json:"turn,omitempty"`
	Text    string       `json:"text,omitempty"`
	Tool    *toolLine    `json:"tool,omitempty"`
	Prune   *pruneLine   `json:"prune,omitempty"`
	Session *sessionLine `json:"session,omitempty"`
	Usage   *usageLine   `json:"usage,omitempty"`
	Wire    *wireLine    `json:"wire,omitempty"`
	Run     *RunInfo     `json:"run,omitempty"`
	Err     string       `json:"err,omitempty"`
}

// The payload shapes below mirror axon's event structs field for field, and
// exist because axon's own structs carry no JSON tags. Marshalling those
// directly would publish Go field names — "PromptTokens", "BlockID" — as this
// file's public key names, which means an upstream rename silently rewrites
// the format every downstream reader parses. Naming the keys here makes the
// trace format cortex's to keep.
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

type usageLine struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// Wire kinds. They share the Kind field with axon's own event names, so they
// are prefixed to make an accidental collision with a future axon.Kind
// impossible to mistake for one.
const (
	kindWireRequest = "wire_request"
	kindWireReply   = "wire_reply"
	kindRun         = "run"
)

// Run identifies the session a trace belongs to: which build of cortex, and
// which models.
//
// axon's own session_start event deliberately leaves provider and model
// blank, and is right to — Model is an interface an embedder may implement
// with anything, so the runtime has no way to ask one what it is. cortex does
// know, because it resolved them from config, and a trace that cannot say
// which model produced it cannot be compared against another one.
type RunInfo struct {
	Version        string `json:"version,omitempty"`
	Provider       string `json:"provider,omitempty"`
	Model          string `json:"model,omitempty"`
	PrunerProvider string `json:"pruner_provider,omitempty"`
	PrunerModel    string `json:"pruner_model,omitempty"`
	Cwd            string `json:"cwd,omitempty"`
}

// wireLine is one side of one model call: what cortex sent, or what came
// back. Request and reply are separate lines rather than one paired record
// because a call that never returns — a hang, a cancelled turn, a process
// killed mid-stream — must still leave its prompt in the file. That is
// exactly the case worth debugging.
type wireLine struct {
	// Role names which model this call belongs to: "main" for the agent's
	// own model, "pruner" for the curator. Without it the curator's traffic
	// is indistinguishable from the agent's, and the two have very different
	// prompts.
	Role string `json:"role"`

	// Call pairs a reply with its request. Monotonic per process and per
	// role, assigned when the request goes out.
	Call int `json:"call"`

	// Messages is the conversation exactly as the provider received it:
	// system message first, already windowed and with parked blocks replaced
	// by their breadcrumbs. This is the prompt — the thing being engineered.
	Messages []axon.Msg `json:"messages,omitempty"`

	// Tools lists the callable names only. The full schemas are not repeated
	// here because axon composes them into the system message verbatim, so
	// Messages[0] already carries every byte of them.
	Tools []string `json:"tools,omitempty"`

	MaxTokens      int             `json:"max_tokens,omitempty"`
	ResponseFormat json.RawMessage `json:"response_format,omitempty"`

	// Reply and ElapsedMS are set on kindWireReply only.
	Reply     *axon.Msg `json:"reply,omitempty"`
	ElapsedMS int64     `json:"elapsed_ms,omitempty"`
	Err       string    `json:"err,omitempty"`
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
		Time: e.Time,
		Kind: e.Kind.String(),
		Turn: e.Turn,
		Text: e.Text,
	}
	if e.Err != nil {
		line.Err = e.Err.Error()
	}
	if t := e.Tool; t != nil {
		line.Tool = &toolLine{
			ID: t.ID, Name: t.Name, Args: t.Args,
			ArgsDelta: t.ArgsDelta, Result: t.Result, BlockID: t.BlockID,
		}
	}
	if p := e.Prune; p != nil {
		line.Prune = &pruneLine{Before: p.Before, After: p.After, Rejected: p.Rejected}
	}
	if s := e.Session; s != nil {
		line.Session = &sessionLine{
			ID: s.ID, Cwd: s.Cwd, Provider: s.Provider,
			Model: s.Model, Path: s.Path, PrunerOn: s.PrunerOn,
		}
	}
	if u := e.Usage; u != nil {
		line.Usage = &usageLine{PromptTokens: u.PromptTokens, CompletionTokens: u.CompletionTokens}
	}

	t.write(line)
}

// write is the one place a line reaches the file. Every producer — events,
// wiretaps — goes through here, so ordering and locking are decided once.
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

// Describe records the run's identity. Call it once, before the first turn,
// so the trace's opening line says what produced everything after it.
func (t *TraceLog) Describe(info RunInfo) {
	t.write(traceLine{Kind: kindRun, Run: &info})
}

// Wiretap returns model wrapped so that every call it makes is recorded to
// this trace under the given role ("main", "pruner"). It returns model
// untouched when the trace is disabled, so a nil trace costs nothing and
// changes no behaviour.
//
// The wrapper is transparent by construction: it forwards the request
// unmodified, returns the reply unmodified, and never converts an error into
// a success. Anything else would mean the trace changes the run it is
// supposed to be observing.
func (t *TraceLog) Wiretap(role string, model axon.Model) axon.Model {
	if t == nil || model == nil {
		return model
	}

	return &wiretap{trace: t, role: role, inner: model}
}

// wiretap is the axon.Model decorator behind TraceLog.Wiretap.
type wiretap struct {
	trace *TraceLog
	role  string
	inner axon.Model

	// calls counts requests for this role. Atomic because a pruner pass and
	// a turn are not guaranteed by axon's contract to be serialized against
	// each other, and a duplicated call number would silently mispair a
	// reply with someone else's prompt.
	calls atomic.Int64
}

// Complete records the outgoing request, delegates, then records the reply.
func (w *wiretap) Complete(ctx context.Context, req axon.Request) (*axon.Msg, error) {
	call := int(w.calls.Add(1))

	names := make([]string, len(req.Tools))
	for i, spec := range req.Tools {
		names[i] = spec.Name
	}

	w.trace.write(traceLine{Kind: kindWireRequest, Wire: &wireLine{
		Role:           w.role,
		Call:           call,
		Messages:       req.Messages,
		Tools:          names,
		MaxTokens:      req.MaxTokens,
		ResponseFormat: req.ResponseFormat,
	}})

	start := time.Now()
	reply, err := w.inner.Complete(ctx, req)

	line := &wireLine{
		Role:      w.role,
		Call:      call,
		Reply:     reply,
		ElapsedMS: time.Since(start).Milliseconds(),
	}
	if err != nil {
		line.Err = err.Error()
	}
	w.trace.write(traceLine{Kind: kindWireReply, Wire: line})

	return reply, err
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

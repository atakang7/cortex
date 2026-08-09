package ui

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/atakang7/axon/v2"
)

// readTrace returns every line of a trace file, decoded.
func readTrace(t *testing.T, path string) []map[string]any {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open trace: %v", err)
	}
	defer f.Close()

	var lines []map[string]any
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for scanner.Scan() {
		var line map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			t.Fatalf("trace line is not JSON: %v\n%s", err, scanner.Text())
		}
		lines = append(lines, line)
	}
	return lines
}

func newTestTrace(t *testing.T) (*TraceLog, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "trace.jsonl")
	trace, err := NewTraceLog(path)
	if err != nil {
		t.Fatalf("NewTraceLog: %v", err)
	}
	t.Cleanup(func() { trace.Close() })
	return trace, path
}

// fakeModel is an axon.Model whose reply and error are fixed by the test.
type fakeModel struct {
	reply *axon.Msg
	err   error
	seen  axon.Request
}

func (f *fakeModel) Complete(_ context.Context, req axon.Request) (*axon.Msg, error) {
	f.seen = req
	return f.reply, f.err
}

// The trace exists to observe a run without altering it. A wiretap that
// changed a request, swallowed an error or substituted a reply would make
// every trace a record of a different run than the one that happened.
func TestWiretapIsTransparent(t *testing.T) {
	trace, _ := newTestTrace(t)

	reply := &axon.Msg{Role: "assistant", Content: "hi"}
	failure := errors.New("provider exploded")
	inner := &fakeModel{reply: reply, err: failure}

	req := axon.Request{
		Messages:  []axon.Msg{{Role: "system", Content: "role"}, {Role: "user", Content: "go"}},
		Tools:     []axon.ToolSpec{{Name: "read"}},
		MaxTokens: 512,
	}

	got, err := trace.Wiretap("main", inner).Complete(context.Background(), req)

	if got != reply {
		t.Errorf("reply was substituted: got %v, want %v", got, reply)
	}
	if !errors.Is(err, failure) {
		t.Errorf("error was not passed through: got %v, want %v", err, failure)
	}
	if len(inner.seen.Messages) != 2 || inner.seen.MaxTokens != 512 {
		t.Errorf("request reached the model altered: %+v", inner.seen)
	}
}

// A disabled trace must cost callers no special case: Wiretap hands back the
// model untouched rather than a wrapper that writes nowhere.
func TestWiretapOnNilTraceReturnsModelUnchanged(t *testing.T) {
	var trace *TraceLog
	inner := &fakeModel{reply: &axon.Msg{Role: "assistant"}}

	if got := trace.Wiretap("main", inner); got != axon.Model(inner) {
		t.Errorf("nil trace should return the model itself, got %T", got)
	}

	if got := (&TraceLog{}).Wiretap("main", nil); got != nil {
		t.Errorf("a nil model should stay nil, got %T", got)
	}
}

// The prompt is the thing being engineered, so it is the thing the trace most
// has to preserve: the exact messages sent, and which model they went to.
func TestWiretapRecordsThePromptAndPairsTheReply(t *testing.T) {
	trace, path := newTestTrace(t)

	inner := &fakeModel{reply: &axon.Msg{Role: "assistant", Content: "done"}}
	req := axon.Request{
		Messages: []axon.Msg{{Role: "system", Content: "you are cortex"}},
		Tools:    []axon.ToolSpec{{Name: "read"}, {Name: "write"}},
	}

	if _, err := trace.Wiretap("pruner", inner).Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	lines := readTrace(t, path)
	if len(lines) != 2 {
		t.Fatalf("want a request line and a reply line, got %d", len(lines))
	}

	request, reply := lines[0], lines[1]
	if request["kind"] != kindWireRequest || reply["kind"] != kindWireReply {
		t.Fatalf("wrong kinds: %v, %v", request["kind"], reply["kind"])
	}

	wire := request["wire"].(map[string]any)
	if wire["role"] != "pruner" {
		t.Errorf("curator traffic must be distinguishable from the agent's, got role %v", wire["role"])
	}
	if got := wire["messages"].([]any)[0].(map[string]any)["content"]; got != "you are cortex" {
		t.Errorf("system message not recorded verbatim: %v", got)
	}
	if got := len(wire["tools"].([]any)); got != 2 {
		t.Errorf("want 2 tool names recorded, got %d", got)
	}

	if wire["call"] != reply["wire"].(map[string]any)["call"] {
		t.Error("reply does not carry its request's call number, so the two cannot be paired")
	}
}

// A call that never returns must still leave its prompt behind. That is
// exactly the run worth debugging, and a paired single-record format would
// lose it.
func TestWiretapRecordsTheRequestBeforeTheModelRuns(t *testing.T) {
	trace, path := newTestTrace(t)

	var linesDuringCall int
	probe := modelFunc(func(context.Context, axon.Request) (*axon.Msg, error) {
		linesDuringCall = len(readTrace(t, path))
		return nil, errors.New("hung")
	})

	_, _ = trace.Wiretap("main", probe).Complete(context.Background(), axon.Request{
		Messages: []axon.Msg{{Role: "user", Content: "go"}},
	})

	if linesDuringCall != 1 {
		t.Errorf("request should be on disk before the model is called, saw %d lines", linesDuringCall)
	}
}

type modelFunc func(context.Context, axon.Request) (*axon.Msg, error)

func (f modelFunc) Complete(ctx context.Context, r axon.Request) (*axon.Msg, error) { return f(ctx, r) }

// Event payloads must reach the file under cortex's own key names. Publishing
// axon's Go field names would mean an upstream rename silently rewrites the
// format every trace reader parses.
func TestEmitWritesStableKeyNames(t *testing.T) {
	trace, path := newTestTrace(t)

	trace.Emit(context.Background(), axon.Event{
		Kind:  axon.KindUsage,
		Usage: &axon.UsageInfo{PromptTokens: 120, CompletionTokens: 45},
	})
	trace.Emit(context.Background(), axon.Event{
		Kind: axon.KindToolResult,
		Tool: &axon.ToolEvent{ID: "t1", Name: "read", Result: "ok", BlockID: "m7"},
	})

	lines := readTrace(t, path)

	usage := lines[0]["usage"].(map[string]any)
	if usage["prompt_tokens"] != float64(120) || usage["completion_tokens"] != float64(45) {
		t.Errorf("usage keys wrong: %v", usage)
	}

	tool := lines[1]["tool"].(map[string]any)
	if tool["block_id"] != "m7" || tool["name"] != "read" {
		t.Errorf("tool keys wrong: %v", tool)
	}
}

// Token accounting was absent from the trace entirely for a while, because
// traceLine simply had no field for it. The cost of a run is the first thing
// asked of a trace.
func TestEmitRecordsUsage(t *testing.T) {
	trace, path := newTestTrace(t)

	trace.Emit(context.Background(), axon.Event{
		Kind:  axon.KindUsage,
		Usage: &axon.UsageInfo{PromptTokens: 1, CompletionTokens: 2},
	})

	if lines := readTrace(t, path); lines[0]["usage"] == nil {
		t.Fatal("usage event reached the trace with its payload dropped")
	}
}

func TestDescribeRecordsWhichModelProducedTheTrace(t *testing.T) {
	trace, path := newTestTrace(t)

	trace.Describe(RunInfo{Provider: "openrouter", Model: "z-ai/glm-5.2", PrunerModel: "deepseek/deepseek-v4-flash-0731"})

	lines := readTrace(t, path)
	if lines[0]["kind"] != kindRun {
		t.Fatalf("want a run line first, got %v", lines[0]["kind"])
	}
	run := lines[0]["run"].(map[string]any)
	if run["model"] != "z-ai/glm-5.2" || run["pruner_model"] != "deepseek/deepseek-v4-flash-0731" {
		t.Errorf("run identity not recorded: %v", run)
	}
}

// A nil TraceLog is the disabled state, and every method must tolerate it —
// main.go keeps using the value after NewTraceLog fails.
func TestNilTraceLogIsInert(t *testing.T) {
	var trace *TraceLog

	trace.Emit(context.Background(), axon.Event{Kind: axon.KindTurnStart})
	trace.Describe(RunInfo{Model: "m"})

	if got := trace.Path(); got != "" {
		t.Errorf("Path on a disabled trace should be empty, got %q", got)
	}
	if err := trace.Close(); err != nil {
		t.Errorf("Close on a disabled trace should succeed, got %v", err)
	}
}

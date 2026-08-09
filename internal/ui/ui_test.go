package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/atakang7/axon/v2"
)

// TestMain pins the colour profile so rendered output is plain text. Without
// it these assertions would depend on whatever terminal the test runs under.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)

	os.Exit(m.Run())
}

// ---------------------------------------------------------------------------
// A model that needs no network
// ---------------------------------------------------------------------------

// scriptedModel replays a fixed list of replies, one per call. axon.Model is
// a one-method interface precisely so the loop can be driven this way — these
// tests exercise the real agent, the real tools and the real filesystem, and
// fake only the thing that would otherwise cost money.
type scriptedModel struct {
	replies []axon.Msg
	calls   int
}

func (m *scriptedModel) Complete(_ context.Context, req axon.Request) (*axon.Msg, error) {
	if m.calls >= len(m.replies) {
		return nil, errors.New("scriptedModel: more calls than replies")
	}

	reply := m.replies[m.calls]
	m.calls++

	// Stream the content the way a real provider would, so anything reading
	// tokens sees them arrive in pieces rather than all at once.
	if req.Stream.Token != nil {
		for _, word := range strings.SplitAfter(reply.Content, " ") {
			if word != "" {
				req.Stream.Token(word)
			}
		}
	}

	return &reply, nil
}

func toolCallReply(id, name string, args map[string]any) axon.Msg {
	encoded, _ := json.Marshal(args)

	call := axon.ToolCall{ID: id, Type: "function"}
	call.Function.Name = name
	call.Function.Arguments = string(encoded)

	return axon.Msg{Role: "assistant", ToolCalls: []axon.ToolCall{call}}
}

// newTestAgent builds a real agent in a throwaway workspace. The returned
// directory is the agent's cwd, so a test can put files where its tools will
// find them.
func newTestAgent(t *testing.T, model axon.Model, onEvent func(context.Context, axon.Event)) (*axon.Agent, string) {
	t.Helper()

	// Keep the session file and background-shell logs inside the test's own
	// temporary tree rather than the developer's real data directory.
	t.Setenv("AXON_DATA_DIR", t.TempDir())

	workDir := t.TempDir()

	agent, err := axon.New(axon.Config{
		Model:        model,
		SystemPrompt: "You are a test agent.",
		Cwd:          workDir,
		OnEvent:      onEvent,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { agent.Close() })

	return agent, workDir
}

// ---------------------------------------------------------------------------
// The non-interactive renderer
// ---------------------------------------------------------------------------

// The stream split is a contract other programs depend on: redirecting stdout
// must capture the answer and nothing else, or `cortex --prompt` cannot be
// used in a pipeline.
func TestPlainSendsOnlyTheAnswerToStdout(t *testing.T) {
	var out, errOut bytes.Buffer
	renderer := NewPlain(&out, &errOut)

	model := &scriptedModel{replies: []axon.Msg{
		toolCallReply("call_1", "read", map[string]any{"path": "greeting.txt", "mode": "full"}),
		{Role: "assistant", Content: "The file says hello."},
	}}

	agent, workDir := newTestAgent(t, model, renderer.Emit)

	if err := os.WriteFile(filepath.Join(workDir, "greeting.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := agent.Step(context.Background(), "what does greeting.txt say?"); err != nil {
		t.Fatal(err)
	}
	renderer.Close()

	answer := out.String()
	if !strings.Contains(answer, "The file says hello.") {
		t.Errorf("stdout = %q, want the assistant's answer", answer)
	}
	if strings.Contains(answer, "read") || strings.Contains(answer, "greeting.txt") {
		t.Errorf("stdout = %q, want no tool activity — that belongs on stderr", answer)
	}

	activity := errOut.String()
	if !strings.Contains(activity, "read") || !strings.Contains(activity, "greeting.txt") {
		t.Errorf("stderr = %q, want the tool card naming the tool and its subject", activity)
	}
}

// A tool that fails produces two events from axon: KindToolError, and then
// KindToolResult carrying the error text as the tool's output. Both renderers
// must print one outcome line, not two.
func TestFailedToolPrintsOneOutcome(t *testing.T) {
	var out, errOut bytes.Buffer
	renderer := NewPlain(&out, &errOut)

	tool := axon.ToolEvent{ID: "call_1", Name: "exec", Args: json.RawMessage(`{"command":"false"}`)}

	renderer.Emit(context.Background(), axon.Event{Kind: axon.KindToolCall, Tool: &tool})
	renderer.Emit(context.Background(), axon.Event{
		Kind: axon.KindToolError,
		Tool: &tool,
		Err:  errors.New("exit status 1"),
	})
	renderer.Emit(context.Background(), axon.Event{
		Kind: axon.KindToolResult,
		Tool: &axon.ToolEvent{ID: "call_1", Name: "exec", Result: "exit status 1"},
	})

	if got := strings.Count(errOut.String(), glyphToolTail); got != 1 {
		t.Errorf("outcome lines = %d, want exactly 1\n%s", got, errOut.String())
	}
	if !strings.Contains(errOut.String(), "exit status 1") {
		t.Errorf("want the failure reported, got:\n%s", errOut.String())
	}
}

// The same double event must not resurrect a card in the interactive model.
func TestFailedToolClosesTheCardOnce(t *testing.T) {
	m := newTestModel(t)

	m = apply(t, m,
		toolCallMsg{ID: "call_1", Name: "exec", Args: json.RawMessage(`{"command":"false"}`)},
		toolErrorMsg{ID: "call_1", Name: "exec", Err: errors.New("exit status 1")},
		toolResultMsg{ID: "call_1", Name: "exec", Result: "exit status 1"},
	)

	if len(m.cards) != 0 {
		t.Errorf("cards = %d, want the card closed and not reopened", len(m.cards))
	}
}

// ---------------------------------------------------------------------------
// The interactive model's state machine
// ---------------------------------------------------------------------------

func newTestModel(t *testing.T) Model {
	t.Helper()

	agent, _ := newTestAgent(t, &scriptedModel{}, nil)

	return New(Options{
		Agent:     agent,
		Context:   context.Background(),
		ModelName: "test-model",
		AgentName: "cortex",
	})
}

// apply feeds messages through Update the way the runtime would, returning the
// resulting model.
func apply(t *testing.T, m Model, msgs ...tea.Msg) Model {
	t.Helper()

	for _, msg := range msgs {
		next, _ := m.Update(msg)

		typed, ok := next.(Model)
		if !ok {
			t.Fatalf("Update returned %T, want ui.Model", next)
		}
		m = typed
	}

	return m
}

// A card opens on the call and must close on the result. A card that never
// closes stays in the live area forever, showing "running…" under a tool that
// finished — the most visible way this UI could break.
func TestToolCardOpensOnCallAndClosesOnResult(t *testing.T) {
	m := newTestModel(t)

	m = apply(t, m, toolCallMsg{ID: "call_1", Name: "read", Args: json.RawMessage(`{"path":"main.go"}`)})

	if len(m.cards) != 1 {
		t.Fatalf("cards = %d, want 1 open card", len(m.cards))
	}
	if !strings.Contains(m.View(), "running") {
		t.Error("want the live area to show the call as running")
	}

	m = apply(t, m, toolResultMsg{ID: "call_1", Name: "read", Result: "package main\n"})

	if len(m.cards) != 0 {
		t.Fatalf("cards = %d, want the card committed and dropped from the live area", len(m.cards))
	}
}

// A provider that omits the call ID on results must not strand the card.
func TestToolCardClosesByNameWhenTheIDIsMissing(t *testing.T) {
	m := newTestModel(t)

	m = apply(t, m,
		toolCallMsg{ID: "call_1", Name: "search", Args: json.RawMessage(`{"pattern":"TODO"}`)},
		toolResultMsg{Name: "search", Result: "3 matches"},
	)

	if len(m.cards) != 0 {
		t.Fatalf("cards = %d, want the card matched by name and closed", len(m.cards))
	}
}

// Streamed text lives in the model only until the finished message arrives.
// Leaving it behind would print the answer twice: once from the buffer and
// once from the authoritative text.
func TestStreamedTextIsClearedWhenTheFinalTextArrives(t *testing.T) {
	m := newTestModel(t)

	m = apply(t, m, tokenMsg("Hello "), tokenMsg("world"))

	if m.stream != "Hello world" {
		t.Fatalf("stream = %q, want the accumulated tokens", m.stream)
	}
	if !strings.Contains(m.View(), "Hello world") {
		t.Error("want streamed text visible in the live area")
	}

	m = apply(t, m, assistantMsg("Hello world"))

	if m.stream != "" {
		t.Errorf("stream = %q, want it cleared once committed", m.stream)
	}
}

// An interrupt lands mid-turn, so whatever streamed must still be committed
// and every open card must be closed. Anything left in these fields would
// leak into the next turn's display.
func TestInterruptedTurnLeavesNothingInFlight(t *testing.T) {
	m := newTestModel(t)
	m.busy = true

	m = apply(t, m,
		toolCallMsg{ID: "call_1", Name: "exec", Args: json.RawMessage(`{"command":"sleep 60"}`)},
		tokenMsg("partial answer"),
		turnDoneMsg{Err: axon.ErrInterrupted},
	)

	if m.busy {
		t.Error("busy = true, want the turn finished")
	}
	if m.stream != "" {
		t.Errorf("stream = %q, want it committed and cleared", m.stream)
	}
	if len(m.cards) != 0 {
		t.Errorf("cards = %d, want every open card closed", len(m.cards))
	}
}

// Reasoning is committed when the step produces real output, not held until
// the turn ends — otherwise thinking would print after the answer it preceded.
func TestReasoningIsCommittedBeforeContent(t *testing.T) {
	m := newTestModel(t)

	m = apply(t, m, reasoningMsg("let me look"))
	if m.reasoning == "" {
		t.Fatal("want reasoning buffered while it streams")
	}

	m = apply(t, m, tokenMsg("Found it."))
	if m.reasoning != "" {
		t.Errorf("reasoning = %q, want it flushed when content began", m.reasoning)
	}
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func TestIsCommand(t *testing.T) {
	for _, line := range []string{"/help", "/cd ..", "/undo"} {
		if !isCommand(line) {
			t.Errorf("isCommand(%q) = false, want true", line)
		}
	}

	// A path is not a command, and neither is a lone slash.
	for _, line := range []string{"/", "//comment", "fix /etc/hosts", ""} {
		if isCommand(line) {
			t.Errorf("isCommand(%q) = true, want false", line)
		}
	}
}

func TestUnknownCommandIsReportedNotSent(t *testing.T) {
	agent, _ := newTestAgent(t, &scriptedModel{}, nil)

	res := runCommand(agent, "/nope")
	if res.Err == nil {
		t.Fatal("want an error for an unknown command")
	}
	if !strings.Contains(res.Err.Error(), "/help") {
		t.Errorf("err = %v, want it to point at /help", res.Err)
	}
}

func TestQuitAliases(t *testing.T) {
	agent, _ := newTestAgent(t, &scriptedModel{}, nil)

	for _, line := range []string{"/quit", "/exit", "/q"} {
		if !runCommand(agent, line).Quit {
			t.Errorf("%s did not quit", line)
		}
	}
}

func TestHelpListsEveryCommandOnce(t *testing.T) {
	text := helpText()

	for name := range commands {
		if strings.Count(text, name) != 1 {
			t.Errorf("help mentions %s %d times, want exactly once", name, strings.Count(text, name))
		}
	}
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

// The card header shows the one argument that says what a call is about.
// Showing the whole payload makes a transcript unreadable; showing nothing
// makes it useless.
func TestSummarizeArgsPicksTheSubject(t *testing.T) {
	cases := []struct {
		tool string
		args string
		want string
	}{
		{"read", `{"path":"internal/ui/view.go","mode":"full"}`, "internal/ui/view.go  full"},
		{"exec", `{"command":"go test ./...","timeout":60}`, "go test ./..."},
		{"search", `{"pattern":"TODO","mode":"regex"}`, "TODO  regex"},
		{"task", `{"goal":"add a health endpoint"}`, "add a health endpoint"},

		// An unknown tool — every MCP tool is one — falls back to the shared
		// key list rather than dumping JSON.
		{"deploy", `{"service":"api","url":"https://staging"}`, "https://staging"},

		// Nothing recognisable: compact JSON beats an empty header.
		{"mystery", `{"alpha":1}`, `{"alpha":1}`},
	}

	for _, c := range cases {
		if got := summarizeArgs(c.tool, json.RawMessage(c.args)); got != c.want {
			t.Errorf("summarizeArgs(%q, %s) = %q, want %q", c.tool, c.args, got, c.want)
		}
	}
}

func TestSummarizeArgsSurvivesMalformedJSON(t *testing.T) {
	// The model's arguments are not guaranteed to parse — axon documents this
	// on ToolCall. The renderer must not panic on them.
	if got := summarizeArgs("read", json.RawMessage(`{"path":`)); got == "" {
		t.Error("want some subject rendered for unparseable args")
	}
}

func TestToolCardFooterReportsOutcome(t *testing.T) {
	running := toolCard{Name: "read"}
	if !strings.Contains(running.render(80), "running") {
		t.Error("an open card must say it is running")
	}

	failed := toolCard{Name: "exec", Done: true, Err: errors.New("exit status 1")}
	if !strings.Contains(failed.render(80), "exit status 1") {
		t.Error("a failed card must show the error")
	}

	long := toolCard{Name: "read", Done: true, Result: strings.Repeat("line\n", 40)}
	rendered := long.render(80)
	if !strings.Contains(rendered, "40 lines") {
		t.Errorf("want the full line count reported, got %q", rendered)
	}
	if strings.Count(rendered, "line") > resultPreviewLines+2 {
		t.Error("want long output truncated to the preview cap")
	}
}

func TestTruncateRespectsTheLimit(t *testing.T) {
	if got := truncate("abcdefghij", 5); lipgloss.Width(got) > 5 {
		t.Errorf("truncate produced %q, wider than the limit", got)
	}

	if got := truncate("abc", 10); got != "abc" {
		t.Errorf("truncate(%q) = %q, want it untouched", "abc", got)
	}
}

// Token usage accumulates across API calls and shows in the status line, so a
// session-long readout is available without a separate command.
func TestUsageAccumulatesAndRenders(t *testing.T) {
	m := newTestModel(t)

	m = apply(t, m, usageMsg{PromptTokens: 120, CompletionTokens: 30})
	m = apply(t, m, usageMsg{PromptTokens: 80, CompletionTokens: 20})

	if m.usage.prompt != 200 || m.usage.completion != 50 {
		t.Fatalf("usage = %d↑ %d↓, want 200↑ 50↓", m.usage.prompt, m.usage.completion)
	}

	view := m.View()
	if !strings.Contains(view, "tokens 200↑ 50↓") {
		t.Errorf("view = %q, want the status line to show accumulated tokens", view)
	}
}

// A narrow terminal must degrade, not produce negative widths that panic the
// wrapper.
func TestRenderingSurvivesANarrowTerminal(t *testing.T) {
	m := newTestModel(t)
	m = m.resize(10)

	m = apply(t, m,
		toolCallMsg{ID: "c", Name: "read", Args: json.RawMessage(`{"path":"a/very/long/path/to/a/file.go"}`)},
		tokenMsg("some text that is much wider than ten columns"),
	)

	if m.View() == "" {
		t.Error("want something rendered even at ten columns")
	}
}

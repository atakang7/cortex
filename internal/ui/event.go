package ui

import (
	"context"
	"encoding/json"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/atakang7/axon/v2"
)

// event.go is the seam between the two concurrency models in this program.
//
// axon calls Config.OnEvent synchronously, on whichever goroutine is running
// the turn. Bubble Tea owns the terminal from its own goroutine and will only
// accept work as messages. Bridge is the single place that crosses between
// them: it translates one axon.Event into one tea.Msg and hands it over.
//
// Nothing else in cortex may touch the terminal from a turn goroutine, and
// nothing else may interpret an axon.Event. Both rules are enforced by this
// being the only file that imports both worlds.

// Bridge forwards runtime events into a Bubble Tea program.
//
// The program does not exist when the agent is constructed — the agent needs
// an OnEvent handler, and the program needs the agent — so the pointer is
// filled in by Attach once both halves exist. Events emitted before Attach
// are dropped, which is correct: there is no screen to draw them on yet.
type Bridge struct {
	program atomic.Pointer[tea.Program]
}

// Attach binds the bridge to the running program. Safe to call once, before
// the program starts.
func (b *Bridge) Attach(p *tea.Program) {
	b.program.Store(p)
}

// Emit satisfies axon's Config.OnEvent signature.
//
// It runs on the turn goroutine and must stay cheap: translate, send, return.
// Every decision about how an event looks belongs in the view, not here.
func (b *Bridge) Emit(_ context.Context, e axon.Event) {
	program := b.program.Load()
	if program == nil {
		return
	}

	if msg := translate(e); msg != nil {
		program.Send(msg)
	}
}

// translate maps a runtime event to its UI message, or nil for events the UI
// has nothing to say about. Returning nil rather than a no-op message keeps
// the Update switch free of cases that do nothing.
func translate(e axon.Event) tea.Msg {
	switch e.Kind {
	case axon.KindSessionStart:
		if e.Session == nil {
			return nil
		}
		return sessionStartMsg{Provider: e.Session.Provider, Model: e.Session.Model, PrunerOn: e.Session.PrunerOn}

	case axon.KindSessionEnd:
		return sessionEndMsg{}

	case axon.KindUserInput:
		return userInputMsg(e.Text)

	case axon.KindTurnStart:
		return turnStartMsg{Turn: e.Turn}

	case axon.KindTurnEnd:
		return turnEndMsg{Turn: e.Turn}

	case axon.KindAPICall:
		return apiCallMsg{Turn: e.Turn}

	case axon.KindToolArgDelta:
		if e.Tool == nil || e.Tool.ArgsDelta == "" {
			return nil
		}
		return toolArgDeltaMsg{ID: e.Tool.ID, Name: e.Tool.Name, Delta: e.Tool.ArgsDelta}

	case axon.KindToken:
		return tokenMsg(e.Text)

	case axon.KindReasoning:
		return reasoningMsg(e.Text)

	case axon.KindAssistantEnd:
		return assistantMsg(e.Text)

	case axon.KindToolCall:
		if e.Tool == nil {
			return nil
		}
		return toolCallMsg{ID: e.Tool.ID, Name: e.Tool.Name, Args: e.Tool.Args}

	case axon.KindToolResult:
		if e.Tool == nil {
			return nil
		}
		return toolResultMsg{ID: e.Tool.ID, Name: e.Tool.Name, Result: e.Tool.Result}

	case axon.KindToolError:
		if e.Tool == nil {
			return nil
		}
		return toolErrorMsg{ID: e.Tool.ID, Name: e.Tool.Name, Err: e.Err}

	case axon.KindPruneStart:
		if e.Prune == nil {
			return nil
		}
		return pruneStartMsg{Before: e.Prune.Before}

	case axon.KindPruneEnd:
		if e.Prune == nil {
			return nil
		}
		return pruneEndMsg{Before: e.Prune.Before, After: e.Prune.After, Rejected: e.Prune.Rejected}

	case axon.KindInfo:
		return noticeMsg(e.Text)

	case axon.KindError:
		return runtimeErrorMsg{Err: e.Err, Text: e.Text}

	case axon.KindUsage:
		if e.Usage == nil {
			return nil
		}
		return usageMsg{PromptTokens: e.Usage.PromptTokens, CompletionTokens: e.Usage.CompletionTokens}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Messages
//
// One message per thing the UI reacts to. They are values, not pointers, so
// a message in flight can never be mutated by the goroutine that sent it.
// ---------------------------------------------------------------------------

// tokenMsg is one chunk of streamed assistant text.
type tokenMsg string

// usageMsg carries token accounting for a completed API call.
type usageMsg struct {
	PromptTokens     int
	CompletionTokens int
}

// reasoningMsg is one chunk of streamed reasoning text.
type reasoningMsg string

// assistantMsg is the assistant's finished text for a turn. It arrives after
// the tokens that composed it and carries the authoritative full string, so
// the view commits this rather than its own accumulated buffer.
type assistantMsg string

// toolCallMsg opens a tool card.
type toolCallMsg struct {
	ID   string
	Name string
	Args json.RawMessage
}

// toolArgDeltaMsg is one streamed fragment of a tool call's arguments,
// arriving before the call itself resolves into a toolCallMsg. Applied to
// the matching open card so a long write's contents grow on screen as the
// model produces them, rather than appearing all at once at the end.
type toolArgDeltaMsg struct {
	ID    string
	Name  string
	Delta string
}

// toolResultMsg closes a tool card successfully.
type toolResultMsg struct {
	ID     string
	Name   string
	Result string
}

// toolErrorMsg closes a tool card with a failure.
type toolErrorMsg struct {
	ID   string
	Name string
	Err  error
}

// noticeMsg is runtime chatter worth showing but not worth alarm — a retry
// notice, for instance.
type noticeMsg string

// runtimeErrorMsg is an error the runtime recovered from or is reporting
// mid-turn. A fatal one arrives as turnDoneMsg.Err instead.
type runtimeErrorMsg struct {
	Err  error
	Text string
}

// pruneStartMsg signals the curator is looking at the context.
type pruneStartMsg struct {
	Before int
}

// pruneEndMsg signals the curator has parked stale context.
type pruneEndMsg struct {
	Before int
	After  int

	// Rejected lists block ids the curator named that could not be parked.
	// Not a failure — the pass parked everything it legitimately could — but
	// a steady stream of them means the curator is inventing ids.
	Rejected []string
}

// turnDoneMsg reports that Step returned. It is produced by the command that
// ran the turn, not by the bridge, because it is the completion of work the
// UI itself started.
type turnDoneMsg struct {
	Err error
}

// sessionStartMsg announces the runtime a session is bound to.
type sessionStartMsg struct {
	Provider string
	Model    string
	PrunerOn bool
}

// sessionEndMsg marks the end of a session.
type sessionEndMsg struct{}

// userInputMsg echoes the input the runtime received for a turn, as reported
// by axon itself rather than by cortex's own input handling.
type userInputMsg string

// turnStartMsg and turnEndMsg mark a turn's boundaries as the runtime sees
// them, independent of the UI's own busy/idle bookkeeping.
type turnStartMsg struct{ Turn int }
type turnEndMsg struct{ Turn int }

// apiCallMsg marks a model API request beginning.
type apiCallMsg struct{ Turn int }

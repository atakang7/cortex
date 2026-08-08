package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/atakang7/axon/v2"
)

type modelItem struct {
	provider string
	model    string
}

func (i modelItem) Title() string       { return i.model }
func (i modelItem) Description() string { return i.provider }
func (i modelItem) FilterValue() string { return i.provider + "/" + i.model }

// model.go holds the UI's state and the rules for changing it. Everything the
// user sees is derived from these fields; nothing here writes to the terminal
// directly, and nothing here calls the model API. Update reacts, view.go
// draws, and the turn itself runs in a command.
//
// The transcript is not kept in memory. A block is rendered once and handed
// to the terminal's own scrollback with tea.Println, which is what lets a
// long session scroll, copy and paste like ordinary command output. Only
// work still in flight lives in this struct.

// Options configures a Model. Every field is required.
type Options struct {
	// Agent is the runtime this UI drives.
	Agent *axon.Agent

	// Context is the parent context for every turn. Cancelling it — via a
	// signal, typically — ends the program.
	Context context.Context

	// ModelName is shown in the status line, e.g. "gpt-4o".
	ModelName string

	// PrunerName is the curator model shown in the status line, e.g.
	// "gpt-4o-mini". Empty means no pruner is configured.
	PrunerName string

	// AgentName labels the personality in the banner, e.g. "reviewer".
	AgentName string

	// Settings provides the available endpoints and models for the picker.
	Settings axon.Settings
}

// Model is the Bubble Tea model. It is passed by value, so every field must
// be safe to copy — which is why the streamed text is a string rather than a
// strings.Builder.
type Model struct {
	agent   *axon.Agent
	turnCtx context.Context

	modelName  string
	prunerName string
	agentName  string

	input   textarea.Model
	spinner spinner.Model
	width   int

	settings      axon.Settings
	picker        list.Model
	pickingModel  bool
	pickingPruner bool

	// busy is true from submitting a turn until Step returns. It gates input
	// and drives the spinner.
	busy      bool
	turnStart time.Time

	// interrupted records that the user asked to cancel this turn, so the
	// ErrInterrupted that comes back is reported as a choice, not a failure.
	interrupted bool

	// stream is the assistant text received so far this turn, held only
	// until the authoritative full text arrives and is committed.
	stream string

	// reasoning is the model's thinking for the current step, committed when
	// the step produces its first real output.
	reasoning string

	// cards are tool calls still running. A finished card is committed to
	// scrollback and dropped from this slice, so it is normally empty or has
	// a single entry.
	cards []toolCard
}

// New builds the model. The program is not running yet, so this only
// constructs — the first frame is produced by Init.
func New(opts Options) Model {
	m := Model{
		agent:      opts.Agent,
		turnCtx:    opts.Context,
		modelName:  opts.ModelName,
		prunerName: opts.PrunerName,
		agentName:  opts.AgentName,
		input:      newInput(),
		spinner:    newSpinner(),
		settings:   opts.Settings,
	}

	var items []list.Item
	for pName, p := range opts.Settings.Providers {
		for _, mName := range p.ModelNames() {
			items = append(items, modelItem{provider: pName, model: mName})
		}
	}
	m.picker = list.New(items, list.NewDefaultDelegate(), defaultWidth, 14)
	m.picker.Title = "Select Model"
	m.picker.SetShowStatusBar(false)
	m.picker.SetFilteringEnabled(true)

	// Size everything now rather than waiting for the terminal to report
	// itself, so the very first frame is laid out correctly even if the
	// WindowSizeMsg is late or never arrives.
	return m.resize(defaultWidth)
}

// defaultWidth is used until the terminal reports its real size, which it
// does within the first frame.
const defaultWidth = 80

func newInput() textarea.Model {
	input := textarea.New()

	input.Placeholder = "ask for a change, or /help"
	input.ShowLineNumbers = false
	input.CharLimit = 0
	input.SetHeight(1)

	// Enter submits, so the newline binding moves to the chords a terminal
	// can actually deliver. Enter itself is intercepted in Update and never
	// reaches the textarea.
	input.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("ctrl+j", "alt+enter"),
		key.WithHelp("ctrl+j", "newline"),
	)

	// The frame around the input draws the border; the textarea contributes
	// no chrome of its own beyond the prompt glyph on its first line.
	input.SetPromptFunc(promptWidth, func(lineIndex int) string {
		if lineIndex == 0 {
			return glyphPrompt + " "
		}

		return strings.Repeat(" ", promptWidth)
	})

	input.FocusedStyle = inputStyle()
	input.BlurredStyle = inputStyle()

	input.Focus()

	return input
}

// elapsedTickMsg drives redraws of the "thinking Ns" counter while a turn is
// in flight. It carries no data — elapsed() reads m.turnStart fresh each
// render — its only job is to force Update/View to run on a schedule the
// spinner's own tick chain doesn't guarantee.
type elapsedTickMsg struct{}

// elapsedTickInterval is coarse enough not to burn CPU on a redraw loop and
// fine enough that "thinking 3s" never visibly lags real time.
const elapsedTickInterval = 500 * time.Millisecond

func tickElapsed() tea.Cmd {
	return tea.Tick(elapsedTickInterval, func(time.Time) tea.Msg { return elapsedTickMsg{} })
}

func newSpinner() spinner.Model {
	s := spinner.New()

	s.Spinner = spinner.MiniDot
	s.Style = styleSpinner

	return s
}

// Init prints the banner and starts the spinner ticking.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.Println(m.banner()),
		textarea.Blink,
		m.spinner.Tick,
	)
}

// Update is the single place UI state changes. It is organised by where a
// message came from: the terminal, the runtime, or the turn command.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.pickingModel || m.pickingPruner {
		var cmd tea.Cmd
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.Type == tea.KeyEsc {
				m.pickingModel = false
				m.pickingPruner = false
				return m, nil
			}
			if msg.Type == tea.KeyEnter {
				i, ok := m.picker.SelectedItem().(modelItem)
				notice := ""
				if ok {
					newModel, err := m.settings.NewClient(i.provider, i.model)
					if err == nil {
						if m.pickingPruner {
							m.agent.SetPrunerModel(newModel)
							m.prunerName = i.model
							notice = "pruner changed to " + i.model
						} else {
							m.agent.SetModel(newModel)
							m.modelName = i.model
							notice = "model changed to " + i.model
						}
					}
				}
				m.pickingModel = false
				m.pickingPruner = false
				return m, tea.Println(renderNotice(notice, m.width))
			}
		case tea.WindowSizeMsg:
			m.picker.SetWidth(msg.Width)
			m.picker.SetHeight(msg.Height)
		}
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		return m.resize(msg.Width), nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)

		return m, cmd

	case elapsedTickMsg:
		// The elapsed counter reads m.turnStart fresh on every render, so this
		// message exists only to force one — no state to update. It stops
		// rescheduling itself the moment the turn is no longer busy, rather
		// than relying on the spinner's own tick chain, which can silently
		// stop advancing if a tick is ever dropped.
		if !m.busy {
			return m, nil
		}

		return m, tickElapsed()

	// ----- session and turn boundaries --------------------------------------

	case sessionStartMsg:
		text := fmt.Sprintf("session started · %s/%s", msg.Provider, msg.Model)
		if msg.PrunerOn {
			text += " · pruner on"
		}
		return m, tea.Println(renderNotice(text, m.width))

	case sessionEndMsg:
		return m, tea.Println(renderNotice("session ended", m.width))

	case userInputMsg:
		return m, tea.Println(renderNotice(fmt.Sprintf("input · %s", string(msg)), m.width))

	case turnStartMsg:
		return m, tea.Println(renderNotice(fmt.Sprintf("turn %d started", msg.Turn), m.width))

	case turnEndMsg:
		return m, tea.Println(renderNotice(fmt.Sprintf("turn %d ended", msg.Turn), m.width))

	case apiCallMsg:
		return m, tea.Println(renderNotice(fmt.Sprintf("turn %d · calling model...", msg.Turn), m.width))

	// ----- streamed output -------------------------------------------------

	case tokenMsg:
		commit := m.commitReasoning()
		m.stream += string(msg)

		return m, commit

	case reasoningMsg:
		m.reasoning += string(msg)

		return m, nil

	case assistantMsg:
		commit := m.commitReasoning()
		m.stream = ""

		return m, tea.Sequence(commit, tea.Println(renderAssistant(string(msg), m.width)))

	// ----- tool calls ------------------------------------------------------

	case toolCallMsg:
		commit := m.commitReasoning()
		if index := m.findCard(msg.ID, msg.Name); index >= 0 {
			m.cards[index].Args = msg.Args
		} else {
			m.cards = append(m.cards, toolCard{ID: msg.ID, Name: msg.Name, Args: msg.Args})
		}

		return m, commit

	case toolArgDeltaMsg:
		// Deltas can arrive before the call resolves into a toolCallMsg, so a
		// card may not exist yet — open one early rather than dropping the
		// fragment, since KindToolCall for the same ID will still land on
		// top of it once the runtime knows the full args.
		if index := m.findCard(msg.ID, msg.Name); index >= 0 {
			m.cards[index].Args = append(m.cards[index].Args, msg.Delta...)
		} else {
			m.cards = append(m.cards, toolCard{ID: msg.ID, Name: msg.Name, Args: json.RawMessage(msg.Delta)})
		}

		return m, nil

	case toolResultMsg:
		return m.closeCard(msg.ID, msg.Name, func(c *toolCard) {
			c.Result = msg.Result
		})

	case toolErrorMsg:
		return m.closeCard(msg.ID, msg.Name, func(c *toolCard) {
			c.Err = msg.Err
		})

	// ----- runtime chatter -------------------------------------------------

	case pruneStartMsg:
		return m, tea.Println(renderPruner(fmt.Sprintf("pruning context (%s tokens)...", compactCount(msg.Before)), m.width))

	case pruneEndMsg:
		text := fmt.Sprintf("context pruned · %s → %s tokens",
			compactCount(msg.Before), compactCount(msg.After))
		// The curator naming a block it could not park is a note about the
		// curator, not a failed pass — the arrow above already says what the
		// pass achieved.
		if n := len(msg.Rejected); n > 0 {
			text += fmt.Sprintf(" · ignored %d invented id(s): %s",
				n, strings.Join(msg.Rejected, ", "))
		}
		return m, tea.Println(renderPruner(text, m.width))

	case noticeMsg:
		return m, tea.Println(renderNotice(string(msg), m.width))

	case runtimeErrorMsg:
		if isPrunerErr(msg.Err) {
			return m, tea.Println(renderPruner("pruning failed: "+strings.TrimPrefix(msg.Err.Error(), "pruner: "), m.width))
		}

		if text := errorText(msg.Err, msg.Text); text != "" {
			return m, tea.Println(renderNotice(text, m.width))
		}

		return m, nil

	// ----- the turn --------------------------------------------------------

	case turnDoneMsg:
		return m.finishTurn(msg.Err)
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Terminal input
// ---------------------------------------------------------------------------

func (m Model) resize(width int) Model {
	m.width = width
	m.input.SetWidth(width - inputChromeWidth)

	return m
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {

	case tea.KeyCtrlC:
		return m.cancel()

	case tea.KeyEsc:
		if m.busy {
			return m.cancel()
		}

		return m, nil

	case tea.KeyCtrlD:
		if m.input.Value() == "" && !m.busy {
			return m, tea.Quit
		}

	case tea.KeyEnter:
		if m.busy {
			return m, nil
		}

		return m.submit()
	}

	// Typing during a turn is allowed and queues up in the box; only Enter is
	// refused. That way a user can compose the next request while waiting.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.input.SetHeight(inputHeight(m.input.LineCount()))

	return m, cmd
}

// cancel interrupts a running turn, or exits when nothing is running. This is
// the Ctrl-C contract: it always stops the innermost thing first.
func (m Model) cancel() (tea.Model, tea.Cmd) {
	if m.busy {
		m.interrupted = true
		m.agent.Interrupt()

		return m, nil
	}

	if m.input.Value() != "" {
		m.input.Reset()
		m.input.SetHeight(1)

		return m, nil
	}

	return m, tea.Quit
}

// submit takes what is in the box and either runs it as a command or sends it
// to the model.
func (m Model) submit() (tea.Model, tea.Cmd) {
	line := strings.TrimSpace(m.input.Value())
	if line == "" {
		return m, nil
	}

	m.input.Reset()
	m.input.SetHeight(1)

	echo := tea.Println(renderUser(line, m.width))

	if isCommand(line) {
		updated, cmd := m.applyCommand(runCommand(m.agent, line))

		return updated, tea.Sequence(echo, cmd)
	}

	m.busy = true
	m.interrupted = false
	m.turnStart = time.Now()

	return m, tea.Sequence(echo, tea.Batch(m.runTurn(line), tickElapsed()))
}

// runTurn drives one Step on its own goroutine. Output does not come back
// through this command — it arrives as messages from the Bridge while Step is
// still running. Only the completion is reported here.
func (m Model) runTurn(input string) tea.Cmd {
	agent, ctx := m.agent, m.turnCtx

	return func() tea.Msg {
		_, err := agent.Step(ctx, input)

		return turnDoneMsg{Err: err}
	}
}

// applyCommand turns a command's report into state changes and output. The
// command decided what happened; this decides what the screen does about it.
func (m Model) applyCommand(res commandResult) (tea.Model, tea.Cmd) {
	if res.Reset {
		m.stream = ""
		m.reasoning = ""
		m.cards = nil
	}

	if res.Quit {
		return m, tea.Quit
	}

	if res.Err != nil {
		return m, tea.Println(renderError(res.Err.Error(), m.width))
	}

	if res.Notice != "" {
		return m, tea.Println(renderNotice(res.Notice, m.width))
	}

	if res.SelectModel {
		m.pickingModel = true
		m.picker.Title = "Select Model"
		return m, nil
	}

	if res.SelectPruner {
		m.pickingPruner = true
		m.picker.Title = "Select Pruner"
		return m, nil
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Committing work to scrollback
// ---------------------------------------------------------------------------

// commitReasoning flushes buffered thinking, if any. It is called wherever
// thinking is known to have ended — the first content token, a tool call, or
// the end of the assistant's text.
func (m *Model) commitReasoning() tea.Cmd {
	if m.reasoning == "" {
		return nil
	}

	block := renderReasoning(m.reasoning, m.width)
	m.reasoning = ""

	if block == "" {
		return nil
	}

	return tea.Println(block)
}

// closeCard completes an in-flight tool card, commits it, and drops it from
// the live area. The mutation is passed in so success and failure share one
// path through the lookup and the commit.
func (m Model) closeCard(id, name string, complete func(*toolCard)) (tea.Model, tea.Cmd) {
	index := m.findCard(id, name)
	if index < 0 {
		return m, nil
	}

	card := m.cards[index]
	card.Done = true
	complete(&card)

	m.cards = append(m.cards[:index:index], m.cards[index+1:]...)

	return m, tea.Println(card.render(m.width))
}

// findCard locates an in-flight card. Matching prefers the call ID, but falls
// back to the tool name because a provider that omits IDs on results would
// otherwise strand every card as permanently running.
func (m Model) findCard(id, name string) int {
	if id != "" {
		for i, c := range m.cards {
			if c.ID == id {
				return i
			}
		}
	}

	for i, c := range m.cards {
		if c.Name == name && !c.Done {
			return i
		}
	}

	return -1
}

// finishTurn settles everything left over when Step returns: uncommitted
// text, cards the runtime never closed, and the outcome itself.
func (m Model) finishTurn(err error) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if cmd := m.commitReasoning(); cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Text that streamed but never arrived as a finished assistant message —
	// which is what an interrupt mid-sentence looks like.
	if strings.TrimSpace(m.stream) != "" {
		cmds = append(cmds, tea.Println(renderAssistant(m.stream, m.width)))
	}
	m.stream = ""

	// A card still open here means the turn ended while a tool was running.
	for _, card := range m.cards {
		card.Done = true
		if card.Err == nil && card.Result == "" {
			card.Err = errors.New("interrupted")
		}
		cmds = append(cmds, tea.Println(card.render(m.width)))
	}
	m.cards = nil

	switch {
	case m.interrupted || errors.Is(err, axon.ErrInterrupted):
		cmds = append(cmds, tea.Println(renderNotice("interrupted", m.width)))

	case errors.Is(err, context.Canceled):
		m.busy = false

		return m, tea.Quit

	case err != nil:
		cmds = append(cmds, tea.Println(renderError(err.Error(), m.width)))
	}

	m.busy = false
	m.interrupted = false

	return m, tea.Sequence(cmds...)
}

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

// isPrunerErr reports whether err came from a curator pass. Pruning is an
// optimisation, so its failure never interrupts the turn — but it is still
// reported as a quiet notice rather than dropped, so the user can tell the
// curator ran and what went wrong instead of guessing whether it exists.
func isPrunerErr(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "pruner:")
}

// errorText renders a mid-turn runtime error.
func errorText(err error, text string) string {
	if err == nil {
		return text
	}

	message := err.Error()
	if text != "" {
		return text + ": " + message
	}

	return message
}

// compactCount renders a token count the way a status line wants it.
func compactCount(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	return fmt.Sprintf("%.1fk", float64(n)/1000)
}

// cwdLabel is the working directory shortened to something that fits a status
// line: the last two path segments.
func cwdLabel(path string) string {
	if path == "" {
		return ""
	}

	parent, leaf := filepath.Split(filepath.Clean(path))
	if parent == "" || parent == string(filepath.Separator) {
		return leaf
	}

	return filepath.Base(filepath.Clean(parent)) + string(filepath.Separator) + leaf
}

package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"
)

// view.go draws the live area: the part of the screen that is redrawn every
// frame, as opposed to the transcript, which is committed once to the
// terminal's scrollback and never touched again.
//
// Keeping the live area small is the whole design. It holds only what is
// still changing — a tool that has not returned, text still streaming, the
// input box, the status line — so a long session costs the same to render as
// a short one, and the terminal's own scroll, search and copy keep working.

// Input geometry.
const (
	// promptWidth is the gutter the prompt glyph occupies inside the box.
	promptWidth = 2

	// inputChromeWidth is what the frame takes from the terminal width:
	// two border columns and two padding columns.
	inputChromeWidth = 4

	// maxInputHeight caps how far the box grows before it scrolls, so a
	// pasted wall of text cannot push the transcript off screen.
	maxInputHeight = 8
)

// lipglossNone is the zero style, used to strip a component's built-in
// decoration when this package draws that decoration itself.
var lipglossNone = lipgloss.NewStyle()

// inputStyle is the textarea's styling. Focused and blurred are identical:
// the box is always the only focusable thing on screen, so a "blurred" look
// would communicate nothing.
func inputStyle() textarea.Style {
	s := textarea.Style{
		Base:        lipglossNone,
		CursorLine:  lipglossNone,
		EndOfBuffer: lipglossNone,
		Placeholder: styleStatus,
		Prompt:      styleUserMark,
		Text:        styleAssistant,
	}

	return s
}

// inputHeight sizes the box to its content, within bounds.
func inputHeight(lines int) int {
	switch {
	case lines < 1:
		return 1

	case lines > maxInputHeight:
		return maxInputHeight
	}

	return lines
}

// ---------------------------------------------------------------------------
// The frame
// ---------------------------------------------------------------------------

// View renders the live area. Order matters: work in progress sits above the
// box so it reads as a continuation of the transcript, and the status line
// sits below so the box stays the visual centre.
func (m Model) View() string {
	if m.pickingModel || m.pickingPruner || m.pickingSession {
		return m.picker.View()
	}

	var sections []string

	if live := m.liveWork(); live != "" {
		sections = append(sections, live, "")
	}

	sections = append(sections, m.inputBox(), m.statusLine())

	return strings.Join(sections, "\n")
}

// liveWork renders everything happening right now: open tool cards, thinking
// in progress, text mid-stream, and the spinner when none of those has
// produced output yet.
func (m Model) liveWork() string {
	var blocks []string

	for _, card := range m.cards {
		blocks = append(blocks, card.render(m.width))
	}

	if block := renderReasoning(m.reasoning, m.width); block != "" {
		blocks = append(blocks, block)
	}

	if block := renderAssistant(m.stream, m.width); block != "" {
		blocks = append(blocks, block)
	}

	if len(blocks) == 0 && m.busy {
		blocks = append(blocks, m.waiting())
	}

	return strings.Join(blocks, "\n")
}

// waiting is what the user sees between submitting and the first output.
func (m Model) waiting() string {
	return indent + m.spinner.View() + " " + styleStatus.Render("thinking "+elapsed(m.turnStart))
}

// inputBox draws the bordered composer. Its border colour is the one signal
// that a turn is running, which is why it changes rather than the text.
func (m Model) inputBox() string {
	frame := styleInputFrame
	if m.busy {
		frame = styleInputFrameBusy
	}

	return frame.Width(m.width - 2).Render(m.input.View())
}

// ---------------------------------------------------------------------------
// Status line
// ---------------------------------------------------------------------------

// statusLine is the one persistent readout: where you are, what you are
// talking to, and which key does the thing you most likely want next.
func (m Model) statusLine() string {
	facts := []string{styleStatus.Render(m.modelName)}

	if m.prunerName != "" {
		facts = append(facts, stylePruner.Render("pruner: "+m.prunerName))
	} else {
		facts = append(facts, styleStatus.Render("pruner: off"))
	}

	if cwd := cwdLabel(m.agent.Session().Cwd); cwd != "" {
		facts = append(facts, styleStatus.Render(cwd))
	}

	if turn := m.agent.Session().Turn; turn > 0 {
		facts = append(facts, styleStatus.Render(fmt.Sprintf("turn %d", turn)))
	}

	if m.usage.lastPrompt > 0 {
		facts = append(facts, styleStatus.Render(
			fmt.Sprintf("ctx %d", m.usage.lastPrompt)))
	}

	if m.usage.prompt > 0 || m.usage.completion > 0 {
		facts = append(facts, styleStatus.Render(
			fmt.Sprintf("tokens %d↑ %d↓", m.usage.prompt, m.usage.completion)))
	}

	if m.busy {
		facts = append(facts, styleStatus.Render(elapsed(m.turnStart)))
	}

	line := indent + strings.Join(facts, styleStatus.Render(" · "))

	if hint := m.hint(); hint != "" {
		line += styleStatus.Render("  ·  ") + hint
	}

	return line
}

// hint names the key that matters in the current state, and nothing else. A
// full keymap belongs behind /help, not under every prompt.
func (m Model) hint() string {
	if m.busy {
		return styleStatusKey.Render("esc") + styleStatus.Render(" interrupt")
	}

	return styleStatusKey.Render("ctrl+j") + styleStatus.Render(" newline") +
		styleStatus.Render(" · ") +
		styleStatusKey.Render("/help") + styleStatus.Render(" commands")
}

// elapsed formats a turn's running time at a granularity that does not
// distract: whole seconds up to a minute, then minutes and seconds.
func elapsed(since time.Time) string {
	d := time.Since(since).Round(time.Second)

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}

	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

// ---------------------------------------------------------------------------
// Banner
// ---------------------------------------------------------------------------

// banner is printed once at startup. It states what is connected and where,
// because those are the two things a user checks before trusting the agent
// with a repository.
func (m Model) banner() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(styleBanner.Render(m.agentName))
	b.WriteString(styleStatus.Render("  " + m.modelName))

	if title := m.agent.Session().Title; title != "" {
		b.WriteString(styleStatus.Render("  · " + title))
	}

	if cwd := m.agent.Session().Cwd; cwd != "" {
		b.WriteString("\n")
		b.WriteString(styleStatus.Render(cwd))
	}

	return b.String()
}

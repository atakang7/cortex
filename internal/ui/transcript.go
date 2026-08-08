package ui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// transcript.go turns conversation events into blocks of styled text.
//
// Every function here is pure: state in, string out, no reads of the model
// and no writes to the terminal. That is what lets the same renderer serve
// both the live area (redrawn every frame) and the scrollback (rendered once
// and committed), with no chance of the two drifting apart.

// indent is the left gutter every block except the user's own line sits in.
// It gives the transcript a spine to read down.
const indent = "  "

// ---------------------------------------------------------------------------
// Conversation blocks
// ---------------------------------------------------------------------------

// renderUser echoes what the user submitted, so the committed scrollback
// reads as a conversation rather than a list of answers.
func renderUser(text string, width int) string {
	return block(styleUserText, styleUserMark.Render(glyphPrompt)+" ", indent, text, width)
}

// renderAssistant lays out the model's prose.
func renderAssistant(text string, width int) string {
	return block(styleAssistant, indent, indent, text, width)
}

// renderReasoning lays out the model's thinking. Kept visually quiet: it is
// context for a user who wants it, not content competing with the answer.
func renderReasoning(text string, width int) string {
	return block(styleReasoning, indent, indent, text, width)
}

// renderNotice lays out runtime chatter — retries, prune reports, the output
// of a slash command. It wraps and indents every line, because /help and
// /session both produce more than one.
func renderNotice(text string, width int) string {
	return block(styleNotice, indent, indent, text, width)
}

// renderError lays out a failure.
func renderError(text string, width int) string {
	return block(styleError, indent+glyphBad+" ", indent+"  ", text, width)
}

// block wraps text to the available width and styles it line by line.
//
// Styling each line separately, rather than handing the whole paragraph to a
// style with a Width, is deliberate: lipgloss pads a width-constrained block
// out to its full width, which litters the scrollback with trailing spaces
// that show up the moment anyone copies a transcript.
func block(style lipgloss.Style, firstPrefix, restPrefix, text string, width int) string {
	text = strings.TrimRight(text, "\n \t")
	if strings.TrimSpace(text) == "" {
		return ""
	}

	lines := strings.Split(wrap(text, textWidth(width, lipgloss.Width(firstPrefix))), "\n")

	for i, line := range lines {
		prefix := restPrefix
		if i == 0 {
			prefix = firstPrefix
		}

		lines[i] = prefix + style.Render(line)
	}

	return strings.Join(lines, "\n")
}

// wrap breaks text at word boundaries, splitting words that are longer than
// the limit rather than letting them overflow.
func wrap(text string, limit int) string {
	return ansi.Wrap(text, limit, "")
}

// ---------------------------------------------------------------------------
// Tool cards
// ---------------------------------------------------------------------------

// toolCard is one tool invocation as the user sees it. It is created when the
// call is announced and completed when the result arrives, so the same value
// renders first as in-flight and then as finished.
type toolCard struct {
	ID   string
	Name string
	Args json.RawMessage

	// Done separates "still running" from "returned nothing", which look
	// identical if you only test Result for emptiness.
	Done   bool
	Result string
	Err    error
}

// resultPreviewLines caps how much of a tool's output reaches the screen.
// The model gets the whole thing; the user gets enough to see it worked.
const resultPreviewLines = 8

// render draws the whole card. It is split into header and tail because the
// two renderers need the halves at different times: the live UI redraws the
// card as one block every frame, while the non-interactive renderer prints
// the header when the call starts and the tail when it returns, with the
// tool's own runtime in between.
func (c toolCard) render(width int) string {
	return c.header(width) + "\n" + c.tail(width)
}

// header names the tool and the one argument that says what the call is about.
func (c toolCard) header(width int) string {
	var b strings.Builder

	b.WriteString(indent)
	b.WriteString(styleToolRail.Render(glyphToolHead))
	b.WriteString(" ")
	b.WriteString(styleToolName.Render(c.Name))

	if subject := summarizeArgs(c.Name, c.Args); subject != "" {
		b.WriteString("  ")
		b.WriteString(styleToolArgs.Render(truncate(subject, textWidth(width, len(indent)+len(c.Name)+6))))
	}

	return b.String()
}

// tail is the output preview and the outcome line, or the running marker
// while the call is still in flight.
func (c toolCard) tail(width int) string {
	var b strings.Builder

	if !c.Done {
		b.WriteString(indent)
		b.WriteString(styleToolRail.Render(glyphToolTail + " "))
		b.WriteString(styleToolArgs.Render("running…"))

		return b.String()
	}

	for _, line := range c.previewLines(width) {
		b.WriteString(indent)
		b.WriteString(styleToolRail.Render(glyphToolRail + " "))
		b.WriteString(styleToolArgs.Render(line))
		b.WriteString("\n")
	}

	b.WriteString(indent)
	b.WriteString(styleToolRail.Render(glyphToolTail + " "))
	b.WriteString(c.footer())

	return b.String()
}

// previewLines returns the output lines worth showing, already truncated to
// the terminal width.
func (c toolCard) previewLines(width int) []string {
	body := strings.TrimRight(c.Result, "\n")
	if body == "" || c.Err != nil {
		return nil
	}

	lines := strings.Split(body, "\n")
	if len(lines) > resultPreviewLines {
		lines = lines[:resultPreviewLines]
	}

	limit := textWidth(width, len(indent)+2)
	for i, line := range lines {
		lines[i] = truncate(strings.TrimRight(line, "\r\t "), limit)
	}

	return lines
}

// footer states the outcome in one line: what went wrong, or how much came
// back.
func (c toolCard) footer() string {
	if c.Err != nil {
		return styleToolBad.Render(glyphBad + " " + c.Err.Error())
	}

	body := strings.TrimRight(c.Result, "\n")
	if body == "" {
		return styleToolOK.Render(glyphOK + " ok")
	}

	total := strings.Count(body, "\n") + 1
	if total > resultPreviewLines {
		return styleToolOK.Render(fmt.Sprintf("%s %d lines (%d shown)", glyphOK, total, resultPreviewLines))
	}

	return styleToolOK.Render(fmt.Sprintf("%s %d %s", glyphOK, total, plural(total, "line", "lines")))
}

// ---------------------------------------------------------------------------
// Argument summarising
// ---------------------------------------------------------------------------

// argSubjects names, per tool, the argument that identifies what the call is
// about. The header shows that one value rather than the whole payload: a
// user scanning a transcript wants to see which file, not which schema.
//
// A tool not listed here falls back to the first key it does have from the
// shared list below, then to compact JSON.
var argSubjects = map[string][]string{
	"read":        {"path"},
	"write":       {"path"},
	"search":      {"pattern", "query"},
	"exec":        {"command"},
	"task":        {"goal"},
	"bash_output": {"shell_id", "id"},
	"kill_shell":  {"shell_id", "id"},
}

// fallbackSubjects are tried for any tool with no entry above, which is every
// custom and MCP-provided tool.
var fallbackSubjects = []string{"path", "file", "command", "query", "name", "url", "pattern"}

// summarizeArgs picks the one value that says what a call is about.
func summarizeArgs(tool string, args json.RawMessage) string {
	if len(args) == 0 {
		return ""
	}

	var fields map[string]any
	if err := json.Unmarshal(args, &fields); err != nil {
		return compact(string(args))
	}

	keys := argSubjects[tool]
	if keys == nil {
		keys = fallbackSubjects
	}

	for _, key := range keys {
		if v, ok := fields[key]; ok {
			if s := scalar(v); s != "" {
				return s + modeSuffix(fields)
			}
		}
	}

	return compact(string(args))
}

// modeSuffix appends the call's mode when it has one, because "write" alone
// does not distinguish creating a file from replacing four lines in it.
func modeSuffix(fields map[string]any) string {
	mode, ok := fields["mode"].(string)
	if !ok || mode == "" {
		return ""
	}

	return "  " + mode
}

// scalar renders a JSON value as a single line, or "" if it is a structure
// that would not fit a header.
func scalar(v any) string {
	switch t := v.(type) {
	case string:
		return compact(t)

	case float64:
		return strings.TrimSuffix(fmt.Sprintf("%v", t), ".0")

	case bool:
		return fmt.Sprintf("%t", t)
	}

	return ""
}

// ---------------------------------------------------------------------------
// Text helpers
// ---------------------------------------------------------------------------

// textWidth is the room left for content after a gutter, floored so a very
// narrow terminal degrades instead of producing negative widths.
func textWidth(total, gutter int) int {
	const minimum = 20

	if w := total - gutter; w > minimum {
		return w
	}

	return minimum
}

// compact collapses whitespace so a multi-line value can sit on one line.
func compact(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// truncate shortens to limit, marking the cut with an ellipsis.
func truncate(s string, limit int) string {
	if limit < 4 || lipgloss.Width(s) <= limit {
		return s
	}

	runes := []rune(s)
	if len(runes) > limit-1 {
		runes = runes[:limit-1]
	}

	return string(runes) + "…"
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}

	return many
}

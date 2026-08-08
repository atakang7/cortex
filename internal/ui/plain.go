package ui

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/atakang7/axon"
)

// plain.go is the non-interactive renderer, used by `cortex --prompt`. There
// is no Bubble Tea program here and no live area: output is written straight
// through as it arrives.
//
// The stream split is the point of this file. The assistant's answer goes to
// stdout and everything else — tool activity, notices, errors — goes to
// stderr, so `cortex --prompt "…" > answer.txt` captures the answer and
// nothing else, while the operator still watches the work happen on the
// terminal.

// Plain renders runtime events for a single non-interactive run.
type Plain struct {
	out io.Writer
	err io.Writer

	// width bounds tool-card rendering, which has no terminal to measure in
	// this mode.
	width int

	// atLineStart tracks whether the last byte written ended a line, so a
	// tool card never begins mid-sentence after streamed text.
	atLineStart bool

	// open holds the cards whose tools have not returned. It exists because
	// a failed tool produces two events — an error and then a result — and
	// only the first of them may print a footer.
	open map[string]toolCard
}

// plainWidth is the assumed terminal width for non-interactive output. Fixed
// rather than measured because the destination is as likely to be a pipe or a
// CI log as a terminal.
const plainWidth = 100

// NewPlain builds a renderer writing answers to out and everything else to
// errOut.
func NewPlain(out, errOut io.Writer) *Plain {
	return &Plain{
		out:         out,
		err:         errOut,
		width:       plainWidth,
		atLineStart: true,
		open:        map[string]toolCard{},
	}
}

// Emit satisfies axon's Config.OnEvent signature.
func (p *Plain) Emit(_ context.Context, e axon.Event) {
	switch e.Kind {

	case axon.KindToken:
		if e.Text == "" {
			return
		}
		fmt.Fprint(p.out, e.Text)
		p.atLineStart = strings.HasSuffix(e.Text, "\n")

	case axon.KindToolCall:
		if e.Tool == nil {
			return
		}
		card := toolCard{ID: e.Tool.ID, Name: e.Tool.Name, Args: e.Tool.Args}
		p.open[cardKey(e.Tool.ID, e.Tool.Name)] = card
		p.line(card.header(p.width))

	case axon.KindToolResult:
		if e.Tool == nil {
			return
		}
		p.closeCard(e.Tool.ID, e.Tool.Name, func(c *toolCard) {
			c.Result = e.Tool.Result
		})

	case axon.KindToolError:
		if e.Tool == nil {
			return
		}
		p.closeCard(e.Tool.ID, e.Tool.Name, func(c *toolCard) {
			c.Err = e.Err
		})

	case axon.KindPruneStart:
		if e.Prune == nil {
			return
		}
		p.line(renderPruner(fmt.Sprintf("pruning context (%s tokens)...", compactCount(e.Prune.Before)), p.width))

	case axon.KindPruneEnd:
		if e.Prune == nil {
			return
		}
		p.line(renderPruner(fmt.Sprintf("context pruned · %s → %s tokens",
			compactCount(e.Prune.Before), compactCount(e.Prune.After)), p.width))

	case axon.KindInfo:
		p.line(renderNotice(e.Text, p.width))

	case axon.KindError:
		if isPrunerErr(e.Err) {
			p.line(renderPruner("pruning failed: "+strings.TrimPrefix(e.Err.Error(), "pruner: "), p.width))
			return
		}

		if text := errorText(e.Err, e.Text); text != "" {
			p.line(renderError(text, p.width))
		}
	}
}

// closeCard completes an open card and prints its outcome. A card that is
// already gone is ignored, which is what makes a failed tool print one
// footer: axon reports the failure as an error and then again as a result,
// and only the first of the two finds the card still open.
func (p *Plain) closeCard(id, name string, complete func(*toolCard)) {
	key, ok := p.findCard(id, name)
	if !ok {
		return
	}

	card := p.open[key]
	delete(p.open, key)

	card.Done = true
	complete(&card)

	p.line(card.tail(p.width))
}

// findCard locates an open card. Matching prefers the call ID but falls back
// to the tool name, because a provider that omits the ID on results would
// otherwise leave every card open forever.
func (p *Plain) findCard(id, name string) (string, bool) {
	if id != "" {
		if _, ok := p.open[id]; ok {
			return id, true
		}
	}

	for key, card := range p.open {
		if card.Name == name {
			return key, true
		}
	}

	return "", false
}

// cardKey identifies an in-flight call, keyed by ID when the provider gives
// one and by tool name when it does not.
func cardKey(id, name string) string {
	if id != "" {
		return id
	}

	return "name:" + name
}

// line writes a block to stderr, guaranteeing it starts on a fresh line.
func (p *Plain) line(text string) {
	if text == "" {
		return
	}

	if !p.atLineStart {
		fmt.Fprintln(p.err)
	}

	fmt.Fprintln(p.err, text)
	p.atLineStart = true
}

// Close finishes the output with a trailing newline when the answer did not
// end with one, so a shell prompt does not land mid-line.
func (p *Plain) Close() {
	if p.atLineStart {
		return
	}

	fmt.Fprintln(p.out)
	p.atLineStart = true
}

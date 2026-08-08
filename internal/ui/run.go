package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

// run.go starts the interactive UI. It exists as its own file because it is
// the only place that resolves the construction cycle at the heart of this
// program: the agent needs an event handler before it exists, and the event
// handler needs a program that cannot be built without the agent.
//
// The cycle is broken by Bridge, which is created first and empty, handed to
// the agent as its handler, and filled in here once the program exists. Any
// event the runtime emits in the gap is dropped, which is correct — there is
// no terminal to draw it on until Run is called.

// Run starts the terminal UI and blocks until the user exits.
//
// The bridge must be the same one whose Emit was given to axon as
// Config.OnEvent. Passing a different one produces a UI that renders nothing
// and gives no error, so this is worth getting right at the call site.
func Run(ctx context.Context, bridge *Bridge, opts Options) error {
	program := tea.NewProgram(
		New(opts),

		// No alternate screen. The transcript is committed to the terminal's
		// own scrollback, so the session stays scrollable, searchable and
		// copyable after cortex exits — the single most important property
		// this UI has, and the one an altscreen would destroy.
		tea.WithContext(ctx),
	)

	bridge.Attach(program)

	_, err := program.Run()

	return err
}

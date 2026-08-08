package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/atakang7/axon"
)

// command.go owns the slash commands: input that cortex handles itself
// instead of sending to the model.
//
// A handler never touches the terminal and never mutates the UI. It acts on
// the agent, then describes what happened in a result the model applies. That
// split is what keeps "what a command does" readable in one table, separate
// from "how the screen changes".

// commandResult is what a handler reports back. The zero value means the
// command did nothing worth saying.
type commandResult struct {
	// Notice is shown to the user as runtime chatter.
	Notice string

	// Err marks the command as failed; Notice is ignored when it is set.
	Err error

	// Quit ends the program.
	Quit bool

	// Reset tells the UI to clear its own transcript state, because the
	// session behind it was wiped.
	Reset bool

	// SelectModel tells the UI to open the model selection list.
	SelectModel bool
}

// command is one slash command.
type command struct {
	// Name is typed with a leading slash, e.g. "/undo".
	Name string

	// Arg names the argument in help output, empty when there is none.
	Arg string

	// Help is the one-line description.
	Help string

	// Run performs the command. It receives the argument with surrounding
	// space already trimmed.
	Run func(agent *axon.Agent, arg string) commandResult
}

// commands is the whole set, keyed by name. Adding a command means adding one
// entry here and nothing else.
var commands = map[string]command{
	"/new": {
		Name: "/new",
		Help: "wipe the session and start over",
		Run: func(agent *axon.Agent, _ string) commandResult {
			agent.Reset()

			return commandResult{Notice: "session cleared", Reset: true}
		},
	},

	"/undo": {
		Name: "/undo",
		Help: "revert the last file edit, byte for byte",
		Run: func(agent *axon.Agent, _ string) commandResult {
			path, ok := agent.Undo()
			if !ok {
				return commandResult{Notice: "nothing to undo"}
			}

			return commandResult{Notice: "reverted " + path}
		},
	},

	"/cd": {
		Name: "/cd",
		Arg:  "<path>",
		Help: "change the working directory",
		Run: func(agent *axon.Agent, arg string) commandResult {
			if arg == "" {
				return commandResult{Err: fmt.Errorf("/cd needs a path")}
			}

			cwd, err := agent.Cd(arg)
			if err != nil {
				return commandResult{Err: err}
			}

			return commandResult{Notice: "cwd " + cwd}
		},
	},

	"/pwd": {
		Name: "/pwd",
		Help: "show the working directory",
		Run: func(agent *axon.Agent, _ string) commandResult {
			return commandResult{Notice: agent.Session().Cwd}
		},
	},

	"/session": {
		Name: "/session",
		Help: "show session file, turn count and pending undos",
		Run: func(agent *axon.Agent, _ string) commandResult {
			session := agent.Session()

			return commandResult{Notice: fmt.Sprintf("turn %d · %d messages · %d %s · %s",
				session.Turn,
				len(session.Messages),
				len(session.Edits), plural(len(session.Edits), "edit", "edits"),
				agent.SessionPath(),
			)}
		},
	},

	"/model": {
		Name: "/model",
		Help: "select a different model for this session",
		Run: func(*axon.Agent, string) commandResult {
			return commandResult{SelectModel: true}
		},
	},

	"/quit": {
		Name: "/quit",
		Help: "exit cortex",
		Run: func(*axon.Agent, string) commandResult {
			return commandResult{Quit: true}
		},
	},
}

// /help is registered here rather than in the literal above because its
// handler renders that literal, and Go rejects a package-level variable whose
// initialiser refers to itself through a function. Registering it separately
// keeps one entry per command, which listing it by hand in helpText would not.
func init() {
	commands["/help"] = command{
		Name: "/help",
		Help: "list these commands",
		Run: func(*axon.Agent, string) commandResult {
			return commandResult{Notice: helpText()}
		},
	}
}

// aliases are alternate spellings. They are kept out of the table above so
// help output lists each command exactly once.
var aliases = map[string]string{
	"/exit": "/quit",
	"/q":    "/quit",
	"/?":    "/help",
	"/h":    "/help",
}

// isCommand reports whether a line should be handled locally rather than sent
// to the model. A bare "/" is not a command — it is someone typing a path.
func isCommand(line string) bool {
	return len(line) > 1 && strings.HasPrefix(line, "/") && !strings.HasPrefix(line, "//")
}

// runCommand dispatches a line already known to be a command.
func runCommand(agent *axon.Agent, line string) commandResult {
	name, arg, _ := strings.Cut(line, " ")
	name = strings.ToLower(name)

	if target, ok := aliases[name]; ok {
		name = target
	}

	cmd, ok := commands[name]
	if !ok {
		return commandResult{Err: fmt.Errorf("unknown command %s — try /help", name)}
	}

	return cmd.Run(agent, strings.TrimSpace(arg))
}

// helpText renders the command table, sorted so the list is stable between
// runs rather than following Go's map iteration order.
func helpText() string {
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	for i, name := range names {
		cmd := commands[name]

		usage := cmd.Name
		if cmd.Arg != "" {
			usage += " " + cmd.Arg
		}

		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%-16s %s", usage, cmd.Help)
	}

	return b.String()
}

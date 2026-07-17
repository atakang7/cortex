// Command cortex is a terminal coding agent built on the axon runtime.
//
// All runtime logic lives in github.com/atakang7/axon/agent. This binary
// wires the runtime to a terminal: an interactive provider picker, a
// YAML loader for agent personalities, a colored TTY renderer, and
// slash commands.
//
// If you want to embed the runtime from your own Go code (HTTP server,
// orchestrator, alternate UI), import the agent package directly and
// see https://github.com/atakang7/axon/tree/main/examples/minimal for
// the minimum-viable embed. cortex is one product on top of axon.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/atakang7/axon/agent"
)

func main() {
	var (
		flagPrompt = flag.String("prompt", "", "Run a single prompt non-interactively and exit when the assistant emits a final reply. Requires LLM_PROVIDER env to be set to skip the provider picker.")
		flagAgent  = flag.String("agent", "", "Named agent config to load from $CORTEX_AGENTS_DIR (default ~/.config/cortex/agents/<name>.yaml). Empty = built-in default coding agent.")
	)
	flag.Parse()

	agentCfg, err := LoadAgentConfig(*flagAgent)
	if err != nil {
		fmt.Fprintln(os.Stderr, "agent config:", err)
		os.Exit(1)
	}

	nonInteractive := *flagPrompt != ""
	if nonInteractive {
		uiSilent = true
	}

	providers, err := agent.LoadProviders()
	if err != nil {
		uiError(err)
		return
	}
	lc := loadLastChoice()

	var (
		p       agent.Provider
		mainKey string
	)
	if nonInteractive {
		p, err = agent.ResolveProvider(providers)
		if err != nil {
			fmt.Fprintln(os.Stderr, "non-interactive mode requires LLM_PROVIDER:", err)
			os.Exit(1)
		}
		mainKey = canonicalKey(providers, p)
	} else {
		p, mainKey, err = resolveProviderInteractive(providers, lc.Main)
		if err != nil {
			uiError(err)
			return
		}
	}

	var (
		prunerProvider agent.Provider
		prunerKey      string
		pruner         *agent.Pruner
	)
	if nonInteractive {
		if sel := agent.EnvString("LLM_PRUNER_PROVIDER"); sel != "" && sel != "off" && sel != "none" {
			prunerProvider, prunerKey, err = resolvePrunerInteractive(providers, lc.Pruner)
			if err != nil {
				uiError(err)
				return
			}
		}
	} else {
		prunerProvider, prunerKey, err = resolvePrunerInteractive(providers, lc.Pruner)
		if err != nil {
			uiError(err)
			return
		}
	}
	if prunerKey != "" {
		pc, err := agent.NewClient(prunerProvider)
		if err != nil {
			uiError(err)
			return
		}
		pruner = agent.NewPruner(pc)
	}

	if !nonInteractive {
		saveLastChoice(lastChoice{Main: mainKey, Pruner: prunerKey})
	}

	// Resolve system prompt: YAML wins, otherwise the CLI's default.
	systemPrompt := defaultCLIPrompt
	if agentCfg != nil {
		if body, err := agentCfg.LoadSystemPrompt(); err == nil && strings.TrimSpace(body) != "" {
			systemPrompt = body
		} else if err != nil {
			fmt.Fprintln(os.Stderr, "warning: agent system_prompt:", err)
		}
	}
	customTools, err := agentCfg.BuildTools()
	if err != nil {
		uiError(err)
		return
	}

	tty := newTTYHandler()

	ag, err := agent.New(agent.Config{
		Provider:     p,
		SystemPrompt: systemPrompt,
		Tools:        customTools,
		Pruner:       pruner,
		OnEvent:      tty.Handle,
	})
	if err != nil {
		uiError(err)
		return
	}
	defer ag.Close()

	if !nonInteractive {
		uiHeader(p.Name, p.Model, ag.Session())
		if pruner != nil {
			uiInfo(fmt.Sprintf("pruner: %s/%s", prunerProvider.Name, prunerProvider.Model))
		} else {
			uiInfo("pruner: disabled")
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()

	var inputFn func() (string, bool)
	if nonInteractive {
		inputFn = singleShotInput(*flagPrompt)
	} else {
		inputFn = pasteAwareInput(os.Stdin)
	}

	if !nonInteractive {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		go func() {
			for range sigint {
				if ag.Interrupt() {
					continue
				}
				_ = ag.Close()
				os.Exit(130)
			}
		}()
	}

	// REPL: read input, handle slash commands, otherwise drive a Step.
	for {
		uiPrompt()
		line, ok := inputFn()
		if !ok {
			break
		}
		uiAfterInput()
		trimmed := strings.TrimSpace(line)
		if handleSlash(ag, trimmed) {
			continue
		}
		if _, err := ag.Step(ctx, line); err != nil {
			uiError(err)
		}
	}
}

// defaultCLIPrompt is the role text the reference CLI uses when no
// --agent personality is supplied. The runtime itself has no default
// prompt; if you're building a different product on top of the agent
// package you should provide your own.
const defaultCLIPrompt = `You are cortex, a terminal coding agent.

You work in a real repository on the user's machine. The runtime gives
you built-in tools: read, write, exec, search, task, bash_output,
kill_shell. Every tool call must articulate intent in its "reason" field.

Principles:
- Read before you write. Search before you read.
- One change per turn. Verify with exec.
- Atomic edits only. /undo is byte-exact; don't fight it.
- Act, don't narrate.
- Stop when the goal is met. Don't invent follow-up work.`

// Command cortex is a terminal coding agent built on the axon runtime.
//
// All runtime logic lives in github.com/atakang7/axon/axon. This binary
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

	"github.com/atakang7/axon"
)

func main() {
	var (
		flagPrompt = flag.String("prompt", "", "Run a single prompt non-interactively and exit when the assistant emits a final reply. Requires LLM_PROVIDER env to be set to skip the provider picker.")
		flagAgent  = flag.String("agent", "", "Named agent config to load from $CORTEX_AGENTS_DIR (default ~/.config/cortex/agents/<name>.yaml). Empty = built-in default coding axon.")
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

	providers, err := LoadProviders()
	if err != nil {
		uiError(err)
		return
	}
	lc := loadLastChoice()

	var (
		p       axon.Provider
		mainKey string
	)
	if nonInteractive {
		p, err = ResolveProvider(providers)
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
		prunerProvider axon.Provider
		prunerKey      string
		pruner         *axon.Pruner
	)
	if nonInteractive {
		if sel := EnvString("LLM_PRUNER_PROVIDER"); sel != "" && sel != "off" && sel != "none" {
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
		pc, err := axon.OpenAI(axon.ClientConfig{Provider: prunerProvider, ExcludeReasoning: true})
		if err != nil {
			uiError(err)
			return
		}

		dp := &dynamicPruner{base: pc}
		if p, ok := providers["openrouter/nvidia/nemotron-3-ultra:free"]; ok {
			dp.normal, _ = axon.OpenAI(axon.ClientConfig{Provider: p, ExcludeReasoning: true})
		}
		if p, ok := providers["openrouter/poolside/laguna-s-2.1:free"]; ok {
			dp.heavy, _ = axon.OpenAI(axon.ClientConfig{Provider: p, ExcludeReasoning: true})
		}
		pruner = axon.NewPruner(axon.PrunerConfig{Model: dp})
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
	var customTools []axon.Tool
	var mcpServers []axon.MCPServer

	if agentCfg != nil {
		customTools, mcpServers, err = agentCfg.BuildTools()
		if err != nil {
			uiError(err)
			return
		}
	}


	tty := newTTYHandler()

	m, err := axon.OpenAI(axon.ClientConfig{Provider: p})
	if err != nil {
		uiError(err)
		return
	}

	ag, err := axon.New(axon.Config{
		Model:        m,
		SystemPrompt: systemPrompt,
		Tools:        customTools,
		MCPServers:   mcpServers,
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
const defaultCLIPrompt = `You are cortex, a terminal coding axon.

You work in a real repository on the user's machine. The runtime gives
you built-in tools: read, write, exec, search, task, bash_output,
kill_shell. Every tool call must articulate intent in its "reason" field.

Principles:
- Read before you write. Search before you read.
- One change per turn. Verify with exec.
- Atomic edits only. /undo is byte-exact; don't fight it.
- Act, don't narrate.
- Stop when the goal is met. Don't invent follow-up work.`

type dynamicPruner struct {
	base   axon.Model
	normal axon.Model
	heavy  axon.Model
}

func (d *dynamicPruner) Complete(ctx context.Context, req axon.Request) (*axon.Msg, error) {
	tokens := 0
	for _, m := range req.Messages {
		tokens += len(m.Content)
		for _, tc := range m.ToolCalls {
			tokens += len(tc.Function.Arguments) + len(tc.Function.Name)
		}
	}
	tokens /= 4

	if tokens > 50000 && d.heavy != nil {
		fmt.Println("\n\033[38;5;221m  ⚠  WARNING: Context > 50k tokens. Using biggest free model (laguna-s-2.1) to oversummarize. This may break some context history!\033[0m")
		if len(req.Messages) > 0 && req.Messages[0].Role == "system" {
			req.Messages[0].Content = oversummarizeSystemPrompt
		}
		msg, err := d.heavy.Complete(ctx, req)
		if err == nil && msg != nil && msg.Content != "" {
			return msg, nil
		}
	}
	if tokens > 10000 && d.normal != nil {
		msg, err := d.normal.Complete(ctx, req)
		if err == nil && msg != nil && msg.Content != "" {
			return msg, nil
		}
	}
	if d.base != nil {
		return d.base.Complete(ctx, req)
	}
	return nil, fmt.Errorf("no pruner model available")
}

const oversummarizeSystemPrompt = `You keep an agent's working memory small. The context has grown exceedingly large.

You are shown the agent's task and a numbered log of what has happened. Decide
which blocks it no longer needs in order to finish that task. You should be extremely aggressive and park as much as possible to save context space.

Answer with one JSON object and nothing that matters after it:

{"park":[3,7,9]}

Block ids are the integers in the labels (m7 is 7).

Never park:
- a block naming a file the agent has edited or is editing
- a block holding an unresolved error or a failing test

Parking is one-way: the agent cannot get a parked block back.`

// Command cortex is a terminal coding agent built on the axon runtime.
//
// axon owns the loop: streaming the model, dispatching tools, persisting an
// append-only session, and pruning context under pressure. cortex owns the
// product around it — resolving which model to talk to, the role text that
// makes it a coding agent, and the terminal it talks through.
//
// This file is wiring and nothing else. Every decision it makes is delegated:
// configuration to internal/config, presentation to internal/ui, and the loop
// itself to axon. If logic accumulates here, it belongs in one of those.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/atakang7/axon/v2"

	"github.com/atakang7/cortex/v2/internal/config"
	"github.com/atakang7/cortex/v2/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "cortex: %v\n", err)
		os.Exit(1)
	}
}

// Build metadata, injected at link time by goreleaser.
//
// DEPLOYMENT COUPLING: the names of these three variables are written into
// the ldflags in .goreleaser.yaml. Renaming one here does not break the
// build — the linker ignores a -X for a symbol that does not exist — it just
// silently stops being set. Change both files together.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func run() error {
	var (
		configPath  = flag.String("config", "", "path to a config file, instead of the usual cascade")
		prompt      = flag.String("prompt", "", "run one prompt non-interactively and exit")
		showVersion = flag.Bool("version", false, "print the version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("cortex %s (%s, built %s)\n", version, commit, date)

		return nil
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		if errors.Is(err, config.ErrNoModel) {
			return errors.New(onboarding)
		}

		return err
	}

	// axon owns providers, credentials and every runtime knob. cortex's
	// config says which provider and model to use; axon's says where they
	// live and how they are authenticated.
	settings, err := axon.Load()
	if err != nil {
		if errors.Is(err, axon.ErrMissingConfig) || errors.Is(err, axon.ErrMissingEnv) {
			return fmt.Errorf("%w\n\n%s", err, axonSetup)
		}

		return err
	}

	model, err := settings.NewClient(cfg.Provider, cfg.ModelName)
	if err != nil {
		return err
	}

	// A pruner is optional. When configured, it runs a cheap secondary model
	// that parks stale context so the main model stays under its window.
	var prunerModel axon.Model
	if cfg.Pruner.Model != "" {
		prunerProvider := cfg.Pruner.Provider
		if prunerProvider == "" {
			prunerProvider = cfg.Provider
		}

		pm, err := settings.NewClient(prunerProvider, cfg.Pruner.Model)
		if err != nil {
			return fmt.Errorf("pruner: %w", err)
		}
		prunerModel = pm
	}

	// The trace log is a third, unfiltered event sink alongside whichever
	// renderer the mode below picks. It never fails the run: a broken cache
	// dir means no trace, not no cortex.
	trace, err := ui.NewTraceLog(ui.DefaultTracePath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "cortex: trace log disabled: %v\n", err)
	}
	defer trace.Close()
	if path := trace.Path(); path != "" {
		fmt.Fprintf(os.Stderr, "cortex: trace: %s\n", path)
	}

	// Tapping the models here, at the one place they are resolved, is what
	// puts the actual prompts in the trace. Both are tapped: the curator's
	// context decisions are as worth auditing as the agent's, and they are
	// invisible in the event stream, which only reports how many tokens a
	// pass moved and never why.
	model = trace.Wiretap("main", model)
	prunerModel = trace.Wiretap("pruner", prunerModel)

	cwd, _ := os.Getwd()
	trace.Describe(ui.RunInfo{
		Version:        version,
		Provider:       cfg.Provider,
		Model:          cfg.ModelName,
		PrunerProvider: cfg.Pruner.Provider,
		PrunerModel:    cfg.Pruner.Model,
		Cwd:            cwd,
	})

	if *prompt != "" {
		return runOnce(cfg, settings, model, prunerModel, trace, *prompt)
	}

	return runInteractive(cfg, settings, model, prunerModel, trace)
}

// runInteractive starts the full terminal UI.
//
// Interrupt is deliberately absent from the signal set: in raw mode Ctrl-C
// arrives as a key, and the UI binds it to cancelling the turn rather than
// killing the process. Only a signal the terminal cannot deliver as a
// keystroke should end the program from outside.
func runInteractive(cfg config.Config, settings axon.Settings, model axon.Model, prunerModel axon.Model, trace *ui.TraceLog) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	bridge := &ui.Bridge{}

	// Unbounded: a human is watching, and Ctrl-C ends a turn that has lost
	// its way. See maxIterationsOnce for why -prompt is not given the same
	// freedom.
	agent, err := newAgent(cfg, settings, model, prunerModel, fanOut(bridge.Emit, trace.Emit), 0)
	if err != nil {
		return err
	}
	defer agent.Close()

	return ui.Run(ctx, bridge, ui.Options{
		Agent:      agent,
		Context:    ctx,
		ModelName:  cfg.ModelName,
		PrunerName: cfg.Pruner.Model,
		AgentName:  cfg.Name,
		Settings:   settings,
		OnPrunerChanged: func(provider, model string) {
			if err := config.SavePruner(provider, model); err != nil {
				fmt.Fprintf(os.Stderr, "cortex: could not save pruner: %v\n", err)
			}
		},
		OnModelChanged: func(provider, model string) {
			if err := config.SaveModel(provider, model); err != nil {
				fmt.Fprintf(os.Stderr, "cortex: could not save model: %v\n", err)
			}
		},
	})
}

// runOnce drives a single turn with no TUI. Ctrl-C ends the process here,
// which is the ordinary expectation for a one-shot command.
func runOnce(cfg config.Config, settings axon.Settings, model axon.Model, prunerModel axon.Model, trace *ui.TraceLog, prompt string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	renderer := ui.NewPlain(os.Stdout, os.Stderr)
	defer renderer.Close()

	agent, err := newAgent(cfg, settings, model, prunerModel, fanOut(renderer.Emit, trace.Emit), maxIterationsOnce)
	if err != nil {
		return err
	}
	defer agent.Close()

	if _, err := agent.Step(ctx, prompt); err != nil {
		if errors.Is(err, axon.ErrInterrupted) || errors.Is(err, context.Canceled) {
			return nil
		}

		return err
	}

	return nil
}

// fanOut combines event sinks into the single func axon.Config.OnEvent
// accepts. Every sink sees every event, in order, on the same goroutine —
// that goroutine is always the turn's own, so none of them may block.
func fanOut(sinks ...func(context.Context, axon.Event)) func(context.Context, axon.Event) {
	return func(ctx context.Context, e axon.Event) {
		for _, sink := range sinks {
			sink(ctx, e)
		}
	}
}

// maxIterationsOnce bounds how many model calls a single non-interactive run
// may make.
//
// Interactive sessions are deliberately unbounded: a human is watching and
// Ctrl-C ends a turn that has lost its way. -prompt has neither. Without a
// bound, a model that keeps calling tools and never answers spends the user's
// budget until something else kills the process, which is the failure mode
// axon.Config.MaxIterations exists to prevent and which every unattended
// embedder is told to guard against.
//
// The number is chosen to be far above any real task — traced runs of a
// multi-file feature settle around a dozen calls — so reaching it means the
// loop is stuck, not that the work was large.
const maxIterationsOnce = 120

// newAgent constructs the runtime. Both modes go through here so they cannot
// drift into configuring the agent differently; what legitimately differs
// between them is passed in.
func newAgent(cfg config.Config, settings axon.Settings, model axon.Model, prunerModel axon.Model, onEvent func(context.Context, axon.Event), maxIterations int) (*axon.Agent, error) {
	servers := make([]axon.MCPServer, 0, len(cfg.MCPServers))
	for _, s := range cfg.MCPServers {
		servers = append(servers, axon.MCPServer{Command: s.Command, Args: s.Args, Env: s.Env})
	}

	return axon.New(axon.Config{
		Model:         model,
		SystemPrompt:  cfg.SystemPrompt,
		MCPServers:    servers,
		OnEvent:       onEvent,
		Settings:      settings,
		Pruner:        prunerModel,
		MaxIterations: maxIterations,
		// The same model that answers turns also names them. A title request
		// is one cheap, tool-free call on turn 1; a dedicated model is not
		// worth a second config knob. If the call fails, axon falls back to
		// the deterministic truncation, so this can never break a turn.
		TitleModel: model,
	})
}

// onboarding is what a first run with nothing configured prints.
const onboarding = `no model configured

Write ~/.config/cortex/config.yaml:

    provider: openrouter
    model: deepseek/deepseek-v3.2

A ./cortex.yaml in the working directory overrides the user config, and
LLM_PROVIDER / LLM_MODEL override both.

Providers and credentials are configured in axon:

    See https://github.com/atakang7/axon#configuration`

// axonSetup tells the user how to set up axon when its config is missing.
const axonSetup = `cortex needs axon configured first.

    mkdir -p ~/.config/axon
    # Copy axon.example.yaml from the axon repo to ~/.config/axon/axon.yaml
    printf 'OPENROUTER_API_KEY=sk-or-...\n' > ~/.config/axon/.env
    chmod 600 ~/.config/axon/.env

See https://github.com/atakang7/axon#configuration`

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

	"github.com/atakang7/axon"

	"github.com/atakang7/cortex/internal/config"
	"github.com/atakang7/cortex/internal/ui"
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

	if *prompt != "" {
		return runOnce(cfg, settings, model, prunerModel, *prompt)
	}

	return runInteractive(cfg, settings, model, prunerModel)
}

// runInteractive starts the full terminal UI.
//
// Interrupt is deliberately absent from the signal set: in raw mode Ctrl-C
// arrives as a key, and the UI binds it to cancelling the turn rather than
// killing the process. Only a signal the terminal cannot deliver as a
// keystroke should end the program from outside.
func runInteractive(cfg config.Config, settings axon.Settings, model axon.Model, prunerModel axon.Model) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	bridge := &ui.Bridge{}

	agent, err := newAgent(cfg, settings, model, prunerModel, bridge.Emit)
	if err != nil {
		return err
	}
	defer agent.Close()

	return ui.Run(ctx, bridge, ui.Options{
		Agent:     agent,
		Context:   ctx,
		ModelName: cfg.ModelName,
		AgentName: cfg.Name,
		Settings:  settings,
	})
}

// runOnce drives a single turn with no TUI. Ctrl-C ends the process here,
// which is the ordinary expectation for a one-shot command.
func runOnce(cfg config.Config, settings axon.Settings, model axon.Model, prunerModel axon.Model, prompt string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	renderer := ui.NewPlain(os.Stdout, os.Stderr)
	defer renderer.Close()

	agent, err := newAgent(cfg, settings, model, prunerModel, renderer.Emit)
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

// newAgent constructs the runtime. Both modes go through here so they cannot
// drift into configuring the agent differently.
func newAgent(cfg config.Config, settings axon.Settings, model axon.Model, prunerModel axon.Model, onEvent func(context.Context, axon.Event)) (*axon.Agent, error) {
	servers := make([]axon.MCPServer, 0, len(cfg.MCPServers))
	for _, s := range cfg.MCPServers {
		servers = append(servers, axon.MCPServer{Command: s.Command, Args: s.Args, Env: s.Env})
	}

	return axon.New(axon.Config{
		Model:        model,
		SystemPrompt: cfg.SystemPrompt,
		MCPServers:   servers,
		OnEvent:      onEvent,
		Settings:     settings,
		Pruner:       prunerModel,
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

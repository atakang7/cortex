package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"strings"
	"syscall"

	"github.com/atakang7/axon"
	"github.com/chzyer/readline"
	"gopkg.in/yaml.v3"
)

// AgentConfig represents the declarative YAML configuration for the agent.
type AgentConfig struct {
	Name         string            `yaml:"name"`
	Model        ModelConfig       `yaml:"model"`
	SystemPrompt string            `yaml:"system_prompt"`
	MCPServers   map[string]MCPSrv `yaml:"mcp_servers"`
}

type ModelConfig struct {
	Provider string `yaml:"provider"`
	Name     string `yaml:"name"`
	BaseURL  string `yaml:"base_url"`
	APIKey   string `yaml:"api_key"` // can be an env var name like $OPENAI_API_KEY
}

type MCPSrv struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
	Env     []string `yaml:"env"`
}

// UI colors
const (
	reset  = "\033[0m"
	brand  = "\033[38;5;215m"
	mute   = "\033[38;5;243m"
	bad    = "\033[38;5;203m"
	toolFg = "\033[38;5;110m"
	think  = "\033[38;5;245m"
)

func main() {
	var (
		flagConfig = flag.String("config", "", "Path to cortex.yaml config file")
		flagPrompt = flag.String("prompt", "", "Run a single prompt non-interactively")
	)
	flag.Parse()

	// 1. Load Configuration
	configFile := *flagConfig
	if configFile == "" {
		configFile = "cortex.yaml"
	}

	var cfg AgentConfig
	data, err := os.ReadFile(configFile)
	if err != nil {
		if *flagConfig != "" {
			fmt.Printf("Config error: %v\n", err)
			os.Exit(1)
		}
		// Graceful default if no cortex.yaml exists
		cfg = AgentConfig{
			Name: "default",
			SystemPrompt: "You are cortex, a minimal and highly capable terminal coding agent.\n\nPrinciples:\n- Read before you write. Search before you read.\n- One change per turn. Verify with exec.\n- Atomic edits only. /undo is byte-exact; don't fight it.\n- Act, don't narrate.\n- Stop when the goal is met.",
			Model: ModelConfig{
				Provider: "openai",
				Name:     "gpt-4o",
				APIKey:   "$OPENAI_API_KEY",
			},
		}
	} else {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			fmt.Printf("Config parse error: %v\n", err)
			os.Exit(1)
		}
	}

	if cfg.Model.Name == "" {
		fmt.Println("Error: model configuration is required in your config file.")
		os.Exit(1)
	}

	// Resolve API Key (allow $ENV_VAR syntax in config)
	apiKey := cfg.Model.APIKey
	if strings.HasPrefix(apiKey, "$") {
		apiKey = os.Getenv(strings.TrimPrefix(apiKey, "$"))
	}
	if apiKey == "" {
		fmt.Println("Error: API Key is required. Set it in the config or via the environment variable.")
		os.Exit(1)
	}

	model, err := axon.NewClient(axon.ClientConfig{
		Provider: axon.Provider{
			Name:    cfg.Model.Provider,
			Model:   cfg.Model.Name,
			APIKey:  apiKey,
			BaseURL: cfg.Model.BaseURL,
		},
	})
	if err != nil {
		fmt.Printf("Model error: %v\n", err)
		os.Exit(1)
	}

	// Map MCP servers
	var mcpServers []axon.MCPServer
	for _, mcp := range cfg.MCPServers {
		mcpServers = append(mcpServers, axon.MCPServer{
			Command: mcp.Command,
			Args:    mcp.Args,
			Env:     mcp.Env,
		})
	}

	// 3. Initialize Engine
	ag, err := axon.New(axon.Config{
		Model:        model,
		SystemPrompt: cfg.SystemPrompt,
		MCPServers:   mcpServers,
		OnEvent:      handleEvent,
	})
	if err != nil {
		fmt.Printf("Agent init error: %v\n", err)
		os.Exit(1)
	}
	defer ag.Close()

	fmt.Printf("\n%s cortex%s · %s%s%s\n\n", brand, reset, mute, cfg.Model.Name, reset)

	// 4. Non-Interactive Mode
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGHUP, os.Interrupt)
	defer cancel()

	if *flagPrompt != "" {
		if _, err := ag.Step(ctx, *flagPrompt); err != nil {
			fmt.Printf("\n%sError: %v%s\n", bad, err, reset)
		}
		return
	}

	// 5. Interactive REPL Loop
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          fmt.Sprintf("%s❯%s ", brand, reset),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			break // EOF or Ctrl-D
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Handle slash commands
		if strings.HasPrefix(line, "/cd ") {
			if cwd, err := ag.Cd(strings.TrimPrefix(line, "/cd ")); err == nil {
				fmt.Printf("%scwd: %s%s\n", mute, cwd, reset)
			} else {
				fmt.Printf("%sError: %v%s\n", bad, err, reset)
			}
			continue
		}
		if line == "/undo" {
			if p, ok := ag.Undo(); ok {
				fmt.Printf("%sundone: %s%s\n", mute, p, reset)
			} else {
				fmt.Printf("%snothing to undo%s\n", mute, reset)
			}
			continue
		}
		if line == "/new" {
			ag.Reset()
			fmt.Printf("%snew session started%s\n", mute, reset)
			continue
		}

		// Execute turn
		if _, err := ag.Step(ctx, line); err != nil {
			fmt.Printf("\n%sError: %v%s\n", bad, err, reset)
		}
		fmt.Println()
	}
}

// handleEvent formats Axon runtime events for the terminal.
func handleEvent(_ context.Context, e axon.Event) {
	switch e.Kind {
	case axon.KindToken:
		fmt.Print(e.Text)
		os.Stdout.Sync()
	case axon.KindReasoning:
		fmt.Printf("%s%s%s", think, e.Text, reset)
		os.Stdout.Sync()
	case axon.KindToolCall:
		if e.Tool != nil {
			fmt.Printf("\n%s  ⎿  %s%s\n", mute, toolFg+e.Tool.Name, reset)
			// Pretty print arguments
			var raw map[string]any
			if json.Unmarshal([]byte(e.Tool.Args), &raw) == nil {
				if b, err := json.MarshalIndent(raw, "     ", "  "); err == nil {
					fmt.Printf("%s     %s%s\n", mute, string(b), reset)
				}
			}
		}
	case axon.KindToolResult:
		if e.Tool != nil {
			lines := strings.Split(strings.TrimSpace(e.Tool.Result), "\n")
			for _, l := range lines {
				fmt.Printf("%s     │ %s%s\n", mute, l, reset)
			}
		}
	case axon.KindToolError:
		if e.Err != nil {
			fmt.Printf("%s  ✗  %v%s\n", bad, e.Err, reset)
		}
	case axon.KindInfo:
		fmt.Printf("%s  %s%s\n", mute, e.Text, reset)
	case axon.KindError:
		if e.Err != nil && !strings.HasPrefix(e.Err.Error(), "pruner:") {
			fmt.Printf("\n%s  ✗  %v%s\n", bad, e.Err, reset)
		}
	}
}

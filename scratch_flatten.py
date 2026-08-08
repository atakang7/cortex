import os
import shutil

cortex_dir = "cmd/cortex"

main_go_content = """package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/atakang7/axon"
	"github.com/chzyer/readline"
	"gopkg.in/yaml.v3"
)

// AgentConfig represents the declarative YAML configuration for the agent.
type AgentConfig struct {
	Name         string              `yaml:"name"`
	SystemPrompt string              `yaml:"system_prompt"`
	MCPServers   map[string]MCPSrv   `yaml:"mcp_servers"`
}

type MCPSrv struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
	Env     []string `yaml:"env"`
}

// UI colors
const (
	reset  = "\\033[0m"
	brand  = "\\033[38;5;215m"
	mute   = "\\033[38;5;243m"
	bad    = "\\033[38;5;203m"
	toolFg = "\\033[38;5;110m"
	think  = "\\033[38;5;245m"
)

func main() {
	var (
		flagConfig = flag.String("config", "", "Path to cortex.yaml config file")
		flagPrompt = flag.String("prompt", "", "Run a single prompt non-interactively")
	)
	flag.Parse()

	// 1. Resolve Provider via Env
	modelName := os.Getenv("CORTEX_MODEL")
	if modelName == "" {
		modelName = "openai/gpt-4o"
	}
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" && os.Getenv("OPENAI_API_KEY") != "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		fmt.Println("Error: API_KEY or OPENAI_API_KEY environment variable is required.")
		os.Exit(1)
	}

	model, err := axon.NewClient(axon.ClientConfig{
		Provider: axon.ProviderConfig{
			Name:  strings.Split(modelName, "/")[0],
			Model: strings.Split(modelName, "/")[1],
			Key:   apiKey,
		},
	})
	if err != nil {
		fmt.Printf("Model error: %v\\n", err)
		os.Exit(1)
	}

	// 2. Load Configuration
	var cfg AgentConfig
	if *flagConfig != "" {
		data, err := os.ReadFile(*flagConfig)
		if err != nil {
			fmt.Printf("Config error: %v\\n", err)
			os.Exit(1)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			fmt.Printf("Config parse error: %v\\n", err)
			os.Exit(1)
		}
	} else {
		// Default
		cfg.SystemPrompt = "You are cortex, a minimal and highly capable terminal coding agent.\\n\\nPrinciples:\\n- Read before you write. Search before you read.\\n- One change per turn. Verify with exec.\\n- Atomic edits only. /undo is byte-exact; don't fight it.\\n- Act, don't narrate.\\n- Stop when the goal is met."
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
		fmt.Printf("Agent init error: %v\\n", err)
		os.Exit(1)
	}
	defer ag.Close()

	fmt.Printf("\\n%s cortex%s · %s%s%s\\n\\n", brand, reset, mute, modelName, reset)

	// 4. Non-Interactive Mode
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGHUP, os.Interrupt)
	defer cancel()

	if *flagPrompt != "" {
		if _, err := ag.Step(ctx, *flagPrompt); err != nil {
			fmt.Printf("\\n%sError: %v%s\\n", bad, err, reset)
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
				fmt.Printf("%scwd: %s%s\\n", mute, cwd, reset)
			} else {
				fmt.Printf("%sError: %v%s\\n", bad, err, reset)
			}
			continue
		}
		if line == "/undo" {
			if p, ok := ag.Undo(); ok {
				fmt.Printf("%sundone: %s%s\\n", mute, p, reset)
			} else {
				fmt.Printf("%snothing to undo%s\\n", mute, reset)
			}
			continue
		}
		if line == "/new" {
			ag.Reset()
			fmt.Printf("%snew session started%s\\n", mute, reset)
			continue
		}

		// Execute turn
		if _, err := ag.Step(ctx, line); err != nil {
			fmt.Printf("\\n%sError: %v%s\\n", bad, err, reset)
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
			fmt.Printf("\\n%s  ⎿  %s%s\\n", mute, toolFg+e.Tool.Name, reset)
			// Pretty print arguments
			var raw map[string]any
			if json.Unmarshal([]byte(e.Tool.Args), &raw) == nil {
				if b, err := json.MarshalIndent(raw, "     ", "  "); err == nil {
					fmt.Printf("%s     %s%s\\n", mute, string(b), reset)
				}
			}
		}
	case axon.KindToolResult:
		if e.Tool != nil {
			lines := strings.Split(strings.TrimSpace(e.Tool.Result), "\\n")
			for _, l := range lines {
				fmt.Printf("%s     │ %s%s\\n", mute, l, reset)
			}
		}
	case axon.KindToolError:
		if e.Err != nil {
			fmt.Printf("%s  ✗  %v%s\\n", bad, e.Err, reset)
		}
	case axon.KindInfo:
		fmt.Printf("%s  %s%s\\n", mute, e.Text, reset)
	case axon.KindError:
		if e.Err != nil && !strings.HasPrefix(e.Err.Error(), "pruner:") {
			fmt.Printf("\\n%s  ✗  %v%s\\n", bad, e.Err, reset)
		}
	}
}
"""

with open(os.path.join(cortex_dir, "main.go"), "w") as f:
    f.write(main_go_content)

files_to_remove = [
    "commands.go",
    "customtool.go",
    "picker.go",
    "providers.go",
    "tty_handler.go",
    "yamlcfg.go",
]

for file in files_to_remove:
    path = os.path.join(cortex_dir, file)
    if os.path.exists(path):
        os.remove(path)

print("Cortex successfully flattened and rewritten to a single main.go")

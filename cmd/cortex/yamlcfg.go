package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/atakang7/axon"
	"gopkg.in/yaml.v3"
)

// yamlcfg.go — YAML agent personality loader.
//
// An "agent personality" is a system prompt plus optional custom tools.
// YAML is one way to express it; this file lives in the CLI because the
// runtime (package agent) is YAML-agnostic. The loader produces an
// axon.Config that gets handed to axon.New.

const (
	// Built-in tool names — kept in sync with the runtime so YAML configs
	// can validate disable_builtins entries without importing internals.
	toolRead       = "read"
	toolWrite      = "write"
	toolExec       = "exec"
	toolBashOutput = "bash_output"
	toolKillShell  = "kill_shell"
	toolSearch     = "search"
	toolTask       = "task"
)

var builtinNames = map[string]bool{
	toolRead: true, toolWrite: true, toolExec: true, toolSearch: true,
	toolTask: true, toolBashOutput: true, toolKillShell: true,
}

// AgentConfig is the YAML-on-disk shape of an agent definition.
type AgentConfig struct {
	Name               string       `yaml:"name"`
	Description        string       `yaml:"description,omitempty"`
	SystemPrompt       string       `yaml:"system_prompt,omitempty"`
	SystemPromptInline string       `yaml:"system_prompt_inline,omitempty"`
	Tools              []ToolConfig          `yaml:"tools,omitempty"`
	MCPServers         map[string]MCPConfig  `yaml:"mcp_servers,omitempty"`

	sourcePath string `yaml:"-"`
}

// MCPConfig is one MCP server definition.
type MCPConfig struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args,omitempty"`
	Env     []string `yaml:"env,omitempty"`
}

// ToolConfig is one custom tool definition.
type ToolConfig struct {
	Name           string         `yaml:"name"`
	Type           string         `yaml:"type"`
	Description    string         `yaml:"description"`
	Schema         map[string]any `yaml:"schema"`
	Command        string         `yaml:"command,omitempty"`
	Cwd            string         `yaml:"cwd,omitempty"`
	TimeoutSeconds int            `yaml:"timeout_seconds,omitempty"`
}

// AgentsDir returns the directory where agent YAML files live. Override via
// CORTEX_AGENTS_DIR; default is $XDG_CONFIG_HOME/axon/agents (or
// ~/.config/axon/agents).
func AgentsDir() string {
	if d := os.Getenv("CORTEX_AGENTS_DIR"); d != "" {
		return d
	}
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "cortex", "agents")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "cortex-agents"
	}
	return filepath.Join(home, ".config", "cortex", "agents")
}

// LoadAgentConfig reads <name>.yaml from AgentsDir() and validates it.
// name == "" or "default" returns a zero AgentConfig (built-in defaults).
func LoadAgentConfig(name string) (*AgentConfig, error) {
	if name == "" || name == "default" {
		return &AgentConfig{Name: "default"}, nil
	}
	path := filepath.Join(AgentsDir(), name+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("agent %q not found: %s does not exist", name, path)
		}
		return nil, fmt.Errorf("read agent %q: %w", name, err)
	}
	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse agent %q: %w", name, err)
	}
	cfg.sourcePath = path
	if cfg.Name == "" {
		cfg.Name = name
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("agent %q: %w", name, err)
	}
	return &cfg, nil
}

func (c *AgentConfig) validate() error {
	seen := map[string]bool{}
	for i, t := range c.Tools {
		if t.Name == "" {
			return fmt.Errorf("tools[%d]: name is required", i)
		}
		if builtinNames[t.Name] {
			return fmt.Errorf("tools[%d] %q: collides with a built-in tool name", i, t.Name)
		}
		if seen[t.Name] {
			return fmt.Errorf("tools[%d] %q: duplicate tool name", i, t.Name)
		}
		seen[t.Name] = true
		if t.Description == "" {
			return fmt.Errorf("tools[%d] %q: description is required", i, t.Name)
		}
		switch t.Type {
		case "shell":
			if strings.TrimSpace(t.Command) == "" {
				return fmt.Errorf("tools[%d] %q: shell tools require a command", i, t.Name)
			}
		default:
			return fmt.Errorf("tools[%d] %q: unknown type %q (expected: shell)", i, t.Name, t.Type)
		}
	}
	
	for name, mc := range c.MCPServers {
		if strings.TrimSpace(mc.Command) == "" {
			return fmt.Errorf("mcp_servers[%q]: command is required", name)
		}
	}
	return nil
}

// LoadSystemPrompt resolves the role text (file or inline).
func (c *AgentConfig) LoadSystemPrompt() (string, error) {
	if c.SystemPrompt == "" {
		return c.SystemPromptInline, nil
	}
	path := c.SystemPrompt
	if !filepath.IsAbs(path) && c.sourcePath != "" {
		path = filepath.Join(filepath.Dir(c.sourcePath), path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read system_prompt %s: %w", path, err)
	}
	return string(data), nil
}

// BuildTools materializes the YAML tool list as runtime tools (shell
// templates rendered into axon.Tool values).
func (c *AgentConfig) BuildTools() ([]axon.Tool, []axon.MCPServer, error) {
	var tools []axon.Tool
	for _, tc := range c.Tools {
		t, err := buildCustomTool(tc)
		if err != nil {
			return nil, nil, fmt.Errorf("custom tool %q: %w", tc.Name, err)
		}
		tools = append(tools, t)
	}
	
	var mcpServers []axon.MCPServer
	for _, mc := range c.MCPServers {
		mcpServers = append(mcpServers, axon.MCPServer{
			Command: mc.Command,
			Args:    mc.Args,
			Env:     mc.Env,
		})
	}
	
	return tools, mcpServers, nil
}

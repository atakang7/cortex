// Package config resolves the one thing cortex must decide before it can
// start: which model to talk to, with what role text, and which MCP servers
// to spawn.
//
// axon owns the runtime — providers, credentials, tool caps, retry policy.
// cortex owns the product — which of those providers to use, the role text
// that makes it a coding agent, and the terminal it talks through.
//
// This package is where that boundary is drawn. It reads cortex's own config,
// then hands the provider and model name to the caller so they can resolve it
// against axon's Settings.
//
// BOUNDARY RULE: config is a leaf. It imports nothing else from this module.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// The resolved shape
// ---------------------------------------------------------------------------

// Config is the fully resolved answer: every field is final, nothing here
// still needs the environment consulted. Load is the only constructor.
type Config struct {
	// Name labels this agent personality, e.g. "cortex" or "reviewer".
	Name string

	// Provider is the axon provider name, e.g. "openrouter".
	Provider string

	// ModelName is the model within that provider, e.g. "deepseek/deepseek-v3.2".
	ModelName string

	// SystemPrompt is the agent's role text, already read from disk if the
	// config pointed at a file.
	SystemPrompt string

	// MCPServers are subprocesses axon will spawn and harvest tools from.
	MCPServers []MCPServer

	// Pruner names the secondary model for context pruning. Both fields
	// empty means no pruner — context grows until the provider refuses it.
	Pruner PrunerRef

	// Source records which file this came from, for the status line. Empty
	// when the built-in defaults were used.
	Source string
}

// MCPServer is one Model Context Protocol subprocess.
type MCPServer struct {
	Command string
	Args    []string
	Env     []string
}

// PrunerRef points at the secondary model used for context pruning.
// Both fields empty means no pruner.
type PrunerRef struct {
	Provider string
	Model    string
}

// ErrNoModel means neither a config file nor the environment named a model.
// The caller turns this into onboarding help rather than a bare failure.
var ErrNoModel = errors.New("config: no model configured")

// ---------------------------------------------------------------------------
// The file format
// ---------------------------------------------------------------------------

// file mirrors the YAML on disk. Provider and model are plain strings that
// reference entries in axon's config — cortex does not define endpoints or
// carry credentials.
type file struct {
	Name         string            `yaml:"name"`
	Provider     string            `yaml:"provider"`
	Model        string            `yaml:"model"`
	SystemPrompt string            `yaml:"system_prompt"`
	Pruner       prunerFile        `yaml:"pruner"`
	MCPServers   map[string]mcpSrv `yaml:"mcp_servers"`
}

type mcpSrv struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
	Env     []string `yaml:"env"`
}

type prunerFile struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

// ---------------------------------------------------------------------------
// Loading
// ---------------------------------------------------------------------------

// Load resolves the configuration.
//
// With an explicit path, that file is the only file consulted and a missing
// one is an error — an operator who names a file means it.
//
// Otherwise two sources are layered, each overriding the last:
//
//  1. the user config at ~/.config/cortex/config.yaml
//  2. the project config at ./cortex.yaml
//
// The environment (LLM_PROVIDER, LLM_MODEL) overrides both, so a one-off
// shell export can switch models without editing anything.
//
// Providers and credentials are not resolved here — they live in axon's
// config. The caller loads axon.Settings separately and calls
// settings.NewClient(cfg.Provider, cfg.ModelName) to build the model.
func Load(explicitPath string) (Config, error) {
	var (
		merged  file
		sources []string
	)

	if explicitPath != "" {
		f, err := readFile(explicitPath)
		if err != nil {
			return Config{}, err
		}
		merged = f
		sources = append(sources, explicitPath)
	} else {
		for _, path := range candidatePaths() {
			f, err := readFile(path)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return Config{}, err
			}

			overlay(&merged, f)
			sources = append(sources, path)
		}
	}

	applyEnv(&merged)

	return finalize(merged, strings.Join(sources, ", "))
}

// candidatePaths lists the config files to layer, least specific first.
func candidatePaths() []string {
	var paths []string

	if dir := userConfigDir(); dir != "" {
		paths = append(paths, filepath.Join(dir, "cortex", "config.yaml"))
	}

	paths = append(paths, "cortex.yaml")

	return paths
}

func userConfigDir() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return dir
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".config")
}

// readFile parses one YAML file. A missing file surfaces as os.ErrNotExist so
// the cascade can skip it while an explicit path can refuse it.
func readFile(path string) (file, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return file{}, err
	}

	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return file{}, fmt.Errorf("config: parse %s: %w", path, err)
	}

	return f, nil
}

// overlay copies every field the higher-priority file actually set onto the
// accumulated result. Absent fields leave the lower layer intact — that is
// what makes a project config able to override only the model name.
func overlay(dst *file, src file) {
	setString(&dst.Name, src.Name)
	setString(&dst.Provider, src.Provider)
	setString(&dst.Model, src.Model)
	setString(&dst.SystemPrompt, src.SystemPrompt)
	setString(&dst.Pruner.Provider, src.Pruner.Provider)
	setString(&dst.Pruner.Model, src.Pruner.Model)

	if len(src.MCPServers) > 0 {
		if dst.MCPServers == nil {
			dst.MCPServers = map[string]mcpSrv{}
		}
		for name, srv := range src.MCPServers {
			dst.MCPServers[name] = srv
		}
	}
}

// applyEnv lets the environment win over every file. Only model selection is
// overridable — credentials and endpoints are axon's concern.
func applyEnv(f *file) {
	setString(&f.Provider, os.Getenv("LLM_PROVIDER"))
	setString(&f.Model, os.Getenv("LLM_MODEL"))
}

// setString assigns only when the incoming value carries content, so an
// unset field never blanks out a value a lower layer supplied.
func setString(dst *string, src string) {
	if v := strings.TrimSpace(src); v != "" {
		*dst = v
	}
}

// ---------------------------------------------------------------------------
// Finalisation
// ---------------------------------------------------------------------------

// finalize turns the merged file into a Config: defaults filled, indirections
// resolved. Credential resolution is deliberately absent — axon handles it.
func finalize(f file, source string) (Config, error) {
	cfg := Config{
		Name:      f.Name,
		Provider:  f.Provider,
		ModelName: f.Model,
		Source:    source,
	}

	if cfg.Name == "" {
		cfg.Name = "cortex"
	}

	if cfg.ModelName == "" {
		return Config{}, ErrNoModel
	}

	if cfg.Provider == "" {
		return Config{}, fmt.Errorf("config: model %q has no provider — set provider: in the config or export LLM_PROVIDER", cfg.ModelName)
	}

	cfg.Pruner = PrunerRef{
		Provider: f.Pruner.Provider,
		Model:    f.Pruner.Model,
	}

	prompt, err := resolveSystemPrompt(f.SystemPrompt)
	if err != nil {
		return Config{}, err
	}
	cfg.SystemPrompt = prompt

	for _, srv := range f.MCPServers {
		if srv.Command == "" {
			continue
		}
		cfg.MCPServers = append(cfg.MCPServers, MCPServer{
			Command: srv.Command,
			Args:    srv.Args,
			Env:     srv.Env,
		})
	}

	return cfg, nil
}

// resolveSystemPrompt returns the role text. A value that names a file is
// read from disk, so a long prompt can live in Markdown next to the config
// instead of being folded into a YAML scalar.
func resolveSystemPrompt(v string) (string, error) {
	v = strings.TrimSpace(v)

	if v == "" {
		return DefaultSystemPrompt, nil
	}

	if !looksLikePath(v) {
		return v, nil
	}

	data, err := os.ReadFile(v)
	if err != nil {
		return "", fmt.Errorf("config: read system_prompt %s: %w", v, err)
	}

	return string(data), nil
}

// looksLikePath distinguishes a prompt from a pointer to one. A prompt is
// prose: it has spaces and newlines. A path does not, and ends in a document
// extension or starts with a path separator.
func looksLikePath(v string) bool {
	if strings.ContainsAny(v, " \n\t") {
		return false
	}

	return strings.HasSuffix(v, ".md") ||
		strings.HasSuffix(v, ".txt") ||
		strings.HasPrefix(v, "./") ||
		strings.HasPrefix(v, "../") ||
		strings.HasPrefix(v, "/") ||
		strings.HasPrefix(v, "~/")
}

// SaveModel persists a model selection back to the user config file so the
// choice survives restarts. The file is rewritten with yaml.Node so comments
// and field order are preserved; only the top-level provider and model fields
// are touched.
//
// The path is the user config (~/.config/cortex/config.yaml). If it does not
// exist, it is created with just the provider and model, so a session that
// started from environment overrides can still pin a model.
func SaveModel(provider, model string) error {
	path := userConfigPath()
	if path == "" {
		return errors.New("config: cannot locate user config dir")
	}

	var root yaml.Node
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("config: parse %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	m := rootMapping(&root)
	setMapField(m, "provider", provider)
	setMapField(m, "model", model)

	out, err := yaml.Marshal(&root)
	if err != nil {
		return fmt.Errorf("config: marshal %s: %w", path, err)
	}

	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("config: mkdir %s: %w", dir, err)
		}
	}

	return os.WriteFile(path, out, 0o644)
}

// SavePruner persists a pruner selection back to the user config file so the
// choice survives restarts. The file is rewritten with yaml.Node so comments
// and field order are preserved; only pruner.model (and pruner.provider when
// it differs from the main provider) are touched.
//
// The path is the user config (~/.config/cortex/config.yaml). If it does not
// exist, it is created with just the pruner block, so a session that started
// from environment overrides can still pin a pruner.
func SavePruner(provider, model string) error {
	path := userConfigPath()
	if path == "" {
		return errors.New("config: cannot locate user config dir")
	}

	var root yaml.Node
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("config: parse %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	m := rootMapping(&root)
	prunerNode := findOrCreateMap(m, "pruner")
	setMapField(prunerNode, "provider", provider)
	setMapField(prunerNode, "model", model)

	out, err := yaml.Marshal(&root)
	if err != nil {
		return fmt.Errorf("config: marshal %s: %w", path, err)
	}

	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("config: mkdir %s: %w", dir, err)
		}
	}

	return os.WriteFile(path, out, 0o644)
}

// userConfigPath returns the path to the user-level config file, or "" if the
// config dir cannot be resolved.
func userConfigPath() string {
	dir := userConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "cortex", "config.yaml")
}

// rootMapping resolves the top-level mapping node from a parsed document.
// yaml.Unmarshal into a yaml.Node yields a DocumentNode (Kind==DocumentNode)
// whose first child is the mapping; when the file is empty the node is blank
// and must be created. Callers that mutate fields must operate on this
// mapping, not the document wrapper, or the marshal output is malformed.
func rootMapping(root *yaml.Node) *yaml.Node {
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		m := root.Content[0]
		if m.Kind == yaml.MappingNode {
			return m
		}
	}

	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if root.Kind == 0 {
		root.Kind = yaml.DocumentNode
	}
	root.Content = []*yaml.Node{m}
	return m
}

// findOrCreateMap returns the mapping node under key in a mapping parent,
// creating it (as a mapping) if absent. If key exists but is not a mapping,
// it is replaced.
func findOrCreateMap(parent *yaml.Node, key string) *yaml.Node {
	if parent.Kind != yaml.MappingNode {
		parent.Kind = yaml.MappingNode
	}

	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			child := parent.Content[i+1]
			if child.Kind != yaml.MappingNode {
				child = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
				parent.Content[i+1] = child
			}
			return child
		}
	}

	child := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	parent.Content = append(parent.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		child,
	)
	return child
}

// setMapField sets a string field in a mapping node, creating or replacing it.
func setMapField(m *yaml.Node, key, value string) {
	if m.Kind != yaml.MappingNode {
		m.Kind = yaml.MappingNode
	}

	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1].Value = value
			m.Content[i+1].Kind = yaml.ScalarNode
			m.Content[i+1].Tag = "!!str"
			return
		}
	}

	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
	)
}

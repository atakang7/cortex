package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The cascade is the risky part of this package: it decides which of two
// sources wins, and getting it wrong points the agent at the wrong model with
// no visible symptom until a request fails. Everything here exercises the
// real Load against real files on disk.

// workspace redirects both config locations at temporary directories and
// clears every environment variable Load consults, so a test observes only
// what it set up. It returns the user-config directory and the working
// directory, in that order.
func workspace(t *testing.T) (userDir, workDir string) {
	t.Helper()

	root := t.TempDir()

	userDir = filepath.Join(root, "xdg")
	if err := os.MkdirAll(filepath.Join(userDir, "cortex"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", userDir)

	for _, key := range []string{"LLM_PROVIDER", "LLM_MODEL"} {
		t.Setenv(key, "")
	}

	workDir = filepath.Join(root, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workDir)

	return userDir, workDir
}

func write(t *testing.T, path, body string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProjectConfigOverridesUserConfigFieldByField(t *testing.T) {
	userDir, workDir := workspace(t)

	write(t, filepath.Join(userDir, "cortex", "config.yaml"), `
name: personal
provider: openai
model: gpt-4o
`)

	// The project names only a model. Everything else must survive from the
	// user config — that is the whole point of layering rather than replacing.
	write(t, filepath.Join(workDir, "cortex.yaml"), `
model: gpt-4o-mini
`)

	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}

	if cfg.ModelName != "gpt-4o-mini" {
		t.Errorf("model = %q, want the project's gpt-4o-mini", cfg.ModelName)
	}
	if cfg.Provider != "openai" {
		t.Errorf("provider = %q, want openai inherited from the user config", cfg.Provider)
	}
	if cfg.Name != "personal" {
		t.Errorf("name = %q, want personal inherited from the user config", cfg.Name)
	}
}

func TestEnvironmentOverridesEveryFile(t *testing.T) {
	_, workDir := workspace(t)

	write(t, filepath.Join(workDir, "cortex.yaml"), `
provider: openai
model: gpt-4o
`)

	t.Setenv("LLM_MODEL", "llama3")

	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}

	if cfg.ModelName != "llama3" {
		t.Errorf("model = %q, want llama3 from the environment", cfg.ModelName)
	}
}

func TestExplicitPathIgnoresTheCascade(t *testing.T) {
	userDir, workDir := workspace(t)

	write(t, filepath.Join(userDir, "cortex", "config.yaml"), "name: personal\nprovider: openai\nmodel: gpt-4o\n")
	write(t, filepath.Join(workDir, "cortex.yaml"), "name: project\nprovider: openai\nmodel: gpt-4o\n")

	explicit := filepath.Join(workDir, "reviewer.yaml")
	write(t, explicit, "name: reviewer\nprovider: openai\nmodel: o3\n")

	cfg, err := Load(explicit)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Name != "reviewer" {
		t.Errorf("name = %q, want reviewer — the other files must not be read", cfg.Name)
	}
}

func TestExplicitPathMustExist(t *testing.T) {
	workspace(t)

	if _, err := Load("nope.yaml"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want os.ErrNotExist — a named file that is missing is a mistake, not a fallback", err)
	}
}

func TestNoModelAnywhereIsDistinguishable(t *testing.T) {
	workspace(t)

	// A first run with nothing configured must be tellable apart from a
	// broken config, because the caller turns it into onboarding help.
	if _, err := Load(""); !errors.Is(err, ErrNoModel) {
		t.Fatalf("err = %v, want ErrNoModel", err)
	}
}

func TestMissingProviderIsAnError(t *testing.T) {
	_, workDir := workspace(t)

	write(t, filepath.Join(workDir, "cortex.yaml"), "model: gpt-4o\n")

	_, err := Load("")
	if err == nil {
		t.Fatal("want an error: a model with no provider cannot be resolved")
	}
	if !strings.Contains(err.Error(), "provider") {
		t.Errorf("err = %v, want it to mention provider", err)
	}
}

func TestSystemPromptIsReadFromAFileWhenItNamesOne(t *testing.T) {
	_, workDir := workspace(t)

	write(t, filepath.Join(workDir, "reviewer.md"), "You review code.")
	write(t, filepath.Join(workDir, "cortex.yaml"), "provider: openai\nmodel: gpt-4o\nsystem_prompt: ./reviewer.md\n")

	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}

	if cfg.SystemPrompt != "You review code." {
		t.Errorf("system prompt = %q, want the file's contents", cfg.SystemPrompt)
	}
}

func TestSystemPromptProseIsUsedLiterally(t *testing.T) {
	_, workDir := workspace(t)

	write(t, filepath.Join(workDir, "cortex.yaml"), "provider: openai\nmodel: gpt-4o\nsystem_prompt: You are terse.\n")

	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}

	if cfg.SystemPrompt != "You are terse." {
		t.Errorf("system prompt = %q, want it used as written", cfg.SystemPrompt)
	}
}

func TestAbsentSystemPromptFallsBackToTheDefault(t *testing.T) {
	_, workDir := workspace(t)

	write(t, filepath.Join(workDir, "cortex.yaml"), "provider: openai\nmodel: gpt-4o\n")

	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}

	if cfg.SystemPrompt != DefaultSystemPrompt {
		t.Error("want the built-in coding-agent prompt when the config supplies none")
	}
}

func TestLooksLikePath(t *testing.T) {
	paths := []string{"./reviewer.md", "../shared/prompt.md", "/etc/cortex/prompt.txt", "~/prompts/go.md", "prompt.md"}
	for _, v := range paths {
		if !looksLikePath(v) {
			t.Errorf("looksLikePath(%q) = false, want true", v)
		}
	}

	prose := []string{"You are terse.", "Review code.\nBe brief.", "You are cortex"}
	for _, v := range prose {
		if looksLikePath(v) {
			t.Errorf("looksLikePath(%q) = true, want false", v)
		}
	}
}

func TestDefaultNameIsCortex(t *testing.T) {
	_, workDir := workspace(t)

	write(t, filepath.Join(workDir, "cortex.yaml"), "provider: ollama\nmodel: llama3\n")

	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Name != "cortex" {
		t.Errorf("name = %q, want cortex as the default", cfg.Name)
	}
}

package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// corruptInput is the exact bytes that were on the user's disk before this
// fix — a leading "?" explicit-key marker from the old SaveModel bug.
const corruptInput = `? # cortex — user defaults.
#
# Providers and credentials live in axon's config (~/.config/axon/axon.yaml
# and .env). This file says which of those to use and what personality to
# adopt. A ./cortex.yaml in a project overrides these field by field, and
# LLM_PROVIDER / LLM_MODEL override both.
name: cortex
provider: openrouter
model: deepseek/deepseek-v3.2
pruner:
    provider: openrouter
    model: poolside/laguna-s-2.1:free
`

// validInput is the same content but with the header as a real comment block.
const validInput = `# cortex — user defaults.
#
# Providers and credentials live in axon's config (~/.config/axon/axon.yaml
# and .env). This file says which of those to use and what personality to
# adopt. A ./cortex.yaml in a project overrides these field by field, and
# LLM_PROVIDER / LLM_MODEL override both.
name: cortex
provider: openrouter
model: deepseek/deepseek-v3.2
pruner:
    provider: openrouter
    model: poolside/laguna-s-2.1:free
`

func parsesClean(t *testing.T, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var v map[string]any
	if err := yaml.Unmarshal(b, &v); err != nil {
		t.Fatalf("output is not valid YAML: %v\n--- file ---\n%s", err, b)
	}
	if v["provider"] != "anthropic" {
		t.Errorf("provider=%v, want anthropic", v["provider"])
	}
	if v["model"] != "claude-3-5-sonnet" {
		t.Errorf("model=%v, want claude-3-5-sonnet", v["model"])
	}
	if p, _ := v["pruner"].(map[string]any); p != nil {
		if p["model"] != "poolside/laguna-s-2.1:free" {
			t.Errorf("pruner.model=%v, want poolside/laguna-s-2.1:free (must be untouched)", p["model"])
		}
	}
	if v["name"] != "cortex" {
		t.Errorf("name=%v, want cortex", v["name"])
	}
}

func TestSaveModelFromValidFileProducesCleanYAML(t *testing.T) {
	userDir, _ := workspace(t)
	write(t, filepath.Join(userDir, "cortex", "config.yaml"), validInput)
	if err := SaveModel("anthropic", "claude-3-5-sonnet"); err != nil {
		t.Fatal(err)
	}
	parsesClean(t, filepath.Join(userDir, "cortex", "config.yaml"))
}

func TestSaveModelRecoversFromCorruptFile(t *testing.T) {
	userDir, _ := workspace(t)
	write(t, filepath.Join(userDir, "cortex", "config.yaml"), corruptInput)
	// The corrupt file cannot be unmarshaled into a Node cleanly; SaveModel
	// must still produce valid YAML rather than re-corrupting it.
	err := SaveModel("anthropic", "claude-3-5-sonnet")
	if err != nil {
		t.Fatalf("SaveModel on a corrupt input should recover, got: %v", err)
	}
	parsesClean(t, filepath.Join(userDir, "cortex", "config.yaml"))
}

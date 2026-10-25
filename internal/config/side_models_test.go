package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A config written for the retired free review pool must still load, with the
// pool ignored and the user told so: checkpoints fall to the main model.
func TestRetiredPrunerPoolLoadsAndIsReported(t *testing.T) {
	cfg, err := finalize(file{Provider: "openrouter", Model: "main", Pruner: prunerFile{Models: []string{"a:free", "b:free"}}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Pruner.Model != "" {
		t.Fatalf("the pool became a checkpoint model: %+v", cfg.Pruner)
	}
	if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], "the main model") {
		t.Fatalf("warnings %v", cfg.Warnings)
	}
}

func TestExplorerAndPrunerLayerLikeTheModel(t *testing.T) {
	f := file{Provider: "openrouter", Model: "main", Explorer: modelFile{Model: "cheap"}}
	overlay(&f, file{Pruner: prunerFile{Model: "judge"}, Explorer: modelFile{Provider: "local", Model: "cheaper"}})
	cfg, err := finalize(f, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Pruner.Model != "judge" || cfg.Explorer != (ExplorerRef{Provider: "local", Model: "cheaper"}) {
		t.Fatalf("pruner %+v explorer %+v", cfg.Pruner, cfg.Explorer)
	}
}

func TestSaveSideModelsSetAndClear(t *testing.T) {
	userDir, _ := workspace(t)
	path := filepath.Join(userDir, "cortex", "config.yaml")
	write(t, path, "# mine\nprovider: openrouter\nmodel: main\npruner:\n  models: [a:free, b:free]\n")

	if err := SavePruner("openrouter", "judge"); err != nil {
		t.Fatal(err)
	}
	if err := SaveExplorer("openrouter", "cheap"); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Pruner.Model != "judge" || cfg.Explorer.Model != "cheap" || len(cfg.Warnings) != 0 {
		t.Fatalf("pruner %+v explorer %+v warnings %v", cfg.Pruner, cfg.Explorer, cfg.Warnings)
	}

	if err := SavePruner("", ""); err != nil {
		t.Fatal(err)
	}
	if err := SaveExplorer("", ""); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Pruner.Model != "" || cfg.Explorer.Model != "" {
		t.Fatalf("not cleared: pruner %+v explorer %+v", cfg.Pruner, cfg.Explorer)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "# mine") || strings.Contains(string(data), "a:free") {
		t.Fatalf("config rewritten badly:\n%s", data)
	}
}

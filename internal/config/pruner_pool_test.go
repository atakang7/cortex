package config

import (
	"path/filepath"
	"testing"
)

func TestPrunerPoolValidationAndSingleModelOverride(t *testing.T) {
	for _, models := range [][]string{{"a:free"}, {"a:free", "a:free"}, {"a:free", "b:paid"}} {
		_, err := finalize(file{Provider: "openrouter", Model: "main", Pruner: prunerFile{Models: models}}, "")
		if err == nil {
			t.Fatalf("accepted %v", models)
		}
	}
	f := file{Provider: "openrouter", Model: "main", Pruner: prunerFile{Models: []string{"a:free", "b:free"}}}
	if _, err := finalize(f, ""); err != nil {
		t.Fatal(err)
	}
	overlay(&f, file{Pruner: prunerFile{Model: "single"}})
	if len(f.Pruner.Models) != 0 {
		t.Fatal("single override left pool active")
	}
}

func TestSavePrunerRemovesPool(t *testing.T) {
	userDir, _ := workspace(t)
	write(t, filepath.Join(userDir, "cortex", "config.yaml"), "provider: openrouter\nmodel: main\npruner:\n  models: [a:free, b:free]\n")
	if err := SavePruner("openrouter", "single"); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Pruner.Models) != 0 || cfg.Pruner.Model != "single" {
		t.Fatalf("wrong pruner: %+v", cfg.Pruner)
	}
}

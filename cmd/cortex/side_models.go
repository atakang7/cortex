package main

import (
	"fmt"

	"github.com/atakang7/axon/v2"
	"github.com/atakang7/cortex/v2/internal/config"
	"github.com/atakang7/cortex/v2/internal/ui"
)

// sideModels are the models besides the main one. Both are optional:
// without a pruner the main model decides checkpoints, which is the default,
// and without an explorer there is no explore tool.
type sideModels struct {
	pruner axon.Model
	// explorers are tried in turn: the configured explorer, then the pool.
	explorers []axon.Model
}

// resolveSideModels builds the configured side models. Either may use any
// configured model; which one is the user's choice. A model left empty takes
// the main model's provider.
func resolveSideModels(cfg config.Config, s axon.Settings, trace *ui.TraceLog) (sideModels, error) {
	var out sideModels
	if cfg.Pruner.Model != "" {
		m, err := s.NewClient(orProvider(cfg.Pruner.Provider, cfg.Provider), cfg.Pruner.Model)
		if err != nil {
			return out, fmt.Errorf("pruner: %w", err)
		}
		out.pruner = trace.Wiretap("pruner", m)
	}
	if cfg.Explorer.Model != "" {
		for _, id := range append([]string{cfg.Explorer.Model}, cfg.Explorer.Models...) {
			m, err := s.NewClient(orProvider(cfg.Explorer.Provider, cfg.Provider), id)
			if err != nil {
				return out, fmt.Errorf("explorer %s: %w", id, err)
			}
			out.explorers = append(out.explorers, trace.Wiretap("explorer", m))
		}
	}
	return out, nil
}

func orProvider(provider, fallback string) string {
	if provider == "" {
		return fallback
	}
	return provider
}

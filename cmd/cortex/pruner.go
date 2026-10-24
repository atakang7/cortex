package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/atakang7/axon/v2"
	"github.com/atakang7/cortex/v2/internal/config"
	"github.com/atakang7/cortex/v2/internal/ui"
)

func resolvePruner(cfg config.Config, s axon.Settings, trace *ui.TraceLog) (axon.Model, error) {
	provider := cfg.Pruner.Provider
	if provider == "" {
		provider = cfg.Provider
	}
	if len(cfg.Pruner.Models) == 0 {
		if cfg.Pruner.Model == "" {
			return nil, nil
		}
		m, err := s.NewPrunerClient(provider, cfg.Pruner.Model)
		if err != nil {
			return nil, fmt.Errorf("pruner: %w", err)
		}
		return trace.Wiretap("pruner", m), nil
	}
	var reviewers []axon.Model
	for slot, id := range cfg.Pruner.Models {
		m, err := s.NewPrunerClient(provider, id, slot)
		if err != nil {
			return nil, err
		}
		reviewers = append(reviewers, trace.Wiretap("pruner:"+id, m))
	}
	return axon.NewConsensusPruner(reviewers...)
}

func freePrunerProvider(s axon.Settings, provider, id string) (axon.Provider, error) {
	if !strings.HasSuffix(id, ":free") {
		return axon.Provider{}, fmt.Errorf("pruner pool accepts only :free models")
	}
	p, err := s.Provider(provider, id)
	if err != nil {
		return axon.Provider{}, err
	}
	u, err := url.Parse(p.BaseURL)
	if err != nil || u.Scheme != "https" || u.Host != "openrouter.ai" {
		return axon.Provider{}, fmt.Errorf("free pruner pool requires the HTTPS OpenRouter endpoint")
	}
	options := map[string]any{}
	if len(p.Extra) > 0 {
		if err := json.Unmarshal(p.Extra, &options); err != nil {
			return axon.Provider{}, err
		}
	}
	// Free-only at both model and endpoint level: no silent paid fallback.
	options["max_price"] = map[string]float64{"prompt": 0, "completion": 0, "request": 0}
	options["require_parameters"] = true
	p.Extra, _ = json.Marshal(options)
	return p, nil
}

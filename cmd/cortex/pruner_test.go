package main

import (
	"encoding/json"
	"testing"

	"github.com/atakang7/axon/v2"
)

func TestFreePrunerForbidsPaidRouting(t *testing.T) {
	s := axon.DefaultSettings()
	s.Providers = map[string]axon.Endpoint{"openrouter": {BaseURL: "https://openrouter.ai/api/v1", Models: map[string]axon.ModelOptions{"example/model:free": {}, "example/paid": {}}}}
	p, err := freePrunerProvider(s, "openrouter", "example/model:free")
	if err != nil {
		t.Fatal(err)
	}
	var opts struct {
		MaxPrice map[string]float64 `json:"max_price"`
		Require  bool               `json:"require_parameters"`
	}
	if err := json.Unmarshal(p.Extra, &opts); err != nil {
		t.Fatal(err)
	}
	if !opts.Require || len(opts.MaxPrice) != 3 {
		t.Fatalf("missing price/capability guard: %s", p.Extra)
	}
	for _, v := range opts.MaxPrice {
		if v != 0 {
			t.Fatal("nonzero price")
		}
	}
	if _, err := freePrunerProvider(s, "openrouter", "example/paid"); err == nil {
		t.Fatal("paid model accepted")
	}
	e := s.Providers["openrouter"]
	e.BaseURL = "https://openrouter.ai.evil.invalid/api/v1"
	s.Providers["openrouter"] = e
	if _, err := freePrunerProvider(s, "openrouter", "example/model:free"); err == nil {
		t.Fatal("wrong host accepted")
	}
}

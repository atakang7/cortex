package main

import (
	"math"
	"testing"

	"github.com/atakang7/axon/v2"
)

func TestCheapestEndpointAccountsForCacheHits(t *testing.T) {
	eps := []modelEndpoint{
		// Cheapest list price, but no prompt cache.
		{Tag: "nocache", Prompt: 1e-6, Completion: 4e-6, Tools: true, Context: 200000, Uptime: 100},
		// Pricier input, cheap cache reads.
		{Tag: "cached", Prompt: 1.2e-6, Completion: 4e-6, CacheRead: 0.1e-6, Caching: true, Tools: true, Context: 200000, Uptime: 100},
		// Cheapest of all, but cannot call tools.
		{Tag: "notools", Prompt: 0.1e-6, Completion: 0.1e-6, Tools: false, Context: 200000, Uptime: 100},
		// Too small a context for the agent's budget.
		{Tag: "small", Prompt: 0.2e-6, Completion: 0.2e-6, Tools: true, Context: 32000, Uptime: 100},
		// Down.
		{Tag: "flaky", Prompt: 0.3e-6, Completion: 0.3e-6, Tools: true, Context: 200000, Uptime: 40},
	}
	heavyCache := usageProfile{Input: 30000, Output: 1000, HitRate: 0.8}
	if i := rankEndpoints(eps, heavyCache); eps[i].Tag != "cached" {
		t.Fatalf("picked %s; with an 80%% hit rate the caching endpoint is cheaper", eps[i].Tag)
	}
	want := 30000*(0.2*1.2e-6+0.8*0.1e-6) + 1000*4e-6
	if got := eps[1].CostPerReq; math.Abs(got-want) > 1e-12 {
		t.Fatalf("cost %g, want %g", got, want)
	}
	noCache := usageProfile{Input: 30000, Output: 1000, HitRate: 0}
	if i := rankEndpoints(eps, noCache); eps[i].Tag != "nocache" {
		t.Fatalf("picked %s; without cache hits the lower list price wins", eps[i].Tag)
	}
	for _, e := range eps[2:] {
		if e.Eligible {
			t.Fatalf("%s should be ineligible", e.Tag)
		}
	}
}

func TestProfileComesFromTheSessionsAccounting(t *testing.T) {
	if p := profileOf(nil); p.Input == 0 || p.HitRate == 0 {
		t.Fatalf("no default profile: %+v", p)
	}
	s := &axon.Session{PromptTokens: 100000, CompletionTokens: 4000, CachedTokens: 75000, Requests: 4}
	p := profileOf(s)
	if p.Input != 25000 || p.Output != 1000 || p.HitRate != 0.75 {
		t.Fatalf("profile %+v", p)
	}
}

func TestFavoritesAndSearch(t *testing.T) {
	var p modelPrefs
	p.setFavorite("z-ai/glm-5.2", true)
	p.setFavorite("a/b", true)
	p.setFavorite("z-ai/glm-5.2", false)
	p.setFavorite("a/b", true)
	if len(p.Favorites) != 1 || p.Favorites[0] != "a/b" {
		t.Fatalf("favorites %v", p.Favorites)
	}
	models := []catalogModel{{ID: "x/coder", Name: "X Coder", Tools: true}, {ID: "a/b", Name: "Zed B", Tools: true}, {ID: "x/chat", Name: "X Chat", Tools: false}}
	got := searchCatalog(models, "", p.Favorites, 10)
	if len(got) != 2 || got[0].ID != "a/b" {
		t.Fatalf("favourites should lead and tool-less models be hidden: %v", got)
	}
	if got := searchCatalog(models, "x cod", nil, 10); len(got) != 1 || got[0].ID != "x/coder" {
		t.Fatalf("search %v", got)
	}
}

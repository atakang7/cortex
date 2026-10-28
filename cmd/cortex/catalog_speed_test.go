package main

import "testing"

func TestRankPrefersCheapestAmongFastEndpoints(t *testing.T) {
	eps := []modelEndpoint{
		{Tag: "cheap-slow", Prompt: 0.1, Completion: 0.1, Tools: true, Context: 200000, Uptime: 100, Speed: 13},
		{Tag: "fast-pricey", Prompt: 0.5, Completion: 0.5, Tools: true, Context: 200000, Uptime: 100, Speed: 70},
		{Tag: "fast-cheaper", Prompt: 0.3, Completion: 0.3, Tools: true, Context: 200000, Uptime: 100, Speed: 50},
	}
	if i := rankEndpoints(eps, usageProfile{Input: 1000, Output: 100}); eps[i].Tag != "fast-cheaper" {
		t.Fatalf("picked %s, want the cheapest of the fast ones", eps[i].Tag)
	}
	for i := range eps {
		eps[i].Speed = 0
	}
	if i := rankEndpoints(eps, usageProfile{Input: 1000, Output: 100}); eps[i].Tag != "cheap-slow" {
		t.Fatalf("without speed figures price decides; picked %s", eps[i].Tag)
	}
}

func TestRankNeverPicksANonCachingEndpointWhenAFastOneCaches(t *testing.T) {
	eps := []modelEndpoint{
		{Tag: "nocache-cheap", Prompt: 0.05, Completion: 0.1, Tools: true, Context: 200000, Uptime: 100, Speed: 60},
		{Tag: "cache", Prompt: 0.2, Completion: 0.3, CacheRead: 0.02, Caching: true, Tools: true, Context: 200000, Uptime: 100, Speed: 55},
	}
	if i := rankEndpoints(eps, usageProfile{Input: 1000, Output: 1000, HitRate: 0.01}); eps[i].Tag != "cache" {
		t.Fatalf("picked %s, want the caching endpoint", eps[i].Tag)
	}
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/atakang7/axon/v2"
	"github.com/atakang7/cortex/v2/internal/config"
)

// The model catalog: OpenRouter lists every model and, per model, every
// provider endpoint with its prices. Cortex picks from that directly, so the
// config no longer has to name models. Choices (favourites, the selected
// model, the provider route) live in models.json next to config.yaml.

const (
	openRouterAPI   = "https://openrouter.ai/api/v1"
	catalogTTL      = 6 * time.Hour
	catalogTimeout  = 15 * time.Second
	endpointTimeout = 8 * time.Second
	// minEndpointContext keeps endpoints that fit the agent's 70K input
	// budget plus a long reply.
	minEndpointContext = 100000
)

// modelPrefs are the user's model choices.
type modelPrefs struct {
	Favorites []string `json:"favorites,omitempty"`
	Provider  string   `json:"provider,omitempty"` // axon endpoint name, e.g. "openrouter"
	Model     string   `json:"model,omitempty"`
	// Route is an OpenRouter provider tag to prefer; "" picks automatically.
	Route string `json:"route,omitempty"`
}

func prefsPath() string {
	if dir := config.UserDir(); dir != "" {
		return filepath.Join(dir, "models.json")
	}
	return ""
}

func loadModelPrefs() modelPrefs {
	var p modelPrefs
	if path := prefsPath(); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, &p)
		}
	}
	return p
}

func saveModelPrefs(p modelPrefs) error {
	path := prefsPath()
	if path == "" {
		return fmt.Errorf("no user config directory to save model preferences in")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return axon.WriteFileAtomic(path, data)
}

func (p *modelPrefs) setFavorite(id string, on bool) {
	out := p.Favorites[:0:0]
	for _, f := range p.Favorites {
		if f != id {
			out = append(out, f)
		}
	}
	if on {
		out = append(out, id)
	}
	p.Favorites = out
}

// catalogModel is one OpenRouter model. Prices are USD per token.
type catalogModel struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Context    int     `json:"context"`
	Prompt     float64 `json:"prompt"`
	Completion float64 `json:"completion"`
	CacheRead  float64 `json:"cache_read"`
	Tools      bool    `json:"tools"`
	// Variable marks router models (openrouter/auto, …) priced "-1": they
	// bill whatever model they route to, so they are never free.
	Variable bool `json:"variable,omitempty"`
}

type rawPricing struct {
	Prompt         string `json:"prompt"`
	Completion     string `json:"completion"`
	InputCacheRead string `json:"input_cache_read"`
}

func price(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	if f < 0 { // OpenRouter uses -1 for "variable" router pricing; see variable
		return 0
	}
	return f
}

// variable reports router pricing: the cost is the routed model's, unknown here.
func (p rawPricing) variable() bool {
	in, _ := strconv.ParseFloat(p.Prompt, 64)
	out, _ := strconv.ParseFloat(p.Completion, 64)
	return in < 0 || out < 0
}

func hasTools(params []string) bool {
	for _, p := range params {
		if p == "tools" {
			return true
		}
	}
	return false
}

// loadCatalog returns OpenRouter's model list, from a cache younger than
// catalogTTL when there is one, refreshed otherwise. A failed refresh falls
// back to a stale cache rather than an empty picker.
func loadCatalog(ctx context.Context) ([]catalogModel, error) {
	cache := ""
	if dir := config.UserDir(); dir != "" {
		cache = filepath.Join(dir, "catalog-v2.json") // v1 recorded router prices as zero
	}
	var cached struct {
		Fetched time.Time      `json:"fetched"`
		Models  []catalogModel `json:"models"`
	}
	if cache != "" {
		if data, err := os.ReadFile(cache); err == nil && json.Unmarshal(data, &cached) == nil && time.Since(cached.Fetched) < catalogTTL && len(cached.Models) > 0 {
			return cached.Models, nil
		}
	}
	models, err := fetchCatalog(ctx)
	if err != nil {
		if len(cached.Models) > 0 {
			return cached.Models, nil
		}
		return nil, err
	}
	if cache != "" {
		cached.Fetched, cached.Models = time.Now(), models
		if data, err := json.Marshal(cached); err == nil {
			_ = os.MkdirAll(filepath.Dir(cache), 0o755)
			_ = axon.WriteFileAtomic(cache, data)
		}
	}
	return models, nil
}

func fetchCatalog(ctx context.Context) ([]catalogModel, error) {
	var body struct {
		Data []struct {
			ID            string     `json:"id"`
			Name          string     `json:"name"`
			ContextLength int        `json:"context_length"`
			Pricing       rawPricing `json:"pricing"`
			Params        []string   `json:"supported_parameters"`
		} `json:"data"`
	}
	if err := getJSON(ctx, openRouterAPI+"/models", catalogTimeout, &body); err != nil {
		return nil, err
	}
	out := make([]catalogModel, 0, len(body.Data))
	for _, m := range body.Data {
		out = append(out, catalogModel{ID: m.ID, Name: m.Name, Context: m.ContextLength,
			Prompt: price(m.Pricing.Prompt), Completion: price(m.Pricing.Completion), CacheRead: price(m.Pricing.InputCacheRead), Tools: hasTools(m.Params),
			Variable: m.Pricing.variable()})
	}
	return out, nil
}

func getJSON(ctx context.Context, url string, timeout time.Duration, into any) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}

// isFree reports a model OpenRouter serves at no charge (":free" variants and
// zero-priced models). Routers with variable pricing bill the model they pick.
func isFree(m catalogModel) bool {
	return !m.Variable && (strings.HasSuffix(m.ID, ":free") || (m.Prompt == 0 && m.Completion == 0))
}

// freeModels are every free model, tools or not: the pruner only needs JSON.
func freeModels(models []catalogModel) []catalogModel {
	var out []catalogModel
	for _, m := range models {
		if isFree(m) {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// searchCatalog returns tool-capable models matching every word of query
// (only free ones when freeOnly), favourites first, then by name; at most
// limit. The limit used to be 60, which cut the list off before most free
// models in alphabetical order.
func searchCatalog(models []catalogModel, query string, favorites []string, limit int, freeOnly ...bool) []catalogModel {
	fav := map[string]bool{}
	for _, f := range favorites {
		fav[f] = true
	}
	words := strings.Fields(strings.ToLower(query))
	var out []catalogModel
	for _, m := range models {
		if !m.Tools || (len(freeOnly) > 0 && freeOnly[0] && !isFree(m)) {
			continue
		}
		hay := strings.ToLower(m.ID + " " + m.Name)
		ok := true
		for _, w := range words {
			if !strings.Contains(hay, w) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if fav[out[i].ID] != fav[out[j].ID] {
			return fav[out[i].ID]
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// modelEndpoint is one provider serving a model, with its expected cost per
// request for this session's usage.
type modelEndpoint struct {
	Provider   string  `json:"provider"`
	Tag        string  `json:"tag"`
	Context    int     `json:"context"`
	Prompt     float64 `json:"prompt"`
	Completion float64 `json:"completion"`
	CacheRead  float64 `json:"cache_read"`
	Caching    bool    `json:"caching"`
	Tools      bool    `json:"tools"`
	Uptime     float64 `json:"uptime"`
	Status     int     `json:"status"`
	Quant      string  `json:"quantization,omitempty"`
	Variable   bool    `json:"variable,omitempty"`
	Eligible   bool    `json:"eligible"`
	CostPerReq float64 `json:"cost_per_request"`
}

func fetchEndpoints(ctx context.Context, model string) ([]modelEndpoint, error) {
	var body struct {
		Data struct {
			Endpoints []struct {
				ProviderName string     `json:"provider_name"`
				Tag          string     `json:"tag"`
				Context      int        `json:"context_length"`
				Pricing      rawPricing `json:"pricing"`
				Params       []string   `json:"supported_parameters"`
				Status       int        `json:"status"`
				Uptime30m    *float64   `json:"uptime_last_30m"`
				Implicit     bool       `json:"supports_implicit_caching"`
				Quantization string     `json:"quantization"`
			} `json:"endpoints"`
		} `json:"data"`
	}
	if err := getJSON(ctx, openRouterAPI+"/models/"+model+"/endpoints", endpointTimeout, &body); err != nil {
		return nil, err
	}
	var out []modelEndpoint
	for _, e := range body.Data.Endpoints {
		ep := modelEndpoint{Provider: e.ProviderName, Tag: e.Tag, Context: e.Context, Prompt: price(e.Pricing.Prompt),
			Completion: price(e.Pricing.Completion), CacheRead: price(e.Pricing.InputCacheRead), Tools: hasTools(e.Params),
			Status: e.Status, Quant: e.Quantization, Uptime: -1, Variable: e.Pricing.variable()}
		ep.Caching = e.Implicit || ep.CacheRead > 0
		if e.Uptime30m != nil {
			ep.Uptime = *e.Uptime30m
		}
		if ep.Tag == "" {
			ep.Tag = strings.ToLower(ep.Provider)
		}
		out = append(out, ep)
	}
	return out, nil
}

// usageProfile is what one request of this session costs in tokens.
type usageProfile struct {
	Input, Output float64
	HitRate       float64 // share of input served from the prompt cache
}

// profileOf derives the profile from a session's accounting. Without data it
// assumes a typical agent request: a large, mostly repeated prompt and a
// short reply.
func profileOf(s *axon.Session) usageProfile {
	if s == nil || s.Requests == 0 || s.PromptTokens == 0 {
		return usageProfile{Input: 30000, Output: 1500, HitRate: 0.6}
	}
	n := float64(s.Requests)
	return usageProfile{Input: float64(s.PromptTokens) / n, Output: float64(s.CompletionTokens) / n,
		HitRate: min(1, float64(s.CachedTokens)/float64(s.PromptTokens))}
}

// costPerRequest is the expected USD cost of one request on e:
//
//	input × ((1 − h) × prompt + h × cache_read) + output × completion
//
// h is the cache hit rate, taken as zero where the endpoint does not cache.
// Staying on one provider is also what keeps the hit rate high.
func costPerRequest(e modelEndpoint, u usageProfile) float64 {
	h, cache := u.HitRate, e.CacheRead
	if !e.Caching {
		h = 0
	}
	if cache <= 0 {
		cache = e.Prompt
	}
	return u.Input*((1-h)*e.Prompt+h*cache) + u.Output*e.Completion
}

// rankEndpoints fills costs and eligibility and returns the index of the
// cheapest eligible endpoint (tools, context for the budget, up), or -1.
// Ties go to the higher uptime.
func rankEndpoints(eps []modelEndpoint, u usageProfile) int {
	best := -1
	for i := range eps {
		e := &eps[i]
		e.CostPerReq = costPerRequest(*e, u)
		// An unknown price cannot be ranked, and must never look like zero.
		e.Eligible = !e.Variable && e.Tools && e.Context >= minEndpointContext && e.Status >= 0 && (e.Uptime < 0 || e.Uptime >= 90)
		if !e.Eligible {
			continue
		}
		if best < 0 || e.CostPerReq < eps[best].CostPerReq || (e.CostPerReq == eps[best].CostPerReq && e.Uptime > eps[best].Uptime) {
			best = i
		}
	}
	return best
}

// resolveRoute returns the provider tag to route model to: the user's
// manual choice, or the cheapest eligible endpoint for profile. "" means let
// OpenRouter route (not OpenRouter, lookup failed, or nothing eligible).
func resolveRoute(ctx context.Context, settings axon.Settings, provider, model string, prefs modelPrefs, u usageProfile) (string, []modelEndpoint) {
	if !isOpenRouterEndpoint(settings, provider) || strings.Contains(model, ":") {
		return "", nil
	}
	if prefs.Route != "" && prefs.Model == model {
		return prefs.Route, nil
	}
	eps, err := fetchEndpoints(ctx, model)
	if err != nil {
		return "", nil
	}
	if i := rankEndpoints(eps, u); i >= 0 {
		return eps[i].Tag, eps
	}
	return "", eps
}

func isOpenRouterEndpoint(settings axon.Settings, provider string) bool {
	e, ok := settings.Providers[provider]
	return ok && strings.Contains(e.BaseURL, "openrouter.ai")
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/atakang7/axon"
)

var ErrAmbiguousProvider = fmt.Errorf("multiple provider/model pairs configured")

func DataDir() string {
	if dir := os.Getenv("CORTEX_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cortex")
}

func ProvidersPath() string {
	if p := os.Getenv("CORTEX_PROVIDERS_PATH"); p != "" {
		return p
	}
	return filepath.Join(DataDir(), "providers.json")
}

func EnvString(k string) string {
	return os.Getenv(k)
}

func LoadProviders() (map[string]axon.Provider, error) {
	out := map[string]axon.Provider{}
	data, err := os.ReadFile(ProvidersPath())
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg struct {
		Providers []struct {
			Name     string          `json:"name"`
			BaseURL  string          `json:"base_url"`
			Model    string          `json:"model"`
			APIKey   string          `json:"api_key"`
			Provider json.RawMessage `json:"provider"`
			Models   []struct {
				Model    string          `json:"model"`
				Alias    string          `json:"alias,omitempty"`
				Provider json.RawMessage `json:"provider,omitempty"`
			} `json:"models"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	for _, p := range cfg.Providers {
		if p.Name == "" {
			return nil, fmt.Errorf("provider name required")
		}
		name := strings.ToLower(p.Name)
		defaultExtra := normalizeExtra(p.Provider)
		models := p.Models
		if len(models) == 0 && p.Model != "" {
			models = append(models, struct {
				Model    string          `json:"model"`
				Alias    string          `json:"alias,omitempty"`
				Provider json.RawMessage `json:"provider,omitempty"`
			}{Model: p.Model})
		}
		if len(models) == 0 {
			return nil, fmt.Errorf("provider %q has no models", name)
		}
		for _, m := range models {
			if m.Model == "" {
				return nil, fmt.Errorf("provider %q has a model entry with no model field", name)
			}
			extra := normalizeExtra(m.Provider)
			if extra == nil {
				extra = defaultExtra
			}
			out[name+"/"+m.Model] = axon.Provider{
				Name: name, BaseURL: p.BaseURL, Model: m.Model, APIKey: p.APIKey, Extra: extra,
			}
		}
	}
	return out, nil
}

func normalizeExtra(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	if !strings.HasPrefix(strings.TrimSpace(string(raw)), "\"") {
		return raw
	}
	var slug string
	if err := json.Unmarshal(raw, &slug); err != nil {
		return raw
	}
	if slug == "" {
		return nil
	}
	expanded, _ := json.Marshal(map[string]any{"order": []string{slug}, "allow_fallbacks": true})
	return expanded
}

func ResolveProvider(providers map[string]axon.Provider) (axon.Provider, error) {
	if sel := EnvString("LLM_PROVIDER"); sel != "" {
		if p, ok := providers[strings.ToLower(sel)]; ok {
			return ApplyEnvOverrides(p)
		}
		matches := providersByName(providers, sel)
		if len(matches) == 1 {
			return ApplyEnvOverrides(providers[matches[0]])
		}
		if len(matches) > 1 {
			return axon.Provider{}, fmt.Errorf("LLM_PROVIDER=%q is ambiguous; use one of: %s", sel, strings.Join(matches, ", "))
		}
		if p, ok, err := providerFromEnv(); err != nil {
			return axon.Provider{}, err
		} else if ok && strings.EqualFold(p.Name, sel) {
			return p, nil
		}
		return axon.Provider{}, fmt.Errorf("provider %q not found in %s", sel, ProvidersPath())
	}
	if len(providers) == 1 {
		for _, p := range providers {
			return ApplyEnvOverrides(p)
		}
	}
	if p, ok, err := providerFromEnv(); err != nil {
		return axon.Provider{}, err
	} else if ok {
		return p, nil
	}
	if len(providers) == 0 {
		return axon.Provider{}, fmt.Errorf("no provider configured; set LLM_MODEL and LLM_BASE_URL or create %s", ProvidersPath())
	}
	return axon.Provider{}, ErrAmbiguousProvider
}

func providerFromEnv() (axon.Provider, bool, error) {
	model := EnvString("LLM_MODEL")
	baseURL := EnvString("LLM_BASE_URL")
	apiKey := EnvString("LLM_API_KEY")
	extraText := EnvString("LLM_PROVIDER_EXTRA")
	if model == "" && baseURL == "" && apiKey == "" && extraText == "" {
		return axon.Provider{}, false, nil
	}
	if model == "" || baseURL == "" {
		return axon.Provider{}, false, fmt.Errorf("LLM_MODEL and LLM_BASE_URL are required when provider config is supplied via env")
	}
	name := EnvString("LLM_PROVIDER_NAME")
	if name == "" {
		name = EnvString("LLM_PROVIDER")
	}
	if name == "" {
		name = "env"
	}
	var extra json.RawMessage
	if extraText != "" {
		if !json.Valid([]byte(extraText)) {
			return axon.Provider{}, false, fmt.Errorf("LLM_PROVIDER_EXTRA must be valid JSON")
		}
		extra = json.RawMessage(extraText)
	}
	return axon.Provider{
		Name:    strings.ToLower(name),
		BaseURL: baseURL,
		Model:   model,
		APIKey:  apiKey,
		Extra:   extra,
	}, true, nil
}

func ApplyEnvOverrides(p axon.Provider) (axon.Provider, error) {
	if baseURL := EnvString("LLM_BASE_URL"); baseURL != "" {
		p.BaseURL = baseURL
	}
	if model := EnvString("LLM_MODEL"); model != "" {
		p.Model = model
	}
	if apiKey := EnvString("LLM_API_KEY"); apiKey != "" {
		p.APIKey = apiKey
	}
	if extraText := EnvString("LLM_PROVIDER_EXTRA"); extraText != "" {
		if !json.Valid([]byte(extraText)) {
			return axon.Provider{}, fmt.Errorf("LLM_PROVIDER_EXTRA must be valid JSON")
		}
		p.Extra = json.RawMessage(extraText)
	}
	return p, nil
}

func ProviderNames(providers map[string]axon.Provider) []string {
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func providersByName(providers map[string]axon.Provider, sel string) []string {
	var out []string
	for key, p := range providers {
		if strings.EqualFold(p.Name, sel) {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

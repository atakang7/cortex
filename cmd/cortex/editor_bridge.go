package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/atakang7/axon/v2"
	"github.com/atakang7/cortex/v2/internal/config"
	"github.com/atakang7/cortex/v2/internal/ui"
)

// editorBridge implements the NDJSON contract used by extension-v2.js.
// Agent/session mutations are serialized; cancel remains available during Step.
type editorBridge struct {
	mu       sync.Mutex
	out      sync.Mutex
	wg       sync.WaitGroup
	agent    *axon.Agent
	cfg      config.Config
	settings axon.Settings
	trace    *ui.TraceLog
	busy     bool
	cancel   context.CancelFunc
	// prefs are the editor's model choices; route is the provider tag the
	// current model is routed to ("" lets OpenRouter route).
	prefs modelPrefs
	route string
	// reply is a command's payload, sent with its response.
	reply any
}
type editorRequest struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Text     string `json:"text"`
	Path     string `json:"path"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Mode     string `json:"mode"`
	Query    string `json:"query"`
	On       bool   `json:"on"`
	Route    string `json:"route"`
	Filter   string `json:"filter"`
}

func (b *editorBridge) write(v any) {
	b.out.Lock()
	defer b.out.Unlock()
	_ = json.NewEncoder(os.Stdout).Encode(v)
}
func (b *editorBridge) emit(ctx context.Context, e axon.Event) {
	ev := map[string]any{"kind": e.Kind.String(), "turn": e.Turn, "time": e.Time, "text": e.Text, "tool": e.Tool, "prune": e.Prune, "usage": e.Usage, "session": e.Session}
	if e.Err != nil {
		ev["error"] = e.Err.Error()
	}
	b.write(map[string]any{"type": "event", "event": ev})
}

// Called with mu held and only while Step is idle.
func (b *editorBridge) state() map[string]any {
	s := b.agent.Session()
	session := map[string]any{"id": s.ID, "path": s.Path(), "cwd": s.Cwd, "title": s.Title, "turn": s.Turn, "messages": len(s.Messages), "edits": s.Edits, "task": s.CurrentTask}
	providers := map[string][]string{}
	for name, p := range b.settings.Providers {
		for model := range p.Models {
			providers[name] = append(providers[name], model)
		}
	}
	sessions := []map[string]any{}
	for _, s := range axon.ListSessions(b.agent.SessionsDir(), b.agent.SessionPath()) {
		sessions = append(sessions, map[string]any{"id": s.ID, "path": s.Path, "cwd": s.Cwd, "title": s.Title, "turn": s.Turn, "started_at": s.StartedAt, "current": s.Current})
	}
	// An empty pruner_model means the main model decides checkpoints; an
	// empty explorer_model means no explore tool.
	return map[string]any{"provider": b.cfg.Provider, "model": b.cfg.ModelName, "pruner_provider": b.cfg.Pruner.Provider, "pruner_model": b.cfg.Pruner.Model, "explorer_provider": b.cfg.Explorer.Provider, "explorer_model": b.cfg.Explorer.Model,
		"prune_mode": b.settings.Pruner.Mode, "checkpoint_tokens": b.settings.Pruner.Trigger(), "checkpoints": len(s.Checkpoints),
		"providers": providers, "session": session, "session_items": sessions, "busy": b.busy, "usage": map[string]any{"prompt": s.PromptTokens, "completion": s.CompletionTokens, "cost": s.Cost, "cached": s.CachedTokens, "requests": s.Requests, "explorer": s.ExplorerTokens}, "goal": s.ActiveGoalView(),
		"favorites": b.prefs.Favorites, "route": b.route, "route_auto": b.prefs.Route == "" || b.prefs.Model != b.cfg.ModelName, "openrouter": isOpenRouterEndpoint(b.settings, b.cfg.Provider)}
}
func (b *editorBridge) respond(r editorRequest, err error) {
	v := map[string]any{"type": "response", "id": r.ID, "ok": err == nil}
	if err != nil {
		v["error"] = err.Error()
	}
	if b.reply != nil {
		v["data"] = b.reply
		b.reply = nil
	}
	if !b.busy {
		v["state"] = b.state()
	}
	b.write(v)
}
func runEditorBridge(cfg config.Config, settings axon.Settings, model axon.Model, side sideModels, trace *ui.TraceLog) error {
	b := &editorBridge{cfg: cfg, settings: settings, trace: trace, prefs: loadModelPrefs()}
	if e, ok := settings.Providers[cfg.Provider]; ok {
		b.route = e.Models[cfg.ModelName].Route // applied at startup by run
	}
	var err error
	b.agent, err = newAgent(cfg, settings, model, side, fanOut(b.emit, trace.Emit), maxIterationsOnce)
	if err != nil {
		return err
	}
	defer b.agent.Close()
	b.write(map[string]any{"type": "ready", "state": b.state()})
	for _, w := range cfg.Warnings {
		b.write(map[string]any{"type": "notice", "text": w})
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 65536), 16*1024*1024)
	for scanner.Scan() {
		var r editorRequest
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			b.write(map[string]any{"type": "notice", "text": "Invalid JSON request"})
			continue
		}
		b.mu.Lock()
		if r.Type == "cancel" {
			if b.cancel != nil {
				b.cancel()
			}
			b.respond(r, nil)
			b.mu.Unlock()
			continue
		}
		if b.busy {
			b.respond(r, errors.New("Stop the active turn before changing the session"))
			b.mu.Unlock()
			continue
		}
		if r.Type == "step" {
			if r.Text == "" {
				b.respond(r, errors.New("A prompt is required"))
				b.mu.Unlock()
				continue
			}
			ctx, cancel := context.WithCancel(context.Background())
			b.cancel = cancel
			b.busy = true
			b.respond(r, nil)
			b.wg.Add(1)
			go func() {
				defer b.wg.Done()
				defer cancel()
				_, err := b.agent.Step(ctx, r.Text)
				b.mu.Lock()
				defer b.mu.Unlock()
				b.busy = false
				b.cancel = nil
				interrupted := errors.Is(err, axon.ErrInterrupted) || errors.Is(err, context.Canceled)
				result := map[string]any{"type": "turn_complete", "ok": err == nil, "interrupted": interrupted, "state": b.state()}
				if err != nil && !interrupted {
					result["error"] = err.Error()
				}
				b.write(result)
			}()
			b.mu.Unlock()
			continue
		}
		err := b.command(r)
		b.respond(r, err)
		b.mu.Unlock()
	}
	b.mu.Lock()
	if b.cancel != nil {
		b.cancel()
	}
	b.mu.Unlock()
	b.wg.Wait()
	return scanner.Err()
}
func (b *editorBridge) command(r editorRequest) error {
	switch r.Type {
	case "context":
		// The next request as the model will see it, block by block. Commands
		// only run between turns, so the session is not being written to.
		b.reply = b.agent.ContextSnapshot()
		return nil
	case "get_state":
		return nil
	case "reset":
		b.agent.Reset()
		return b.agent.Session().Save()
	case "switch_session":
		if err := b.agent.SwitchSession(r.Path); err != nil {
			return err
		}
		return b.agent.Session().Save()
	case "change_cwd":
		_, err := b.agent.Cd(r.Path)
		return err
	case "undo":
		if _, ok := b.agent.Undo(); !ok {
			return errors.New("No edit could be restored")
		}
		return nil
	case "set_prune_mode":
		mode := axon.PruneMode(r.Mode)
		if !mode.Valid() {
			return errors.New("Invalid prune mode")
		}
		b.agent.SetPruneMode(mode)
		b.settings.Pruner.Mode = mode
		return nil
	case "models":
		models, err := loadCatalog(context.Background())
		if err != nil {
			return err
		}
		b.reply = map[string]any{"models": searchCatalog(models, r.Query, b.prefs.Favorites, 1000, r.Filter == "free"), "favorites": favoriteModels(models, b.prefs.Favorites), "free": freeModels(models)}
		return nil
	case "favorite":
		b.prefs.setFavorite(r.Model, r.On)
		return saveModelPrefs(b.prefs)
	case "endpoints":
		model := r.Model
		if model == "" {
			model = b.cfg.ModelName
		}
		eps, err := fetchEndpoints(context.Background(), model)
		if err != nil {
			return err
		}
		u := profileOf(b.agent.Session())
		auto := ""
		if i := rankEndpoints(eps, u); i >= 0 {
			auto = eps[i].Tag
		}
		b.reply = map[string]any{"model": model, "endpoints": eps, "auto": auto, "route": b.route,
			"profile": map[string]float64{"input": u.Input, "output": u.Output, "hit_rate": u.HitRate}}
		return nil
	case "select_route":
		b.prefs.Provider, b.prefs.Model, b.prefs.Route = b.cfg.Provider, b.cfg.ModelName, r.Route
		if err := saveModelPrefs(b.prefs); err != nil {
			return err
		}
		return b.useModel(b.cfg.Provider, b.cfg.ModelName)
	case "select_model":
		// A newly picked model starts on automatic provider choice.
		b.prefs.Provider, b.prefs.Model, b.prefs.Route = r.Provider, r.Model, ""
		if err := saveModelPrefs(b.prefs); err != nil {
			return err
		}
		return b.useModel(r.Provider, r.Model)
	case "select_pruner":
		// An empty model hands checkpoints back to the main model.
		model, err := b.sideModel("pruner", r)
		if err != nil {
			return err
		}
		if err := config.SavePruner(r.Provider, r.Model); err != nil {
			return err
		}
		b.agent.SetPrunerModel(model)
		b.cfg.Pruner = config.PrunerRef{Provider: r.Provider, Model: r.Model}
		return nil
	case "select_explorer":
		// An empty model turns the explore tool off.
		model, err := b.sideModel("explorer", r)
		if err != nil {
			return err
		}
		if err := config.SaveExplorer(r.Provider, r.Model); err != nil {
			return err
		}
		b.agent.SetExplorerModel(model)
		b.cfg.Explorer = config.ExplorerRef{Provider: r.Provider, Model: r.Model}
		return nil
	default:
		return fmt.Errorf("Unknown editor command %q", r.Type)
	}
}

// sideModel builds the client for a pruner or explorer choice; an empty model
// is a nil client, the role's default. A model without a provider takes the
// main model's.
func (b *editorBridge) sideModel(role string, r editorRequest) (axon.Model, error) {
	if r.Model == "" {
		return nil, nil
	}
	client, err := b.settings.NewClient(orProvider(r.Provider, b.cfg.Provider), r.Model)
	if err != nil {
		return nil, err
	}
	return b.trace.Wiretap(role, client), nil
}

// useModel switches the agent to provider/model, routed to the user's chosen
// provider or the cheapest one for this session's usage.
func (b *editorBridge) useModel(provider, model string) error {
	route, _ := resolveRoute(context.Background(), b.settings, provider, model, b.prefs, profileOf(b.agent.Session()))
	settings := b.settings.WithRoute(provider, model, route)
	client, err := settings.NewFreeFirstClient(provider, model)
	if err != nil {
		return err
	}
	b.agent.SetModel(b.trace.Wiretap("main", client))
	b.settings, b.route = settings, route
	b.cfg.Provider, b.cfg.ModelName = provider, model
	return nil
}

// favoriteModels returns the catalog entries of the favourites, in their order.
func favoriteModels(models []catalogModel, favorites []string) []catalogModel {
	byID := map[string]catalogModel{}
	for _, m := range models {
		byID[m.ID] = m
	}
	var out []catalogModel
	for _, id := range favorites {
		if m, ok := byID[id]; ok {
			out = append(out, m)
		} else {
			out = append(out, catalogModel{ID: id, Name: id})
		}
	}
	return out
}

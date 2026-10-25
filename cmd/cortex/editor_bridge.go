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
}
type editorRequest struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Text     string `json:"text"`
	Path     string `json:"path"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Mode     string `json:"mode"`
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
	return map[string]any{"provider": b.cfg.Provider, "model": b.cfg.ModelName, "pruner_provider": b.cfg.Pruner.Provider, "pruner_model": b.cfg.Pruner.Model, "pruner_models": b.cfg.Pruner.Models, "prune_mode": b.settings.Pruner.Mode, "providers": providers, "session": session, "session_items": sessions, "busy": b.busy, "usage": map[string]any{"prompt": s.PromptTokens, "completion": s.CompletionTokens, "cost": s.Cost}, "goal": s.ActiveGoalView()}
}
func (b *editorBridge) respond(r editorRequest, err error) {
	v := map[string]any{"type": "response", "id": r.ID, "ok": err == nil}
	if err != nil {
		v["error"] = err.Error()
	}
	if !b.busy {
		v["state"] = b.state()
	}
	b.write(v)
}
func runEditorBridge(cfg config.Config, settings axon.Settings, model axon.Model, pruner axon.Model, trace *ui.TraceLog) error {
	b := &editorBridge{cfg: cfg, settings: settings, trace: trace}
	var err error
	b.agent, err = newAgent(cfg, settings, model, pruner, fanOut(b.emit, trace.Emit), maxIterationsOnce)
	if err != nil {
		return err
	}
	defer b.agent.Close()
	b.write(map[string]any{"type": "ready", "state": b.state()})
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
	case "select_model", "select_pruner":
		var model axon.Model
		var err error
		if r.Type == "select_model" {
			model, err = b.settings.NewFreeFirstClient(r.Provider, r.Model)
		} else {
			model, err = b.settings.NewPrunerClient(r.Provider, r.Model)
		}
		if err != nil {
			return err
		}
		if r.Type == "select_model" {
			if err := config.SaveModel(r.Provider, r.Model); err != nil {
				return err
			}
			b.agent.SetModel(b.trace.Wiretap("main", model))
			b.cfg.Provider = r.Provider
			b.cfg.ModelName = r.Model
		} else {
			if err := config.SavePruner(r.Provider, r.Model); err != nil {
				return err
			}
			b.agent.SetPrunerModel(b.trace.Wiretap("pruner", model))
			b.cfg.Pruner = config.PrunerRef{Provider: r.Provider, Model: r.Model}
		}
		return nil
	default:
		return fmt.Errorf("Unknown editor command %q", r.Type)
	}
}

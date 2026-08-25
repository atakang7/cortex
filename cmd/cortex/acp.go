package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/atakang7/axon/v2"

	"github.com/atakang7/cortex/v2/internal/config"
	"github.com/atakang7/cortex/v2/internal/ui"
)

// Cortex speaks the stable ACP v1 subset Setpoint and editors need directly
// over stdio. The protocol is deliberately a thin transport around axon: axon
// still owns the session, tool loop, persistence and model streaming.
//
// Keeping this adapter here avoids making ACP part of axon's core contract
// before the integration has proven itself. Once it has, this file can move
// behind an axon transport package without changing the wire protocol.

type acpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type acpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *acpRPCError    `json:"error,omitempty"`
}

type acpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type acpSession struct {
	agent *axon.Agent

	// turnMu serializes prompts so one Axon session can never execute two
	// turns concurrently. stateMu is deliberately separate: cancellation
	// must be able to reach a turn while turnMu is held by Agent.Step.
	turnMu  sync.Mutex
	stateMu sync.Mutex
	cancel  context.CancelFunc

	// Some embedders stream tokens and some only emit the final assistant
	// event. Track this per turn so ACP always gets one copy of the answer,
	// never an empty response and never streamed text plus a duplicate final.
	streamed atomic.Bool
}

type acpServer struct {
	cfg         config.Config
	settings    axon.Settings
	model       axon.Model
	prunerModel axon.Model
	trace       *ui.TraceLog

	writeMu   sync.Mutex
	sessionMu sync.Mutex
	sessions  map[string]*acpSession
	nextID    atomic.Uint64
}

func runACP(cfg config.Config, settings axon.Settings, model axon.Model, prunerModel axon.Model, trace *ui.TraceLog) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	s := &acpServer{
		cfg:         cfg,
		settings:    settings,
		model:       model,
		prunerModel: prunerModel,
		trace:       trace,
		sessions:    make(map[string]*acpSession),
	}
	defer s.close()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	var prompts sync.WaitGroup
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			prompts.Wait()
			return nil
		default:
		}

		var req acpRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			_ = s.write(acpResponse{JSONRPC: "2.0", Error: &acpRPCError{Code: -32700, Message: "parse error"}})
			continue
		}

		// A prompt is long-running. Handle it concurrently so session/cancel can
		// still be read from stdin while axon is inside a model or tool call.
		if req.Method == "session/prompt" && len(req.ID) > 0 {
			prompts.Add(1)
			go func(req acpRequest) {
				defer prompts.Done()
				s.handleAndRespond(ctx, req)
			}(req)
			continue
		}

		if len(req.ID) == 0 {
			s.handleNotification(req)
			continue
		}
		s.handleAndRespond(ctx, req)
	}

	prompts.Wait()
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("acp stdin: %w", err)
	}
	return nil
}

func (s *acpServer) handleAndRespond(ctx context.Context, req acpRequest) {
	result, rpcErr := s.handle(ctx, req)
	resp := acpResponse{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rpcErr}
	if rpcErr != nil {
		resp.Result = nil
	}
	_ = s.write(resp)
}

func (s *acpServer) handle(ctx context.Context, req acpRequest) (any, *acpRPCError) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": 1,
			"agentCapabilities": map[string]any{
				"loadSession": false,
			},
			"agentInfo": map[string]any{"name": "cortex", "version": version},
		}, nil

	case "authenticate":
		return map[string]any{}, nil

	case "session/new":
		var params struct {
			Cwd string `json:"cwd"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, invalidParams(err)
		}

		sid := fmt.Sprintf("cortex-%d", s.nextID.Add(1))
		sess := &acpSession{}
		emit := fanOut(s.eventSink(sid, sess), s.trace.Emit)
		agent, err := newAgent(s.cfg, s.settings, s.model, s.prunerModel, emit, maxIterationsOnce)
		if err != nil {
			return nil, internalError(err)
		}
		sess.agent = agent

		s.sessionMu.Lock()
		s.sessions[sid] = sess
		s.sessionMu.Unlock()

		return map[string]any{"sessionId": sid}, nil

	case "session/prompt":
		return s.prompt(ctx, req.Params)

	case "session/close":
		var params struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, invalidParams(err)
		}
		s.closeSession(params.SessionID)
		return map[string]any{}, nil

	case "session/set_mode":
		return map[string]any{}, nil

	case "session/load", "session/resume", "session/list", "session/set_config_option", "logout":
		return nil, &acpRPCError{Code: -32601, Message: "method not supported"}

	default:
		return nil, &acpRPCError{Code: -32601, Message: "method not found"}
	}
}

func (s *acpServer) handleNotification(req acpRequest) {
	if req.Method != "session/cancel" {
		return
	}
	var params struct {
		SessionID string `json:"sessionId"`
	}
	if json.Unmarshal(req.Params, &params) != nil {
		return
	}

	s.sessionMu.Lock()
	sess := s.sessions[params.SessionID]
	s.sessionMu.Unlock()
	if sess == nil {
		return
	}

	// Do not acquire turnMu here: the whole point of cancellation is to reach
	// Agent.Step while the active prompt owns that lock.
	sess.stateMu.Lock()
	cancel := sess.cancel
	sess.stateMu.Unlock()
	if cancel != nil {
		cancel()
	}
	sess.agent.Interrupt()
}

func (s *acpServer) prompt(parent context.Context, raw json.RawMessage) (any, *acpRPCError) {
	var params struct {
		SessionID string `json:"sessionId"`
		Prompt    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"prompt"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, invalidParams(err)
	}

	s.sessionMu.Lock()
	sess := s.sessions[params.SessionID]
	s.sessionMu.Unlock()
	if sess == nil {
		return nil, &acpRPCError{Code: -32602, Message: "unknown session"}
	}

	var parts []string
	for _, block := range params.Prompt {
		if block.Type == "text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	text := strings.TrimSpace(strings.Join(parts, "\n"))
	if text == "" {
		return nil, &acpRPCError{Code: -32602, Message: "prompt contains no text"}
	}

	// One turn at a time per ACP session. This is also what preserves the
	// coding worker's Axon session across Setpoint CONTINUE iterations.
	sess.turnMu.Lock()
	defer sess.turnMu.Unlock()

	turnCtx, cancel := context.WithCancel(parent)
	sess.streamed.Store(false)
	sess.stateMu.Lock()
	sess.cancel = cancel
	sess.stateMu.Unlock()
	defer func() {
		cancel()
		sess.stateMu.Lock()
		sess.cancel = nil
		sess.stateMu.Unlock()
	}()

	_, err := sess.agent.Step(turnCtx, text)
	if err != nil {
		if errors.Is(err, axon.ErrInterrupted) || errors.Is(err, context.Canceled) {
			return map[string]any{"stopReason": "cancelled"}, nil
		}
		return nil, internalError(err)
	}
	return map[string]any{"stopReason": "end_turn"}, nil
}

func (s *acpServer) eventSink(sessionID string, sess *acpSession) func(context.Context, axon.Event) {
	return func(ctx context.Context, e axon.Event) {
		var update map[string]any
		switch e.Kind {
		case axon.KindToken:
			if e.Text == "" {
				return
			}
			sess.streamed.Store(true)
			update = map[string]any{
				"sessionUpdate": "agent_message_chunk",
				"content":       map[string]any{"type": "text", "text": e.Text},
			}
		case axon.KindAssistantEnd:
			// Most Cortex providers stream KindToken. If a custom Axon model only
			// returns a final message, still give ACP clients the answer exactly once.
			if e.Text == "" || sess.streamed.Load() {
				return
			}
			update = map[string]any{
				"sessionUpdate": "agent_message_chunk",
				"content":       map[string]any{"type": "text", "text": e.Text},
			}
		case axon.KindReasoning:
			if e.Text == "" {
				return
			}
			update = map[string]any{
				"sessionUpdate": "agent_thought_chunk",
				"content":       map[string]any{"type": "text", "text": e.Text},
			}
		case axon.KindToolCall:
			if e.Tool == nil {
				return
			}
			var rawInput any
			if len(e.Tool.Args) > 0 {
				if json.Unmarshal(e.Tool.Args, &rawInput) != nil {
					rawInput = string(e.Tool.Args)
				}
			}
			update = map[string]any{
				"sessionUpdate": "tool_call",
				"toolCallId":    e.Tool.ID,
				"title":         e.Tool.Name,
				"status":        "pending",
				"rawInput":      rawInput,
			}
		case axon.KindToolResult:
			if e.Tool == nil {
				return
			}
			update = map[string]any{
				"sessionUpdate": "tool_call_update",
				"toolCallId":    e.Tool.ID,
				"status":        "completed",
				"rawOutput":     e.Tool.Result,
			}
		case axon.KindToolError:
			if e.Tool == nil {
				return
			}
			msg := "tool failed"
			if e.Err != nil {
				msg = e.Err.Error()
			}
			update = map[string]any{
				"sessionUpdate": "tool_call_update",
				"toolCallId":    e.Tool.ID,
				"status":        "failed",
				"rawOutput":     msg,
			}
		default:
			return
		}

		_ = s.write(map[string]any{
			"jsonrpc": "2.0",
			"method":  "session/update",
			"params": map[string]any{
				"sessionId": sessionID,
				"update":    update,
			},
		})
	}
}

func (s *acpServer) write(v any) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return json.NewEncoder(os.Stdout).Encode(v)
}

func (s *acpServer) closeSession(id string) {
	s.sessionMu.Lock()
	sess := s.sessions[id]
	delete(s.sessions, id)
	s.sessionMu.Unlock()
	if sess != nil {
		sess.stateMu.Lock()
		cancel := sess.cancel
		sess.stateMu.Unlock()
		if cancel != nil {
			cancel()
		}
		sess.agent.Interrupt()
		sess.agent.Close()
	}
}

func (s *acpServer) close() {
	s.sessionMu.Lock()
	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	s.sessionMu.Unlock()
	for _, id := range ids {
		s.closeSession(id)
	}
}

func invalidParams(err error) *acpRPCError {
	return &acpRPCError{Code: -32602, Message: err.Error()}
}

func internalError(err error) *acpRPCError {
	return &acpRPCError{Code: -32603, Message: err.Error()}
}

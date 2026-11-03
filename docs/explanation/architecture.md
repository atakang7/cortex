# Architecture

`cmd/cortex` chooses a model, loads the Cortex config and Axon settings, and constructs one Axon agent.

Axon runs model turns, dispatches tools, and persists sessions. Cortex keeps transport and presentation in separate packages, so the interactive terminal, one-shot mode, editor bridge, and ACP handler share the same agent constructor.

`internal/ui` consumes runtime events. A separate trace sink records events and wraps model requests and replies in JSONL, which makes model inputs inspectable after a run.

A failed trace-log open prints a warning and leaves the agent usable. Trace writes are best-effort and cannot stop a turn.

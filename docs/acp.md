# ACP mode

Cortex can run as a stable ACP v1 agent over stdio:

```bash
cortex --acp
```

ACP is a transport around Cortex's existing Axon runtime. Axon still owns the model/tool loop, session history, context pruning, and persistence; ACP only exposes that runtime to an external controller.

Each ACP `session/new` creates one Axon agent/session. Repeated `session/prompt` calls with that same ACP session ID reuse the same Axon session, so implementation context persists across controller-driven iterations.

Cortex currently implements the ACP subset needed by Setpoint and editor clients:

- `initialize`
- `authenticate`
- `session/new`
- `session/prompt`
- `session/cancel`
- `session/close`
- `session/set_mode`
- streamed agent message/reasoning/tool updates

Durable ACP `session/load` / `session/resume` are not advertised yet. Axon's own session persistence remains unchanged.

## Setpoint

Use Cortex as Setpoint's coding agent with:

```yaml
agent:
  protocol: acp
  command: cortex
  args: ["--acp"]
  permissions: auto-allow

models:
  provider: agent
```

Setpoint then keeps the Cortex/Axon session alive across `CONTINUE` iterations while creating fresh agent sessions for the North Star definer, progress judges, and final jurors according to its role configuration.

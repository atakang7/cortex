# Use Cortex with Setpoint

Build or install `cortex` on the path used by Setpoint.

```yaml
agent:
  protocol: acp
  command: cortex
  args: ["--acp"]
  permissions: auto-allow

models:
  provider: agent
```

Setpoint starts Cortex over stdio; each ACP `session/new` creates an Axon session.

See [supported ACP methods](../reference/acp.md).

# ACP interface

`cortex --acp` serves the ACP v1 subset over standard input and output.

| method | behavior |
| --- | --- |
| `initialize` | Negotiates client capabilities |
| `authenticate` | Accepts the ACP authentication handshake |
| `session/new` | Creates an Axon agent and session |
| `session/prompt` | Runs a turn in the named session |
| `session/cancel` | Cancels an active turn |
| `session/close` | Closes an ACP session |
| `session/set_mode` | Selects the session mode |

Cortex emits `session/update` notifications for agent messages, reasoning, and tool activity.

`session/load`, `session/resume`, `session/list`, `session/set_config_option`, and `logout` return method-not-supported errors.

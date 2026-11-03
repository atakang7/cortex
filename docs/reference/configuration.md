# Configuration reference

Cortex resolves model selection from these sources in order.

| priority | path or input | behavior |
| --- | --- | --- |
| 1 | `$XDG_CONFIG_HOME/cortex/config.yaml` | User defaults; `~/.config` on Unix if unset |
| 2 | `./cortex.yaml` | Overrides user YAML fields |
| 3 | `LLM_PROVIDER`, `LLM_MODEL` | Overrides both files |

`--config <path>` reads the named file instead of the first two sources; environment overrides still apply.

## YAML fields

| field | default | meaning |
| --- | --- | --- |
| `name` | `cortex` | Display name |
| `provider` | required | Axon provider identifier |
| `model` | required | Model name within the provider |
| `system_prompt` | built-in prompt | Inline text or a path to `.md` or `.txt` |
| `pruner.provider` | main provider | Provider for the checkpoint model |
| `pruner.model` | main model | Optional checkpoint model |
| `explorer.provider` | main provider | Provider for the read-only explorer |
| `explorer.model` | unset | Optional explorer |
| `explorer.models` | empty | Fallback explorer model names |
| `mcp_servers.<name>.command` | unset | Executable for an MCP server |
| `mcp_servers.<name>.args` | empty | Process arguments |
| `mcp_servers.<name>.env` | empty | Environment strings for the subprocess |

Provider endpoints, retry policy, and credentials are resolved by [Axon](https://github.com/atakang7/axon).

`pruner.models` is retired; loading it emits a warning.

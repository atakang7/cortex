Part of [cortex](../../README.md).

# internal/config

Resolves Cortex's model selection, system prompt, and MCP subprocess definitions.

## Run

Called by `cmd/cortex` during startup; this package has no executable.

## Test

```sh
go test ./internal/config
```

## Config

| var | default | what |
| --- | --- | --- |
| `LLM_PROVIDER` | unset | Overrides `provider` in YAML |
| `LLM_MODEL` | unset | Overrides `model` in YAML |
| `XDG_CONFIG_HOME` | `~/.config` on Unix | Base directory for the user config |

The YAML fields and lookup order are in [configuration reference](../../docs/reference/configuration.md).

## Interfaces

| function | input or output |
| --- | --- |
| `Load(path)` | Resolved `Config`; explicit path skips the file cascade |
| `SaveModel` | Writes model choice to user `config.yaml` |
| `SavePruner` | Writes checkpoint model choice |
| `SaveExplorer` | Writes read-only explorer model choice |
| `UserDir` | User configuration directory |

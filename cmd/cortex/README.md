Part of [cortex](../../README.md).

# cmd/cortex

The `cortex` executable selects the model and runs the terminal, one-shot, ACP, or editor transport.

## Run

```sh
go run ./cmd/cortex --version
go run ./cmd/cortex --prompt "list TODO comments"
```

## Test

```sh
go test ./cmd/cortex
```

## Config

| var | default | what |
| --- | --- | --- |
| `LLM_PROVIDER` | config file | Overrides the selected provider |
| `LLM_MODEL` | config file | Overrides the selected model |
| `LLM_API_KEY` | unset | Credential fallback handled by Axon |

[Configuration](../../internal/config/README.md) owns file precedence.

## Interfaces

| flag | behavior |
| --- | --- |
| `--config <path>` | Read the named config file |
| `--prompt <text>` | Run one turn; answer on stdout, activity on stderr |
| `--acp` | Serve ACP v1 over stdio |
| `--stdio` | Serve the editor JSON bridge over stdio |
| `--version` | Print version, commit, and build date |

The ACP methods are in [the protocol reference](../../docs/reference/acp.md).

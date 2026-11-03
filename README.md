# cortex

Cortex is a Go terminal coding agent for developers who need inspectable model requests and sessions, built on [Axon](https://github.com/atakang7/axon).

```text
+-----------------+  config  +------------+
| internal/config | -------> | cmd/cortex |
+-----------------+          +------------+
                                   |
                                   | model and tools
                                   v
                            +--------------+
                            | axon runtime |
                            +--------------+
                                   |
                                   | events
                                   v
                            +-------------+
                            | internal/ui |
                            +-------------+
```

## Quick start

With Go 1.26.2 and an [Axon provider configured](https://github.com/atakang7/axon#configuration), run:

```sh
go build -o cortex ./cmd/cortex
export LLM_PROVIDER=openrouter
export LLM_MODEL=deepseek/deepseek-v3.2
./cortex
```

## Layout

| path | what it is |
| --- | --- |
| [cmd/cortex](cmd/cortex/README.md) | CLI, ACP and editor transports |
| [internal/config](internal/config/README.md) | YAML, environment and model selection |
| [internal/ui](internal/ui/README.md) | Terminal and trace renderers |
| [scripts](scripts/README.md) | Installer and developer scripts |
| [scripts/eval](scripts/eval/README.md) | Fixture runner and grading |
| [docs](docs/README.md) | Documentation index and Astro site |

## Docs

[How-to](docs/how-to/run-evaluation.md) | [Reference](docs/reference/configuration.md) | [Explanation](docs/explanation/architecture.md) | [ACP](docs/reference/acp.md)

CI and releases on `main`:

```text
push -> verify -> semantic-release -> GoReleaser
```

## Limits

- An Axon provider and credentials must be configured before a model run.
- `--prompt` has a 120-call limit.
- ACP does not support `session/load` or `session/resume`.
- Trace-log creation errors are written to stderr; agent execution continues.

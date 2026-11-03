Part of [cortex](../../README.md).

# internal/ui

Renders Axon events as a Bubble Tea terminal session or a plain text stream.

## Run

```sh
go run ./cmd/cortex
```

## Test

```sh
go test ./internal/ui
```

## Config

The UI has no separate config file. The trace path uses the platform's user cache directory.

## Interfaces

| interface | behavior |
| --- | --- |
| `Run` | Interactive terminal session |
| `Bridge.Emit` | Delivers runtime events to the terminal |
| `Plain.Emit` | Sends answer text to stdout and activity to stderr |
| `TraceLog` | Writes JSONL events and model requests to the user cache |

The terminal uses stdin and stdout. No network port is opened by this package.

# cortex

**A terminal coding agent built on the [axon](https://github.com/atakang7/axon) runtime.**

axon ships the loop — streaming chat, tool dispatch, an append-only session, secondary-LLM context pruning, and seven built-in tools. cortex is the product on top: resolving which model to talk to, the role text that makes it a coding agent, and the terminal it talks through.

```
github.com/atakang7/axon    ← the runtime (the loop, the tools, the session)
github.com/atakang7/cortex  ← the coding agent (config, prompt, terminal)
```

## Install

```sh
go install github.com/atakang7/cortex/cmd/cortex@latest
```

Pre-built binaries for linux/darwin × amd64/arm64 are attached to each [release](https://github.com/atakang7/cortex/releases).

## Run

```sh
cortex                                   # interactive
cortex --prompt "summarize the TODOs"    # one turn, then exit
cortex --config reviewer.yaml            # a specific config, no cascade
cortex --version
```

`--prompt` writes the assistant's answer to **stdout** and everything else — tool activity, notices, errors — to **stderr**, so it composes:

```sh
cortex --prompt "list every exported function in internal/ui" > api.txt
```

## Configuration

cortex needs one thing before it can start: a model. Three sources are layered, each overriding the last.

1. `~/.config/cortex/config.yaml` — your defaults (honours `XDG_CONFIG_HOME`)
2. `./cortex.yaml` — this project's overrides
3. the environment — `LLM_PROVIDER`, `LLM_MODEL`, `LLM_BASE_URL`, `LLM_API_KEY`

Layering is field by field, so a project config that names only a model keeps the API key from your user config.

The fastest possible start:

```sh
export LLM_MODEL=gpt-4o
export LLM_API_KEY=sk-...
cortex
```

The full file:

```yaml
name: reviewer                  # shown in the banner

model:
  provider: openai              # supplies a default base_url for known providers
  name: gpt-4o
  base_url: https://api.openai.com
  api_key: $OPENAI_API_KEY      # a leading $ reads the environment variable

system_prompt: ./reviewer.md    # a path is read from disk; prose is used as written

mcp_servers:                    # spawned at startup, their tools auto-discovered
  github:
    command: docker
    args: ["run", "-i", "--rm", "ghcr.io/github/github-mcp-server"]
    env: ["GITHUB_TOKEN=..."]
```

`provider` supplies `base_url` for `openai`, `openrouter`, `anthropic`, `groq`, `deepseek` and `ollama`. Any other provider must state its own. A `localhost` endpoint needs no API key.

### Where the API key comes from

Checked in order; the first one that answers wins:

1. `api_key` in the config (a leading `$` reads that environment variable)
2. `LLM_API_KEY`
3. `~/.config/agent/providers.json`, matched on the `provider` name — the shared file the axon-family tools already use (override with `AXON_PROVIDERS_PATH`)

The third exists so a key has **one home on disk**. If you already keep credentials in `providers.json`, omit `api_key` from your cortex config entirely and cortex will find it:

```json
{ "providers": [
    { "name": "openrouter", "base_url": "https://openrouter.ai/api", "api_key": "sk-or-..." }
] }
```

cortex reads only `name` and `api_key` from that file; the model lists and routing options in it belong to other tools and are ignored.

Omit `system_prompt` and cortex uses its built-in coding-agent role text. Note that the **tool catalog is appended by axon** — a custom prompt must not enumerate tools, or you pay for those tokens twice on every call.

## The terminal

The transcript is committed to your terminal's own scrollback rather than held in an alternate screen, so a finished session stays scrollable, searchable and copyable — including after cortex exits. Only work still in flight redraws.

| key | |
| --- | --- |
| `enter` | send |
| `ctrl+j` / `alt+enter` | newline |
| `ctrl+c` / `esc` | interrupt the running turn |
| `ctrl+c` (idle) | clear the input, then exit |
| `ctrl+d` | exit |

### Slash commands

| | |
| --- | --- |
| `/new` | wipe the session and start over |
| `/undo` | revert the last file edit, byte for byte |
| `/cd <path>` | change the working directory |
| `/pwd` | show the working directory |
| `/session` | session file, turn count, pending undos |
| `/help` | list the commands |
| `/quit` | exit (`/exit`, `/q`) |

## Layout

Layers point one direction. `config` and the theme are leaves; `ui` owns the terminal; `main` only wires.

```
cmd/cortex/main.go        flags, wiring, mode dispatch — no logic
internal/config/          resolve the model, the prompt, the MCP servers
  config.go                 the cascade: files, then environment
  prompt.go                 the built-in coding-agent role text
internal/ui/
  run.go                    starts the Bubble Tea program
  model.go                  UI state and the rules for changing it
  view.go                   the live area: input box, status, spinner
  transcript.go             pure rendering — blocks of styled text
  event.go                  axon.Event → tea.Msg, the concurrency seam
  command.go                slash commands
  theme.go                  every colour and style, one owner
  plain.go                  the non-interactive renderer
```

Two rules worth knowing before changing anything:

- **`internal/config` imports nothing else from this module.** If a value there needs a type from a higher layer, it belongs in that layer.
- **`internal/ui/event.go` is the only file that touches both axon and Bubble Tea.** axon calls its event handler synchronously on the turn goroutine; Bubble Tea owns the terminal from its own. Nothing else may cross that line.

## Tests

```sh
go test ./...
```

`axon.Model` is a one-method interface, so the tests drive the real agent, the real tools and the real filesystem against a scripted model — no network, no API key.

## License

MIT. See `LICENSE`.

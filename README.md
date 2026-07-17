# cortex

**A terminal coding agent built on the [axon](https://github.com/atakang7/axon) runtime.**

axon ships the loop (streaming chat, tool dispatch, append-only session, secondary-LLM pruning). cortex is the product on top: a coding-agent system prompt, an interactive provider picker, a colored TTY renderer, YAML agent personalities, and slash commands.

```
github.com/atakang7/axon/agent  ← runtime (signals)
github.com/atakang7/cortex      ← coding agent (the layer axon feeds)
```

## Install

Pre-built binaries for linux/darwin × amd64/arm64 are attached to each [GitHub Release](https://github.com/atakang7/cortex/releases).

From source:

```sh
go install github.com/atakang7/cortex/cmd/cortex@latest
```

## Run

```sh
cortex                                  # interactive
cortex --prompt "summarize TODOs"       # single-shot, exits when the assistant stops
cortex --agent reviewer                 # load ~/.config/cortex/agents/reviewer.yaml
```

### Provider config

cortex reads `${XDG_CONFIG_HOME:-~/.config}/agent/providers.json` (override with `AXON_PROVIDERS_PATH`):

```json
{
  "providers": [
    {
      "name": "ollama",
      "base_url": "http://localhost:11434",
      "model": "llama3"
    },
    {
      "name": "openai",
      "base_url": "https://api.openai.com",
      "model": "gpt-4o",
      "api_key": "sk-..."
    },
    {
      "name": "claude",
      "base_url": "https://api.anthropic.com",
      "model": "claude-3-opus",
      "api_key": "sk-ant-..."
    }
  ]
}
```

Or pure env:

```sh
LLM_MODEL=gpt-4o LLM_BASE_URL=https://api.openai.com LLM_API_KEY=sk-... cortex
```

`LLM_PROVIDER` selects one when multiple are configured. With no env hint, cortex offers an interactive picker on first run and remembers your last choice.

### Slash commands

- `/new` — wipe session, restart
- `/undo` — revert the last file edit (byte-exact)
- `/cd <path>` — change cwd
- `/pwd` — show cwd
- `/session` — show session info

### YAML agent personalities

Place YAML configs under `${CORTEX_AGENTS_DIR:-~/.config/cortex/agents}/`:

```yaml
name: reviewer
system_prompt: ./reviewer.md
tools:
  - name: submit_review
    type: shell
    description: "Submit a GitHub PR review."
    schema:
      type: object
      properties:
        verdict: { type: string, enum: [approve, request_changes] }
        body: { type: string }
      required: [verdict, body]
    command: gh pr review --{{.verdict}} --body {{.body | shellQuote}}
    timeout_seconds: 10
```

Then `cortex --agent reviewer`.

## What's built in

cortex inherits axon's tool set: `read`, `write`, `exec`, `search`, `task`, `bash_output`, `kill_shell`. Every tool call requires a `reason` field. File writes are atomic, so `/undo` is byte-exact. Ctrl-C cancels the in-flight turn — both the model stream and any running subprocess.

There is no sandbox. Use Ctrl-C, `/undo`, and `git` as your guardrails.

## Why a separate repo

axon is the runtime — a library that drives the agent loop and can host any agent personality. cortex is one such personality: opinionated for coding work in a terminal. Keeping them separate means:

- axon stays minimal and importable for other agents (reviewer, researcher, orchestrators).
- cortex can ship binaries, slash commands, paste-aware input, and coding-specific defaults without bloating the library.

## License

MIT. See [LICENSE](LICENSE).

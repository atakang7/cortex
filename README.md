<div align="center">

# ◆ cortex

**A terminal coding agent built on the [axon](https://github.com/atakang7/axon) runtime.**

Streaming chat · tool dispatch · append-only sessions · secondary-LLM context pruning — all from your terminal.

[![Go Reference](https://pkg.go.dev/badge/github.com/atakang7/cortex.svg)](https://pkg.go.dev/github.com/atakang7/cortex)
[![CI](https://github.com/atakang7/cortex/actions/workflows/ci.yml/badge.svg)](https://github.com/atakang7/cortex/actions/workflows/ci.yml)
[![Docs](https://github.com/atakang7/cortex/actions/workflows/docs.yml/badge.svg)](https://atakang7.github.io/cortex/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[Documentation](https://atakang7.github.io/cortex/) · [Install](#install) · [Quick start](#quick-start) · [Architecture](#architecture) · [Playground](#playground) · [GitHub](https://github.com/atakang7/cortex)

</div>

---

> **The full documentation site is at [atakang7.github.io/cortex](https://atakang7.github.io/cortex/).**
> This README renders that same documentation in markdown for reading on GitHub, in your editor, or offline.

---

## Documentation map

```
cortex/
├── getting-started/
│   ├── install              — pre-built binaries · go install · build from source
│   └── quick-start          — env vars · interactive & non-interactive · first config
├── guides/
│   ├── configuration        — the three-layer cascade · fields · known providers
│   ├── interactive          — the transcript · Ctrl-C · the banner
│   ├── non-interactive      — stdout/stderr split · iteration limits · piping
│   ├── commands             — /new · /undo · /cd · /session · /help · /quit
│   └── keyboard             — every key the terminal binds
├── concepts/
│   ├── architecture         — axon owns the loop · cortex owns the product · two boundary rules
│   ├── pruner               — the secondary model · what it sees · when to skip it
│   ├── trace-log            — the third event sink · both models tapped · why it exists
│   └── mcp-servers          — MCP protocol · auto-discovery · multiple servers
├── playground/
│   ├── overview             — run.sh & trace.sh · the run directory · the six views
│   ├── running-tasks        — step by step · what a run keeps · comparing runs
│   ├── reading-traces       — timeline · prompt · system · tools · usage · errors
│   └── tasks                — ledger · report · pipeline · what the playground found
└── reference/
    ├── cli                  — --config · --prompt · --version · exit codes
    ├── config-file          — every field · pruner · mcp_servers · env overrides
    └── api-keys             — resolution order · providers.json · the leading $
```

---

## Install

Get the cortex binary on your machine in one of three ways.

### Pre-built binaries (recommended)

Every [release](https://github.com/atakang7/cortex/releases) ships binaries for `linux` and `darwin` on `amd64` and `arm64`. Download and extract in one line:

```bash
curl -L https://github.com/atakang7/cortex/releases/latest/download/cortex-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz \
  | tar xz -C /usr/local/bin cortex
```

### go install

If you have a Go toolchain:

```bash
go install github.com/atakang7/cortex/v2/cmd/cortex@latest
```

The binary lands in your `$GOPATH/bin` — make sure that's on your `$PATH`.

### Build from source

```bash
git clone https://github.com/atakang7/cortex.git
cd cortex
go build -o cortex ./cmd/cortex
```

### Verify

Check that it runs and prints build metadata:

```bash
cortex --version
```

```
cortex 1.2.3 (abc1234, built 2025-01-01T00:00:00Z)
```

> **ℹ️ What's next?** cortex needs a model before it can start. Continue to [Quick start](#quick-start) to configure one.

---

## Quick start

Configure a model and run cortex in under a minute.

### The fastest possible start

cortex needs one thing before it can start: a model. Two environment variables are enough:

```bash
export LLM_MODEL=gpt-4o
export LLM_API_KEY=sk-...
cortex
```

### Interactive mode

Run cortex with no arguments to start an interactive session in your current directory:

```bash
cortex                                   # interactive
cortex --prompt "summarize the TODOs"    # one turn, then exit
cortex --config reviewer.yaml            # a specific config, no cascade
cortex --version
```

### Non-interactive mode

`--prompt` writes the assistant's answer to **stdout** and everything else — tool activity, notices, errors — to **stderr**, so it composes:

```bash
cortex --prompt "list every exported function in internal/ui" > api.txt
```

See [Non-interactive mode](#non-interactive-mode) for the full picture.

### A config file for repeat use

If you don't want to pass env vars every time, write a config file:

```yaml
# ~/.config/cortex/config.yaml
provider: openrouter
model: deepseek/deepseek-v3.2
```

See [Configuration](#configuration) for the full three-layer cascade.

### Onboarding

If cortex starts without a model configured, it prints an onboarding message explaining the three places a model can come from — it does not guess or default.

> **💡 Tip:** The transcript is committed to your terminal's own scrollback, not held in an alternate screen. A finished session stays scrollable and copyable after cortex exits.

---

## Configuration

Three sources, each overriding the last. Layering is field by field.

### The cascade

1. `~/.config/cortex/config.yaml` — your defaults (honours `XDG_CONFIG_HOME`)
2. `./cortex.yaml` — this project's overrides
3. the environment — `LLM_PROVIDER`, `LLM_MODEL`, `LLM_BASE_URL`, `LLM_API_KEY`

Layering is **field by field**, so a project config that names only a model keeps the provider from your user config. You only specify what you want to override.

> **⚠️ Specifying `--config`** disables the cascade entirely — only that file is read, plus the environment. Use it when you want a specific, self-contained config.

### The full file

```yaml
name: reviewer                  # shown in the banner

provider: openrouter            # the axon provider name
model: deepseek/deepseek-v3.2   # the model within that provider

system_prompt: ./reviewer.md    # a path is read from disk; prose is used as written

pruner:                         # optional secondary model for context pruning
  model: meta-llama/llama-3.1-8b
  provider: openrouter          # defaults to the main provider if omitted

mcp_servers:                    # spawned at startup, their tools auto-discovered
  github:
    command: docker
    args: ["run", "-i", "--rm", "ghcr.io/github/github-mcp-server"]
    env: ["GITHUB_TOKEN=..."]
```

### Providers and credentials are not in this file

cortex's config only names *which* provider and model to use. Endpoints and credentials live in axon's config — the shared `providers.json` the axon-family tools already use, or the `LLM_API_KEY` / `LLM_BASE_URL` environment variables. See [API keys](#api-keys) for the resolution order.

### Known providers

`provider` supplies `base_url` for `openai`, `openrouter`, `anthropic`, `groq`, `deepseek` and `ollama`. Any other provider must state its own `base_url` (via axon's config or `LLM_BASE_URL`). A `localhost` endpoint (e.g. ollama) needs no API key.

### System prompt

Omit `system_prompt` and cortex uses its built-in coding-agent role text. The tool catalog is appended by axon, so a custom prompt **must not enumerate tools** — or you pay for those tokens twice on every call. See [Architecture](#architecture) for why.

### The fastest possible start

```bash
export LLM_MODEL=gpt-4o
export LLM_API_KEY=sk-...
cortex
```

---

## Interactive mode

The terminal UI — a transcript committed to your scrollback, not an alternate screen.

### Starting a session

```bash
cortex
```

cortex runs in your current working directory. It reads the repo, streams the model, dispatches tools, and renders everything into your terminal's own scrollback. When cortex exits, the finished session stays scrollable, searchable and copyable.

### How the transcript works

The transcript is committed to your terminal's own scrollback rather than held in an alternate screen. Only work still in flight redraws. This is a deliberate choice: a finished session is something you scroll back through, search, and copy from — including after cortex exits.

### Interrupting a turn

Press `Ctrl-C` to interrupt the running turn. cortex treats Ctrl-C in raw mode as a key, not a kill signal — it cancels the turn cleanly and returns you to the prompt.

> **ℹ️ Why not SIGINT?** In raw mode Ctrl-C arrives as a key. cortex binds it to cancelling the turn rather than killing the process. Only signals the terminal cannot deliver as a keystroke (`SIGTERM`, `SIGHUP`) end the program from outside.

### The banner

Each session opens with a banner showing the cortex version, the provider and model in use, and the current turn count. The `name` field from your config appears here too.

### Changing the model mid-session

When the model is changed interactively, cortex persists the new provider and model to your user config. The same applies to the pruner. So the next session picks up where you left off.

For the keys and commands available inside a session, see:
- [Keyboard reference](#keyboard-reference)
- [Slash commands](#slash-commands)

---

## Non-interactive mode

Pipe cortex into scripts with `--prompt`.

### The split: stdout for answers, stderr for everything else

`--prompt` runs a single turn with no TUI, then exits. The assistant's answer goes to **stdout**; tool activity, notices and errors go to **stderr**. That separation lets you compose:

```bash
cortex --prompt "list every exported function in internal/ui" > api.txt
cortex --prompt "summarize the TODOs" | fold -s | head -40
```

### Cancellation

In non-interactive mode, `Ctrl-C` ends the process — the ordinary expectation for a one-shot command. This is different from interactive mode, where Ctrl-C cancels the turn.

### Iteration limits

A `--prompt` run is bounded by `maxIterationsOnce`, a cap on how many tool turns the agent may take before it must answer. Interactive mode is unbounded — a human is watching, and Ctrl-C ends a turn that has lost its way.

> **ℹ️ Why bounded?** With no one watching, an agent that loops forever is a bill, not a bug. The cap is deliberately conservative for one-shot use.

### Combining with a config

```bash
cortex --config reviewer.yaml --prompt "review the last commit"
```

`--config` disables the cascade and reads only that file plus the environment. Combine with `--prompt` for reproducible one-shot runs against a fixed configuration.

---

## Slash commands

Type a slash command at the prompt to control the session.

| Command | What it does |
|---|---|
| `/new` | Wipe the session and start over. |
| `/undo` | Revert the last file edit, byte for byte. |
| `/cd <path>` | Change the working directory. |
| `/pwd` | Show the working directory. |
| `/session` | Session file, turn count, pending undos. |
| `/model` | Select a different model for this session. |
| `/pruner` | Select a different pruner model for this session. |
| `/switch` | Switch to a different saved session. |
| `/help` | List the commands. |
| `/quit` | Exit (`/exit`, `/q` also work). |

### `/undo` — byte-for-byte revert

Every file edit is atomic and recorded. `/undo` reverts the last edit exactly — the previous content is restored, not reconstructed. Repeat `/undo` to step backwards through the session's edits.

### `/session` — inspect state

Shows the path to the append-only session file, the current turn count, and how many undos are pending. Useful when you want to find the session on disk or understand how far the conversation has gone.

### `/new` — clean slate

Wipes the in-memory session and starts a fresh turn counter. The old session file remains on disk — `/new` only affects the live conversation, not persisted history.

> **💡 Aliases:** `/quit`, `/exit` and `/q` all exit.

---

## Keyboard reference

Every key the terminal binds, in one table.

| Key | Action |
|---|---|
| `enter` | Send the current input. |
| `ctrl+j` / `alt+enter` | Insert a newline. |
| `ctrl+c` / `esc` | Interrupt the running turn. |
| `ctrl+c` (when idle) | Clear the input, then exit. |
| `ctrl+d` | Exit. |

### Ctrl-C has two behaviours

Ctrl-C is context-sensitive. While a turn is running, it cancels the turn — the agent stops and you return to the prompt. When idle (no turn in flight), the first Ctrl-C clears the input buffer; a second exits.

### Esc interrupts too

`Esc` is bound to the same interrupt as Ctrl-C, so you can cancel a turn even on a terminal where Ctrl-C is captured by the shell or a multiplexer.

### Newlines without sending

`Ctrl+J` and `Alt+Enter` both insert a literal newline, so you can write multi-line prompts without accidentally sending them.

> **ℹ️ Raw mode.** cortex runs the terminal in raw mode, so keystrokes are intercepted individually rather than line-buffered. That's why Ctrl-C is a key, not a signal — see [Interactive mode](#interactive-mode).

---

## Architecture

> **axon owns the loop. cortex owns the product on top. Layers point one direction.**

```
┌─────────────────────────────────────────────────────────┐
│  cortex — the product                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │ main.go  │  │ config   │  │ ui       │              │
│  │ flags    │  │ resolve  │  │ terminal │              │
│  │ wiring   │  │ model    │  │ transcrip│              │
│  │ dispatch │  │ prompt   │  │ commands │              │
│  └──────────┘  └──────────┘  └──────────┘              │
│  config & theme are leaves · ui owns the terminal       │
└───────────────────────┬─────────────────────────────────┘
                        │ the only seam
┌───────────────────────┴─────────────────────────────────┐
│  axon — the runtime  (↗ docs)                           │
│  ┌────────┐ ┌─────────┐ ┌────────┐ ┌────────┐          │
│  │ loop   │ │ session │ │ pruner │ │ tools  │          │
│  │ stream │ │ append  │ │ 2nd-LLM│ │ 7 built│          │
│  │ chat    │ │ undo    │ │ context│ │ + MCP  │          │
│  └────────┘ └─────────┘ └────────┘ └────────┘          │
└─────────────────────────────────────────────────────────┘
```

### The two boundary rules

1. **`internal/config` imports nothing else from this module.** It is a leaf. If a value there needs a type from a higher layer, it belongs in that layer.

2. **`internal/ui/event.go` is the only file that touches both axon and Bubble Tea.** axon calls its event handler synchronously on the turn goroutine; Bubble Tea owns the terminal from its own. Nothing else may cross that line.

### What `main.go` wires

`main.go` is wiring and nothing else. Every decision it makes is delegated: configuration to `internal/config`, presentation to `internal/ui`, and the loop itself to [axon](https://atakang7.github.io/axon/).

```
Parse flags → Load config → Tap models → Run loop
  ⚓            ⚙️             📡            ▶
  interactive   resolve model   trace        build axon Options
  non-interactive  & pruner     wiretap     attach handler
  version        vs axon        main+pruner  go
```

### Why the system prompt must not list tools

The tool catalog is appended by [axon](https://atakang7.github.io/axon/tools/builtins/), natively, to every request. If a custom `system_prompt` also enumerates the tools, the model sees the catalog twice — and you pay for those duplicated tokens on every single call. This is exactly the kind of thing the [trace log](#trace-log) was built to catch: a `~2,100-token` duplication that was invisible in the event stream and obvious in the wire log.

### Why the transcript isn't an alternate screen

| alternate screen | committed scrollback |
|---|---|
| Vanishes on exit. The session is gone the moment cortex stops — nothing to scroll, search or copy. | Stays in your terminal's own scrollback. A finished session is scrollable, searchable and copyable — even after cortex exits. Only work in flight redraws. |

> **📖 Deeper into axon:** the loop, session, pruner and tools are all axon's. Read [axon's architecture](https://atakang7.github.io/axon/internals/architecture/), [context management](https://atakang7.github.io/axon/concepts/context/), and [the built-in tools](https://atakang7.github.io/axon/tools/builtins/) for the layer cortex builds on.

---

## The pruner

A cheap secondary model that parks stale context so the main model stays under its window.

### The problem

As a session grows, the conversation approaches the model's context window. The naive fix — dropping old messages — loses the thread. The pruner does something smarter: it runs a separate, cheap model that *parks* stale context by summarising it, so the main model keeps the thread without paying for every old token on every call.

### How it's configured

The pruner is optional. When configured, it runs alongside the main model:

```yaml
pruner:
  model: meta-llama/llama-3.1-8b   # a cheap, fast model
  provider: openrouter             # defaults to the main provider if omitted
```

If `pruner.provider` is omitted, it falls back to the main provider. If `pruner.model` is empty, no pruner runs — the main model manages its own window.

### What the pruner sees

The pruner's context decisions are as worth auditing as the agent's, and they are invisible in the event stream — the stream only reports how many tokens a pass moved, never why. That's why the pruner model is **also tapped by the trace log**, so you can read its curator calls and understand what it decided to park and what it kept.

> **ℹ️ Two models, one trace.** Both the main model and the pruner are tapped at the single place they are resolved. See [Trace log](#trace-log) for how to read what each one sent.

### Persisting the pruner choice

When the pruner model is changed interactively, cortex saves the new provider and model to your user config. The next session picks it up automatically.

> **📖 The pruner is axon's.** cortex only names *which* cheap model to run. axon owns the context windowing, the curator calls and the parking strategy — see [axon's context management](https://atakang7.github.io/axon/concepts/context/) for how it keeps long sessions stable. cortex taps the pruner model into the [trace log](#trace-log) so you can audit those decisions.

### When not to use a pruner

- For short sessions that never approach the window — the pruner only adds cost.
- When you want the main model to see the full, unwindowed conversation — e.g. careful auditing.

---

## Trace log

A third, unfiltered event sink — wire-level request and reply logging.

```
          ┌──────────────────┐
          │  axon runtime    │
          │  stream · tools  │
          │  pruner          │
          └────────┬─────────┘
                   │ every request & reply
          ┌────────┴──────────────────────┐
          ▼                ▼              ▼
   ┌─────────────┐ ┌──────────────┐ ┌──────────────┐
   │  the TUI    │ │ plain render │ │ the trace    │
   │  what user  │ │ stdout = ans │ │ the wire     │
   │  sees       │ │ stderr = all │ │ exact req/rsp│
   └─────────────┘ └──────────────┘ └──────────────┘
```

Both the main model and the pruner are tapped at the one place they are resolved — so the trace holds the actual prompts, not just the rendered replies.

### Where it lives

The trace log is written to `DefaultTracePath()`, a path under your cache directory. At startup cortex prints the path to stderr:

```
cortex: trace: /home/you/.cache/cortex/trace-20250101-120000.jsonl
```

### It never fails the run

A broken cache directory means no trace, not no cortex. If the trace log can't be opened, cortex prints a notice to stderr and continues without it:

```
cortex: trace log disabled: <error>
```

### Both models are tapped

The pruner's context decisions are as worth auditing as the agent's, and they are invisible in the event stream — the stream only reports how many tokens a pass moved, never why. Tapping both models at the one place they are resolved is what puts those decisions in the trace, so you can read what the pruner decided to park and what it kept.

### Run metadata

The trace opens with a `Describe` block recording the cortex version, the provider and model for both the main agent and the pruner, and the working directory. This anchors every trace to the exact configuration that produced it.

### Why it exists

The trace log is the tool you reach for when the agent did something surprising and you want to know *why* — what prompt the model actually received, what context the pruner had decided to park, what the model said before the UI rendered it. It's prompt-engineering against reality, not against the abstraction.

> **📖 The event stream is axon's.** axon emits structured events describing every reply — [the events reference](https://atakang7.github.io/axon/concepts/events/) covers them. The trace log adds the wire layer those events are built on top of. See [Reading traces](#reading-traces) for the tool that views them.

---

## MCP servers

Spawn any Model Context Protocol server at startup; its tools are wired in automatically.

### What MCP is

The [Model Context Protocol](https://modelcontextprotocol.io/) is an open standard for exposing tools, resources and prompts to LLMs. An MCP server is a process that speaks the protocol — GitHub's server, a filesystem server, a database server, anything.

### Configuring a server

```yaml
mcp_servers:
  github:
    command: docker
    args: ["run", "-i", "--rm", "ghcr.io/github/github-mcp-server"]
    env: ["GITHUB_TOKEN=ghp_..."]
```

Each entry under `mcp_servers` names a server and gives the command, arguments and environment to spawn it. cortex starts the server at startup, discovers its tools over the protocol, and wires them into the agent's tool set.

### Auto-discovery

You don't enumerate the tools a server provides — cortex asks the server what it offers and adds every tool to the catalog automatically. If the server adds a new tool, it shows up on the next session with no config change.

### Environment variables

The `env` list is passed to the spawned process. Use it for API tokens and any other secrets the server needs. The values are strings; a leading `$` does **not** expand here (unlike `api_key`), so pass the literal value or interpolate from your shell before writing the config.

> **⚠️ Security:** an MCP server runs with the same filesystem and network access as the cortex process. Only spawn servers you trust.

### Multiple servers

```yaml
mcp_servers:
  github:
    command: docker
    args: ["run", "-i", "--rm", "ghcr.io/github/github-mcp-server"]
    env: ["GITHUB_TOKEN=..."]
  sqlite:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-sqlite", "--db-path", "./data.db"]
```

Tools from all servers are merged into a single catalog. Name collisions are resolved by the server prefix.

> **📖 MCP is axon's.** cortex spawns the servers in config; axon speaks the protocol, discovers the tools, and merges them with its [built-in tools](https://atakang7.github.io/axon/tools/builtins/) into one catalog. The auto-discovery is axon's — see [axon's tools reference](https://atakang7.github.io/axon/tools/builtins/) for the full toolset cortex gets on top of MCP.

---

## Playground

> A harness for watching cortex work and finding out *why* it works the way it does.
>
> Not a benchmark — `benchmarks/polyglot` scores cortex. This one shows its reasoning so a bad habit can be traced to the prompt or tool description that caused it.

### The commands

```bash
./run.sh <task> [label]             # run cortex on one task, keep everything
./batch.sh [-n runs] [task ...]     # repeat a suite and keep going on failure
./report.sh [runs/<task-label> ...]  # summarize pass/fail, cost and behavior
./trace.sh <view> <trace>            # read what happened inside one run
```

### What a run keeps

Each run gets its own directory under `runs/` holding a pristine copy of the task's seed project, the trace cortex wrote, cortex's own stdout, and verifier output. The seed is never touched, so a run is repeatable and two runs are directly comparable:

- `trace.jsonl` — the wire-level record
- `diff.patch` — exactly what the agent changed
- `stdout.txt` / `stderr.txt` — the agent's own output
- `verify_stdout.txt` / `verify_stderr.txt` — final project verification
- `status.txt` — git status of the work tree

### The trace

Newline-delimited JSON with one discriminator, `kind`. Most kinds are axon event names. Three are cortex's own:

| kind | what it holds |
|---|---|
| `run` | which build and which models produced this trace |
| `wire_request` | the exact messages sent to a model, and to which one |
| `wire_reply` | what came back, and how long it took |

The wire kinds are the reason this harness exists. Events are emitted *downstream* of a request — they describe the reply and never the prompt — so the composed system message, the windowed conversation and the pruner's own curator calls are all invisible without them. Prompt engineering against the event stream alone is guesswork.

### The views

`trace.sh` offers six views, each one `jq` filter over the trace:

```bash
./trace.sh timeline runs/report-final/trace.jsonl
./trace.sh system   runs/report-final/trace.jsonl   # the prompt under test
./trace.sh prompt   runs/report-final/trace.jsonl pruner
./trace.sh tools    runs/report-final/trace.jsonl   # shows a wrong approach fastest
```

`report.sh` is the first thing to read after a suite. It flags failed verification, tool errors, runtime errors, no-op prunes, large call counts, and no-diff runs before you dive into individual traces.

Continue to [Running tasks](#running-tasks) and [Reading traces](#reading-traces) for the details.

---

### Running tasks

`run.sh` drives cortex through one playground task and keeps everything it did.

#### Usage

```bash
./run.sh <task> [label]
```

#### What happens, step by step

1. `run.sh ledger iter1` copies `tasks/ledger/seed` into `runs/ledger-iter1/work`.
2. It commits the seed in a fresh per-run git repo. `git diff` in the work directory is then *exactly* what the agent changed, with no other setup.
3. A `.gitignore` is written so build artefacts (`__pycache__/`, `*.pyc`, `.pytest_cache/`) don't fill the diff with binary noise and bury the one source change that matters.
4. It builds the cortex in the working tree — **not** whatever was on `$PATH`. The whole point is to test the code you're working on.
5. It detects the seed project's verifier (e.g. `go test ./...`, `python -m pytest -q`) and runs it in the work directory after cortex exits, recording the result.
6. It runs cortex non-interactively on `tasks/<task>/TASK.txt`, writing stdout and stderr to the run directory.
7. cortex announces its trace file on stderr; `run.sh` copies it into the run directory so the directory is self-contained once the cache is cleaned.
8. It runs the seed project's verifier (`go test ./...`, `python -m pytest -q`, or `npm test`) and records the result.
9. It stages all changes and writes `diff.patch` — staged so files the agent created show up as additions rather than untracked paths the patch never mentions.

#### The run directory

```
runs/ledger-iter1/
├── work/           # pristine seed copy + the agent's changes
├── cortex          # the freshly-built binary
├── trace.jsonl     # the wire-level record
├── diff.patch      # exactly what the agent changed
├── stdout.txt      # the agent's answer (stdout)
├── stderr.txt      # notices, tool activity, errors
├── verify.status   # verifier exit status
├── verify_stdout.txt
├── verify_stderr.txt
└── status.txt      # git status of the work tree
```

#### Batch runs

```bash
./batch.sh -n 3 report pipeline
```

Batch runs keep going after failures and write a markdown summary under `runs/`. Use this for prompt or tool changes so one unlucky run does not hide the rest of the suite.

> **💡 Comparing runs:** because the seed is never touched and each run is self-contained, two runs are directly comparable. Diff the `diff.patch` files, or compare `trace.jsonl` with the [trace views](#reading-traces).

---

### Reading traces

`trace.sh` reads a cortex trace. Six views, each one `jq` filter.

```bash
./trace.sh <view> <trace.jsonl> [arg]
```

#### `timeline`

One line per thing that happened, with just enough payload to see the shape of a run. Streaming token noise is dropped — it's the same text the `assistant_end` line carries in full. Great for getting the overall shape of a run at a glance.

```bash
./trace.sh timeline runs/report-final/trace.jsonl
```

#### `prompt`

Every message of one model call, exactly as the provider saw it. Defaults to the last main-model request; pass a call number for a specific one, or `pruner` for the curator's last.

```bash
./trace.sh prompt runs/report-final/trace.jsonl          # last main call
./trace.sh prompt runs/report-final/trace.jsonl 3        # call #3
./trace.sh prompt runs/report-final/trace.jsonl pruner   # the pruner's last call
```

#### `system`

The system message alone — the prompt under test, plus whatever axon appends (including the tool catalog). This is the view for prompt engineering: it shows the composed system prompt the model actually received.

```bash
./trace.sh system runs/report-final/trace.jsonl
```

#### `tools`

What the agent actually did, in order, with full arguments and a clipped result. This is the view that shows a wrong approach fastest.

```bash
./trace.sh tools runs/report-final/trace.jsonl
```

#### `usage`

Token spend per call and in total. Reported by the provider, not estimated.

```bash
./trace.sh usage runs/report-final/trace.jsonl
```

#### `errors`

Every error, tool error and wire error, with timestamps.

```bash
./trace.sh errors runs/report-final/trace.jsonl
```

> **💡 The pattern worth keeping:** when the agent misbehaves, read `tools` first to see what it did, then `prompt` to see what it was told. The cause is almost always something it was told, and usually somewhere more structural than the role prompt.

---

### Tasks reference

A task is a `seed/` project and a `TASK.txt` written the way a user would write it.

Seeds ship with a passing test suite so a regression is unambiguous. The debugging tasks are the useful ones — feature work rewards a competent model; debugging rewards one that plans, reads before concluding, and knows what it has not checked.

| task | shape | what it tests |
|---|---|---|
| `ledger` | small CLI, suite passes | a feature across four files, with a backward-compatibility trap in the on-disk format |
| `report` | four modules, four failing tests | a bug whose symptom and cause are in different files, with a plausible wrong fix at the symptom |
| `pipeline` | 16 modules, 41KB, three failing tests | a bug visible only by comparing two files neither of which is wrong alone; large enough to put the pruner under load |

> **⚠️ `pipeline` is the only task big enough to exercise the pruner.** Keep it that way — a curator that never runs is a curator nobody is testing.

#### What the playground found

Each of these was invisible in the event stream and obvious in the wire log:

**Duplicated tool catalog (~2,100 tokens/call)** — axon restated the whole tool catalog in the system prompt while the same tools were already sent natively. ~2,100 duplicated tokens on every call.

**`write` steered to whole-file rewrites** — Rewriting the tool's prose changed nothing; reordering the `mode` enum in its JSON Schema changed everything. **Where prose and schema disagree, the schema wins.**

**`replace_string`'s not-found error recommended `replace_lines`** — A run took the advice, used line numbers from the same stale read, destroyed a docstring and spent two calls repairing it.

**The pruner fired on an empty log** — The pruner fired on a context of 10,229 tokens and sent the curator an empty log — no task, no blocks, 223 bytes. `ShouldFire` counts tokens while the recency window counts blocks, so a few large files early in a task clear the floor while every block is still inside the window. The curator answered `{"park":[]}`, correctly, having been shown nothing; the context was unchanged, so it re-fired every `growth_tokens` after that.

> **💡 The pattern worth keeping:** when the agent misbehaves, read `tools` first to see what it did, then `prompt` to see what it was told. The cause is almost always something it was told, and usually somewhere more structural than the role prompt.

---

## CLI reference

cortex takes a small number of flags. Everything else lives in config.

### Flags

| Flag | Default | Description |
|---|---|---|
| `--config <path>` | — | Path to a config file instead of the usual cascade. **Disables the cascade entirely** — only that file is read, plus the environment. |
| `--prompt <text>` | — | Run one prompt non-interactively and exit. The answer goes to stdout; everything else to stderr. Bounded by `maxIterationsOnce`. |
| `--version` | — | Print the version, commit and build date, then exit. |

### Modes

| Invocation | Mode |
|---|---|
| `cortex` | Interactive (the TUI) |
| `cortex --prompt "…"` | Non-interactive (one turn, then exit) |
| `cortex --version` | Version only |

### Exit codes

- `0` — normal exit
- `1` — an error occurred (printed to stderr as `cortex: <error>`)

### Build metadata

The version, commit and build date are injected at link time by goreleaser. A source build without ldflags reports `cortex dev (none, built unknown)`.

> **ℹ️ Why so few flags?** Configuration belongs in the [config file](#config-file-reference), not on the command line. The only flags are mode switches and the override path.

---

## Config file reference

Every field in a cortex config file, what it means and what it overrides.

### Locations

| Source | Path | Notes |
|---|---|---|
| User config | `~/.config/cortex/config.yaml` | Honours `XDG_CONFIG_HOME` |
| Project config | `./cortex.yaml` | Overrides user config, field by field |
| Environment | `LLM_*` | Overrides both |
| Explicit | `--config <path>` | Disables the cascade entirely |

### Fields

| Field | Type | Description |
|---|---|---|
| `name` | string | Labels this agent personality (e.g. `cortex`, `reviewer`). Shown in the banner. |
| `provider` | string | The axon provider name (e.g. `openai`, `openrouter`). Supplies a default `base_url` for known providers. |
| `model` | string | The model within that provider (e.g. `gpt-4o`, `deepseek/deepseek-v3.2`). |
| `system_prompt` | string | The agent's role text. A path is read from disk; prose is used as written. Omit to use the built-in coding-agent prompt. **Must not enumerate tools** — axon appends the catalog. |
| `pruner` | object | The secondary model for context pruning. See below. |
| `mcp_servers` | map | MCP subprocesses to spawn at startup. See below. |

### `pruner`

| Field | Type | Description |
|---|---|---|
| `pruner.provider` | string | Provider for the pruner model. Defaults to the main provider if omitted. |
| `pruner.model` | string | The cheap, fast model used for context pruning. Empty means no pruner. |

### `mcp_servers`

Each key is a server name. Each value has:

| Field | Type | Description |
|---|---|---|
| `command` | string | The executable to spawn. |
| `args` | []string | Arguments passed to the command. |
| `env` | []string | Environment variables for the spawned process. Literal values — no `$` expansion. |

### Environment overrides

Only model selection is overridable by the environment. Credentials and endpoints are axon's concern.

| Variable | Overrides |
|---|---|
| `LLM_PROVIDER` | `provider` |
| `LLM_MODEL` | `model` |
| `LLM_BASE_URL` | The provider's default `base_url` |
| `LLM_API_KEY` | API key (see [API keys](#api-keys)) |

> **⚠️ Providers and credentials are not in this file.** cortex does not define endpoints or carry credentials — those live in axon's `providers.json` or the `LLM_*` environment. cortex only names which provider and model to use.

---

## API keys

Where the key comes from. Checked in order; the first one that answers wins.

1. `api_key` in axon's `providers.json` — a leading `$` reads that environment variable at startup, so you never commit the key itself.
2. The `LLM_API_KEY` environment variable.
3. `~/.config/agent/providers.json`, matched on the `provider` name — the shared file the axon-family tools already use. Override its path with `AXON_PROVIDERS_PATH`.

The third exists so a key has **one home on disk**. If you already keep credentials in `providers.json`, omit the key from your cortex config entirely and cortex will find it:

```json
{ "providers": [
    { "name": "openrouter", "base_url": "https://openrouter.ai/api", "api_key": "sk-or-..." }
] }
```

cortex reads only `name` and `api_key` from that file; the model lists and routing options in it belong to other tools and are ignored.

### Local endpoints need no key

A `localhost` endpoint (e.g. ollama running locally) needs no API key. Point `LLM_BASE_URL` at it and leave the key unset.

### The leading `$`

Wherever a config accepts an `api_key` value, a leading `$` reads the rest as an environment variable name and substitutes its value at startup. This keeps the literal key out of your config file:

```json
{ "providers": [
    { "name": "openai", "api_key": "$OPENAI_API_KEY" }
] }
```

> **ℹ️ Why three sources?** The cascade lets you keep a default provider in your user config, override only the model per project, and keep the actual credential in one shared file. No copying keys between configs.

---

<div align="center">

**[atakang7.github.io/cortex](https://atakang7.github.io/cortex/)** · MIT · built on [axon](https://github.com/atakang7/axon)

</div>

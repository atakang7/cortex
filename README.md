<div align="center">

# cortex

### A terminal coding agent that acts on the repository, verifies its work, and gets out of the way.

Built in Go on top of the [Axon](https://github.com/atakang7/axon) agent runtime.

[![Release](https://img.shields.io/github/v/release/atakang7/cortex?style=flat-square)](https://github.com/atakang7/cortex/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.26.2-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![Axon](https://img.shields.io/badge/runtime-Axon-6366f1?style=flat-square)](https://github.com/atakang7/axon)
[![License](https://img.shields.io/github/license/atakang7/cortex?style=flat-square)](LICENSE)

**Search → read → edit → verify.**  
No browser tab. No IDE plugin. No second editor living inside your editor.

</div>

---

```text
❯ cortex

  cortex
  z-ai/glm-5.2 · pruner: deepseek/deepseek-v4-flash-0731

❯ add an LLM-generated title for each new session

  search  deriveTitle
  read    session.go
  read    loop.go
  write   session.go
  write   loop.go
  exec    go test ./...

✓ Both repos build, vet, and test green. Done.

## What changed

axon: added an optional title model with deterministic fallback.
cortex: wired the active model into session title generation.

## Verified

axon:   go build ./... · go vet ./... · go test ./...  PASS
cortex: go build ./... · go vet ./... · go test ./...  PASS

❯ ask for a change, or /help
```

Cortex is designed for the part of coding-agent work that actually matters: **inspect the real repository, make the smallest correct change, run the command that proves it, and report exactly what happened.**

It is intentionally split into two projects:

```text
┌─────────────────────────────────────────────────────────────────┐
│ cortex                                                          │
│ product layer                                                   │
│                                                                 │
│ model choice · coding prompt · terminal UI · commands · config │
└──────────────────────────────┬──────────────────────────────────┘
                               │ axon.New(...)
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│ axon                                                            │
│ agent runtime                                                   │
│                                                                 │
│ turn loop · tools · sessions · context · retries · streaming   │
│ background processes · MCP · events · usage                    │
└─────────────────────────────────────────────────────────────────┘
```

Cortex does **not** hide another agent framework inside its UI. The terminal is a product built on one reusable runtime.

## Why Cortex

A coding agent should feel less like a chatbot with shell access and more like a careful engineer working directly in your repository.

| Principle | Cortex behavior |
| --- | --- |
| **Act, don't narrate** | It does the work first instead of spending turns announcing what it plans to do. |
| **Search before reading** | It finds the relevant code before guessing paths or loading giant files. |
| **Read before writing** | It does not edit files it has not inspected. |
| **Small changes** | Targeted replacements are preferred over gratuitous file rewrites. |
| **Verify every change** | Tests, builds, linters, or concrete commands close the loop after edits. |
| **Stop when done** | No surprise refactors, adjacent cleanup, or invented scope. |
| **Report failures honestly** | A failed test stays a failed test in the final answer. |
| **Keep the terminal yours** | Finished output remains in normal terminal scrollback — searchable, selectable, copyable. |

That behavior is not marketing copy. It is the default role Cortex ships with in [`internal/config/prompt.go`](internal/config/prompt.go).

---

# Quick start

## 1. Install Cortex

With Go:

```sh
go install github.com/atakang7/cortex/cmd/cortex@latest
```

Or download a release binary for:

```text
linux  amd64 / arm64
darwin amd64 / arm64
```

from [GitHub Releases](https://github.com/atakang7/cortex/releases).

## 2. Configure a provider in Axon

Cortex chooses **which** provider/model to use. Axon owns **where** that provider lives, its credentials, model-level runtime settings, retry policy, and tool limits.

Create `~/.config/axon/axon.yaml`:

```yaml
providers:
  openrouter:
    base_url: https://openrouter.ai/api
    api_key: ${OPENROUTER_API_KEY}
    models:
      z-ai/glm-5.2:
      deepseek/deepseek-v3.2:
      qwen/qwen3.6-flash:
```

Put secrets beside it in `~/.config/axon/.env`:

```dotenv
OPENROUTER_API_KEY=sk-or-...
```

```sh
chmod 600 ~/.config/axon/.env
```

Axon also supports local OpenAI-compatible endpoints such as Ollama without an API key.

See the [Axon configuration reference](https://atakang7.github.io/axon/configuration/yaml/) for retries, request limits, reasoning effort, context policy, tool caps, and provider routing.

## 3. Tell Cortex which model to use

Create `~/.config/cortex/config.yaml`:

```yaml
provider: openrouter
model: z-ai/glm-5.2
```

Then:

```sh
cd your-repository
cortex
```

That's the minimum.

---

# What Cortex can do

Cortex inherits Axon's built-in engineering toolset and turns it into a coding workflow.

| Capability | What it means in practice |
| --- | --- |
| **Repository search** | Literal and regex search through the workspace with ripgrep. |
| **Focused file reads** | Read slices instead of throwing an entire repository into context. |
| **Atomic edits** | Save, replace strings, replace line ranges, or insert at a specific line. |
| **Undo** | Every Axon-managed file edit is recorded and can be reverted byte-for-byte with `/undo`. |
| **Foreground commands** | Run tests, builds, linters, Git commands, migrations, or arbitrary shell commands. |
| **Background commands** | Start servers/watchers without hanging the turn, poll only new output, then terminate the process group cleanly. |
| **Long sessions** | Persist conversation state and optionally prune stale context with a cheaper secondary model. |
| **MCP tools** | Spawn MCP servers at startup and expose their tools beside the built-ins. |
| **Live model switching** | Change the active model from inside the terminal with `/model`. |
| **Live pruner switching** | Change or enable the context-pruner model with `/pruner`. |
| **Structured traces** | Every runtime event is also written as JSONL for post-mortem inspection. |
| **Automation mode** | Use `--prompt` in scripts with the final answer on stdout and runtime activity on stderr. |

## The built-in tool surface

Axon gives Cortex seven default tools:

```text
read
write
exec
bash_output
kill_shell
search
task
```

The important part is not the names. It is the lifecycle around them.

### Editing is reversible

```text
model asks for write
       │
       ▼
atomic tmp + rename
       │
       ├── previous bytes recorded
       │
       ▼
repository changed
       │
       └── /undo → exact previous bytes restored
```

### Long-running processes do not own the turn

```text
exec(run_in_background=true)
              │
              ▼
      managed process group
              │
              ├── logfile
              ├── shell_id = bash_1
              │
              ▼
      bash_output(bash_1)
      returns only new output
              │
              ▼
      kill_shell(bash_1)
       TERM → grace → KILL
```

That is why Cortex can start a dev server, inspect its logs, make another change, retest it, and clean the server up without turning the terminal into process-management soup.

---

# One turn, end to end

A user message is not one model call.

```text
user request
    │
    ▼
append to durable session
    │
    ▼
project current context
    │
    ▼
stream model response
    │
    ├── text
    ├── reasoning
    ├── tool arguments
    └── token usage
    │
    ▼
tool calls?
    │
    ├── yes → execute → append results → call model again ──┐
    │                                                       │
    └── no  → final answer ◄────────────────────────────────┘
```

Cortex owns the coding behavior and terminal experience. Axon's `Step()` owns that continuation loop until the model has no more work to dispatch.

This distinction matters: a task such as “fix the failing tests” can naturally become ten reads, three edits, four test runs, a background process, and multiple model calls while still being **one Cortex turn**.

---

# The terminal

Cortex uses Bubble Tea for the live area without putting the whole transcript inside an alternate screen.

That means completed conversation remains normal terminal history:

- scroll it with your terminal
- search it with your terminal
- select and copy it normally
- keep it after Cortex exits

Only work that is actively changing is redrawn.

## Keys

| Key | Action |
| --- | --- |
| `enter` | Send the current input |
| `ctrl+j` / `alt+enter` | Insert a newline |
| `ctrl+c` / `esc` while running | Interrupt the active turn |
| `ctrl+c` while idle | Clear input; press again on empty input to exit |
| `ctrl+d` | Exit |

## Slash commands

Commands are handled locally. They are not sent to the model and do not burn a model turn.

| Command | Purpose |
| --- | --- |
| `/new` | Wipe the current session and start over |
| `/undo` | Revert the last recorded file edit byte-for-byte |
| `/cd <path>` | Change the agent working directory |
| `/pwd` | Show the current working directory |
| `/session` | Show turn count, message count, pending edits, and session path |
| `/model` | Select a different model for the current session |
| `/pruner` | Select a different context-pruner model |
| `/help` | List commands |
| `/quit` | Exit Cortex |

Aliases:

```text
/exit → /quit
/q    → /quit
/?    → /help
/h    → /help
```

---

# Interactive and automation modes

## Interactive

```sh
cortex
```

Use this for normal repository work.

## One-shot

```sh
cortex --prompt "find every exported function in internal/ui"
```

One-shot mode uses the **same Axon agent construction** as the interactive TUI. There is no reduced “script path” with different runtime behavior.

The output contract is intentionally Unix-friendly:

```text
stdout  final assistant answer
stderr  tool activity, notices, errors, trace path
```

So Cortex composes naturally:

```sh
cortex --prompt "summarize every TODO in this repository" > todos.md
```

```sh
report=$(cortex --prompt "explain the public API changes since HEAD~5")
```

```sh
cortex --prompt "find flaky tests and explain why" 2>cortex.log | tee analysis.md
```

This is useful in shell scripts, CI experiments, repository analysis, and benchmark harnesses without teaching Cortex a second interface.

---

# Configuration

Cortex's config is intentionally small because runtime configuration belongs to Axon.

## The cascade

Without `--config`, Cortex layers:

```text
~/.config/cortex/config.yaml   user defaults
             ↓
./cortex.yaml                  project overrides
             ↓
LLM_PROVIDER / LLM_MODEL       one-off environment overrides
```

Fields merge individually. A project can override only the model and inherit everything else from the user's config.

With:

```sh
cortex --config reviewer.yaml
```

that explicit file is used instead of the normal file cascade.

## Full Cortex config

```yaml
name: cortex

provider: openrouter
model: z-ai/glm-5.2

# Optional secondary model used by Axon to park stale context.
pruner:
  provider: openrouter
  model: deepseek/deepseek-v4-flash-0731

# Optional. Can be inline prose or a path to .md/.txt.
system_prompt: ./prompts/cortex.md

# Optional MCP servers. Axon spawns these and discovers their tools.
mcp_servers:
  github:
    command: docker
    args:
      - run
      - -i
      - --rm
      - -e
      - GITHUB_PERSONAL_ACCESS_TOKEN
      - ghcr.io/github/github-mcp-server
    env:
      - GITHUB_PERSONAL_ACCESS_TOKEN=...
```

### What does *not* belong here

Cortex no longer owns:

```text
API keys
base URLs
retry policy
reasoning effort
request / idle timeouts
tool output limits
session storage policy
pruner thresholds
```

Those are runtime concerns and live in `~/.config/axon/axon.yaml`.

That separation is deliberate:

```text
cortex.yaml                 axon.yaml
────────────                ─────────
which model?                where is provider?
which role?                 how authenticated?
which MCP servers?          how retry?
which pruner model?         how much context?
agent name?                 how large can tools run?
```

---

# Models

Cortex can use any model exposed through an Axon provider implementing OpenAI-style streaming chat completions and tool calls.

The model itself is not hardcoded into Cortex.

```yaml
# cortex.yaml
provider: openrouter
model: z-ai/glm-5.2
```

or:

```yaml
provider: ollama
model: qwen2.5-coder:7b
```

Model definitions and endpoints live in Axon:

```yaml
# ~/.config/axon/axon.yaml
providers:
  ollama:
    base_url: http://localhost:11434
    models:
      qwen2.5-coder:7b:
```

During an interactive session, `/model` opens Cortex's model selector and updates the underlying Axon agent without rebuilding the entire product.

---

# Long-session context

Repository work gets large fast. A useful coding agent cannot simply keep concatenating every file read, command output, and tool result forever.

Cortex can therefore configure a secondary **pruner model**:

```yaml
pruner:
  provider: openrouter
  model: qwen/qwen3.6-flash
```

Axon keeps the durable session intact and changes only the projection sent to the main model.

```text
DURABLE SESSION                     MODEL CONTEXT
────────────────                    ─────────────
m1 full history ────────────────→   [m1 gist]
m2 full history ────────────────→   [m2 gist]
m3 full history ── parked ──────→   [m3 parked breadcrumb]
m4 full history ────────────────→   [m4 gist]
m5 recent full ─────────────────→   m5 full
m6 recent full ─────────────────→   m6 full
m7 recent full ─────────────────→   m7 full
```

**Parking is a projection, not deletion.** The original messages stay in the session file.

Axon's default policy starts with:

```text
recent full-content window    30 blocks
pruner floor                  10,000 tokens
re-prune growth                5,000 tokens
```

The exact runtime policy remains configurable in Axon. `/pruner` lets you change the pruner model during an interactive Cortex session.

---

# MCP

Cortex can attach domain-specific capabilities without teaching its core UI about them.

Declare MCP subprocesses:

```yaml
mcp_servers:
  github:
    command: docker
    args: ["run", "-i", "--rm", "ghcr.io/github/github-mcp-server"]
    env:
      - GITHUB_TOKEN=...
```

At startup:

```text
Cortex config
     │
     ▼
MCP subprocess
     │ stdio JSON-RPC
     ▼
Axon discovers tools
     │
     ▼
same tool loop as built-ins
```

From Cortex's perspective, an MCP tool and an Axon built-in enter the model's tool catalog through the same runtime boundary.

This is the path for repository hosting, issue trackers, internal developer platforms, databases, or any other tool surface you do not want hardcoded into Cortex.

---

# Sessions, undo, and working directories

Axon persists Cortex sessions per working directory unless its runtime settings say otherwise.

The session contains the actual turn history, tool interaction blocks, edit history, and task state. Cortex exposes the useful controls directly:

```text
/session     inspect the current session
/new         wipe it and start clean
/undo        restore the last Axon-managed edit
/cd          move the active workspace
/pwd         show where the agent is operating
```

Changing directory updates the Axon workspace rather than spawning a second agent.

## Important boundary

A working directory is **routing, not an OS sandbox**.

Cortex is a local coding agent intentionally capable of executing commands and editing files. If you need containerization, filesystem isolation, restricted credentials, or policy enforcement, run Cortex inside the environment that provides those boundaries.

---

# Trace everything

Every Cortex process creates an unfiltered JSONL event trace under the user's cache directory:

```text
$XDG_CACHE_HOME/cortex/trace/<timestamp>-<pid>.jsonl
```

or the platform-equivalent user cache path.

The path is printed on startup.

Each line represents one Axon event in emission order:

```json
{"time":"...","kind":"turn_start","turn":2}
{"time":"...","kind":"tool_call","turn":2,"tool":{"name":"search"}}
{"time":"...","kind":"tool_result","turn":2,"tool":{"name":"search"}}
{"time":"...","kind":"prune_end","turn":2,"prune":{"before":18000,"after":11900}}
```

The TUI intentionally curates what a human needs to see. The trace does not.

That gives you a durable answer to:

> What did the runtime actually do?

You can inspect it after a bad run or stream it live:

```sh
tail -f ~/.cache/cortex/trace/*.jsonl
```

---

# The default coding role

Cortex's default system prompt is deliberately opinionated about engineering behavior while leaving the tool catalog to Axon.

The core rules are:

```text
search before read
read before write
act instead of narrating
make one change at a time
verify the change
match local code style
prefer targeted edits
background long-running processes
stop what you start
report failures faithfully
stop when the requested goal is met
```

Why does the prompt **not** list tools?

Because Axon already appends the live tool schemas to the system prompt. Repeating them inside Cortex would duplicate tokens on every request and allow the prose to drift away from the real registered tool surface.

You can replace the role entirely:

```yaml
system_prompt: ./reviewer.md
```

or with inline prose:

```yaml
system_prompt: You are a release-focused Go maintainer. Change only what is necessary and always run the narrowest relevant test first.
```

This makes Cortex useful as more than one personality while keeping the runtime and terminal unchanged.

---

# Example profiles

The binary stays the same. A project profile can narrow the behavior.

## Reviewer

```yaml
# reviewer.yaml
name: reviewer
provider: openrouter
model: z-ai/glm-5.2
system_prompt: ./prompts/reviewer.md
```

```sh
cortex --config reviewer.yaml --prompt "review the last commit for correctness risks"
```

## Migration agent

```yaml
# migration.yaml
name: migration
provider: openrouter
model: deepseek/deepseek-v3.2
pruner:
  provider: openrouter
  model: qwen/qwen3.6-flash
system_prompt: ./prompts/migration.md
```

```sh
cortex --config migration.yaml
```

## Repository-local defaults

Commit only the non-secret product choices:

```yaml
# ./cortex.yaml
provider: openrouter
model: z-ai/glm-5.2
pruner:
  model: qwen/qwen3.6-flash
```

Credentials remain in Axon configuration outside the repository.

---

# Architecture

Cortex is intentionally small because most agent machinery lives in Axon.

```text
cmd/cortex/main.go
│
├── parse CLI mode
├── load Cortex product config
├── load Axon runtime settings
├── resolve main + pruner models
├── create trace sink
└── construct one Axon agent
         │
         ├── interactive ──→ internal/ui/Bridge ──→ Bubble Tea
         │
         └── --prompt ─────→ internal/ui/Plain ──→ stdout/stderr
```

Repository layout:

```text
cmd/cortex/
└── main.go                 flags, mode dispatch, dependency wiring

internal/config/
├── config.go               user + project + environment cascade
├── prompt.go               default coding-agent role
└── *_test.go

internal/ui/
├── run.go                  Bubble Tea startup
├── model.go                terminal state machine
├── view.go                 live input/status/spinner region
├── transcript.go           committed terminal transcript rendering
├── event.go                axon.Event → tea.Msg concurrency seam
├── command.go              slash-command table
├── plain.go                non-interactive renderer
├── tracelog.go             full JSONL runtime trace
└── theme.go                visual ownership in one place

benchmarks/
└── swe-rebench/            benchmark runner integration
```

## The most important dependency rule

Dependencies point one way.

```text
config ─────┐
            │
main ───────┼──→ axon
            │
ui ─────────┘
```

`internal/config` is a leaf and imports nothing from the rest of Cortex.

The terminal/runtime concurrency boundary is also intentionally narrow:

```text
Axon turn goroutine
       │
       │ axon.Event
       ▼
internal/ui/event.go
       │
       │ tea.Msg
       ▼
Bubble Tea goroutine
```

`internal/ui/event.go` is the seam. The rest of the UI does not reach into Axon while the runtime is working.

---

# Why Axon is a separate repository

It would be easy to hide all the runtime logic inside Cortex and call it a coding agent.

That would also make every future agent repeat the hardest part.

Axon extracts the reusable substrate:

```text
                          ┌─ Cortex coding agent
                          │
model + tools + runtime ──┼─ Kubernetes operator
                          │
                          ├─ SRE / incident agent
                          │
                          ├─ security remediation agent
                          │
                          └─ your specialized agent
```

Cortex is the reference proof that the boundary works in a real product: its `newAgent()` function mostly maps product choices into `axon.Config`.

```go
return axon.New(axon.Config{
    Model:        model,
    SystemPrompt: cfg.SystemPrompt,
    MCPServers:   servers,
    OnEvent:      onEvent,
    Settings:     settings,
    Pruner:       prunerModel,
})
```

The interactive UI and one-shot mode both use this same constructor.

If you are building a different kind of agent rather than a coding terminal, start with [Axon](https://github.com/atakang7/axon). If you want a repository-native coding product, use Cortex.

---

# Reliability inherited from Axon

Cortex sits on an LLM transport where failures are not just ordinary HTTP failures.

Axon handles runtime cases such as:

- truncated SSE streams
- provider idle stalls
- fragmented tool-call arguments
- retryable network failures
- policy-controlled HTTP retries
- token usage events
- malformed replies that leak tool markup as ordinary text
- background process cleanup on close/reset

Cortex benefits from those behaviors without reimplementing them in the terminal layer.

The default Axon retry policy is:

```yaml
retry:
  max_attempts: 10
  backoff_cap: 60s
  on_status: [429, 500, 502, 503, 504]
```

Transport failures such as truncated streams are handled separately from HTTP status policy.

---

# Development

Clone both projects only if you are actively developing the runtime and product together. Normal Cortex users do not need an Axon checkout; Go modules resolve it normally.

```sh
git clone https://github.com/atakang7/cortex.git
cd cortex

go build ./...
go vet ./...
go test ./...
```

The tests can drive the real agent loop without hitting a network because `axon.Model` is a small interface and scripted models can supply deterministic responses.

For UI changes, preserve the architecture boundary:

1. config resolution stays in `internal/config`
2. terminal state stays in `internal/ui`
3. Axon/Bubble Tea crossing stays in `internal/ui/event.go`
4. `cmd/cortex/main.go` remains wiring rather than business logic

---

# Releases

Releases are built for Linux and macOS on amd64 and arm64.

```sh
cortex --version
```

prints the version, commit, and build date injected by the release pipeline.

Latest release: [github.com/atakang7/cortex/releases/latest](https://github.com/atakang7/cortex/releases/latest)

---

# Project philosophy

Cortex is deliberately opinionated in a few places:

**The repository is the source of truth.**  
The model should inspect code rather than invent an internal representation of it.

**Verification is part of editing.**  
A plausible patch that was never built or tested is unfinished work.

**The terminal already solved terminal history.**  
Do not trap completed output inside a custom full-screen buffer when the user's terminal can scroll, search, select, and persist it better.

**Runtime machinery should be reusable.**  
Retries, context pressure, sessions, tools, streaming, and process lifecycle do not become more valuable when duplicated inside each agent product.

**Small boundaries age better than clever abstractions.**  
Cortex chooses a model, a role, a UI, and product behavior. Axon runs the agent. The operating system runs the work.

---

# Security model

Cortex is powerful by design. It can read and write files and execute shell commands inside the environment where you launch it.

Treat it like any other automation process with those capabilities:

- run it with the least privileges the task needs
- keep production credentials out of broad local environments
- use containers/VMs when you need stronger filesystem or network isolation
- scope MCP credentials to the actions they require
- review high-impact changes before applying them to production systems

Axon narrows what built-in tool functions receive internally, but neither Axon nor Cortex claims to be an operating-system sandbox.

---

<div align="center">

## Build with it. Break it. Make it better.

```sh
go install github.com/atakang7/cortex/cmd/cortex@latest
cortex
```

[Cortex releases](https://github.com/atakang7/cortex/releases) · [Axon runtime](https://github.com/atakang7/axon) · [Axon documentation](https://atakang7.github.io/axon/)

MIT License.

</div>

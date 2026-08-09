<div align="center">

# cortex

### A terminal coding agent that works the repository until the change is proven.

Built in Go · powered by [Axon](https://github.com/atakang7/axon)

[![Release](https://img.shields.io/github/v/release/atakang7/cortex?style=flat-square)](https://github.com/atakang7/cortex/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.26.2-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![Runtime](https://img.shields.io/badge/runtime-Axon-6366f1?style=flat-square)](https://github.com/atakang7/axon)
[![License](https://img.shields.io/github/license/atakang7/cortex?style=flat-square)](LICENSE)

**Search → read → edit → verify → keep going if the evidence says you're wrong.**

[Quickstart](https://atakang7.github.io/cortex/) · [Releases](https://github.com/atakang7/cortex/releases/latest) · [Axon runtime](https://github.com/atakang7/axon)

</div>

<p align="center">
  <img src="assets/cortex-live.gif" alt="The real Cortex binary running in a terminal, searching, reading, editing, and testing a disposable Go repository" width="100%" />
</p>

<p align="center"><sub><b>Real Cortex binary. Real TUI. Real filesystem tools. Real <code>go test ./...</code>.</b> Only the local model endpoint is scripted so this recording is deterministic and reproducible.</sub></p>

Give Cortex a repository task and it works against the repository itself. It searches for evidence before loading files, reads the part it needs, makes a targeted edit, runs the command that can prove or disprove the change, and feeds that result back into the same turn.

```sh
go install github.com/atakang7/cortex/cmd/cortex@latest
cd your-repository
cortex
```

Setup lives in the **[Lightning Quickstart](https://atakang7.github.io/cortex/)**, not in this README.

---

## Why the loop matters

<p align="center">
  <img src="assets/cortex-proof.svg" alt="Concrete Cortex runtime behavior: continuing after tool results, reversible writes, managed background processes, and terminal-native output" width="100%" />
</p>

### It spends context on evidence, not on dumping the repository

Cortex has a repository search tool backed by ripgrep and slice-based file reads. The normal path is:

```text
find the symbol → read the relevant file/range → change it
```

not:

```text
load half the repository → hope the model notices the right thing
```

Full reads are available when they are actually useful, but they are capped. Large files can be paged instead of silently consuming the turn.

### Verification is part of the same turn

A tool call is not where Cortex stops. Axon sends the result back to the model and continues the same turn.

```text
model → search → evidence
  ↑                 ↓
  └──── model ←─────┘
          ↓
        read
          ↓
        edit
          ↓
        test
          ↓
   pass? finish
   fail? continue
```

This is the important behavior behind “agentic”: **the test result can change what the model does next.** A failed build is not a footnote in the final answer; it is new input to the loop.

### Edits are narrow and mechanically reversible

The write tool supports exact-string replacement, line-range replacement, insertion, and full save. Exact replacement refuses ambiguous matches instead of guessing which occurrence the model meant.

Every managed write records the previous bytes and uses an atomic temp-file + rename path.

```text
write → record previous bytes → atomic replace
                         │
                         └── /undo → restore previous bytes
```

`/undo` is not another LLM request. Cortex does not need the model to remember what it changed.

### Servers and watchers do not hijack the turn

Commands that may wait can be started as managed background processes.

```text
exec(background=true) → shell_id
                         │
                         ├── keep editing
                         ├── bash_output → only new log bytes
                         └── kill_shell  → stop the process group
```

That makes the difference on normal engineering tasks: start a dev server, inspect its output, edit while it stays alive, check only the new logs, then clean it up.

### The terminal remains a terminal

Cortex redraws only what is still live: the current tool, streamed text, composer, and status line.

Completed work is committed to ordinary terminal scrollback. Your terminal still owns scrolling, search, selection, copy, and history after Cortex exits.

There is no giant alternate-screen transcript pretending to be a terminal inside your terminal.

### Interactive and automation mode are the same agent

```sh
cortex
```

and

```sh
cortex --prompt "find the regression introduced in the last five commits"
```

construct the same Axon agent. One-shot mode changes the renderer, not the execution machinery.

```text
stdout  final answer
stderr  tools · notices · errors · trace path
```

So the same agent can be used by a human, a shell script, CI, or a benchmark harness without maintaining a weaker automation path.

---

## Cortex's default behavior is intentionally opinionated

The shipped coding role is short on ceremony and strict about the loop:

| Rule | Why it matters |
| --- | --- |
| **Search before guessing** | Find the implementation instead of inventing paths. |
| **Read before writing** | Edit code the model has actually inspected. |
| **Make the smallest useful change** | Do not turn a bug fix into an unsolicited refactor. |
| **Run the command that can falsify the change** | Confidence is not verification. |
| **Use failures as evidence** | A red test should drive the next action, not be hidden in prose. |
| **Stop when the requested state is true** | Do not burn turns polishing unrelated code. |

That policy is the real default prompt in [`internal/config/prompt.go`](internal/config/prompt.go). Replace it and Cortex can become a reviewer, migration agent, repository archaeologist, or another code-specialized role while keeping the same runtime underneath.

---

## Small surface, full runtime underneath

Cortex owns the coding product:

```text
prompt · model/pruner choice · terminal UI · slash commands · project config
```

[Axon](https://github.com/atakang7/axon) owns the reusable agent machinery:

```text
turn continuation · tools · sessions · context projection · streaming · retries
background processes · MCP · events · usage
```

The boundary is deliberately small:

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

That is why Cortex can stay focused on coding behavior instead of growing its own second runtime inside the TUI.

---

## Useful controls, nothing more

```text
/model          switch the active model without throwing away the session
/pruner         switch or disable the secondary context model
/undo           restore the last managed edit byte-for-byte
/session        show turn, message, edit, and session state
/cd <path>      move the agent to another working directory
/new            clear the session
/help           show the rest
```

The status line keeps the state that matters visible: model, pruner, working directory, turn, elapsed time while busy, and the next useful key.

Every run also writes a JSONL trace, so when a turn behaves strangely you can inspect the event sequence instead of reverse-engineering it from the final prose.

---

## Run it

```sh
go install github.com/atakang7/cortex/cmd/cortex@latest
```

Prebuilt Linux and macOS binaries for amd64/arm64 are available in [GitHub Releases](https://github.com/atakang7/cortex/releases/latest).

Then use the **[Lightning Quickstart](https://atakang7.github.io/cortex/)** for provider + model setup and run:

```sh
cortex
```

---

<div align="center">

Built with [Axon](https://github.com/atakang7/axon) · MIT

</div>

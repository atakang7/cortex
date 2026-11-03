# Cortex launch operating plan

Cortex should not be marketed as "another terminal coding agent." That market is crowded.

The public story should be:

> **Cortex is the coding agent you can inspect and measure: reproducible runs, wire-level traces, explicit context pruning, and published token/cost behavior.**

The goal is not to manufacture stars. The goal is to produce evidence developers want to discuss, then make trying Cortex frictionless.

## 1. Launch thesis

### Category
Open-source terminal coding agent / coding-agent harness.

### Differentiator
Inspectable + measurable efficiency.

Cortex already has unusual raw material for this positioning:
- provider-reported token usage per call
- full wire-level request/reply traces
- reproducible seed projects + verifiers
- secondary-model context pruning
- stdout/stderr separation for scripting
- MCP support
- append-only sessions and byte-for-byte undo

Do not lead with the feature list. Lead with a result or experiment.

### Headline hierarchy

Before benchmark results:

> **An inspectable coding agent for people who want to know where their tokens go.**

Supporting line:

> Reproducible coding tasks, wire-level traces, optional cheap-model context pruning, MCP tools, and a terminal-native workflow.

After a defensible benchmark win, replace the headline with the strongest measured result, e.g.:

> **Cortex used X% fewer input tokens than Y on the same successful coding tasks.**

Never publish a percentage until the raw runs support it.

## 2. The launch asset

The main launch asset is not a generic demo video. It is one reproducible experiment.

Create a public benchmark page containing, in this order:

1. One-sentence result
2. One chart: successful task vs total input tokens/cost
3. Three task descriptions
4. Same-model methodology
5. Raw artifacts
6. Exact reproduction commands
7. Failure cases / limitations
8. Install command

The benchmark issue is #15.

## 3. README conversion

A stranger should understand the product before scrolling.

Target first screen:

```md
# cortex

An inspectable coding agent for people who want to know where their tokens go.

[demo GIF / benchmark chart]

`curl ... | tar ...`

**Why Cortex?**
- See the exact requests sent to the model.
- Measure token spend per call and per task.
- Reproduce agent runs from clean seed repos.
- Use a cheap secondary model to prune stale context.

[Benchmark] · [60-second install] · [How traces work]
```

Move the documentation map and deep architecture below the product proof. The current long-form documentation is useful after someone already cares.

## 4. Distribution sequence

Do not launch everywhere simultaneously. Use each audience to improve the next launch.

### Wave 0 — five real testers

Before broad launch, get 5 developers who did not build Cortex to run one task with it.

Ask only:
- Did installation work?
- What did you think Cortex was before running it?
- Why would you use this instead of your current agent?
- Where did you get stuck?

Fix onboarding friction before chasing traffic.

### Wave 1 — LocalLLaMA

This is a strong initial fit because the community actively compares open coding harnesses, local-model support and token/context behavior.

Do not title the post "I built a coding agent." Use the experiment.

Candidate title after results:

> I benchmarked 3 open coding agents on the same model/tasks — here are the raw token traces

Post structure:
- one chart
- 3-sentence methodology
- surprising finding
- Cortex explained only after the finding
- disclose clearly that Cortex is your project
- raw data + GitHub link
- ask what should be added to the benchmark

### Wave 2 — Hacker News / Show HN

HN needs a sharp distinction. Generic terminal-agent launches commonly get the immediate question: "How is this different from OpenCode/Aider/etc.?"

Candidate title:

> Show HN: Cortex – an open-source coding agent with reproducible runs and wire-level traces

First paragraph must answer:
- why you built it
- what is different
- what can be independently verified

Stay available to answer every substantive launch-thread question.

### Wave 3 — programming / Go communities

Pitch the engineering system, not AI hype:
- Bubble Tea terminal architecture
- event boundary between runtime and UI
- append-only sessions
- exact undo
- trace/logging design
- benchmark harness

A post titled "what I learned instrumenting every model call in a Go coding agent" is stronger than "check out my AI project."

### Wave 4 — LinkedIn / X

Use these primarily for hiring signal and repeated exposure.

Good post pattern:
1. concrete engineering problem
2. surprising measurement
3. how Cortex exposed it
4. tiny visual
5. repo link

Example topic already present in Cortex: a duplicated tool catalog cost roughly 2,100 tokens per model call and was visible in the wire trace. Turn discoveries like this into standalone engineering posts.

## 5. Content engine

One benchmark should generate many independent technical stories.

Potential posts:
- "We were accidentally paying ~2,100 duplicate tokens per coding-agent call"
- "Why coding-agent event logs are not enough for debugging"
- "What happens when a context pruner fires with nothing useful to prune"
- "Schema wording beat prompt wording in our agent tool behavior"
- "How to build a reproducible coding-agent benchmark without cherry-picking runs"
- "Why Cortex writes answers to stdout and agent activity to stderr"

Each post should teach something even if the reader never installs Cortex.

## 6. GitHub discovery

Repository settings to add manually if the API surface is unavailable:

Topics:
`coding-agent`, `ai-agent`, `developer-tools`, `terminal`, `cli`, `golang`, `llm`, `mcp`, `openrouter`, `ollama`, `agentic-coding`, `bubbletea`

Also:
- enable Discussions once external users arrive
- create issue templates for bug / feature / benchmark result
- label several genuinely approachable issues `good first issue`
- keep releases easy to install on Linux/macOS

## 7. Conversion goals

Stars are a lagging indicator. Track this funnel weekly:

| Metric | Meaning |
|---|---|
| Repo visitors | distribution worked |
| README → install/docs clicks | positioning worked |
| Successful first runs | onboarding worked |
| Repeat users | product worked |
| Issues/discussions from strangers | users care enough to invest effort |
| External PRs | community has started |
| Stars/forks | social proof |

The first meaningful milestone is not 1,000 stars. It is **10 strangers who successfully used Cortex and 3 who used it again**.

## 8. Rules

- Never buy stars or fake engagement.
- Never mass-spam maintainers or communities.
- Never claim benchmark superiority without raw reproducible evidence.
- Do not hide failed runs.
- Always disclose that Cortex is your project when posting it.
- Prefer useful technical posts over repeated project promotion.
- A post that teaches and gets 20 relevant developers is worth more than 20,000 generic impressions.

## 9. Immediate queue

1. Finish benchmark issue #15.
2. Capture one clean terminal GIF showing install → task → trace usage.
3. Rewrite the README first screen around the benchmark/inspectability thesis.
4. Recruit five independent testers.
5. Launch the benchmark discussion in LocalLLaMA.
6. Fix friction / answer feedback.
7. Publish Show HN.
8. Turn benchmark discoveries into 4–6 engineering posts over the following weeks.
9. Track first-run and repeat-user evidence.
10. Reposition if users consistently cite a different reason for choosing Cortex.

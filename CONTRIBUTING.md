# Contributing

Thanks for considering a contribution to cortex.

## Getting started

### Prerequisites

- Go 1.26.2 or later
- Git

### Setup

```sh
git clone https://github.com/atakang7/cortex
cd cortex
go build ./...
```

The repo currently ships without tests. New behavior changes should land with tests.

## Philosophy

cortex is intentionally thin. The runtime ([axon](https://github.com/atakang7/axon)) does the heavy lifting; cortex supplies a coding-agent personality and a terminal wrapper. Before adding anything, ask:

- Does this belong in the runtime instead?
- Can the user (or the agent) do it themselves?
- Does it serve the "terminal coding agent" core experience?

If the answer to the first is "yes," open a PR against axon, not cortex.

## Commit messages

cortex uses [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/). The release pipeline derives the next semver from these prefixes; commitlint enforces the format on every pull request.

**Format**

```
<type>(<optional scope>): <short imperative subject>

<optional body>

<optional footer(s), including BREAKING CHANGE>
```

**Types and release impact**

| Type       | Purpose                                     | Release bump |
| ---------- | ------------------------------------------- | ------------ |
| `feat`     | New user-facing feature                     | **minor**    |
| `fix`      | Bug fix                                     | **patch**    |
| `perf`     | Performance improvement, no behavior change | **patch**    |
| `refactor` | Internal restructure, no behavior change    | **patch**    |
| `docs`     | Documentation only                          | **patch**    |
| `build`    | Build system, `go.mod`, release tooling     | **patch**    |
| `test`     | Adding or fixing tests                      | none         |
| `ci`       | CI configuration                            | none         |
| `chore`    | Maintenance not covered above               | none         |
| `style`    | Formatting, whitespace                      | none         |
| `revert`   | Reverts a previous commit                   | varies       |

Breaking changes force a **major** bump regardless of type. Mark with `!` after the type or with a `BREAKING CHANGE:` footer:

```
feat!: drop LLM_LEGACY_KEY env var

BREAKING CHANGE: cortex now requires LLM_API_KEY. Set it before upgrading.
```

Subjects are lowercase, imperative, ≤ 100 chars.

## Releases

Fully automated. Every push to `main` runs [semantic-release](https://semantic-release.gitbook.io/) which computes the next semver, updates the GitHub Release, tags it, and triggers [goreleaser](https://goreleaser.com/) to cross-compile `cmd/cortex` for linux/darwin × amd64/arm64.

No manual `git tag` step. Merge a `feat:` commit to ship a feature; merge a `fix:` to ship a fix. Use `chore:` / `ci:` / `test:` / `style:` for commits that should not release.

Configuration:

- `.releaserc.json` — semantic-release rules
- `.goreleaser.yaml` — binary build matrix
- `.commitlintrc.json` — accepted commit types
- `.github/workflows/release.yml` — pipeline

## Questions?

- Open an issue.
- Keep discussions focused.

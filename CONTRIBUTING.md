# Contributing

Cortex uses Go 1.26.2 and the [Axon runtime](https://github.com/atakang7/axon).

## Build and test

```sh
go build ./...
go vet ./...
go test ./...
```

The docs site uses Node 24.

```sh
cd docs
npm ci
npm run build
```

## Changes

The CLI is in [cmd/cortex](cmd/cortex/README.md), config resolution in [internal/config](internal/config/README.md), and terminal rendering in [internal/ui](internal/ui/README.md).

Add a regression test for changed behavior. Keep runtime changes in Axon if they apply to other Axon clients.

## Commits and releases

Use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) with lowercase imperative subjects.

`feat` publishes a minor release; `fix`, `docs`, `perf`, `refactor`, and `build` publish patches. `ci`, `test`, `chore`, and `style` do not publish releases.

The release workflow runs semantic-release on `main` and GoReleaser builds Linux, macOS, and Windows binaries.

## Review

Open a pull request with the affected paths and the commands used to verify the change.

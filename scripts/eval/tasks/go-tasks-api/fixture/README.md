# tasks

A small JSON API for a to-do list, backed by an in-memory store.

    go run ./cmd/server -addr :8080

Tests: `go test ./...`

## Layout

- `cmd/server`: the binary; flags and wiring.
- `internal/store`: the `Store` interface and its in-memory implementation.
- `internal/api`: HTTP handlers, request validation and JSON encoding.

## API

All bodies are JSON. Errors are `{"error": "<message>"}` with a 4xx or 5xx
status.

| Method | Path | Body | Response |
|---|---|---|---|
| GET | /tasks | | `200` array of tasks, in id order |
| POST | /tasks | `{"title": "...", "tags": ["..."]}` | `201` the task |
| GET | /tasks/{id} | | `200` the task, `404` if none |
| PATCH | /tasks/{id} | `{"title"?: "...", "done"?: true, "tags"?: [...]}` | `200` the task |
| DELETE | /tasks/{id} | | `204` |

A task is `{"id": 1, "title": "...", "done": false, "tags": [], "created": "RFC 3339 time"}`.
Titles are 1 to 200 characters.

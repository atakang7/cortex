Part of [cortex](../README.md).

# scripts

Release installer, benchmark tools, and local build helpers.

## Run

```sh
sh scripts/install.sh
```

## Test

```sh
sh -n scripts/install.sh
```

## Config

| var | default | what |
| --- | --- | --- |
| `CORTEX_INSTALL_DIR` | `$HOME/.local/bin` | Destination for the downloaded executable |

## Interfaces

| path | reads or writes |
| --- | --- |
| `install.sh` | Downloads the latest GitHub release for Linux or macOS |
| `eval/` | Runs test fixtures; see [eval](eval/README.md) |
| `record_readme_demo.py` | Records terminal demo assets |

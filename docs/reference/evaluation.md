# Evaluation reference

`scripts/eval/run.sh` takes `CORTEX_BIN TASK MODEL OUT_DIR [LABEL]` and prints one JSON object.

## Task files

| path | purpose |
| --- | --- |
| `tasks/<task>/prompt.md` | Prompt passed to `cortex -prompt` |
| `tasks/<task>/fixture/` | Repository copied into a new git worktree |
| `tasks/<task>/check.sh` | Oracle called with the run repo and original fixture |

## Run files

| path | content |
| --- | --- |
| `<out>/<task>-<label>-<time>/repo/` | Modified fixture |
| `<out>/<task>-<label>-<time>/data/` | Axon sessions |
| `<out>/<task>-<label>-<time>/transcript.txt` | Cortex stdout and stderr |
| `<out>/<task>-<label>-<time>/seconds` | Elapsed wall time |

## Metrics

The JSON contains `task`, `label`, `verdict`, `exit`, `seconds`, `requests`, `replies`, `tool_calls`, `calls_per_reply`, `tools`, `prompt_tokens`, `completion_tokens`, `cache_hit`, and `cost`.

`grade.sh OUT_DIR` re-runs each fixture oracle against saved run directories.

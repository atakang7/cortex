Part of [cortex](../../README.md).

# scripts/eval

Runs coding tasks against fresh fixture repositories and grades results with task oracles.

## Run

```sh
scripts/eval/run.sh "$PWD/cortex" go-pricing "<openrouter-model>" "$PWD/eval-runs"
scripts/eval/grade.sh "$PWD/eval-runs"
```

Build `cortex` first; choose a configured OpenRouter model.

## Test

```sh
bash -n scripts/eval/run.sh scripts/eval/grade.sh
```

## Config

| var | default | what |
| --- | --- | --- |
| `EVAL_AXON_YAML` | unset | Extra Axon YAML applied to a run |
| `AXON_DATA_DIR` | per-run directory | Session storage set by `run.sh` |

## Interfaces

`run.sh` takes `CORTEX_BIN TASK MODEL OUT_DIR [LABEL]` and prints one JSON metrics line.

Each task reads `prompt.md`, copies `fixture/`, and runs `check.sh`. Output includes a session directory, `transcript.txt`, and `seconds`.

See [evaluation reference](../../docs/reference/evaluation.md) for the task contract.

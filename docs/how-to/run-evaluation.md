# Run an evaluation task

Build Cortex and configure an Axon provider before running a fixture.

```sh
go build -o cortex ./cmd/cortex
mkdir -p eval-runs
scripts/eval/run.sh "$PWD/cortex" go-pricing "<openrouter-model>" "$PWD/eval-runs" baseline
scripts/eval/grade.sh "$PWD/eval-runs"
```

Choose a model in place of `<openrouter-model>`. The runner prints a JSON metrics line and stores the run under `eval-runs/`.

See the [task contract](../reference/evaluation.md) for fixture and output paths.

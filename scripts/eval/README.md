# Eval

Fixed tasks with oracles, so a change to the harness or the prompt is judged
by what it does to real work: did the task pass, how long did it take, how many
requests, how much of the prompt hit the cache, what did it cost.

    scripts/eval/run.sh path/to/cortex TASK MODEL OUT_DIR [LABEL]

prints one JSON line. Each task is a directory under `tasks/`:

- `prompt.md`: the user message, sent once with `-prompt`.
- `fixture/`: the repository, copied into a fresh git repo for every run.
- `check.sh REPO FIXTURE`: the oracle; prints PASS or FAIL with the reason.

Compare two builds by running both on the same tasks and model. Put the
toolchains the tasks need (Go, Python) on PATH. On Windows keep OUT_DIR short
(git fails on long paths).

Add a task for every turn that went wrong in a real session: the fixture is
the smallest repository that reproduces it, the oracle what the user wanted.

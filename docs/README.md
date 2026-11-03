Part of [cortex](../README.md).

# docs

The Astro site lives in `src/`. The Markdown pages are grouped by task, fact, and design rationale.

## How-to

- [Run an evaluation](how-to/run-evaluation.md)
- [Use ACP with Setpoint](how-to/use-acp-with-setpoint.md)

## Reference

- [Configuration](reference/configuration.md)
- [ACP methods](reference/acp.md)
- [Evaluation fixtures](reference/evaluation.md)

## Explanation

- [Architecture](explanation/architecture.md)

## Site

```sh
cd docs
npm ci
npm run build
```

The website entry point is [src/pages/docs/index.astro](src/pages/docs/index.astro).

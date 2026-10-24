# Free consensus pruner

Cortex supports `pruner.models` with two or three distinct OpenRouter `:free` IDs.
This enables Axon's semantic pruning mode; existing `pruner.model` configurations
continue using a single model. Explicitly selecting a single model in the UI clears
the saved pool. All pool models must be listed in the Axon endpoint configuration.

```yaml
# cortex config.yaml
pruner:
  provider: openrouter
  models:
    - nvidia/nemotron-3.5-super-vl:free
    - dots-studio/dots-3-note-preview:free
```

```yaml
# Merge into axon.yaml (preserve your endpoint credentials and other models)
providers:
  openrouter:
    base_url: https://openrouter.ai/api/v1
    api_key: ${OPENROUTER_API_KEY}
    models:
      nvidia/nemotron-3.5-super-vl:free: {}
      dots-studio/dots-3-note-preview:free: {}
pruner:
  semantic: true
  mode: extreme
  window_blocks: 6
  floor_tokens: 6000
  growth_tokens: 3000
  max_tokens: 8192
  timeout: 45s
```

Every pool client requires the HTTPS OpenRouter host and applies zero maximum prompt,
completion and request prices. Required parameter support is enforced. There is no
fallback to the paid main model. An unavailable reviewer skips cleanup; content is
retained. Each reviewer has a separate trace label so its prompt, decision, timing
and errors can be inspected. Models are configurable because free availability changes.

The supplied IDs were listed as free in the OpenRouter catalog on 2026-10-24 and
both returned JSON in a live synthetic connection check with reported cost zero.
This is not a benchmark or a claim of proven quality on arbitrary coding tasks.

Build with the companion modified Axon source through a Go workspace until the
library changes have been released. The local package also preserves the existing
VS Code `--stdio` bridge. See Axon's FREE_PRUNER.md for retention and failure semantics.

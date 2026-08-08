## [2.1.0](https://github.com/atakang7/cortex/compare/v2.0.4...v2.1.0) (2026-08-08)


### Features

* adopt native axon MCP integration ([ff6339f](https://github.com/atakang7/cortex/commit/ff6339feb08f38a272fb242d58e0e1cd87824cdb))

## [2.0.4](https://github.com/atakang7/cortex/compare/v2.0.3...v2.0.4) (2026-08-08)


### Refactoring

* update to use flattened axon package ([47df729](https://github.com/atakang7/cortex/commit/47df7298fffa5dd2ae27a2576a9901da4a5a34e6))

## [2.0.3](https://github.com/atakang7/cortex/compare/v2.0.2...v2.0.3) (2026-08-08)


### Bug Fixes

* update NewPruner usage to use PrunerConfig ([abd3ee4](https://github.com/atakang7/cortex/commit/abd3ee4e3727cf2dd06cc4eddbcc13884dd7ebc0))

## [2.0.2](https://github.com/atakang7/cortex/compare/v2.0.1...v2.0.2) (2026-08-07)


### Bug Fixes

* **ui:** suppress all pruner errors gracefully ([789d531](https://github.com/atakang7/cortex/commit/789d5313bc6c73f4f581fef93c571e7ae988aa52))

## [2.0.1](https://github.com/atakang7/cortex/compare/v2.0.0...v2.0.1) (2026-08-07)


### Bug Fixes

* **ui:** add spinner to pruner and prevent pruner context deadline timeouts ([7254970](https://github.com/atakang7/cortex/commit/725497083490a9ab9a24f8c0645e36ac9e7855b4))

## [2.0.0](https://github.com/atakang7/synapse/compare/v1.2.0...v2.0.0) (2026-07-17)


### ⚠ BREAKING CHANGES

* module path is now github.com/atakang7/cortex, the
binary is cortex, the agents dir is ~/.config/cortex/agents, and the
SYNAPSE_AGENTS_DIR env var is now CORTEX_AGENTS_DIR.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>

### Features

* rename project from synapse to cortex ([d62f4b4](https://github.com/atakang7/synapse/commit/d62f4b4397969b379831394425db110575d37028))

## [1.2.0](https://github.com/atakang7/synapse/compare/v1.1.0...v1.2.0) (2026-06-07)


### Features

* rename project from bouton to synapse ([f04fb27](https://github.com/atakang7/synapse/commit/f04fb2712e850f2bf4d940e102a533dd0860e084))


### Bug Fixes

* **cli:** print assistant reply in non-interactive mode ([bec9b86](https://github.com/atakang7/synapse/commit/bec9b860f13513a4e6404e1fd84eac3eafeafaba))

## [1.1.0](https://github.com/atakang7/synapse/compare/v1.0.0...v1.1.0) (2026-05-19)

### Features

- import full coding-agent CLI from axon ([7e79bea](https://github.com/atakang7/synapse/commit/7e79beae859fef9c293ea6f5eb9a873209765d24))

## 1.0.0 (2026-05-19)

### Features

- scaffold synapse coding agent on axon runtime ([99b2bad](https://github.com/atakang7/synapse/commit/99b2bad34d05b1a93721245f87f970c523049f83))

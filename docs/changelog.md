# Changelog

All notable changes to `gha-local-lint` are documented in this file.

---

## [0.5.0](https://github.com/smford/gha-local-lint/releases/tag/v0.5.0) (2026-10-22)

### Features
* Expand `--fix` to auto-remediate missing job timeouts, least-privilege permissions, and self-referencing job outputs.
* Interactive documentation engine powered by [`md-web-renderer`](https://github.com/smford/md-web-renderer).
* Enhanced Homebrew formula generation and automatic tap updates.

---

## [0.4.0](https://github.com/smford/gha-local-lint/releases/tag/v0.4.0) (2026-10-22)

### Features
* Interactive GitHub Actions Expression REPL (`repl` command).
* Dependency DAG graph visualization in Mermaid and Graphviz formats (`graph` command).
* Static shell syntax linting for multi-line bash `run:` steps.
* Hardcoded secrets scanner for API keys and tokens.
* Runner cost and compute budget estimator (`cost` command).
* Standalone HTML audit report generator (`report` command).

---

## [0.3.0](https://github.com/smford/gha-local-lint/releases/tag/v0.3.0) (2026-10-22)

### Features
* Complete offline expression parser and evaluator for core functions (`contains`, `startsWith`, `endsWith`, `format`, `join`, `fromJSON`).
* Strategy matrix expander with `include` and `exclude` resolution.
* Dry-run trigger simulation for push and pull-request events.
* Official GitHub Action composite step (`action.yml`).

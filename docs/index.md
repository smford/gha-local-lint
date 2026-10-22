# gha-local-lint

> **Offline GitHub Actions Expression Evaluator, Matrix Expander, & Pipeline Simulator**

Catch broken conditions, script injections, runner cost blowups, and DAG cycles before `git push`. Zero external network calls. Zero cloud dependencies.

[![Release](https://img.shields.io/github/v/release/smford/gha-local-lint?color=blue&logo=github)](https://github.com/smford/gha-local-lint/releases)
[![CI](https://github.com/smford/gha-local-lint/actions/workflows/ci.yml/badge.svg)](https://github.com/smford/gha-local-lint/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/smford/gha-local-lint)](https://goreportcard.com/report/github.com/smford/gha-local-lint)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

---

## Why gha-local-lint?

Engineers regularly burn dozens of `"fix ci"`, `"fix ci 2"`, `"debug matrix"` commits pushing to GitHub to see if complex expressions or DAG dependencies resolve.

`gha-local-lint` shifts debugging completely left into your local development loop:

* **AST-Level GitHub Actions Expression Engine:** Parses and evaluates functions (`contains()`, `startsWith()`, `format()`, `fromJSON()`, status functions) against simulated runner contexts.
* **Matrix Dimension Expansion:** Computes full cartesian matrix products, evaluating dynamically included or excluded variables with 100% parity with GitHub Actions runners.
* **Deterministic Auto-Remediation (`--fix`):** Safely injects missing `timeout-minutes`, least-privilege `permissions:`, extracts script injections to safe environment variables, pins actions to immutable 40-character commit SHAs, and fixes invalid self-referencing job outputs.
* **Zero-CGO & Instant:** A single static binary with no Python or Docker daemon prerequisites.

---

## Quick Start

### Homebrew (macOS & Linux)

```bash
brew tap smford/tap
brew install gha-local-lint
```

### Run Static Linting & Auto-Fixes

```bash
# Check all workflows in .github/workflows/
gha-local-lint check

# Automatically remediate timeouts, permissions, and injections
gha-local-lint check --fix
```

### Dry-Run Workflow Simulation

```bash
# Simulate push event on main
gha-local-lint simulate .github/workflows/ci.yml --event push --ref refs/heads/main

# Inspect matrix combinations
gha-local-lint matrix .github/workflows/ci.yml
```

---

## Documentation Sections

* [Installation Guide](installation.md) — Homebrew, Go install, Docker, and Pre-commit.
* [CLI Commands Reference](commands.md) — Comprehensive guide to all subcommands and flags.
* [Rules & Auto-Fix Matrix](rules.md) — Full rule catalog and remediation guarantees.
* [Changelog](changelog.md) — Release notes and version history.

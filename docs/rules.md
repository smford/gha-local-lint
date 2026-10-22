# Rules & Automated Remediations

`gha-local-lint` is built around a deterministic, conservative remediation philosophy: automated fixes (`--fix`) only apply when the safe resolution is structurally obvious and maintains pipeline intent.

---

## Remediation & Rule Matrix

| Rule ID | Name | Severity | Auto-Fixable? | Remediation Applied |
| :--- | :--- | :--- | :--- | :--- |
| `GHA001` | **Missing Job Timeout** | Warning | **Yes** | Injects `timeout-minutes: 60` into jobs missing explicit limits. |
| `GHA002` | **Unpinned Action** | Warning | **Yes** | Replaces mutable tags (`@v4`, `@main`) on known official actions with verified 40-char commit SHAs. |
| `GHA003` | **Missing Permissions** | Warning | **Yes** | Injects least-privilege `permissions: contents: read` block at top of workflow. |
| `GHA004` | **Script Injection Risk** | Error | **Yes** | Extracts untrusted context expressions (`github.event.pull_request.title`) into intermediate `env:` variables. |
| `GHA005` | **Deprecated Runner** | Warning / Error | **Yes** | Upgrades retired and deprecated runners (e.g., `ubuntu-18.04` to `ubuntu-22.04`). |
| `GHA006` | **Self-Referencing Output** | Error | **Yes** | Rewrites invalid `needs.<current_job>.outputs` references to `steps.<step_id>.outputs`. |
| `GHA007` | **DAG Circular Dependency** | Error | *No* | Pipeline architecture issue requiring manual dependency graph adjustment. |
| `GHA008` | **Missing `needs:` Target** | Error | *No* | Identifies typos or missing upstream jobs referenced in `needs:`. |
| `GHA009` | **Hardcoded Secrets** | Error | *No* | Flags exposed API keys, tokens, and private keys for migration to GitHub Secrets. |
| `GHA010` | **Shell Syntax Validation** | Error | *No* | Flags shell script syntax errors inside multi-line `run:` blocks. |

---

## Detailed Rule Explanations

### GHA001: Missing Job Timeout
* **Problem**: GitHub Actions jobs default to a 6-hour (360-minute) maximum execution limit. Hanging tests or locked network requests can quietly consume all monthly runner minutes.
* **Auto-Fix**: Adds `timeout-minutes: 60` directly under the job definition with proper indentation.

### GHA002: Action Pinning
* **Problem**: Mutable tags like `@v4` or `@main` can be deleted, force-pushed, or hijacked if upstream accounts are compromised.
* **Auto-Fix**: Converts tag references for common actions (`actions/checkout`, `actions/setup-go`, `actions/setup-python`, etc.) to immutable 40-character commit SHAs while preserving the original version in a comment (`# v4.2.2`).

### GHA003: Least-Privilege Permissions
* **Problem**: By default, GitHub Actions `GITHUB_TOKEN` may receive permissive write access across contents, packages, issues, and deployments.
* **Auto-Fix**: Inserts a top-level `permissions:` block with `contents: read` to enforce a zero-trust default posture.

### GHA004: Script Injection Defense
* **Problem**: Directly embedding `${{ github.event.issue.title }}` inside inline shell scripts allows attackers to escape bash strings and execute arbitrary commands in runner environments.
* **Auto-Fix**: Replaces inline expression interpolations with an intermediate environment variable under `env:` (e.g. `PR_TITLE: ${{ github.event.pull_request.title }}`) and rewrites the command to use `"$PR_TITLE"`.

### GHA005: Deprecated & Retired Runners
* **Problem**: GitHub regularly decommissions older virtual runner environments (such as `ubuntu-18.04`, `ubuntu-20.04`, `macos-11`). Workflows targeting retired runners will fail immediately upon dispatch.
* **Auto-Fix**: Replaces obsolete runner strings with current supported LTS versions.

### GHA006: Self-Referencing Job Outputs
* **Problem**: Workflows attempting to read their own job outputs via `needs.<current_job>.outputs` fail because `needs` only contains upstream predecessor jobs.
* **Auto-Fix**: Detects the originating step and rewrites the expression to `steps.<step_id>.outputs.<output_name>`.

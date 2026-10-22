# Installation

`gha-local-lint` is distributed as a zero-CGO, statically linked binary for macOS, Linux, and Windows.

---

## 1. Homebrew (macOS & Linux)

The recommended install method for macOS and Linux users:

```bash
# Add official tap and install
brew tap smford/tap
brew install gha-local-lint

# Or one-line install
brew install smford/tap/gha-local-lint
```

To upgrade to the latest version:

```bash
brew update
brew upgrade gha-local-lint
```

---

## 2. Go Install (Go 1.23+)

Install directly via `go install`:

```bash
go install github.com/smford/gha-local-lint/cmd/gha-local-lint@latest
```

Ensure `$(go env GOPATH)/bin` is in your `$PATH`.

---

## 3. Pre-Commit Hook (Shift-Left)

Integrate `gha-local-lint` into your pre-commit pipeline to validate workflows prior to every `git commit`.

Add this to your `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: https://github.com/smford/gha-local-lint
    rev: v0.5.0
    hooks:
      - id: gha-local-lint
```

---

## 4. GitHub Actions CI Step

You can invoke `gha-local-lint` directly inside your workflows using the composite action:

```yaml
- name: Lint Workflow Files
  uses: smford/gha-local-lint@v0.5.0
  with:
    require-sha-pin: true
    format: text
```

---

## 5. Docker Container

Run directly via Docker:

```bash
# Pull and run against current repository
docker run --rm -v "$(pwd):/work" -w /work ghcr.io/smford/gha-local-lint:latest check
```

---

## 6. Build from Source

```bash
git clone https://github.com/smford/gha-local-lint.git
cd gha-local-lint

# Build standalone binary with Make
make build

# Output binary will be in bin/gha-local-lint
./bin/gha-local-lint version
```

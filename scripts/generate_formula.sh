#!/usr/bin/env bash
set -euo pipefail

# Generates Homebrew formula for gha-local-lint from GoReleaser artifacts
TAG="${1:-}"
DIST_DIR="${2:-dist}"
OUTPUT_FILE="${3:-Formula/gha-local-lint.rb}"

if [[ -z "$TAG" ]]; then
  echo "Usage: $0 <tag> [dist_dir] [output_file]" >&2
  exit 1
fi

VERSION="${TAG#v}"
CHECKSUMS="${DIST_DIR}/checksums.txt"

if [[ ! -f "$CHECKSUMS" ]]; then
  echo "Error: checksums file not found at $CHECKSUMS" >&2
  exit 1
fi

# Find SHA256 for macOS arm64, macOS amd64, Linux amd64, Linux arm64
get_sha() {
  local os="$1"
  local arch="$2"
  grep "gha-local-lint_${VERSION}_${os}_${arch}.tar.gz" "$CHECKSUMS" | awk '{print $1}' || echo ""
}

DARWIN_ARM64_SHA=$(get_sha "darwin" "arm64")
DARWIN_AMD64_SHA=$(get_sha "darwin" "amd64")
LINUX_AMD64_SHA=$(get_sha "linux" "amd64")
LINUX_ARM64_SHA=$(get_sha "linux" "arm64")

mkdir -p "$(dirname "$OUTPUT_FILE")"

cat <<EOF > "$OUTPUT_FILE"
class GhaLocalLint < Formula
  desc "Offline GitHub Actions Expression & Matrix Simulator"
  homepage "https://github.com/smford/gha-local-lint"
  version "${VERSION}"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/smford/gha-local-lint/releases/download/${TAG}/gha-local-lint_${VERSION}_darwin_arm64.tar.gz"
      sha256 "${DARWIN_ARM64_SHA}"
    else
      url "https://github.com/smford/gha-local-lint/releases/download/${TAG}/gha-local-lint_${VERSION}_darwin_amd64.tar.gz"
      sha256 "${DARWIN_AMD64_SHA}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/smford/gha-local-lint/releases/download/${TAG}/gha-local-lint_${VERSION}_linux_arm64.tar.gz"
      sha256 "${LINUX_ARM64_SHA}"
    else
      url "https://github.com/smford/gha-local-lint/releases/download/${TAG}/gha-local-lint_${VERSION}_linux_amd64.tar.gz"
      sha256 "${LINUX_AMD64_SHA}"
    end
  end

  def install
    bin.install "gha-local-lint"
  end

  test do
    system "#{bin}/gha-local-lint", "--version"
  end
end
EOF

echo "Generated Homebrew formula: $OUTPUT_FILE"

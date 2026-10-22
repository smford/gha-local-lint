#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-}"
DOCS_FILE="${2:-docs/index.html}"

if [[ -z "$VERSION" ]]; then
  echo "Usage: $0 <version> [docs_file]" >&2
  exit 1
fi

CLEAN_VERSION="${VERSION#v}"

if [[ ! -f "$DOCS_FILE" ]]; then
  echo "Docs file $DOCS_FILE not found, skipping version injection." >&2
  exit 0
fi

# Update version badges, text, and install commands in docs
sed -i.bak -E \
  -e "s/v[0-9]+\.[0-9]+\.[0-9]+/v${CLEAN_VERSION}/g" \
  -e "s/VERSION=\"[0-9]+\.[0-9]+\.[0-9]+\"/VERSION=\"${CLEAN_VERSION}\"/g" \
  "$DOCS_FILE"

rm -f "${DOCS_FILE}.bak"

echo "Updated ${DOCS_FILE} with version ${VERSION}"

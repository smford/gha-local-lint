package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFixFile(t *testing.T) {
	content := `name: Insecure CI
on: [pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: echo "PR title: ${{ github.event.pull_request.title }}"
`
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "workflow.yml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	count, err := FixFile(filePath)
	if err != nil {
		t.Fatalf("FixFile failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 fixes, got %d", count)
	}

	fixedData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	fixedStr := string(fixedData)

	// Verify action was pinned
	if !strings.Contains(fixedStr, "actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2") {
		t.Errorf("action checkout was not pinned properly:\n%s", fixedStr)
	}

	// Verify script injection was extracted into env:
	if !strings.Contains(fixedStr, "- env:\n          INPUT_PULL_REQUEST_TITLE: ${{ github.event.pull_request.title }}") {
		t.Errorf("script injection env block not found:\n%s", fixedStr)
	}
	if !strings.Contains(fixedStr, `run: echo "PR title: $INPUT_PULL_REQUEST_TITLE"`) {
		t.Errorf("run script not modified with env var:\n%s", fixedStr)
	}
}

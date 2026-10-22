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
	// 4 fixes: 1 unpinned action + 1 script injection + 1 missing timeout-minutes + 1 missing permissions
	if count != 4 {
		t.Errorf("expected 4 fixes, got %d", count)
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
	if !strings.Contains(fixedStr, `echo "PR title: $INPUT_PULL_REQUEST_TITLE"`) {
		t.Errorf("run script not modified with env var:\n%s", fixedStr)
	}
	if !strings.Contains(fixedStr, "INPUT_PULL_REQUEST_TITLE: ${{ github.event.pull_request.title }}") {
		t.Errorf("script injection env block not found:\n%s", fixedStr)
	}

	// Verify timeout-minutes was inserted
	if !strings.Contains(fixedStr, "timeout-minutes: 60") {
		t.Errorf("timeout-minutes: 60 was not injected:\n%s", fixedStr)
	}

	// Verify permissions was inserted
	if !strings.Contains(fixedStr, "permissions:\n  contents: read") {
		t.Errorf("permissions block was not injected:\n%s", fixedStr)
	}
}

func TestFixDeprecatedRunner(t *testing.T) {
	content := `name: Deprecated Runner CI
on: [push]
permissions:
  contents: read
jobs:
  test:
    timeout-minutes: 10
    runs-on: ubuntu-18.04
    steps:
      - run: echo ok
`
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "deprecated.yml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	count, err := FixFile(filePath)
	if err != nil {
		t.Fatalf("FixFile failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 fix for deprecated runner, got %d", count)
	}

	fixedData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fixedData), "runs-on: ubuntu-22.04") {
		t.Errorf("expected ubuntu-22.04 runner in fixed content:\n%s", string(fixedData))
	}
}

func TestFixSelfReferencingJobOutput(t *testing.T) {
	content := `name: Self Reference CI
on: [push]
permissions:
  contents: read
jobs:
  create-approval:
    timeout-minutes: 15
    runs-on: ubuntu-latest
    outputs:
      issue-number: ${{ steps.create.outputs.issue-number }}
    steps:
      - name: Create issue
        id: create
        run: echo "created"
      - name: Use issue
        env:
          NUM: ${{ needs.create-approval.outputs.issue-number }}
        run: echo "$NUM"
`
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "selfref.yml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	count, err := FixFile(filePath)
	if err != nil {
		t.Fatalf("FixFile failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 fix for self-referencing job output, got %d", count)
	}

	fixedData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fixedData), "${{ steps.create.outputs.issue-number }}") {
		t.Errorf("expected steps.create.outputs.issue-number in fixed content:\n%s", string(fixedData))
	}
	if strings.Contains(string(fixedData), "needs.create-approval.outputs.issue-number") {
		t.Errorf("needs.create-approval should have been replaced:\n%s", string(fixedData))
	}
}

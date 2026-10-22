package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/asc/gha-local-lint/internal/workflow"
)

func TestReusableWorkflowValidation(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Create a valid target reusable workflow
	reusableContent := `name: Reusable
on:
  workflow_call:
    inputs:
      env-name:
        required: true
        type: string
jobs:
  run:
    runs-on: ubuntu-latest
    steps:
      - run: echo "Deploying to ${{ inputs.env-name }}"
`
	reusablePath := filepath.Join(tmpDir, "reusable.yml")
	if err := os.WriteFile(reusablePath, []byte(reusableContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Caller missing required input
	callerMissingInput := `name: Caller
on: [push]
jobs:
  deploy:
    uses: ./` + filepath.Base(reusablePath) + `
`
	callerMissingPath := filepath.Join(tmpDir, "caller1.yml")
	wfMissing, err := workflow.ParseWorkflowBytes(callerMissingPath, []byte(callerMissingInput))
	if err != nil {
		t.Fatal(err)
	}

	findings := Lint(wfMissing, LintOptions{})
	foundMissingInput := false
	for _, f := range findings {
		if f.Rule == "missing-required-workflow-input" {
			foundMissingInput = true
		}
	}
	if !foundMissingInput {
		t.Errorf("expected missing-required-workflow-input error, got: %v", findings)
	}

	// 3. Caller with required input provided
	callerValid := `name: Caller Valid
on: [push]
jobs:
  deploy:
    uses: ./` + filepath.Base(reusablePath) + `
    with:
      env-name: staging
`
	callerValidPath := filepath.Join(tmpDir, "caller2.yml")
	wfValid, err := workflow.ParseWorkflowBytes(callerValidPath, []byte(callerValid))
	if err != nil {
		t.Fatal(err)
	}

	findingsValid := Lint(wfValid, LintOptions{})
	for _, f := range findingsValid {
		if f.Rule == "missing-required-workflow-input" || f.Rule == "missing-reusable-workflow" {
			t.Errorf("unexpected finding in valid caller: %v", f)
		}
	}
}

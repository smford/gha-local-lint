package rules

import (
	"strings"
	"testing"

	"github.com/asc/gha-local-lint/internal/workflow"
)

func TestShellSyntaxValidation(t *testing.T) {
	// 1. Broken shell syntax (unclosed quote / invalid syntax)
	brokenYAML := `name: Broken Script
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    timeout-minutes: 5
    steps:
      - name: Syntax error
        run: |
          for i in 1 2 3; do
            echo "missing done"
`
	wf, err := workflow.ParseWorkflowBytes("broken.yml", []byte(brokenYAML))
	if err != nil {
		t.Fatal(err)
	}

	findings := Lint(wf, LintOptions{})
	foundSyntaxErr := false
	for _, f := range findings {
		if f.Rule == "shell-syntax" {
			foundSyntaxErr = true
			if !strings.Contains(f.Message, "syntax error") && !strings.Contains(f.Message, "unexpected") {
				t.Errorf("unexpected syntax error message: %s", f.Message)
			}
		}
	}
	if !foundSyntaxErr {
		t.Errorf("expected shell-syntax finding for broken shell script, got: %v", findings)
	}

	// 2. Valid shell syntax with GHA expressions
	validYAML := `name: Valid Script
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    timeout-minutes: 5
    steps:
      - name: Valid script with GHA expressions
        run: |
          echo "Ref is ${{ github.ref }}"
          if [ -d "src" ]; then
            echo "src exists"
          fi
`
	wfValid, err := workflow.ParseWorkflowBytes("valid.yml", []byte(validYAML))
	if err != nil {
		t.Fatal(err)
	}

	findingsValid := Lint(wfValid, LintOptions{})
	for _, f := range findingsValid {
		if f.Rule == "shell-syntax" {
			t.Errorf("unexpected shell-syntax finding on valid script: %v", f)
		}
	}
}

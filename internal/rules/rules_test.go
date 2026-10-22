package rules

import (
	"strings"
	"testing"

	"github.com/asc/gha-local-lint/internal/workflow"
)

func TestDAGValidation(t *testing.T) {
	yamlContent := `name: CI
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    needs: [deploy]
    steps:
      - run: echo build
  test:
    runs-on: ubuntu-latest
    needs: [build]
    steps:
      - run: echo test
  deploy:
    runs-on: ubuntu-latest
    needs: [test]
    steps:
      - run: echo deploy
`
	wf, err := workflow.ParseWorkflowBytes("test.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	findings := Lint(wf, LintOptions{})

	foundCycle := false
	for _, f := range findings {
		if f.Rule == "dag-circular-dependency" {
			foundCycle = true
			if !strings.Contains(f.Message, "build") || !strings.Contains(f.Message, "test") {
				t.Errorf("unexpected cycle message: %s", f.Message)
			}
		}
	}
	if !foundCycle {
		t.Errorf("expected circular dependency finding, got: %v", findings)
	}
}

func TestMissingNeedsTarget(t *testing.T) {
	yamlContent := `name: CI
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    needs: [nonexistent]
    steps:
      - run: echo test
`
	wf, err := workflow.ParseWorkflowBytes("test.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	findings := Lint(wf, LintOptions{})
	foundMissing := false
	for _, f := range findings {
		if f.Rule == "dag-missing-need" {
			foundMissing = true
			if f.Line != 6 {
				t.Errorf("expected line 6 for missing need, got %d", f.Line)
			}
		}
	}
	if !foundMissing {
		t.Errorf("expected dag-missing-need error, got: %v", findings)
	}
}

func TestBrokenOutputReferences(t *testing.T) {
	yamlContent := `name: CI
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    outputs:
      artifact_id: ${{ steps.build-step.outputs.id }}
    steps:
      - id: build-step
        run: echo "id=123" >> $GITHUB_OUTPUT
  deploy:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    needs: [build]
    steps:
      - name: Broken Step Output
        run: echo "${{ steps.non_existent_step.outputs.data }}"
      - name: Broken Needs Output
        run: echo "${{ needs.build.outputs.non_existent_output }}"
`
	wf, err := workflow.ParseWorkflowBytes("test.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	findings := Lint(wf, LintOptions{})

	foundStepRef := false
	foundJobOutRef := false
	for _, f := range findings {
		if f.Rule == "invalid-step-reference" {
			foundStepRef = true
		}
		if f.Rule == "invalid-job-output-reference" {
			foundJobOutRef = true
		}
	}
	if !foundStepRef {
		t.Errorf("expected invalid-step-reference finding, got %v", findings)
	}
	if !foundJobOutRef {
		t.Errorf("expected invalid-job-output-reference finding, got %v", findings)
	}
}

func TestScriptInjection(t *testing.T) {
	yamlContent := `name: PR
on: [pull_request]
jobs:
  comment:
    runs-on: ubuntu-latest
    timeout-minutes: 5
    steps:
      - name: Insecure run
        run: |
          echo "Title: ${{ github.event.pull_request.title }}"
`
	wf, err := workflow.ParseWorkflowBytes("test.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	findings := Lint(wf, LintOptions{})
	foundInjection := false
	for _, f := range findings {
		if f.Rule == "script-injection" {
			foundInjection = true
			if f.Line != 9 {
				t.Errorf("expected line 9 for run block, got %d", f.Line)
			}
		}
	}
	if !foundInjection {
		t.Errorf("expected script-injection finding, got %v", findings)
	}
}

func TestActionPinningAndPermissions(t *testing.T) {
	yamlContent := `name: Lint
on: [push]
permissions: write-all
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`
	wf, err := workflow.ParseWorkflowBytes("test.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	findings := Lint(wf, LintOptions{RequireSHAPin: true})

	foundBroadPerms := false
	foundUnpinned := false
	foundMissingTimeout := false

	for _, f := range findings {
		if f.Rule == "broad-permissions" {
			foundBroadPerms = true
		}
		if f.Rule == "unpinned-action" && f.Severity == SeverityError {
			foundUnpinned = true
		}
		if f.Rule == "missing-timeout-minutes" {
			foundMissingTimeout = true
		}
	}

	if !foundBroadPerms {
		t.Errorf("expected broad-permissions finding")
	}
	if !foundUnpinned {
		t.Errorf("expected unpinned-action error when RequireSHAPin is true")
	}
	if !foundMissingTimeout {
		t.Errorf("expected missing-timeout-minutes finding")
	}
}

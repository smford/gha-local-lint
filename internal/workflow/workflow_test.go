package workflow

import (
	"testing"
)

func TestParseWorkflowBytes(t *testing.T) {
	yamlContent := `name: Sample
on:
  push:
    branches: [main]
permissions:
  contents: read
env:
  GLOBAL_VAR: "true"
jobs:
  test:
    name: Run Tests
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - name: Checkout
        uses: actions/checkout@v4
      - name: Run
        run: echo "Hello"
`
	wf, err := ParseWorkflowBytes("sample.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("ParseWorkflowBytes failed: %v", err)
	}

	if wf.Name != "Sample" {
		t.Errorf("expected name 'Sample', got %q", wf.Name)
	}

	if wf.On == nil || len(wf.On.Events) != 1 {
		t.Fatalf("expected 1 event trigger, got %v", wf.On)
	}

	pushFilter := wf.On.Events["push"]
	if len(pushFilter.Branches) != 1 || pushFilter.Branches[0] != "main" {
		t.Errorf("expected branches [main], got %v", pushFilter.Branches)
	}

	if wf.Permissions == nil || wf.Permissions.Scopes["contents"] != "read" {
		t.Errorf("expected permissions.contents == read, got %v", wf.Permissions)
	}

	if len(wf.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(wf.Jobs))
	}

	job := wf.Jobs["test"]
	if job == nil || job.Name != "Run Tests" {
		t.Fatalf("expected job 'test' with name 'Run Tests'")
	}
	if job.TimeoutMinutes == nil || *job.TimeoutMinutes != 10 {
		t.Errorf("expected timeout 10, got %v", job.TimeoutMinutes)
	}
	if len(job.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(job.Steps))
	}
}

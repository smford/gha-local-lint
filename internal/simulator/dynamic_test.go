package simulator

import (
	"testing"

	"github.com/asc/gha-local-lint/internal/workflow"
)

func TestDynamicMatrixSimulation(t *testing.T) {
	yamlContent := `name: Dynamic Matrix
on: [push]
jobs:
  setup:
    runs-on: ubuntu-latest
    outputs:
      targets: '["service-a", "service-b"]'
    steps:
      - run: echo "computing matrix"
  test:
    runs-on: ubuntu-latest
    needs: [setup]
    strategy:
      matrix: ${{ fromJSON(needs.setup.outputs.targets) }}
    steps:
      - run: echo "testing ${{ matrix.value }}"
`
	wf, err := workflow.ParseWorkflowBytes("dyn.yml", []byte(yamlContent))
	if err != nil {
		t.Fatal(err)
	}

	res, err := Simulate(wf, SimulateOptions{
		Event: "push",
		MockOutputs: map[string]any{
			"setup.targets": `[{"service": "auth"}, {"service": "billing"}]`,
		},
	})
	if err != nil {
		t.Fatalf("Simulate failed: %v", err)
	}

	// setup (1) + test (2 dynamic combos) = 3 job runs
	if len(res.Jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(res.Jobs))
	}
}

func TestPathFiltering(t *testing.T) {
	yamlContent := `name: Path Filtered
on:
  push:
    paths:
      - 'src/**'
      - 'go.mod'
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo build
`
	wf, err := workflow.ParseWorkflowBytes("paths.yml", []byte(yamlContent))
	if err != nil {
		t.Fatal(err)
	}

	// 1. Changes in src/main.go -> triggers
	res, err := Simulate(wf, SimulateOptions{
		Event:        "push",
		ChangedFiles: []string{"src/main.go", "docs/README.md"},
	})
	if err != nil || !res.Triggered {
		t.Errorf("expected trigger with src/main.go, got triggered=%v, reason=%s", res.Triggered, res.TriggerReason)
	}

	// 2. Changes only in docs/README.md -> does NOT trigger
	res, err = Simulate(wf, SimulateOptions{
		Event:        "push",
		ChangedFiles: []string{"docs/README.md"},
	})
	if err != nil || res.Triggered {
		t.Errorf("expected NOT triggered for docs change, got triggered=%v", res.Triggered)
	}
}

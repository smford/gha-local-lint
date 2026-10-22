package simulator

import (
	"strings"
	"testing"

	"github.com/asc/gha-local-lint/internal/workflow"
)

func TestSimulatorTriggerFilter(t *testing.T) {
	yamlContent := `name: Deploy
on:
  push:
    branches: [main, 'release/**']
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - run: echo deploy
`
	wf, err := workflow.ParseWorkflowBytes("deploy.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// 1. Should trigger on refs/heads/main
	res, err := Simulate(wf, SimulateOptions{
		Event: "push",
		Ref:   "refs/heads/main",
	})
	if err != nil || !res.Triggered {
		t.Errorf("expected trigger on main, got triggered=%v, err=%v", res.Triggered, err)
	}

	// 2. Should trigger on release/v1.0
	res, err = Simulate(wf, SimulateOptions{
		Event: "push",
		Ref:   "refs/heads/release/v1.0",
	})
	if err != nil || !res.Triggered {
		t.Errorf("expected trigger on release/v1.0, got triggered=%v, err=%v", res.Triggered, err)
	}

	// 3. Should NOT trigger on feature/test
	res, err = Simulate(wf, SimulateOptions{
		Event: "push",
		Ref:   "refs/heads/feature/test",
	})
	if err != nil || res.Triggered {
		t.Errorf("expected NOT triggered on feature/test, got %v", res.Triggered)
	}

	// 4. Should NOT trigger on pull_request event
	res, err = Simulate(wf, SimulateOptions{
		Event: "pull_request",
		Ref:   "refs/heads/main",
	})
	if err != nil || res.Triggered {
		t.Errorf("expected NOT triggered for pull_request event, got %v", res.Triggered)
	}
}

func TestSimulatorMatrixAndConditions(t *testing.T) {
	yamlContent := `name: Matrix Test
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
        node: [18, 20]
    steps:
      - name: All run
        run: echo "test"
      - name: Only node 20
        if: matrix.node == 20
        run: echo "node 20 only"
  deploy:
    runs-on: ubuntu-latest
    needs: [test]
    if: github.ref == 'refs/heads/main'
    steps:
      - run: echo deploy
`
	wf, err := workflow.ParseWorkflowBytes("matrix.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	res, err := Simulate(wf, SimulateOptions{
		Event: "push",
		Ref:   "refs/heads/main",
	})
	if err != nil {
		t.Fatalf("simulation failed: %v", err)
	}

	// test job has 2x2 = 4 combinations + 1 deploy job = 5 jobs in simulation
	if len(res.Jobs) != 5 {
		t.Fatalf("expected 5 job simulations, got %d", len(res.Jobs))
	}

	// Verify step conditions for node 20
	for _, j := range res.Jobs {
		if j.JobID == "test" {
			nodeVal := j.Matrix["node"]
			for _, s := range j.Steps {
				if s.Name == "Only node 20" {
					if nodeVal == 20 && s.Status != StatusWillRun {
						t.Errorf("expected node 20 step to run, got %v", s.Status)
					}
					if nodeVal == 18 && s.Status != StatusSkipped {
						t.Errorf("expected node 18 step to be skipped, got %v", s.Status)
					}
				}
			}
		}
	}

	// Test output formatting
	formatted := FormatSimulation(res)
	if !strings.Contains(formatted, "WORKFLOW SIMULATION") || !strings.Contains(formatted, "[WILL RUN] Job: deploy") {
		t.Errorf("formatted simulation output missing expected text:\n%s", formatted)
	}
}

func TestSimulatorFailurePropagation(t *testing.T) {
	yamlContent := `name: CI
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo build
  deploy:
    runs-on: ubuntu-latest
    needs: [build]
    steps:
      - run: echo deploy
  notify:
    runs-on: ubuntu-latest
    needs: [build]
    if: always()
    steps:
      - run: echo notify
`
	wf, err := workflow.ParseWorkflowBytes("ci.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	// Simulate build failing
	res, err := Simulate(wf, SimulateOptions{
		Event:    "push",
		Ref:      "refs/heads/main",
		FailJobs: []string{"build"},
	})
	if err != nil {
		t.Fatalf("Simulate failed: %v", err)
	}

	for _, j := range res.Jobs {
		if j.JobID == "build" && j.Status != StatusFailed {
			t.Errorf("expected build to be FAILED, got %v", j.Status)
		}
		if j.JobID == "deploy" && j.Status != StatusSkipped {
			t.Errorf("expected deploy to be SKIPPED due to build failure, got %v", j.Status)
		}
		if j.JobID == "notify" && j.Status != StatusWillRun {
			t.Errorf("expected notify with always() to WILL RUN, got %v", j.Status)
		}
	}
}

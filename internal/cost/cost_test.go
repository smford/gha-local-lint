package cost

import (
	"strings"
	"testing"

	"github.com/asc/gha-local-lint/internal/workflow"
)

func TestCostEstimator(t *testing.T) {
	yamlContent := `name: Multi-Arch Build
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
    steps:
      - run: echo test
  deploy:
    runs-on: ubuntu-latest
    steps:
      - run: echo deploy
`
	wf, err := workflow.ParseWorkflowBytes("ci.yml", []byte(yamlContent))
	if err != nil {
		t.Fatal(err)
	}

	est := EstimateCost(wf, CostOptions{
		DefaultDurationMinutes: 10,
		RunsPerMonth:           100,
	})

	// test job has 2 runners (1 linux, 1 macos) + deploy job has 1 linux runner = 3 runners
	if est.TotalRunnerJobs != 3 {
		t.Errorf("expected 3 total runner jobs, got %d", est.TotalRunnerJobs)
	}
	if est.LinuxJobs != 2 || est.MacOSJobs != 1 {
		t.Errorf("expected 2 Linux and 1 macOS job, got %d Linux and %d macOS", est.LinuxJobs, est.MacOSJobs)
	}

	// Cost per run:
	// Linux: 2 * 10m * $0.008 = $0.16
	// macOS: 1 * 10m * $0.064 = $0.64
	// Total per run = $0.80
	if est.CostPerRunUSD < 0.79 || est.CostPerRunUSD > 0.81 {
		t.Errorf("expected ~$0.80 per run, got $%.4f", est.CostPerRunUSD)
	}

	tbl := FormatCostTable(est, 100)
	if !strings.Contains(tbl, "RUNNER COST & QUOTA ESTIMATE") || !strings.Contains(tbl, "macOS") {
		t.Errorf("unexpected table output:\n%s", tbl)
	}
}

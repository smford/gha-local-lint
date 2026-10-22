package simulator

import (
	"strings"
	"testing"

	"github.com/asc/gha-local-lint/internal/workflow"
)

func TestGenerateHTMLReport(t *testing.T) {
	yamlContent := `name: CI Simulation
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./...
  deploy:
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
      - run: ./deploy.sh
`
	wf, err := workflow.ParseWorkflowBytes("ci.yml", []byte(yamlContent))
	if err != nil {
		t.Fatalf("failed to parse workflow: %v", err)
	}

	res, err := Simulate(wf, SimulateOptions{
		Event: "push",
		Ref:   "refs/heads/feature",
	})
	if err != nil {
		t.Fatalf("failed to simulate: %v", err)
	}

	htmlContent := GenerateHTMLReport(res, wf)

	if !strings.Contains(htmlContent, "<!DOCTYPE html>") {
		t.Errorf("expected HTML doctype")
	}
	if !strings.Contains(htmlContent, "CI Simulation") {
		t.Errorf("expected workflow name in HTML")
	}
	if !strings.Contains(htmlContent, "test") || !strings.Contains(htmlContent, "deploy") {
		t.Errorf("expected jobs 'test' and 'deploy' in HTML report")
	}
	if !strings.Contains(htmlContent, "WILL RUN") || !strings.Contains(htmlContent, "SKIPPED") {
		t.Errorf("expected status badges in HTML report")
	}
	if !strings.Contains(htmlContent, "filterCards") {
		t.Errorf("expected interactive filtering script in HTML report")
	}
}

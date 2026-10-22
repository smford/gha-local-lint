package graph

import (
	"strings"
	"testing"

	"github.com/asc/gha-local-lint/internal/workflow"
)

func TestGraphGenerators(t *testing.T) {
	yamlContent := `name: Pipeline
on: [push]
jobs:
  build:
    name: Build Artifacts
    runs-on: ubuntu-latest
  test:
    name: Test Suite
    runs-on: ubuntu-latest
    needs: [build]
`
	wf, err := workflow.ParseWorkflowBytes("pipeline.yml", []byte(yamlContent))
	if err != nil {
		t.Fatal(err)
	}

	// 1. Mermaid
	mermaid := GenerateMermaid(wf)
	if !strings.Contains(mermaid, "flowchart TD") || !strings.Contains(mermaid, "build --> test") {
		t.Errorf("unexpected Mermaid output:\n%s", mermaid)
	}

	// 2. DOT
	dot := GenerateDOT(wf)
	if !strings.Contains(dot, "digraph G") || !strings.Contains(dot, `"build" -> "test";`) {
		t.Errorf("unexpected DOT output:\n%s", dot)
	}
}

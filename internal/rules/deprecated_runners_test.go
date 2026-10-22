package rules

import (
	"testing"

	"github.com/asc/gha-local-lint/internal/workflow"
)

func TestCheckDeprecatedRunner(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		expectedCount int
		expectedSev   Severity
		ruleName      string
	}{
		{
			name: "modern runners pass",
			yamlContent: `name: Valid
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo ok
`,
			expectedCount: 0,
		},
		{
			name: "deprecated runner produces warning",
			yamlContent: `name: Deprecated
on: [push]
jobs:
  build:
    runs-on: ubuntu-20.04
    steps:
      - run: echo deprecated
`,
			expectedCount: 1,
			expectedSev:   SeverityWarning,
			ruleName:      "deprecated-runner",
		},
		{
			name: "retired runner produces error",
			yamlContent: `name: Retired
on: [push]
jobs:
  build:
    runs-on: ubuntu-18.04
    steps:
      - run: echo retired
`,
			expectedCount: 1,
			expectedSev:   SeverityError,
			ruleName:      "deprecated-runner",
		},
		{
			name: "retired macos runner in array",
			yamlContent: `name: Retired Array
on: [push]
jobs:
  build:
    runs-on: [macos-11]
    steps:
      - run: echo macos
`,
			expectedCount: 1,
			expectedSev:   SeverityError,
			ruleName:      "deprecated-runner",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wf, err := workflow.ParseWorkflowBytes("test.yml", []byte(tc.yamlContent))
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
			findings := Lint(wf, LintOptions{})
			var runnerFindings []Finding
			for _, f := range findings {
				if f.Rule == "deprecated-runner" {
					runnerFindings = append(runnerFindings, f)
				}
			}

			if len(runnerFindings) != tc.expectedCount {
				t.Fatalf("expected %d findings, got %d: %+v", tc.expectedCount, len(runnerFindings), runnerFindings)
			}
			if tc.expectedCount > 0 {
				if runnerFindings[0].Severity != tc.expectedSev {
					t.Errorf("expected severity %v, got %v", tc.expectedSev, runnerFindings[0].Severity)
				}
			}
		})
	}
}

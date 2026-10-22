package rules

import (
	"strings"
	"testing"
)

func TestGitHubAnnotationsAndSummary(t *testing.T) {
	findings := []Finding{
		{
			Rule:     "script-injection",
			Severity: SeverityError,
			Path:     ".github/workflows/ci.yml",
			Line:     12,
			Column:   8,
			Message:  "untrusted pull request title",
		},
		{
			Rule:     "missing-timeout-minutes",
			Severity: SeverityWarning,
			Path:     ".github/workflows/ci.yml",
			Line:     20,
			Column:   5,
			Message:  "missing timeout",
		},
	}

	// 1. Check annotations
	ann := FormatGitHubAnnotations(findings)
	if !strings.Contains(ann, "::error file=.github/workflows/ci.yml,line=12,col=8,title=[script-injection]::untrusted pull request title") {
		t.Errorf("unexpected error annotation format: %s", ann)
	}
	if !strings.Contains(ann, "::warning file=.github/workflows/ci.yml,line=20,col=5,title=[missing-timeout-minutes]::missing timeout") {
		t.Errorf("unexpected warning annotation format: %s", ann)
	}

	// 2. Check summary
	summary := FormatStepSummary(findings, 1)
	if !strings.Contains(summary, "gha-local-lint Analysis Report") || !strings.Contains(summary, "🔴 Error") {
		t.Errorf("unexpected summary markdown: %s", summary)
	}
}

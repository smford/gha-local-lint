package rules

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFormatSARIF(t *testing.T) {
	findings := []Finding{
		{
			Rule:     "script-injection",
			Severity: SeverityError,
			Path:     ".github/workflows/ci.yml",
			Line:     15,
			Column:   10,
			Message:  "untrusted input in run block",
		},
		{
			Rule:     "unpinned-action",
			Severity: SeverityWarning,
			Path:     ".github/workflows/ci.yml",
			Line:     20,
			Column:   5,
			Message:  "action not pinned",
		},
	}

	sarifBytes, err := FormatSARIF(findings, "0.2.0")
	if err != nil {
		t.Fatalf("FormatSARIF failed: %v", err)
	}

	type Log struct {
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name  string `json:"name"`
					Rules []struct {
						ID string `json:"id"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID string `json:"ruleId"`
				Level  string `json:"level"`
			} `json:"results"`
		} `json:"runs"`
	}

	var sarif Log
	if err := json.Unmarshal(sarifBytes, &sarif); err != nil {
		t.Fatalf("failed to parse generated SARIF: %v", err)
	}

	if sarif.Version != "2.1.0" {
		t.Errorf("expected version 2.1.0, got %s", sarif.Version)
	}

	if len(sarif.Runs[0].Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(sarif.Runs[0].Results))
	}

	if !strings.Contains(string(sarifBytes), "script-injection") {
		t.Errorf("missing rule id in SARIF: %s", string(sarifBytes))
	}
}

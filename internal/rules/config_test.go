package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".gha-lint.yaml")

	content := `version: 1
exclude:
  - ".github/workflows/legacy-*.yml"
rules:
  missing-timeout: warning
  unpinned-action: error
  hardcoded-secrets: off
simulate:
  default-ref: "refs/heads/main"
  default-event: "pull_request"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if len(cfg.Exclude) != 1 || cfg.Exclude[0] != ".github/workflows/legacy-*.yml" {
		t.Errorf("unexpected exclude: %+v", cfg.Exclude)
	}
	if cfg.Rules["missing-timeout"] != "warning" {
		t.Errorf("expected missing-timeout=warning, got %s", cfg.Rules["missing-timeout"])
	}
	if cfg.Rules["unpinned-action"] != "error" {
		t.Errorf("expected unpinned-action=error, got %s", cfg.Rules["unpinned-action"])
	}
	if cfg.Rules["hardcoded-secrets"] != "off" {
		t.Errorf("expected hardcoded-secrets=off, got %s", cfg.Rules["hardcoded-secrets"])
	}
	if cfg.Simulate.DefaultRef != "refs/heads/main" {
		t.Errorf("unexpected default-ref: %s", cfg.Simulate.DefaultRef)
	}
}

func TestFilterFindings(t *testing.T) {
	cfg := &Config{
		Exclude: []string{".github/workflows/legacy-*"},
		Rules: map[string]string{
			"unpinned-action":   "off",
			"missing-timeout":   "error",
			"deprecated-runner": "info",
		},
	}

	findings := []Finding{
		{Rule: "unpinned-action", Severity: SeverityWarning, Message: "action unpinned"},
		{Rule: "missing-timeout", Severity: SeverityWarning, Message: "missing timeout"},
		{Rule: "deprecated-runner", Severity: SeverityWarning, Message: "runner deprecated"},
		{Rule: "hardcoded-secrets", Severity: SeverityError, Message: "secret exposed"},
	}

	// 1. Excluded file should filter all findings
	excluded := FilterFindings(findings, cfg, ".github/workflows/legacy-build.yml")
	if len(excluded) != 0 {
		t.Errorf("expected 0 findings for excluded file, got %d", len(excluded))
	}

	// 2. Active file should have rules overridden and 'off' filtered
	filtered := FilterFindings(findings, cfg, ".github/workflows/ci.yml")
	if len(filtered) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(filtered))
	}

	ruleMap := make(map[string]Severity)
	for _, f := range filtered {
		ruleMap[f.Rule] = f.Severity
	}

	if _, exists := ruleMap["unpinned-action"]; exists {
		t.Errorf("unpinned-action should have been filtered out")
	}
	if ruleMap["missing-timeout"] != SeverityError {
		t.Errorf("expected missing-timeout to be escalated to error, got %v", ruleMap["missing-timeout"])
	}
	if ruleMap["deprecated-runner"] != SeverityInfo {
		t.Errorf("expected deprecated-runner to be info, got %v", ruleMap["deprecated-runner"])
	}
	if ruleMap["hardcoded-secrets"] != SeverityError {
		t.Errorf("expected hardcoded-secrets to remain error, got %v", ruleMap["hardcoded-secrets"])
	}
}

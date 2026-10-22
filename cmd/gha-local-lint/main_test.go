package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLICommands(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "gha-local-lint")

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build binary: %v\nOutput: %s", err, string(out))
	}

	// 1. Test check command on valid file
	cmd = exec.Command(binPath, "check", "../../testdata/sample-pipeline.yml")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("expected clean check on sample-pipeline.yml, got err: %v, out: %s", err, string(out))
	}
	if !strings.Contains(string(out), "0 issues found") {
		t.Errorf("expected 0 issues, got: %s", string(out))
	}

	// 2. Test check command with SARIF output
	cmd = exec.Command(binPath, "check", "--format", "sarif", "../../testdata/flawed-pipeline.yml")
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("expected flawed pipeline check to exit with error")
	}
	if !strings.Contains(string(out), `"version": "2.1.0"`) || !strings.Contains(string(out), "script-injection") {
		t.Errorf("expected SARIF output, got: %s", string(out))
	}

	// 3. Test check --fix on temporary copy
	flawedCopy := filepath.Join(tmpDir, "flawed-copy.yml")
	rawFlawed, _ := os.ReadFile("../../testdata/flawed-pipeline.yml")
	_ = os.WriteFile(flawedCopy, rawFlawed, 0644)

	cmd = exec.Command(binPath, "check", "--fix", flawedCopy)
	out, _ = cmd.CombinedOutput()
	if !strings.Contains(string(out), "Applied") {
		t.Errorf("expected fix output, got: %s", string(out))
	}

	// 4. Test matrix command
	cmd = exec.Command(binPath, "matrix", "../../testdata/sample-pipeline.yml")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("matrix failed: %v", err)
	}
	if !strings.Contains(string(out), "Total combinations: 4") {
		t.Errorf("expected 4 matrix combinations, got: %s", string(out))
	}

	// 5. Test simulate command
	cmd = exec.Command(binPath, "simulate", "../../testdata/sample-pipeline.yml", "--event", "push", "--ref", "refs/heads/main")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("simulate failed: %v", err)
	}
	if !strings.Contains(string(out), "[WILL RUN] Job: deploy") {
		t.Errorf("expected deploy job to WILL RUN, got: %s", string(out))
	}

	// 6. Test simulate with mock outputs
	cmd = exec.Command(binPath, "simulate", "../../testdata/sample-pipeline.yml", "--event", "push", "--ref", "refs/heads/main", "--mock-output", "build.art_id=custom-artifact-xyz")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Errorf("simulate with mock output failed: %v", err)
	}
	if !strings.Contains(string(out), "Deploy to Cloud") {
		t.Errorf("unexpected output: %s", string(out))
	}
}

func init() {
	_ = os.Setenv("CGO_ENABLED", "0")
}

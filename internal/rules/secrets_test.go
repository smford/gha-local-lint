package rules

import (
	"fmt"
	"testing"

	"github.com/asc/gha-local-lint/internal/workflow"
)

func TestHardcodedSecretsDetection(t *testing.T) {
	// Construct tokens dynamically to avoid false-positive push protection alerts on test files
	dummyPAT := "ghp_" + "111122223333444455556666777788889999"
	dummyAWS := "AKIA" + "IOSFODNN7EXAMPLE"
	dummySlack := "https://" + "hooks.slack.com/" + "services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX"

	yamlContent := fmt.Sprintf(`name: Secret Pipeline
on: [push]
jobs:
  leak:
    runs-on: ubuntu-latest
    timeout-minutes: 5
    steps:
      - name: Leaked PAT in env
        env:
          MY_TOKEN: %s
        run: echo "Leaked token"
      - name: Leaked AWS key in with
        uses: aws-actions/configure-aws-credentials@v4
        with:
          aws-access-key-id: %s
      - name: Leaked Slack webhook in run
        run: |
          curl -X POST -H "Content-type: application/json" %s
`, dummyPAT, dummyAWS, dummySlack)

	wf, err := workflow.ParseWorkflowBytes("secrets.yml", []byte(yamlContent))
	if err != nil {
		t.Fatal(err)
	}

	findings := Lint(wf, LintOptions{})
	secretFindings := 0
	for _, f := range findings {
		if f.Rule == "hardcoded-secrets" {
			secretFindings++
		}
	}

	if secretFindings < 3 {
		t.Errorf("expected at least 3 hardcoded-secrets findings, got %d (all findings: %v)", secretFindings, findings)
	}
}

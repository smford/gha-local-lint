package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/asc/gha-local-lint/internal/workflow"
)

type SecretPattern struct {
	Name    string
	Regex   *regexp.Regexp
	Message string
}

var secretPatterns = []SecretPattern{
	{
		Name:    "github-pat",
		Regex:   regexp.MustCompile(`\bghp_[a-zA-Z0-9]{36}\b`),
		Message: "hardcoded GitHub Personal Access Token detected",
	},
	{
		Name:    "github-fine-grained-pat",
		Regex:   regexp.MustCompile(`\bgithub_pat_[a-zA-Z0-9]{22}_[a-zA-Z0-9]{59}\b`),
		Message: "hardcoded GitHub Fine-Grained Personal Access Token detected",
	},
	{
		Name:    "github-oauth-token",
		Regex:   regexp.MustCompile(`\bgh[osru]_[a-zA-Z0-9]{36}\b`),
		Message: "hardcoded GitHub OAuth/Server token detected",
	},
	{
		Name:    "aws-access-key",
		Regex:   regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
		Message: "hardcoded AWS Access Key ID detected",
	},
	{
		Name:    "slack-webhook",
		Regex:   regexp.MustCompile(`https://hooks\.slack\.com/services/T[0-9A-Z_]+/B[0-9A-Z_]+/[0-9a-zA-Z_]+`),
		Message: "hardcoded Slack Webhook URL detected",
	},
	{
		Name:    "private-key",
		Regex:   regexp.MustCompile(`-----BEGIN (?:[A-Z]+ )?PRIVATE KEY-----`),
		Message: "hardcoded unencrypted Private Key detected",
	},
}

func checkHardcodedSecrets(wf *workflow.Workflow, job *workflow.Job, step *workflow.Step) []Finding {
	var findings []Finding

	// Check step env
	for k, v := range step.Env {
		if f := scanSecretValue(wf, job, step, "env", k, v, step.Pos); f != nil {
			findings = append(findings, *f)
		}
	}

	// Check step with
	for k, v := range step.With {
		vStr := fmt.Sprintf("%v", v)
		if f := scanSecretValue(wf, job, step, "with", k, vStr, step.Pos); f != nil {
			findings = append(findings, *f)
		}
	}

	// Check step run
	if step.Run != "" {
		for _, sp := range secretPatterns {
			if sp.Regex.MatchString(step.Run) {
				findings = append(findings, Finding{
					Rule:     "hardcoded-secrets",
					Severity: SeverityError,
					Path:     wf.Path,
					Line:     step.RunPos.Line,
					Column:   step.RunPos.Column,
					JobID:    job.ID,
					StepID:   step.ID,
					Message:  fmt.Sprintf("%s in run: script; use ${{ secrets.NAME }} instead", sp.Message),
				})
			}
		}
	}

	return findings
}

func scanSecretValue(wf *workflow.Workflow, job *workflow.Job, step *workflow.Step, section, key, val string, pos workflow.Position) *Finding {
	// If value is a dynamic secret reference or expression like ${{ secrets.XYZ }}, it's clean
	if strings.Contains(val, "${{") || strings.Contains(val, "$") {
		return nil
	}

	// Check specific regex patterns
	for _, sp := range secretPatterns {
		if sp.Regex.MatchString(val) {
			return &Finding{
				Rule:     "hardcoded-secrets",
				Severity: SeverityError,
				Path:     wf.Path,
				Line:     pos.Line,
				Column:   pos.Column,
				JobID:    job.ID,
				StepID:   step.ID,
				Message:  fmt.Sprintf("%s in %s.%s; store credentials in repository secrets instead", sp.Message, section, key),
			}
		}
	}

	// Check literal assignments to sensitive variable names
	lowerKey := strings.ToLower(key)
	sensitiveKeys := []string{"token", "secret", "password", "api_key", "apikey", "auth_token", "private_key"}
	for _, sk := range sensitiveKeys {
		if strings.Contains(lowerKey, sk) && len(val) >= 16 && !strings.Contains(val, " ") {
			return &Finding{
				Rule:     "hardcoded-secrets",
				Severity: SeverityError,
				Path:     wf.Path,
				Line:     pos.Line,
				Column:   pos.Column,
				JobID:    job.ID,
				StepID:   step.ID,
				Message:  fmt.Sprintf("potential hardcoded credential in %s.%s (%q); use ${{ secrets.NAME }} instead", section, key, maskValue(val)),
			}
		}
	}

	return nil
}

func maskValue(val string) string {
	if len(val) <= 6 {
		return "******"
	}
	return val[:2] + "****" + val[len(val)-2:]
}

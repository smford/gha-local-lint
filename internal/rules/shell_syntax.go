package rules

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/asc/gha-local-lint/internal/workflow"
)

var ghaExprPattern = regexp.MustCompile(`\$\{\{\s*.*?\s*\}\}`)
var bashLineRe = regexp.MustCompile(`(?:line|Line)\s+(\d+):\s*(.*)`)

func checkShellSyntax(wf *workflow.Workflow, job *workflow.Job, step *workflow.Step) []Finding {
	if step.Run == "" {
		return nil
	}

	bashPath, err := exec.LookPath("bash")
	if err != nil {
		bashPath, err = exec.LookPath("sh")
		if err != nil {
			return nil
		}
	}

	// Sanitize GitHub Actions expressions: replace ${{ ... }} with a safe placeholder string
	sanitizedScript := ghaExprPattern.ReplaceAllString(step.Run, "__GHA_VAR__")

	cmd := exec.Command(bashPath, "-n")
	cmd.Stdin = strings.NewReader(sanitizedScript)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errOutput := strings.TrimSpace(stderr.String())
		if errOutput == "" {
			errOutput = err.Error()
		}

		lineOffset := 0
		msg := errOutput
		if matches := bashLineRe.FindStringSubmatch(errOutput); len(matches) >= 3 {
			if l, parseErr := strconv.Atoi(matches[1]); parseErr == nil && l > 0 {
				lineOffset = l - 1
			}
			msg = matches[2]
		}

		findingLine := step.RunPos.Line + lineOffset
		return []Finding{
			{
				Rule:     "shell-syntax",
				Severity: SeverityError,
				Path:     wf.Path,
				Line:     findingLine,
				Column:   step.RunPos.Column,
				JobID:    job.ID,
				StepID:   step.ID,
				Message:  fmt.Sprintf("shell syntax error in run block: %s", msg),
			},
		}
	}

	return nil
}

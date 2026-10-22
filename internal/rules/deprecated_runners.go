package rules

import (
	"fmt"
	"strings"

	"github.com/asc/gha-local-lint/internal/workflow"
)

type DeprecatedRunnerInfo struct {
	Label       string
	Replacement string
	Retired     bool
}

var deprecatedRunners = map[string]DeprecatedRunnerInfo{
	"ubuntu-18.04": {
		Label:       "ubuntu-18.04",
		Replacement: "ubuntu-22.04 or ubuntu-latest",
		Retired:     true,
	},
	"ubuntu-20.04": {
		Label:       "ubuntu-20.04",
		Replacement: "ubuntu-22.04 or ubuntu-latest",
		Retired:     false,
	},
	"macos-10.15": {
		Label:       "macos-10.15",
		Replacement: "macos-13 or macos-latest",
		Retired:     true,
	},
	"macos-11": {
		Label:       "macos-11",
		Replacement: "macos-13 or macos-latest",
		Retired:     true,
	},
	"macos-12": {
		Label:       "macos-12",
		Replacement: "macos-13 or macos-14",
		Retired:     false,
	},
	"windows-2016": {
		Label:       "windows-2016",
		Replacement: "windows-2022 or windows-latest",
		Retired:     true,
	},
	"windows-2019": {
		Label:       "windows-2019",
		Replacement: "windows-2022 or windows-latest",
		Retired:     false,
	},
}

func checkDeprecatedRunner(wf *workflow.Workflow, job *workflow.Job) []Finding {
	var findings []Finding

	labels := extractRunnerLabels(job.RunsOn)
	for _, label := range labels {
		cleanLabel := strings.ToLower(strings.TrimSpace(label))
		if info, ok := deprecatedRunners[cleanLabel]; ok {
			sev := SeverityWarning
			statusMsg := "is deprecated by GitHub"
			if info.Retired {
				sev = SeverityError
				statusMsg = "has been retired/removed by GitHub and will fail"
			}

			findings = append(findings, Finding{
				Rule:     "deprecated-runner",
				Severity: sev,
				Path:     wf.Path,
				Line:     job.Pos.Line,
				Column:   job.Pos.Column,
				JobID:    job.ID,
				Message:  fmt.Sprintf("runner %q %s; upgrade to %s", label, statusMsg, info.Replacement),
			})
		}
	}

	return findings
}

func extractRunnerLabels(runsOn any) []string {
	if runsOn == nil {
		return nil
	}
	switch v := runsOn.(type) {
	case string:
		return []string{v}
	case []string:
		return v
	case []any:
		var list []string
		for _, item := range v {
			list = append(list, fmt.Sprintf("%v", item))
		}
		return list
	default:
		return nil
	}
}

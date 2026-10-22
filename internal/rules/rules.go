package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/asc/gha-local-lint/internal/expr"
	"github.com/asc/gha-local-lint/internal/workflow"
)

type Severity string

const (
	SeverityError   Severity = "ERROR"
	SeverityWarning Severity = "WARNING"
	SeverityInfo    Severity = "INFO"
)

type Finding struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	Path     string   `json:"path"`
	Line     int      `json:"line"`
	Column   int      `json:"column"`
	JobID    string   `json:"job_id,omitempty"`
	StepID   string   `json:"step_id,omitempty"`
	Message  string   `json:"message"`
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d:%d: [%s] %s (%s)", f.Path, f.Line, f.Column, f.Severity, f.Message, f.Rule)
}

type LintOptions struct {
	RequireSHAPin bool
}

// Untrusted GitHub context patterns that pose code/command injection risks when interpolated in `run:` blocks.
var untrustedInputs = []string{
	"github.event.issue.title",
	"github.event.issue.body",
	"github.event.pull_request.title",
	"github.event.pull_request.body",
	"github.event.pull_request.head.ref",
	"github.event.pull_request.head.label",
	"github.head_ref",
	"github.event.comment.body",
	"github.event.review.body",
	"github.event.review_comment.body",
	"github.event.discussion.title",
	"github.event.discussion.body",
	"github.event.head_commit.message",
	"github.event.head_commit.author.email",
	"github.event.head_commit.author.name",
	"github.event.commits.",
	"github.event.workflow_run.head_branch",
	"github.event.workflow_run.head_commit.message",
}

var shaRegex = regexp.MustCompile(`^[a-fA-F0-9]{40}$`)

func Lint(wf *workflow.Workflow, opts LintOptions) []Finding {
	var findings []Finding

	// 1. Permissions checks
	findings = append(findings, checkPermissions(wf)...)

	// 2. Job-level checks: Timeout, Needs DAG
	findings = append(findings, checkDAG(wf)...)

	// 3. Output references and security rules
	for _, jobID := range wf.JobOrder {
		job := wf.Jobs[jobID]
		findings = append(findings, checkJob(wf, job, opts)...)
	}

	return findings
}

func checkPermissions(wf *workflow.Workflow) []Finding {
	var findings []Finding
	if wf.Permissions == nil {
		findings = append(findings, Finding{
			Rule:     "missing-workflow-permissions",
			Severity: SeverityWarning,
			Path:     wf.Path,
			Line:     wf.Pos.Line,
			Column:   wf.Pos.Column,
			Message:  "workflow is missing top-level permissions block (defaults to write-all in some repository settings)",
		})
	} else if wf.Permissions.IsRawString && strings.EqualFold(wf.Permissions.RawValue, "write-all") {
		findings = append(findings, Finding{
			Rule:     "broad-permissions",
			Severity: SeverityWarning,
			Path:     wf.Path,
			Line:     wf.Permissions.Pos.Line,
			Column:   wf.Permissions.Pos.Column,
			Message:  "workflow uses overly broad 'permissions: write-all', specify minimal required scopes instead",
		})
	}
	return findings
}

func checkDAG(wf *workflow.Workflow) []Finding {
	var findings []Finding

	// Check missing targets in needs
	for jobID, job := range wf.Jobs {
		for _, needed := range job.Needs {
			if _, exists := wf.Jobs[needed]; !exists {
				pos := job.NeedsPos[needed]
				if pos.Line == 0 {
					pos = job.Pos
				}
				findings = append(findings, Finding{
					Rule:     "dag-missing-need",
					Severity: SeverityError,
					Path:     wf.Path,
					Line:     pos.Line,
					Column:   pos.Column,
					JobID:    jobID,
					Message:  fmt.Sprintf("job %q depends on non-existent job %q in needs", jobID, needed),
				})
			}
		}
	}

	// Cycle detection using 3-color DFS
	// 0: unvisited, 1: visiting (in stack), 2: visited
	state := make(map[string]int)
	var path []string

	var dfs func(u string)
	dfs = func(u string) {
		state[u] = 1
		path = append(path, u)

		job, exists := wf.Jobs[u]
		if exists {
			for _, v := range job.Needs {
				if _, ok := wf.Jobs[v]; !ok {
					continue
				}
				if state[v] == 1 {
					// Found cycle! Extract cycle path
					cycleStart := 0
					for idx, p := range path {
						if p == v {
							cycleStart = idx
							break
						}
					}
					cycleChain := append(path[cycleStart:], v)
					pos := job.NeedsPos[v]
					if pos.Line == 0 {
						pos = job.Pos
					}
					findings = append(findings, Finding{
						Rule:     "dag-circular-dependency",
						Severity: SeverityError,
						Path:     wf.Path,
						Line:     pos.Line,
						Column:   pos.Column,
						JobID:    u,
						Message:  fmt.Sprintf("circular dependency detected in job needs: %s", strings.Join(cycleChain, " -> ")),
					})
				} else if state[v] == 0 {
					dfs(v)
				}
			}
		}

		path = path[:len(path)-1]
		state[u] = 2
	}

	for _, jobID := range wf.JobOrder {
		if state[jobID] == 0 {
			dfs(jobID)
		}
	}

	return findings
}

func checkJob(wf *workflow.Workflow, job *workflow.Job, opts LintOptions) []Finding {
	var findings []Finding

	// Check if this is a reusable workflow invocation job
	if job.Uses != "" {
		findings = append(findings, checkReusableWorkflow(wf, job)...)
		return findings
	}

	// Missing timeout-minutes
	if job.TimeoutMinutes == nil {
		findings = append(findings, Finding{
			Rule:     "missing-timeout-minutes",
			Severity: SeverityWarning,
			Path:     wf.Path,
			Line:     job.Pos.Line,
			Column:   job.Pos.Column,
			JobID:    job.ID,
			Message:  fmt.Sprintf("job %q is missing 'timeout-minutes'; hung jobs can consume excessive runner time", job.ID),
		})
	}

	// Broad permissions on job level
	if job.Permissions != nil && job.Permissions.IsRawString && strings.EqualFold(job.Permissions.RawValue, "write-all") {
		findings = append(findings, Finding{
			Rule:     "broad-permissions",
			Severity: SeverityWarning,
			Path:     wf.Path,
			Line:     job.Permissions.Pos.Line,
			Column:   job.Permissions.Pos.Column,
			JobID:    job.ID,
			Message:  fmt.Sprintf("job %q uses overly broad 'permissions: write-all'", job.ID),
		})
	}

	// Collect valid step IDs in this job
	stepIDMap := make(map[string]bool)
	for _, s := range job.Steps {
		if s.ID != "" {
			stepIDMap[strings.ToLower(s.ID)] = true
		}
	}

	// Validate references in job-level fields
	if job.If != "" {
		findings = append(findings, validateReferences(wf, job, nil, job.If, job.IfPos, stepIDMap)...)
	}
	for _, envVal := range job.Env {
		findings = append(findings, validateReferences(wf, job, nil, envVal, job.Pos, stepIDMap)...)
	}
	for _, out := range job.Outputs {
		findings = append(findings, validateReferences(wf, job, nil, out.Value, out.Pos, stepIDMap)...)
	}

	// Validate steps
	for _, step := range job.Steps {
		// Validate unpinned actions
		if step.Uses != "" {
			findings = append(findings, checkActionPinning(wf, job, step, opts)...)
		}

		// Validate script injection in run:
		if step.Run != "" {
			findings = append(findings, checkScriptInjection(wf, job, step)...)
			findings = append(findings, checkShellSyntax(wf, job, step)...)
		}

		// Validate references in step
		if step.If != "" {
			findings = append(findings, validateReferences(wf, job, step, step.If, step.IfPos, stepIDMap)...)
		}
		if step.Run != "" {
			findings = append(findings, validateReferences(wf, job, step, step.Run, step.RunPos, stepIDMap)...)
		}
		for _, ev := range step.Env {
			findings = append(findings, validateReferences(wf, job, step, ev, step.Pos, stepIDMap)...)
		}
	}

	return findings
}

func checkActionPinning(wf *workflow.Workflow, job *workflow.Job, step *workflow.Step, opts LintOptions) []Finding {
	var findings []Finding
	uses := step.Uses
	// Ignore local actions (./...) or docker actions (docker://...)
	if strings.HasPrefix(uses, "./") || strings.HasPrefix(uses, "docker://") {
		return nil
	}

	parts := strings.Split(uses, "@")
	if len(parts) != 2 {
		return nil
	}

	ref := parts[1]
	isSHA := shaRegex.MatchString(ref)
	if !isSHA {
		sev := SeverityWarning
		if opts.RequireSHAPin {
			sev = SeverityError
		}
		findings = append(findings, Finding{
			Rule:     "unpinned-action",
			Severity: sev,
			Path:     wf.Path,
			Line:     step.UsesPos.Line,
			Column:   step.UsesPos.Column,
			JobID:    job.ID,
			StepID:   step.ID,
			Message:  fmt.Sprintf("action %q is not pinned to a full 40-character commit SHA (found %q)", uses, ref),
		})
	}

	return findings
}

func checkScriptInjection(wf *workflow.Workflow, job *workflow.Job, step *workflow.Step) []Finding {
	var findings []Finding
	refs := expr.ExtractReferences(step.Run)

	for _, ref := range refs {
		lowerRef := strings.ToLower(ref)
		for _, untrusted := range untrustedInputs {
			if strings.HasPrefix(lowerRef, untrusted) || lowerRef == untrusted {
				findings = append(findings, Finding{
					Rule:     "script-injection",
					Severity: SeverityError,
					Path:     wf.Path,
					Line:     step.RunPos.Line,
					Column:   step.RunPos.Column,
					JobID:    job.ID,
					StepID:   step.ID,
					Message: fmt.Sprintf("untrusted input %q directly interpolated in run: block; potential code injection. Use an intermediate environment variable instead.",
						ref),
				})
				break
			}
		}
	}

	return findings
}

func validateReferences(wf *workflow.Workflow, job *workflow.Job, step *workflow.Step, exprStr string, pos workflow.Position, stepIDs map[string]bool) []Finding {
	var findings []Finding
	refs := expr.ExtractReferences(exprStr)

	stepID := ""
	if step != nil {
		stepID = step.ID
	}

	for _, ref := range refs {
		parts := strings.Split(ref, ".")
		if len(parts) < 2 {
			continue
		}

		root := strings.ToLower(parts[0])

		// 1. Check needs.<job_id>.outputs.<output_name>
		if root == "needs" {
			if len(parts) >= 2 {
				neededJobID := parts[1]
				// Check if neededJobID is declared in job.Needs
				foundInNeeds := false
				for _, n := range job.Needs {
					if strings.EqualFold(n, neededJobID) {
						foundInNeeds = true
						break
					}
				}
				if !foundInNeeds {
					findings = append(findings, Finding{
						Rule:     "invalid-needs-reference",
						Severity: SeverityError,
						Path:     wf.Path,
						Line:     pos.Line,
						Column:   pos.Column,
						JobID:    job.ID,
						StepID:   stepID,
						Message:  fmt.Sprintf("reference to %q but job %q is not listed in 'needs'", ref, neededJobID),
					})
					continue
				}

				// If it accesses outputs: needs.<job_id>.outputs.<name>
				if len(parts) >= 4 && strings.EqualFold(parts[2], "outputs") {
					outputName := parts[3]
					targetJob := wf.Jobs[neededJobID]
					if targetJob != nil {
						foundOutput := false
						for outKey := range targetJob.Outputs {
							if strings.EqualFold(outKey, outputName) {
								foundOutput = true
								break
							}
						}
						if !foundOutput {
							findings = append(findings, Finding{
								Rule:     "invalid-job-output-reference",
								Severity: SeverityError,
								Path:     wf.Path,
								Line:     pos.Line,
								Column:   pos.Column,
								JobID:    job.ID,
								StepID:   stepID,
								Message:  fmt.Sprintf("job %q does not define output %q (referenced as %q)", neededJobID, outputName, ref),
							})
						}
					}
				}
			}
		}

		// 2. Check steps.<step_id>.outputs.<output_name>
		if root == "steps" && len(parts) >= 3 {
			referencedStepID := strings.ToLower(parts[1])
			if !stepIDs[referencedStepID] {
				findings = append(findings, Finding{
					Rule:     "invalid-step-reference",
					Severity: SeverityError,
					Path:     wf.Path,
					Line:     pos.Line,
					Column:   pos.Column,
					JobID:    job.ID,
					StepID:   stepID,
					Message:  fmt.Sprintf("reference to unknown step id %q in %q; no such step id exists in job %q", referencedStepID, ref, job.ID),
				})
			}
		}
	}

	return findings
}

func checkReusableWorkflow(wf *workflow.Workflow, job *workflow.Job) []Finding {
	var findings []Finding
	if !strings.HasPrefix(job.Uses, "./") {
		return nil
	}

	targetPath := strings.TrimPrefix(job.Uses, "./")
	if _, err := os.Stat(targetPath); err != nil {
		targetPath = filepath.Join(filepath.Dir(wf.Path), strings.TrimPrefix(job.Uses, "./"))
	}

	targetWf, err := workflow.ParseWorkflow(targetPath)
	if err != nil {
		findings = append(findings, Finding{
			Rule:     "missing-reusable-workflow",
			Severity: SeverityError,
			Path:     wf.Path,
			Line:     job.UsesPos.Line,
			Column:   job.UsesPos.Column,
			JobID:    job.ID,
			Message:  fmt.Sprintf("referenced reusable workflow %q cannot be found or parsed: %v", job.Uses, err),
		})
		return findings
	}

	// Verify target workflow has workflow_call trigger
	if targetWf.On == nil || targetWf.On.Events["workflow_call"] == nil {
		findings = append(findings, Finding{
			Rule:     "invalid-reusable-workflow",
			Severity: SeverityError,
			Path:     wf.Path,
			Line:     job.UsesPos.Line,
			Column:   job.UsesPos.Column,
			JobID:    job.ID,
			Message:  fmt.Sprintf("workflow %q is invoked via uses: but does not define 'on: workflow_call'", job.Uses),
		})
	}

	// Verify required inputs are passed
	if targetWf.On != nil {
		for inName, inDef := range targetWf.On.Inputs {
			if inDef.Required {
				if job.With == nil || job.With[inName] == nil {
					findings = append(findings, Finding{
						Rule:     "missing-required-workflow-input",
						Severity: SeverityError,
						Path:     wf.Path,
						Line:     job.Pos.Line,
						Column:   job.Pos.Column,
						JobID:    job.ID,
						Message:  fmt.Sprintf("reusable workflow %q requires input %q, but it is not provided in with:", job.Uses, inName),
					})
				}
			}
		}
	}

	return findings
}

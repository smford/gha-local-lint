package simulator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/asc/gha-local-lint/internal/expr"
	"github.com/asc/gha-local-lint/internal/matrix"
	"github.com/asc/gha-local-lint/internal/workflow"
)

type SimStatus string

const (
	StatusWillRun SimStatus = "WILL RUN"
	StatusSkipped SimStatus = "SKIPPED"
	StatusFailed  SimStatus = "FAILED"
)

type SimulateOptions struct {
	Event        string
	Ref          string
	Actor        string
	Repository   string
	ContextFile  string
	FailJobs     []string
	MockOutputs  map[string]any
	ChangedFiles []string
}

type SimulationResult struct {
	WorkflowName  string          `json:"workflow_name"`
	WorkflowPath  string          `json:"workflow_path"`
	Triggered     bool            `json:"triggered"`
	TriggerReason string          `json:"trigger_reason"`
	Jobs          []*JobSimResult `json:"jobs"`
}

type JobSimResult struct {
	JobID  string             `json:"job_id"`
	Name   string             `json:"name"`
	Matrix matrix.Combination `json:"matrix,omitempty"`
	Status SimStatus          `json:"status"`
	Reason string             `json:"reason,omitempty"`
	Steps  []*StepSimResult   `json:"steps,omitempty"`
}

type StepSimResult struct {
	ID     string    `json:"id,omitempty"`
	Name   string    `json:"name"`
	Status SimStatus `json:"status"`
	Reason string    `json:"reason,omitempty"`
}

func Simulate(wf *workflow.Workflow, opts SimulateOptions) (*SimulationResult, error) {
	if opts.Event == "" {
		opts.Event = "push"
	}
	if opts.Ref == "" {
		opts.Ref = "refs/heads/main"
	}
	if opts.Actor == "" {
		opts.Actor = "octocat"
	}
	if opts.Repository == "" {
		opts.Repository = "octocat/hello-world"
	}

	res := &SimulationResult{
		WorkflowName: wf.Name,
		WorkflowPath: wf.Path,
	}

	// 1. Evaluate triggers
	triggered, reason := evaluateTrigger(wf.On, opts)
	res.Triggered = triggered
	res.TriggerReason = reason
	if !triggered {
		return res, nil
	}

	// 2. Build base mock context
	baseCtxData, err := buildMockContext(opts)
	if err != nil {
		return nil, fmt.Errorf("building mock context: %w", err)
	}

	// 3. Topological sort of jobs
	orderedJobIDs, err := topologicalSortJobs(wf)
	if err != nil {
		return nil, fmt.Errorf("job ordering: %w", err)
	}

	// Track job completion statuses: jobID -> map of matrixKey -> status
	jobStatusMap := make(map[string]map[string]SimStatus)
	// Track job outputs: jobID -> map of outputName -> value
	jobOutputsMap := make(map[string]map[string]any)

	// Pre-seed mock outputs
	for k, v := range opts.MockOutputs {
		parts := strings.SplitN(k, ".", 2)
		if len(parts) == 2 {
			jID := parts[0]
			outName := parts[1]
			if jobOutputsMap[jID] == nil {
				jobOutputsMap[jID] = make(map[string]any)
			}
			jobOutputsMap[jID][outName] = v
		}
	}

	failJobsMap := make(map[string]bool)
	for _, fj := range opts.FailJobs {
		failJobsMap[strings.ToLower(fj)] = true
	}

	for _, jobID := range orderedJobIDs {
		job := wf.Jobs[jobID]
		jobStatusMap[jobID] = make(map[string]SimStatus)
		if jobOutputsMap[jobID] == nil {
			jobOutputsMap[jobID] = make(map[string]any)
		}

		// Prepare pre-context for dynamic matrix evaluation
		preNeedsMap := make(map[string]any)
		for _, needID := range job.Needs {
			needInfo := map[string]any{
				"result":  "success",
				"outputs": jobOutputsMap[needID],
			}
			for _, st := range jobStatusMap[needID] {
				if st == StatusFailed {
					needInfo["result"] = "failure"
				} else if st == StatusSkipped {
					needInfo["result"] = "skipped"
				}
			}
			preNeedsMap[needID] = needInfo
		}
		preCtxData := cloneMap(baseCtxData)
		preCtxData["needs"] = preNeedsMap
		preCtx := expr.NewContext(preCtxData)

		evalFn := func(exprStr string) (any, error) {
			return expr.Interpolate(exprStr, preCtx)
		}

		// Expand matrix for this job (supports dynamic expressions)
		combos, err := matrix.ExpandWithEvaluator(job.Strategy, evalFn)
		if err != nil {
			return nil, fmt.Errorf("matrix expansion for job %q: %w", jobID, err)
		}

		// Check upstream dependencies in needs:
		upstreamFailed := false
		var failedUpstreams []string
		for _, needID := range job.Needs {
			allCombos := jobStatusMap[needID]
			for _, st := range allCombos {
				if st == StatusFailed || st == StatusSkipped {
					upstreamFailed = true
					failedUpstreams = append(failedUpstreams, needID)
					break
				}
			}
		}

		for _, combo := range combos {
			comboKey := combo.KeyString()

			// Prepare context for this job instance
			jobCtxData := cloneMap(baseCtxData)
			jobCtxData["matrix"] = map[string]any(combo)

			// Populate needs context
			needsMap := make(map[string]any)
			for _, needID := range job.Needs {
				needInfo := map[string]any{
					"result":  "success",
					"outputs": jobOutputsMap[needID],
				}
				for _, st := range jobStatusMap[needID] {
					if st == StatusFailed {
						needInfo["result"] = "failure"
					} else if st == StatusSkipped {
						needInfo["result"] = "skipped"
					}
				}
				needsMap[needID] = needInfo
			}
			jobCtxData["needs"] = needsMap

			// Merge workflow env and job env
			envMap := make(map[string]any)
			for k, v := range wf.Env {
				envMap[k] = v
			}
			for k, v := range job.Env {
				envMap[k] = v
			}
			jobCtxData["env"] = envMap

			evalCtx := expr.NewContext(jobCtxData)
			if upstreamFailed {
				evalCtx.JobStatus = expr.StatusFailure
			} else {
				evalCtx.JobStatus = expr.StatusSuccess
			}

			jobSim := &JobSimResult{
				JobID:  jobID,
				Name:   job.Name,
				Matrix: combo,
			}
			if jobSim.Name == "" {
				jobSim.Name = jobID
			}

			// Evaluate job if:
			if job.If != "" {
				shouldRun, err := expr.EvaluateCondition(job.If, evalCtx)
				if err != nil {
					jobSim.Status = StatusSkipped
					jobSim.Reason = fmt.Sprintf("error evaluating 'if: %s': %v", job.If, err)
					jobStatusMap[jobID][comboKey] = StatusSkipped
					res.Jobs = append(res.Jobs, jobSim)
					continue
				}
				if !shouldRun {
					jobSim.Status = StatusSkipped
					jobSim.Reason = fmt.Sprintf("condition '%s' evaluated to false", job.If)
					jobStatusMap[jobID][comboKey] = StatusSkipped
					res.Jobs = append(res.Jobs, jobSim)
					continue
				}
			} else if upstreamFailed {
				// Default status check requires success of upstream jobs
				jobSim.Status = StatusSkipped
				jobSim.Reason = fmt.Sprintf("upstream dependencies failed or skipped (%s)", strings.Join(failedUpstreams, ", "))
				jobStatusMap[jobID][comboKey] = StatusSkipped
				res.Jobs = append(res.Jobs, jobSim)
				continue
			}

			// Job will run (or simulated fail)
			if failJobsMap[strings.ToLower(jobID)] {
				jobSim.Status = StatusFailed
				jobSim.Reason = "simulated failure via --fail-job"
				jobStatusMap[jobID][comboKey] = StatusFailed
			} else {
				jobSim.Status = StatusWillRun
				jobStatusMap[jobID][comboKey] = StatusWillRun
			}

			// Simulate Steps
			stepCtx := evalCtx.Clone()
			stepCtx.JobStatus = expr.StatusSuccess

			for idx, step := range job.Steps {
				stepName := step.Name
				if stepName == "" {
					if step.Run != "" {
						lines := strings.Split(strings.TrimSpace(step.Run), "\n")
						stepName = lines[0]
					} else if step.Uses != "" {
						stepName = step.Uses
					} else {
						stepName = fmt.Sprintf("Step %d", idx+1)
					}
				}

				stepSim := &StepSimResult{
					ID:   step.ID,
					Name: stepName,
				}

				if step.If != "" {
					shouldRunStep, err := expr.EvaluateCondition(step.If, stepCtx)
					if err != nil {
						stepSim.Status = StatusSkipped
						stepSim.Reason = fmt.Sprintf("error evaluating 'if: %s': %v", step.If, err)
					} else if !shouldRunStep {
						stepSim.Status = StatusSkipped
						stepSim.Reason = fmt.Sprintf("condition '%s' evaluated to false", step.If)
					} else {
						stepSim.Status = StatusWillRun
					}
				} else {
					stepSim.Status = StatusWillRun
				}

				jobSim.Steps = append(jobSim.Steps, stepSim)
			}

			// Populate mock outputs (if not overridden by opts.MockOutputs)
			for outKey, outVal := range job.Outputs {
				mockKey := jobID + "." + outKey
				if _, mocked := opts.MockOutputs[mockKey]; !mocked {
					interp, _ := expr.Interpolate(outVal.Value, evalCtx)
					jobOutputsMap[jobID][outKey] = interp
				}
			}

			res.Jobs = append(res.Jobs, jobSim)
		}
	}

	return res, nil
}

func evaluateTrigger(tc *workflow.TriggerConfig, opts SimulateOptions) (bool, string) {
	if tc == nil || len(tc.Events) == 0 {
		return true, "workflow has no triggers specified; assuming active"
	}

	filter, exists := tc.Events[opts.Event]
	if !exists {
		var available []string
		for e := range tc.Events {
			available = append(available, e)
		}
		return false, fmt.Sprintf("event %q does not match configured triggers [%s]", opts.Event, strings.Join(available, ", "))
	}

	if filter == nil {
		return true, fmt.Sprintf("triggered by event %q", opts.Event)
	}

	// Check branches
	cleanBranch := strings.TrimPrefix(opts.Ref, "refs/heads/")
	if len(filter.Branches) > 0 {
		matched := false
		for _, b := range filter.Branches {
			if matchPattern(b, cleanBranch) {
				matched = true
				break
			}
		}
		if !matched {
			return false, fmt.Sprintf("branch %q does not match branches filter [%s]", cleanBranch, strings.Join(filter.Branches, ", "))
		}
	}

	if len(filter.BranchesIgnore) > 0 {
		for _, b := range filter.BranchesIgnore {
			if matchPattern(b, cleanBranch) {
				return false, fmt.Sprintf("branch %q is ignored by branches-ignore [%s]", cleanBranch, strings.Join(filter.BranchesIgnore, ", "))
			}
		}
	}

	// Check paths if ChangedFiles are provided
	if len(opts.ChangedFiles) > 0 {
		if len(filter.Paths) > 0 {
			matchedPath := false
			for _, file := range opts.ChangedFiles {
				for _, p := range filter.Paths {
					if matchPattern(p, file) {
						matchedPath = true
						break
					}
				}
				if matchedPath {
					break
				}
			}
			if !matchedPath {
				return false, fmt.Sprintf("none of the changed files [%s] matched paths filter [%s]",
					strings.Join(opts.ChangedFiles, ", "), strings.Join(filter.Paths, ", "))
			}
		}

		if len(filter.PathsIgnore) > 0 {
			allIgnored := true
			for _, file := range opts.ChangedFiles {
				fileIgnored := false
				for _, p := range filter.PathsIgnore {
					if matchPattern(p, file) {
						fileIgnored = true
						break
					}
				}
				if !fileIgnored {
					allIgnored = false
					break
				}
			}
			if allIgnored {
				return false, fmt.Sprintf("all changed files matched paths-ignore [%s]", strings.Join(filter.PathsIgnore, ", "))
			}
		}
	}

	return true, fmt.Sprintf("triggered by event %q on ref %q", opts.Event, opts.Ref)
}

func matchPattern(pattern, val string) bool {
	if pattern == val {
		return true
	}
	// Convert glob pattern to regex:
	// ** matches anything including /
	// * matches anything except /
	var sb strings.Builder
	sb.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '*' {
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				sb.WriteString(".*")
				i++
			} else {
				sb.WriteString("[^/]*")
			}
		} else if pattern[i] == '?' {
			sb.WriteString(".")
		} else {
			sb.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	sb.WriteString("$")
	re, err := regexp.Compile(sb.String())
	if err != nil {
		matched, _ := filepath.Match(pattern, val)
		return matched
	}
	return re.MatchString(val)
}

func buildMockContext(opts SimulateOptions) (map[string]any, error) {
	shortSHA := "a1b2c3d"
	fullSHA := "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678"

	eventData := make(map[string]any)

	switch opts.Event {
	case "push":
		eventData = map[string]any{
			"ref": opts.Ref,
			"head_commit": map[string]any{
				"id":      fullSHA,
				"message": "simulated commit message",
				"author": map[string]any{
					"name":  opts.Actor,
					"email": opts.Actor + "@users.noreply.github.com",
				},
			},
			"pusher": map[string]any{
				"name": opts.Actor,
			},
		}
	case "pull_request":
		branch := strings.TrimPrefix(opts.Ref, "refs/heads/")
		eventData = map[string]any{
			"action": "opened",
			"number": 42,
			"pull_request": map[string]any{
				"number": 42,
				"title":  "Simulated pull request title",
				"draft":  false,
				"head": map[string]any{
					"ref": branch,
					"sha": fullSHA,
				},
				"base": map[string]any{
					"ref": "main",
					"sha": "0000000000000000000000000000000000000000",
				},
				"user": map[string]any{
					"login": opts.Actor,
				},
			},
		}
	case "release":
		eventData = map[string]any{
			"action": "published",
			"release": map[string]any{
				"tag_name":   "v1.0.0",
				"name":       "Release v1.0.0",
				"prerelease": false,
				"draft":      false,
			},
		}
	case "workflow_dispatch":
		eventData = map[string]any{
			"inputs": map[string]any{},
		}
	}

	// Merge custom context file if provided
	if opts.ContextFile != "" {
		data, err := os.ReadFile(opts.ContextFile)
		if err != nil {
			return nil, fmt.Errorf("reading context file: %w", err)
		}
		var customEvent map[string]any
		if err := json.Unmarshal(data, &customEvent); err != nil {
			return nil, fmt.Errorf("parsing context json file: %w", err)
		}
		for k, v := range customEvent {
			eventData[k] = v
		}
	}

	data := map[string]any{
		"github": map[string]any{
			"action":          "simulate",
			"actor":           opts.Actor,
			"event_name":      opts.Event,
			"event":           eventData,
			"ref":             opts.Ref,
			"ref_name":        strings.TrimPrefix(strings.TrimPrefix(opts.Ref, "refs/heads/"), "refs/tags/"),
			"repository":      opts.Repository,
			"repository_owner": strings.Split(opts.Repository, "/")[0],
			"sha":             fullSHA,
			"run_id":          "1234567890",
			"run_number":      "1",
			"workflow":        "Workflow",
			"workspace":       "/github/workspace",
			"server_url":      "https://github.com",
			"api_url":         "https://api.github.com",
			"graphql_url":     "https://api.github.com/graphql",
		},
		"runner": map[string]any{
			"name":      "Hosted Agent",
			"os":        "Linux",
			"arch":      "X64",
			"temp":      "/tmp",
			"tool_cache": "/opt/hostedtoolcache",
		},
		"vars":    map[string]any{},
		"secrets": map[string]any{"GITHUB_TOKEN": "mock-token-xyz"},
		"inputs":  map[string]any{},
	}
	_ = shortSHA
	return data, nil
}

func topologicalSortJobs(wf *workflow.Workflow) ([]string, error) {
	inDegree := make(map[string]int)
	adj := make(map[string][]string)

	for jobID := range wf.Jobs {
		inDegree[jobID] = 0
	}

	for jobID, job := range wf.Jobs {
		for _, dep := range job.Needs {
			if _, exists := wf.Jobs[dep]; exists {
				adj[dep] = append(adj[dep], jobID)
				inDegree[jobID]++
			}
		}
	}

	var queue []string
	for _, id := range wf.JobOrder {
		if inDegree[id] == 0 {
			queue = append(queue, id)
		}
	}

	var ordered []string
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		ordered = append(ordered, curr)

		for _, neighbor := range adj[curr] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(ordered) != len(wf.Jobs) {
		// Circular dependency exists, fallback to file order for simulation
		return wf.JobOrder, nil
	}

	return ordered, nil
}

func cloneMap(m map[string]any) map[string]any {
	res := make(map[string]any, len(m))
	for k, v := range m {
		if sub, ok := v.(map[string]any); ok {
			res[k] = cloneMap(sub)
		} else {
			res[k] = v
		}
	}
	return res
}

// FormatSimulation formats simulation result as readable tree.
func FormatSimulation(res *SimulationResult) string {
	var buf bytes.Buffer

	buf.WriteString("======================================================================\n")
	buf.WriteString(fmt.Sprintf(" WORKFLOW SIMULATION: %s (%s)\n", res.WorkflowName, res.WorkflowPath))
	buf.WriteString("======================================================================\n")

	if !res.Triggered {
		buf.WriteString(fmt.Sprintf("  Status: NOT TRIGGERED\n  Reason: %s\n", res.TriggerReason))
		return buf.String()
	}

	buf.WriteString(fmt.Sprintf("  Trigger: %s\n\n", res.TriggerReason))

	for _, job := range res.Jobs {
		matrixDesc := ""
		if len(job.Matrix) > 0 {
			matrixDesc = fmt.Sprintf(" [%s]", job.Matrix.KeyString())
		}

		statusColor := ""
		switch job.Status {
		case StatusWillRun:
			statusColor = "[WILL RUN]"
		case StatusSkipped:
			statusColor = "[SKIPPED] "
		case StatusFailed:
			statusColor = "[FAILED]  "
		}

		buf.WriteString(fmt.Sprintf("%s Job: %s%s\n", statusColor, job.JobID, matrixDesc))
		if job.Reason != "" {
			buf.WriteString(fmt.Sprintf("         Reason: %s\n", job.Reason))
		}

		for _, step := range job.Steps {
			stepStatus := ""
			switch step.Status {
			case StatusWillRun:
				stepStatus = "  ✓ [WILL RUN]"
			case StatusSkipped:
				stepStatus = "  - [SKIPPED] "
			case StatusFailed:
				stepStatus = "  ✗ [FAILED]  "
			}
			buf.WriteString(fmt.Sprintf("    %s %s\n", stepStatus, step.Name))
			if step.Reason != "" {
				buf.WriteString(fmt.Sprintf("                   (reason: %s)\n", step.Reason))
			}
		}
		buf.WriteString("\n")
	}

	return buf.String()
}

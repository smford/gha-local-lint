package rules

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/asc/gha-local-lint/internal/workflow"
)

var KnownActionPins = map[string]string{
	"actions/checkout@v4":          "actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2",
	"actions/checkout@v3":          "actions/checkout@f43a0e5ff2bd294095638e18286ca9a3d1956744 # v3.6.0",
	"actions/setup-go@v5":          "actions/setup-go@41dfa10bad2bb2ae585af6ee5bb4d7d973ad74ed # v5.1.0",
	"actions/setup-go@v4":          "actions/setup-go@0c52d547c9bc32b1aa3301fd7a9cb496313a4491 # v4.1.0",
	"actions/setup-node@v4":        "actions/setup-node@39370e3970a6d050c480ffad4ff0ed4d3fdee5af # v4.1.0",
	"actions/upload-artifact@v4":   "actions/upload-artifact@b4b15b8c7c6ac21ea08fcf65892d2ee8f75cf882 # v4.4.3",
	"actions/download-artifact@v4": "actions/download-artifact@fa0a91b85d4f404e444e00e005971372dc801d16 # v4.1.8",
	"actions/cache@v4":             "actions/cache@6849a6489940f00c2f30c0fb92c62741ac857f98 # v4.1.2",
}

// FixFile applies safe automatic remediations to a workflow file:
// 1. Pins known unpinned actions to immutable commit SHAs.
// 2. Extracts script injection expressions into environment variables.
// 3. Upgrades deprecated or retired runner labels.
// 4. Resolves self-referencing job outputs (needs.<this_job>.outputs -> steps.<step>.outputs).
// 5. Injects top-level permissions block (permissions: contents: read) when missing.
// 6. Injects safe job timeout-minutes (timeout-minutes: 60) when missing.
func FixFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	content := string(data)
	hasCRLF := strings.Contains(content, "\r\n")
	if hasCRLF {
		content = strings.ReplaceAll(content, "\r\n", "\n")
	}

	totalFixed := 0

	// 1. Fix unpinned known actions
	newContent, n := fixKnownActionPins(content)
	content = newContent
	totalFixed += n

	// 2. Fix script injection in inline run: lines
	newContent, n = fixScriptInjections(content)
	content = newContent
	totalFixed += n

	// 3. Fix deprecated runner labels
	newContent, n = fixDeprecatedRunners(content)
	content = newContent
	totalFixed += n

	// Parse workflow AST to guide structural fixes (permissions, timeouts, self-references)
	wf, err := workflow.ParseWorkflowBytes(path, []byte(content))
	if err == nil && wf != nil {
		// 4. Fix self-referencing job outputs
		newContent, n = fixSelfReferencingJobOutputs(content, wf)
		content = newContent
		totalFixed += n

		// Re-parse if modified
		if n > 0 {
			if updatedWf, err2 := workflow.ParseWorkflowBytes(path, []byte(content)); err2 == nil {
				wf = updatedWf
			}
		}

		// 5. Fix missing job timeout-minutes
		newContent, n = fixMissingTimeoutMinutes(content, wf)
		content = newContent
		totalFixed += n

		// Re-parse if modified
		if n > 0 {
			if updatedWf, err2 := workflow.ParseWorkflowBytes(path, []byte(content)); err2 == nil {
				wf = updatedWf
			}
		}

		// 6. Fix missing top-level permissions block
		newContent, n = fixMissingWorkflowPermissions(content, wf)
		content = newContent
		totalFixed += n
	}

	if totalFixed > 0 {
		outBytes := []byte(content)
		if hasCRLF {
			outBytes = []byte(strings.ReplaceAll(content, "\n", "\r\n"))
		}
		if err := os.WriteFile(path, outBytes, 0644); err != nil {
			return 0, err
		}
	}

	return totalFixed, nil
}

func fixKnownActionPins(content string) (string, int) {
	fixed := 0
	for unpinned, pinned := range KnownActionPins {
		usesTarget := "uses: " + unpinned
		if strings.Contains(content, usesTarget) {
			content = strings.ReplaceAll(content, usesTarget, "uses: "+pinned)
			fixed++
		}
	}
	return content, fixed
}

func fixScriptInjections(content string) (string, int) {
	fixed := 0
	runInjectionRe := regexp.MustCompile(`(?m)^([ \t]*)(?:-[ \t]+)?run:[ \t]*(.*?\$\{\{\s*(github\.(?:event\.[a-zA-Z0-9_.]+|head_ref))\s*\}\}.*)$`)
	if !runInjectionRe.MatchString(content) {
		return content, 0
	}

	content = runInjectionRe.ReplaceAllStringFunc(content, func(match string) string {
		sub := runInjectionRe.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		indent := sub[1]
		hasDash := strings.Contains(match, "- run:")
		runText := sub[2]

		exprRe := regexp.MustCompile(`\$\{\{\s*(github\.(?:event\.[a-zA-Z0-9_.]+|head_ref))\s*\}\}`)
		innerMatch := exprRe.FindStringSubmatch(runText)
		if len(innerMatch) < 2 {
			return match
		}
		fullExpr := innerMatch[0]
		propPath := innerMatch[1]

		varName := "INPUT_" + strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(propPath, "github.event.", ""), ".", "_"))
		varName = strings.ReplaceAll(varName, "GITHUB_", "")

		replacedRun := strings.Replace(runText, fullExpr, "$"+varName, 1)

		var buf bytes.Buffer
		if hasDash {
			buf.WriteString(fmt.Sprintf("%s- run: |\n", indent))
			buf.WriteString(fmt.Sprintf("%s    %s\n", indent, replacedRun))
			buf.WriteString(fmt.Sprintf("%s  env:\n", indent))
			buf.WriteString(fmt.Sprintf("%s    %s: %s", indent, varName, fullExpr))
		} else {
			buf.WriteString(fmt.Sprintf("%srun: |\n", indent))
			buf.WriteString(fmt.Sprintf("%s  %s\n", indent, replacedRun))
			buf.WriteString(fmt.Sprintf("%senv:\n", indent))
			buf.WriteString(fmt.Sprintf("%s  %s: %s", indent, varName, fullExpr))
		}
		fixed++
		return buf.String()
	})

	return content, fixed
}

func fixDeprecatedRunners(content string) (string, int) {
	fixed := 0
	runnerReplacements := map[string]string{
		"ubuntu-18.04": "ubuntu-22.04",
		"ubuntu-20.04": "ubuntu-22.04",
		"macos-10.15":  "macos-13",
		"macos-11":     "macos-13",
		"macos-12":     "macos-14",
		"windows-2016": "windows-2022",
		"windows-2019": "windows-2022",
	}

	for oldRunner, newRunner := range runnerReplacements {
		patterns := []string{
			"runs-on: " + oldRunner,
			"runs-on: [" + oldRunner + "]",
			"runs-on: ['" + oldRunner + "']",
			"runs-on: [\"" + oldRunner + "\"]",
			"- " + oldRunner,
			"'" + oldRunner + "'",
			"\"" + oldRunner + "\"",
		}
		for _, pat := range patterns {
			if strings.Contains(content, pat) {
				repl := strings.ReplaceAll(pat, oldRunner, newRunner)
				content = strings.ReplaceAll(content, pat, repl)
				fixed++
			}
		}
	}
	return content, fixed
}

func fixSelfReferencingJobOutputs(content string, wf *workflow.Workflow) (string, int) {
	if len(wf.JobOrder) == 0 {
		return content, 0
	}

	lines := strings.Split(content, "\n")
	fixed := 0

	for i, jobID := range wf.JobOrder {
		job := wf.Jobs[jobID]
		if len(job.Outputs) == 0 {
			continue
		}

		startLine := job.Pos.Line - 1
		endLine := len(lines)
		if i+1 < len(wf.JobOrder) {
			nextJob := wf.Jobs[wf.JobOrder[i+1]]
			endLine = nextJob.Pos.Line - 1
			if endLine < startLine {
				endLine = len(lines)
			}
		}

		for outKey, outExpr := range job.Outputs {
			cleanExpr := strings.TrimSpace(outExpr.Value)
			cleanExpr = strings.TrimPrefix(cleanExpr, "${{")
			cleanExpr = strings.TrimSuffix(cleanExpr, "}}")
			cleanExpr = strings.TrimSpace(cleanExpr)

			if !strings.HasPrefix(cleanExpr, "steps.") {
				continue
			}

			selfRefTarget := fmt.Sprintf("needs.%s.outputs.%s", jobID, outKey)
			for lineIdx := startLine; lineIdx < endLine && lineIdx < len(lines); lineIdx++ {
				if strings.Contains(lines[lineIdx], selfRefTarget) {
					lines[lineIdx] = strings.ReplaceAll(lines[lineIdx], selfRefTarget, cleanExpr)
					fixed++
				}
			}
		}
	}

	return strings.Join(lines, "\n"), fixed
}

func fixMissingTimeoutMinutes(content string, wf *workflow.Workflow) (string, int) {
	type jobToFix struct {
		id     string
		line   int
		indent string
	}

	var jobs []jobToFix
	for _, jobID := range wf.JobOrder {
		job := wf.Jobs[jobID]
		if job.TimeoutMinutes == nil && job.Uses == "" && job.Pos.Line > 0 {
			indentLen := job.Pos.Column - 1
			if indentLen <= 0 {
				indentLen = 4
			}
			jobs = append(jobs, jobToFix{
				id:     jobID,
				line:   job.Pos.Line,
				indent: strings.Repeat(" ", indentLen),
			})
		}
	}

	if len(jobs) == 0 {
		return content, 0
	}

	// Sort jobs from bottom to top so line insertions don't invalidate earlier line indices
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].line > jobs[j].line
	})

	lines := strings.Split(content, "\n")
	fixed := 0

	for _, j := range jobs {
		lineIdx := j.line - 1
		if lineIdx < 0 || lineIdx >= len(lines) {
			continue
		}

		newLine := j.indent + "timeout-minutes: 60"
		// Insert right before the first property of the job (at lineIdx)
		lines = append(lines[:lineIdx], append([]string{newLine}, lines[lineIdx:]...)...)
		fixed++
	}

	return strings.Join(lines, "\n"), fixed
}

func fixMissingWorkflowPermissions(content string, wf *workflow.Workflow) (string, int) {
	if wf.Permissions != nil {
		return content, 0
	}

	lines := strings.Split(content, "\n")
	jobsIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")
		if trimmed == "jobs:" || strings.HasPrefix(trimmed, "jobs:") && len(getLeadingWhitespace(line)) == 0 {
			jobsIdx = i
			break
		}
	}

	if jobsIdx < 0 {
		return content, 0
	}

	permBlock := []string{
		"permissions:",
		"  contents: read",
		"",
	}

	lines = append(lines[:jobsIdx], append(permBlock, lines[jobsIdx:]...)...)
	return strings.Join(lines, "\n"), 1
}

func getLeadingWhitespace(s string) string {
	idx := 0
	for idx < len(s) && (s[idx] == ' ' || s[idx] == '\t') {
		idx++
	}
	return s[:idx]
}

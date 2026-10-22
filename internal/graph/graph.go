package graph

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/asc/gha-local-lint/internal/matrix"
	"github.com/asc/gha-local-lint/internal/workflow"
)

type Format string

const (
	FormatMermaid Format = "mermaid"
	FormatDOT     Format = "dot"
)

// GenerateMermaid generates a Mermaid.js flowchart (flowchart TD) of the workflow job DAG.
func GenerateMermaid(wf *workflow.Workflow) string {
	var buf bytes.Buffer
	buf.WriteString("```mermaid\n")
	buf.WriteString("flowchart TD\n")

	for _, jobID := range wf.JobOrder {
		job := wf.Jobs[jobID]
		jobTitle := job.Name
		if jobTitle == "" {
			jobTitle = jobID
		}

		details := ""
		if job.Strategy != nil {
			combos, _ := matrix.Expand(job.Strategy)
			if len(combos) > 1 {
				details = fmt.Sprintf(" (%d matrix jobs)", len(combos))
			}
		}

		cleanTitle := strings.ReplaceAll(jobTitle+details, `"`, `'`)
		buf.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", sanitizeID(jobID), cleanTitle))
	}

	for _, jobID := range wf.JobOrder {
		job := wf.Jobs[jobID]
		for _, needed := range job.Needs {
			buf.WriteString(fmt.Sprintf("    %s --> %s\n", sanitizeID(needed), sanitizeID(jobID)))
		}
	}

	buf.WriteString("```\n")
	return buf.String()
}

// GenerateDOT generates a Graphviz DOT representation of the workflow job DAG.
func GenerateDOT(wf *workflow.Workflow) string {
	var buf bytes.Buffer
	buf.WriteString("digraph G {\n")
	buf.WriteString("    rankdir=TD;\n")
	buf.WriteString("    node [shape=box, style=rounded, fontname=\"Arial\"];\n")

	for _, jobID := range wf.JobOrder {
		job := wf.Jobs[jobID]
		label := job.Name
		if label == "" {
			label = jobID
		}
		if job.Strategy != nil {
			combos, _ := matrix.Expand(job.Strategy)
			if len(combos) > 1 {
				label += fmt.Sprintf(" (%d jobs)", len(combos))
			}
		}
		cleanLabel := strings.ReplaceAll(label, `"`, `\"`)
		buf.WriteString(fmt.Sprintf("    \"%s\" [label=\"%s\"];\n", jobID, cleanLabel))
	}

	for _, jobID := range wf.JobOrder {
		job := wf.Jobs[jobID]
		for _, needed := range job.Needs {
			buf.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\";\n", needed, jobID))
		}
	}

	buf.WriteString("}\n")
	return buf.String()
}

func sanitizeID(id string) string {
	return strings.ReplaceAll(strings.ReplaceAll(id, "-", "_"), ".", "_")
}

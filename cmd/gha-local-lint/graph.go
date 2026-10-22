package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/asc/gha-local-lint/internal/graph"
	"github.com/asc/gha-local-lint/internal/workflow"
)

func runGraph(args []string) int {
	fs := flag.NewFlagSet("graph", flag.ExitOnError)
	format := fs.String("format", "mermaid", "Graph format: mermaid or dot")
	files, err := parseFlagsAndArgs(fs, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Flag error: %v\n", err)
		return 1
	}

	if len(files) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gha-local-lint graph <workflow.yml> [--format=mermaid|dot]")
		return 1
	}

	workflowPath := files[0]
	wf, err := workflow.ParseWorkflow(workflowPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading workflow %s: %v\n", workflowPath, err)
		return 1
	}

	switch strings.ToLower(*format) {
	case "dot":
		fmt.Print(graph.GenerateDOT(wf))
	default:
		fmt.Print(graph.GenerateMermaid(wf))
	}

	return 0
}

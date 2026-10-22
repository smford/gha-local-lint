package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/asc/gha-local-lint/internal/cost"
	"github.com/asc/gha-local-lint/internal/workflow"
)

func runCost(args []string) int {
	fs := flag.NewFlagSet("cost", flag.ExitOnError)
	duration := fs.Int("duration", 10, "Average job duration in minutes")
	runs := fs.Int("runs", 100, "Estimated workflow runs per month")
	format := fs.String("format", "table", "Output format: table or json")
	files, err := parseFlagsAndArgs(fs, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Flag error: %v\n", err)
		return 1
	}

	if len(files) == 0 {
		matches1, _ := filepath.Glob(".github/workflows/*.yml")
		matches2, _ := filepath.Glob(".github/workflows/*.yaml")
		files = append(matches1, matches2...)
		if len(files) == 0 {
			fmt.Println("No workflow files found in .github/workflows/*.yml. Specify paths explicitly.")
			return 0
		}
	}

	costOpts := cost.CostOptions{
		DefaultDurationMinutes: *duration,
		RunsPerMonth:           *runs,
	}

	for _, path := range files {
		wf, err := workflow.ParseWorkflow(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", path, err)
			continue
		}

		est := cost.EstimateCost(wf, costOpts)
		if strings.ToLower(*format) == "json" {
			jsonStr, err := cost.FormatCostJSON(est)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
				return 1
			}
			fmt.Println(jsonStr)
		} else {
			fmt.Print(cost.FormatCostTable(est, *runs))
			fmt.Println()
		}
	}

	return 0
}

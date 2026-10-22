package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/asc/gha-local-lint/internal/matrix"
	"github.com/asc/gha-local-lint/internal/rules"
	"github.com/asc/gha-local-lint/internal/simulator"
	"github.com/asc/gha-local-lint/internal/workflow"
)

var (
	version = "0.3.0"
	commit  = "dev"
	date    = "unknown"
	license = "MIT License"
	repoURL = "https://github.com/smford/gha-local-lint"
	docsURL = "https://smford.github.io/gha-local-lint"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcmd := os.Args[1]
	args := os.Args[2:]

	switch subcmd {
	case "check":
		os.Exit(runCheck(args))
	case "simulate":
		os.Exit(runSimulate(args))
	case "matrix":
		os.Exit(runMatrix(args))
	case "version", "--version", "-v":
		format := "text"
		if len(args) > 0 && (args[0] == "--format=json" || args[0] == "-json") {
			format = "json"
		}
		printVersion(format)
		os.Exit(0)
	case "help", "--help", "-h":
		printUsage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand %q. Run 'gha-local-lint --help' for usage.\n", subcmd)
		os.Exit(1)
	}
}

func printVersion(format string) {
	if format == "json" {
		data, _ := json.MarshalIndent(map[string]string{
			"version": version,
			"commit":  commit,
			"date":    date,
			"license": license,
			"repo":    repoURL,
			"docs":    docsURL,
		}, "", "  ")
		fmt.Println(string(data))
		return
	}
	fmt.Printf("gha-local-lint %s\n", version)
	fmt.Printf("  Commit:  %s\n", commit)
	fmt.Printf("  Date:    %s\n", date)
	fmt.Printf("  License: %s\n", license)
	fmt.Printf("  Repo:    %s\n", repoURL)
	fmt.Printf("  Docs:    %s\n", docsURL)
}

func printUsage() {
	fmt.Printf("gha-local-lint %s — Offline GitHub Actions Expression & Matrix Simulator\n", version)
	fmt.Printf("License: %s | Source: %s\n\n", license, repoURL)
	fmt.Println(`Usage:
  gha-local-lint <command> [arguments]

Commands:
  check     Statically validate workflow files for DAG cycles, broken references, and security issues
  simulate  Dry-run simulate workflow execution against synthetic event contexts
  matrix    Inspect and display expanded matrix combinations
  version   Display version, license, and repository information

Run 'gha-local-lint <command> --help' for details on a specific command.`)
}

func parseFlagsAndArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var posArgs []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) > 0 {
			posArgs = append(posArgs, args[0])
			args = args[1:]
		}
	}
	return posArgs, nil
}

func runCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	requireSHAPin := fs.Bool("require-sha-pin", false, "Fail with an error if third-party actions are not pinned to a 40-character commit SHA")
	failOnWarning := fs.Bool("fail-on-warning", false, "Exit with non-zero status if warnings are found")
	format := fs.String("format", "text", "Output format: text, json, or sarif")
	fix := fs.Bool("fix", false, "Automatically apply safe remediations (e.g. pin known actions, rewrite script injections)")
	paths, err := parseFlagsAndArgs(fs, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Flag error: %v\n", err)
		return 1
	}
	if len(paths) == 0 {
		// Look for .github/workflows/*.yml and *.yaml
		matches1, _ := filepath.Glob(".github/workflows/*.yml")
		matches2, _ := filepath.Glob(".github/workflows/*.yaml")
		paths = append(matches1, matches2...)
		if len(paths) == 0 {
			fmt.Println("No workflow files found in .github/workflows/*.yml. Specify paths explicitly.")
			return 0
		}
	}

	// Expand glob patterns if specified on command line
	var fileList []string
	for _, p := range paths {
		matches, err := filepath.Glob(p)
		if err == nil && len(matches) > 0 {
			fileList = append(fileList, matches...)
		} else {
			fileList = append(fileList, p)
		}
	}

	// Apply fixes if --fix is set
	if *fix {
		totalFixed := 0
		for _, path := range fileList {
			count, err := rules.FixFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error applying fixes to %s: %v\n", path, err)
			} else if count > 0 {
				fmt.Printf("✓ Applied %d remediation(s) to %s\n", count, path)
				totalFixed += count
			}
		}
		if totalFixed > 0 {
			fmt.Printf("Total fixes applied: %d\n\n", totalFixed)
		}
	}

	opts := rules.LintOptions{
		RequireSHAPin: *requireSHAPin,
	}

	var allFindings []rules.Finding
	hasErrors := false
	hasWarnings := false

	for _, path := range fileList {
		wf, err := workflow.ParseWorkflow(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", path, err)
			hasErrors = true
			continue
		}
		findings := rules.Lint(wf, opts)
		allFindings = append(allFindings, findings...)
		for _, f := range findings {
			if f.Severity == rules.SeverityError {
				hasErrors = true
			} else if f.Severity == rules.SeverityWarning {
				hasWarnings = true
			}
		}
	}

	switch strings.ToLower(*format) {
	case "json":
		data, err := json.MarshalIndent(allFindings, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error serializing JSON: %v\n", err)
			return 1
		}
		fmt.Println(string(data))
	case "sarif":
		data, err := rules.FormatSARIF(allFindings, version)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating SARIF: %v\n", err)
			return 1
		}
		fmt.Println(string(data))
	default:
		if len(allFindings) == 0 {
			fmt.Printf("✓ Checked %d workflow file(s): 0 issues found\n", len(fileList))
		} else {
			for _, f := range allFindings {
				fmt.Println(f.String())
			}
			fmt.Printf("\nTotal issues: %d across %d file(s)\n", len(allFindings), len(fileList))
		}
	}

	if hasErrors || (*failOnWarning && hasWarnings) {
		return 1
	}
	return 0
}

func runSimulate(args []string) int {
	fs := flag.NewFlagSet("simulate", flag.ExitOnError)
	event := fs.String("event", "push", "GitHub event name: push, pull_request, release, schedule, workflow_dispatch")
	ref := fs.String("ref", "refs/heads/main", "Git ref to simulate (e.g. refs/heads/main, refs/pull/42/merge)")
	actor := fs.String("actor", "octocat", "GitHub actor username")
	repo := fs.String("repo", "octocat/hello-world", "GitHub repository (owner/repo)")
	contextFile := fs.String("context-file", "", "Path to custom JSON file to merge into github.event context")
	failJobRaw := fs.String("fail-job", "", "Comma-separated list of job IDs to simulate as failed")
	mockOutputRaw := fs.String("mock-output", "", "Mock job outputs (e.g. setup.matrix='[\"a\",\"b\"]' or key=val)")
	diffAgainst := fs.String("diff-against", "", "Git ref or branch to diff against to filter triggers via changed paths")
	detectChanges := fs.Bool("detect-changes", false, "Use uncommitted local git changes to evaluate workflow paths filters")
	interactive := fs.Bool("interactive", false, "Launch interactive workflow simulator")
	format := fs.String("format", "text", "Output format: text or json")
	files, err := parseFlagsAndArgs(fs, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Flag error: %v\n", err)
		return 1
	}

	if len(files) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gha-local-lint simulate <workflow.yml> [flags]")
		return 1
	}

	workflowPath := files[0]
	wf, err := workflow.ParseWorkflow(workflowPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading workflow %s: %v\n", workflowPath, err)
		return 1
	}

	var failJobs []string
	if *failJobRaw != "" {
		for _, fj := range strings.Split(*failJobRaw, ",") {
			trimmed := strings.TrimSpace(fj)
			if trimmed != "" {
				failJobs = append(failJobs, trimmed)
			}
		}
	}

	mockOutputs := make(map[string]any)
	if *mockOutputRaw != "" {
		for _, item := range strings.Split(*mockOutputRaw, ",") {
			parts := strings.SplitN(item, "=", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.TrimSpace(parts[1])
				// Try unmarshaling if JSON
				var jsonVal any
				if err := json.Unmarshal([]byte(v), &jsonVal); err == nil {
					mockOutputs[k] = jsonVal
				} else {
					mockOutputs[k] = v
				}
			}
		}
	}

	var changedFiles []string
	if *detectChanges || *diffAgainst != "" {
		gitFiles, err := getGitChangedFiles(*diffAgainst)
		if err == nil {
			changedFiles = gitFiles
		}
	}

	simOpts := simulator.SimulateOptions{
		Event:        *event,
		Ref:          *ref,
		Actor:        *actor,
		Repository:   *repo,
		ContextFile:  *contextFile,
		FailJobs:     failJobs,
		MockOutputs:  mockOutputs,
		ChangedFiles: changedFiles,
	}

	if *interactive {
		runInteractiveSimulator(wf, simOpts)
		return 0
	}

	res, err := simulator.Simulate(wf, simOpts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Simulation error: %v\n", err)
		return 1
	}

	if *format == "json" {
		data, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error serializing JSON: %v\n", err)
			return 1
		}
		fmt.Println(string(data))
	} else {
		fmt.Print(simulator.FormatSimulation(res))
	}

	return 0
}

func getGitChangedFiles(diffRef string) ([]string, error) {
	var cmd *exec.Cmd
	if diffRef != "" {
		cmd = exec.Command("git", "diff", "--name-only", diffRef)
	} else {
		cmd = exec.Command("git", "diff", "--name-only", "HEAD")
	}
	out, err := cmd.Output()
	if err != nil {
		// Fallback to porcelain status
		cmd = exec.Command("git", "status", "--porcelain")
		out, err = cmd.Output()
		if err != nil {
			return nil, err
		}
		var files []string
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if len(line) > 3 {
				files = append(files, strings.TrimSpace(line[3:]))
			}
		}
		return files, nil
	}

	var files []string
	for _, f := range strings.Split(string(out), "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

func runInteractiveSimulator(wf *workflow.Workflow, baseOpts simulator.SimulateOptions) {
	scanner := bufio.NewScanner(os.Stdin)
	currentOpts := baseOpts

	for {
		res, err := simulator.Simulate(wf, currentOpts)
		if err != nil {
			fmt.Printf("Simulation error: %v\n", err)
		} else {
			fmt.Print(simulator.FormatSimulation(res))
		}

		fmt.Println("\n--- Interactive Simulator Menu ---")
		fmt.Printf("Current: Event=%s, Ref=%s, FailJobs=%v\n", currentOpts.Event, currentOpts.Ref, currentOpts.FailJobs)
		fmt.Println("1) Switch Event (push / pull_request / release / workflow_dispatch)")
		fmt.Println("2) Change Git Ref (e.g. refs/heads/main, refs/heads/feature/x)")
		fmt.Println("3) Toggle Failing Job")
		fmt.Println("q) Quit")
		fmt.Print("\nSelect option: ")

		if !scanner.Scan() {
			break
		}
		choice := strings.TrimSpace(scanner.Text())

		switch choice {
		case "1":
			fmt.Print("Enter event name: ")
			if scanner.Scan() {
				ev := strings.TrimSpace(scanner.Text())
				if ev != "" {
					currentOpts.Event = ev
				}
			}
		case "2":
			fmt.Print("Enter git ref: ")
			if scanner.Scan() {
				r := strings.TrimSpace(scanner.Text())
				if r != "" {
					currentOpts.Ref = r
				}
			}
		case "3":
			fmt.Print("Enter job ID to fail (or empty to clear): ")
			if scanner.Scan() {
				j := strings.TrimSpace(scanner.Text())
				if j == "" {
					currentOpts.FailJobs = nil
				} else {
					currentOpts.FailJobs = []string{j}
				}
			}
		case "q", "quit", "exit":
			return
		}
	}
}

func runMatrix(args []string) int {
	fs := flag.NewFlagSet("matrix", flag.ExitOnError)
	jobFilter := fs.String("job", "", "Filter matrix expansion to a specific job ID")
	format := fs.String("format", "table", "Output format: table or json")
	files, err := parseFlagsAndArgs(fs, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Flag error: %v\n", err)
		return 1
	}

	if len(files) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: gha-local-lint matrix <workflow.yml> [--job <id>] [--format <table|json>]")
		return 1
	}

	workflowPath := files[0]
	wf, err := workflow.ParseWorkflow(workflowPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading workflow %s: %v\n", workflowPath, err)
		return 1
	}

	totalJobs := 0
	type JobMatrixOutput struct {
		JobID        string               `json:"job_id"`
		JobName      string               `json:"job_name"`
		TotalCount   int                  `json:"total_combinations"`
		Combinations []matrix.Combination `json:"combinations"`
	}
	var jsonOutputs []JobMatrixOutput

	for _, jobID := range wf.JobOrder {
		job := wf.Jobs[jobID]
		if *jobFilter != "" && !strings.EqualFold(jobID, *jobFilter) {
			continue
		}

		combos, err := matrix.Expand(job.Strategy)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error expanding matrix for job %s: %v\n", jobID, err)
			continue
		}

		totalJobs += len(combos)

		if *format == "json" {
			jsonOutputs = append(jsonOutputs, JobMatrixOutput{
				JobID:        jobID,
				JobName:      job.Name,
				TotalCount:   len(combos),
				Combinations: combos,
			})
		} else {
			fmt.Printf("Job: %s (defined as %q)\n", jobID, job.Name)
			if job.Strategy == nil || (len(job.Strategy.Dimensions) == 0 && len(job.Strategy.Include) == 0) {
				fmt.Println("  No matrix strategy defined (runs 1 single runner instance).")
				fmt.Println()
			} else {
				fmt.Print(matrix.FormatTable(combos))
				fmt.Println()
			}
		}
	}

	if *format == "json" {
		data, err := json.MarshalIndent(jsonOutputs, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error serializing JSON: %v\n", err)
			return 1
		}
		fmt.Println(string(data))
	} else {
		fmt.Printf("Grand Total Runner Jobs Across Evaluated Targets: %d\n", totalJobs)
	}

	return 0
}

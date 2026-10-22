package cost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/asc/gha-local-lint/internal/matrix"
	"github.com/asc/gha-local-lint/internal/workflow"
)

// Standard GitHub Actions runner pricing per minute (USD)
const (
	RateLinuxMinute   = 0.008
	RateWindowsMinute = 0.016
	RateMacOSMinute   = 0.064
)

type CostOptions struct {
	DefaultDurationMinutes int
	RunsPerMonth           int
}

type WorkflowCostEstimate struct {
	WorkflowPath           string             `json:"workflow_path"`
	WorkflowName           string             `json:"workflow_name"`
	TotalRunnerJobs        int                `json:"total_runner_jobs"`
	LinuxJobs              int                `json:"linux_jobs"`
	WindowsJobs            int                `json:"windows_jobs"`
	MacOSJobs              int                `json:"macos_jobs"`
	EstimatedMinutesPerRun int                `json:"estimated_minutes_per_run"`
	CostPerRunUSD          float64            `json:"cost_per_run_usd"`
	MonthlyMinutes         int                `json:"monthly_minutes"`
	MonthlyCostUSD         float64            `json:"monthly_cost_usd"`
	Jobs                   []JobCostBreakdown `json:"jobs"`
}

type JobCostBreakdown struct {
	JobID          string  `json:"job_id"`
	JobName        string  `json:"job_name"`
	RunnerCount    int     `json:"runner_count"`
	OS             string  `json:"os"`
	DurationMins   int     `json:"duration_minutes"`
	TotalMins      int     `json:"total_minutes"`
	CostPerRunUSD  float64 `json:"cost_per_run_usd"`
	MonthlyCostUSD float64 `json:"monthly_cost_usd"`
}

func EstimateCost(wf *workflow.Workflow, opts CostOptions) *WorkflowCostEstimate {
	if opts.DefaultDurationMinutes <= 0 {
		opts.DefaultDurationMinutes = 10
	}
	if opts.RunsPerMonth <= 0 {
		opts.RunsPerMonth = 100
	}

	est := &WorkflowCostEstimate{
		WorkflowPath: wf.Path,
		WorkflowName: wf.Name,
	}

	for _, jobID := range wf.JobOrder {
		job := wf.Jobs[jobID]
		duration := opts.DefaultDurationMinutes
		if job.TimeoutMinutes != nil && *job.TimeoutMinutes > 0 && *job.TimeoutMinutes < duration {
			duration = *job.TimeoutMinutes
		}

		combos, _ := matrix.Expand(job.Strategy)
		runnerCount := len(combos)
		if runnerCount == 0 {
			runnerCount = 1
		}

		// Detect OS from runs-on or matrix
		for _, c := range combos {
			detectedOS := detectOS(job.RunsOn, c)
			rate := RateLinuxMinute
			switch detectedOS {
			case "macos":
				rate = RateMacOSMinute
				est.MacOSJobs++
			case "windows":
				rate = RateWindowsMinute
				est.WindowsJobs++
			default:
				rate = RateLinuxMinute
				est.LinuxJobs++
			}

			jobRunCost := float64(duration) * rate
			est.CostPerRunUSD += jobRunCost
			est.EstimatedMinutesPerRun += duration
		}

		primaryOS := detectOS(job.RunsOn, nil)
		rate := RateLinuxMinute
		if primaryOS == "macos" {
			rate = RateMacOSMinute
		} else if primaryOS == "windows" {
			rate = RateWindowsMinute
		}

		totalMins := runnerCount * duration
		jobCostPerRun := float64(totalMins) * rate
		est.TotalRunnerJobs += runnerCount

		est.Jobs = append(est.Jobs, JobCostBreakdown{
			JobID:          jobID,
			JobName:        job.Name,
			RunnerCount:    runnerCount,
			OS:             primaryOS,
			DurationMins:   duration,
			TotalMins:      totalMins,
			CostPerRunUSD:  jobCostPerRun,
			MonthlyCostUSD: jobCostPerRun * float64(opts.RunsPerMonth),
		})
	}

	est.MonthlyMinutes = est.EstimatedMinutesPerRun * opts.RunsPerMonth
	est.MonthlyCostUSD = est.CostPerRunUSD * float64(opts.RunsPerMonth)

	return est
}

func detectOS(runsOn any, combo matrix.Combination) string {
	// Check matrix first
	if combo != nil {
		for k, v := range combo {
			lowerK := strings.ToLower(k)
			if lowerK == "os" || lowerK == "runner" {
				val := strings.ToLower(fmt.Sprintf("%v", v))
				if strings.Contains(val, "mac") {
					return "macos"
				}
				if strings.Contains(val, "win") {
					return "windows"
				}
				return "linux"
			}
		}
	}

	// Check runs-on
	val := strings.ToLower(fmt.Sprintf("%v", runsOn))
	if strings.Contains(val, "mac") {
		return "macos"
	}
	if strings.Contains(val, "win") {
		return "windows"
	}
	return "linux"
}

// FormatCostTable formats cost estimates into a visual ASCII report.
func FormatCostTable(est *WorkflowCostEstimate, runsPerMonth int) string {
	var buf bytes.Buffer

	buf.WriteString("======================================================================\n")
	buf.WriteString(fmt.Sprintf(" RUNNER COST & QUOTA ESTIMATE: %s\n", est.WorkflowName))
	buf.WriteString("======================================================================\n")
	buf.WriteString(fmt.Sprintf("Workflow: %s\n", est.WorkflowPath))
	buf.WriteString(fmt.Sprintf("Total Runner Jobs per Run: %d (Linux: %d, macOS: %d, Windows: %d)\n",
		est.TotalRunnerJobs, est.LinuxJobs, est.MacOSJobs, est.WindowsJobs))
	buf.WriteString(fmt.Sprintf("Estimated Compute per Run: %d runner-minutes (~$%.4f/run)\n",
		est.EstimatedMinutesPerRun, est.CostPerRunUSD))
	buf.WriteString(fmt.Sprintf("Monthly Estimate (%d runs): %d runner-minutes (~$%.2f/month)\n\n",
		runsPerMonth, est.MonthlyMinutes, est.MonthlyCostUSD))

	buf.WriteString(fmt.Sprintf("%-20s | %-8s | %-12s | %-12s | %-15s\n",
		"Job ID", "OS", "Runners", "Est. Mins", "Cost / Month"))
	buf.WriteString("---------------------+----------+--------------+--------------+----------------\n")

	for _, j := range est.Jobs {
		buf.WriteString(fmt.Sprintf("%-20s | %-8s | %-12d | %-12d | $%-14.2f\n",
			truncate(j.JobID, 20), j.OS, j.RunnerCount, j.TotalMins, j.MonthlyCostUSD))
	}

	return buf.String()
}

// FormatCostJSON returns formatted JSON representation of cost estimate.
func FormatCostJSON(est *WorkflowCostEstimate) (string, error) {
	bytes, err := json.MarshalIndent(est, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

package simulator

import (
	"bytes"
	"fmt"
	"html"

	"github.com/asc/gha-local-lint/internal/workflow"
)

// GenerateHTMLReport produces an offline, standalone interactive HTML simulation report.
func GenerateHTMLReport(res *SimulationResult, wf *workflow.Workflow) string {
	var buf bytes.Buffer

	willRunCount := 0
	skippedCount := 0
	failedCount := 0
	for _, j := range res.Jobs {
		switch j.Status {
		case StatusWillRun:
			willRunCount++
		case StatusSkipped:
			skippedCount++
		case StatusFailed:
			failedCount++
		}
	}

	buf.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Workflow Simulation — ` + html.EscapeString(res.WorkflowName) + `</title>
  <style>
    :root {
      --bg: #0d1117;
      --card-bg: #161b22;
      --border: #30363d;
      --text: #c9d1d9;
      --heading: #f0f6fc;
      --green: #238636;
      --red: #da3633;
      --yellow: #d29922;
      --blue: #58a6ff;
    }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
      background-color: var(--bg);
      color: var(--text);
      line-height: 1.6;
      margin: 0;
      padding: 2rem;
    }
    .container {
      max-width: 1000px;
      margin: 0 auto;
    }
    h1, h2, h3 { color: var(--heading); }
    .header {
      border-bottom: 1px solid var(--border);
      padding-bottom: 1.5rem;
      margin-bottom: 2rem;
    }
    .stats {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
      gap: 1rem;
      margin-bottom: 2rem;
    }
    .stat-card {
      background: var(--card-bg);
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 1rem;
      text-align: center;
    }
    .stat-number {
      font-size: 2rem;
      font-weight: 700;
      margin-top: 0.25rem;
    }
    .job-card {
      background: var(--card-bg);
      border: 1px solid var(--border);
      border-radius: 6px;
      margin-bottom: 1rem;
      overflow: hidden;
    }
    .job-header {
      padding: 1rem 1.25rem;
      display: flex;
      justify-content: space-between;
      align-items: center;
      cursor: pointer;
      user-select: none;
    }
    .badge {
      display: inline-block;
      padding: 0.2rem 0.6rem;
      font-size: 0.8rem;
      font-weight: 600;
      border-radius: 20px;
      text-transform: uppercase;
    }
    .badge-run { background: rgba(35, 134, 54, 0.2); color: #3fb950; border: 1px solid var(--green); }
    .badge-skip { background: rgba(210, 153, 34, 0.2); color: #e3b341; border: 1px solid var(--yellow); }
    .badge-fail { background: rgba(218, 54, 51, 0.2); color: #f85149; border: 1px solid var(--red); }
    .job-body {
      padding: 0 1.25rem 1.25rem;
      border-top: 1px solid var(--border);
      background: #0a0d12;
    }
    .step-item {
      padding: 0.6rem 0;
      border-bottom: 1px solid rgba(48, 54, 61, 0.4);
      display: flex;
      align-items: center;
      gap: 0.75rem;
    }
    .step-item:last-child { border-bottom: none; }
    .reason {
      font-size: 0.85rem;
      color: #8b949e;
      margin-left: auto;
    }
    .matrix-tags {
      font-family: ui-monospace, monospace;
      font-size: 0.85rem;
      color: var(--blue);
    }
    .search-bar {
      margin-top: 1.5rem;
      margin-bottom: 1rem;
    }
    .search-input {
      width: 100%;
      box-sizing: border-box;
      padding: 0.6rem 1rem;
      background: var(--card-bg);
      border: 1px solid var(--border);
      border-radius: 6px;
      color: var(--text);
      font-size: 0.95rem;
    }
    .search-input:focus {
      outline: none;
      border-color: var(--blue);
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>Workflow Simulation Report</h1>
      <p>Workflow: <strong>` + html.EscapeString(res.WorkflowName) + `</strong> (<code>` + html.EscapeString(res.WorkflowPath) + `</code>)</p>
      <p>Trigger: <strong>` + html.EscapeString(res.TriggerReason) + `</strong></p>
    </div>

    <div class="stats">
      <div class="stat-card">
        <div>Total Jobs</div>
        <div class="stat-number">` + fmt.Sprintf("%d", len(res.Jobs)) + `</div>
      </div>
      <div class="stat-card">
        <div style="color: #3fb950;">Will Run</div>
        <div class="stat-number" style="color: #3fb950;">` + fmt.Sprintf("%d", willRunCount) + `</div>
      </div>
      <div class="stat-card">
        <div style="color: #e3b341;">Skipped</div>
        <div class="stat-number" style="color: #e3b341;">` + fmt.Sprintf("%d", skippedCount) + `</div>
      </div>
      <div class="stat-card">
        <div style="color: #f85149;">Failed</div>
        <div class="stat-number" style="color: #f85149;">` + fmt.Sprintf("%d", failedCount) + `</div>
      </div>
    </div>

    <div class="search-bar">
      <input type="text" id="filterInput" class="search-input" placeholder="Filter jobs by ID, name, or status..." oninput="filterCards()">
    </div>

    <h2>Jobs & Step Breakdown</h2>
`)

	for idx, j := range res.Jobs {
		badgeClass := "badge-run"
		if j.Status == StatusSkipped {
			badgeClass = "badge-skip"
		} else if j.Status == StatusFailed {
			badgeClass = "badge-fail"
		}

		matrixInfo := ""
		if len(j.Matrix) > 0 {
			matrixInfo = fmt.Sprintf("<span class=\"matrix-tags\">[%s]</span>", html.EscapeString(j.Matrix.KeyString()))
		}

		buf.WriteString(fmt.Sprintf(`
    <div class="job-card">
      <div class="job-header" onclick="toggleJob('job-%d')">
        <div>
          <strong>Job: %s</strong> %s
        </div>
        <span class="badge %s">%s</span>
      </div>
      <div class="job-body" id="job-%d">
`, idx, html.EscapeString(j.JobID), matrixInfo, badgeClass, html.EscapeString(string(j.Status)), idx))

		if j.Reason != "" {
			buf.WriteString(fmt.Sprintf("<p style=\"color: #e3b341; margin-top: 0.75rem;\">⚠️ Reason: %s</p>", html.EscapeString(j.Reason)))
		}

		buf.WriteString("<div style=\"margin-top: 0.5rem;\">")
		for _, s := range j.Steps {
			stepBadge := "badge-run"
			icon := "✓"
			if s.Status == StatusSkipped {
				stepBadge = "badge-skip"
				icon = "-"
			} else if s.Status == StatusFailed {
				stepBadge = "badge-fail"
				icon = "✗"
			}

			reasonHtml := ""
			if s.Reason != "" {
				reasonHtml = fmt.Sprintf("<span class=\"reason\">%s</span>", html.EscapeString(s.Reason))
			}

			buf.WriteString(fmt.Sprintf(`
        <div class="step-item">
          <span class="badge %s">%s</span>
          <span>%s</span>
          %s
        </div>
`, stepBadge, icon, html.EscapeString(s.Name), reasonHtml))
		}
		buf.WriteString("</div></div></div>")
	}

	buf.WriteString(`
  </div>
  <script>
    function toggleJob(id) {
      var el = document.getElementById(id);
      if (el.style.display === 'none') {
        el.style.display = 'block';
      } else {
        el.style.display = 'none';
      }
    }
    function filterCards() {
      var query = document.getElementById('filterInput').value.toLowerCase();
      var cards = document.getElementsByClassName('job-card');
      for (var i = 0; i < cards.length; i++) {
        var cardText = cards[i].innerText.toLowerCase();
        if (cardText.indexOf(query) !== -1) {
          cards[i].style.display = 'block';
        } else {
          cards[i].style.display = 'none';
        }
      }
    }
  </script>
</body>
</html>
`)

	return buf.String()
}

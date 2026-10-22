package rules

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var KnownActionPins = map[string]string{
	"actions/checkout@v4":        "actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2",
	"actions/checkout@v3":        "actions/checkout@f43a0e5ff2bd294095638e18286ca9a3d1956744 # v3.6.0",
	"actions/setup-go@v5":        "actions/setup-go@41dfa10bad2bb2ae585af6ee5bb4d7d973ad74ed # v5.1.0",
	"actions/setup-go@v4":        "actions/setup-go@0c52d547c9bc32b1aa3301fd7a9cb496313a4491 # v4.1.0",
	"actions/setup-node@v4":      "actions/setup-node@39370e3970a6d050c480ffad4ff0ed4d3fdee5af # v4.1.0",
	"actions/upload-artifact@v4": "actions/upload-artifact@b4b15b8c7c6ac21ea08fcf65892d2ee8f75cf882 # v4.4.3",
	"actions/download-artifact@v4": "actions/download-artifact@fa0a91b85d4f404e444e00e005971372dc801d16 # v4.1.8",
	"actions/cache@v4":           "actions/cache@6849a6489940f00c2f30c0fb92c62741ac857f98 # v4.1.2",
}

// FixFile applies safe automatic remediations to a workflow file.
// Returns number of fixes applied and any error.
func FixFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	content := string(data)
	fixedCount := 0

	// 1. Fix unpinned known actions
	for unpinned, pinned := range KnownActionPins {
		usesTarget := "uses: " + unpinned
		if strings.Contains(content, usesTarget) {
			content = strings.ReplaceAll(content, usesTarget, "uses: "+pinned)
			fixedCount++
		}
	}

	// 2. Fix script injection in inline run: lines (matches both 'run:' and '- run:')
	runInjectionRe := regexp.MustCompile(`(?m)^([ \t]*)(?:-[ \t]+)?run:[ \t]*(.*?\$\{\{\s*(github\.(?:event\.[a-zA-Z0-9_.]+|head_ref))\s*\}\}.*)$`)
	if runInjectionRe.MatchString(content) {
		content = runInjectionRe.ReplaceAllStringFunc(content, func(match string) string {
			sub := runInjectionRe.FindStringSubmatch(match)
			if len(sub) < 3 {
				return match
			}
			indent := sub[1]
			hasDash := strings.Contains(match, "- run:")
			runText := sub[2]

			// Extract untrusted expr
			exprRe := regexp.MustCompile(`\$\{\{\s*(github\.(?:event\.[a-zA-Z0-9_.]+|head_ref))\s*\}\}`)
			innerMatch := exprRe.FindStringSubmatch(runText)
			if len(innerMatch) < 2 {
				return match
			}
			fullExpr := innerMatch[0]
			propPath := innerMatch[1]

			// Create env var name, e.g. INPUT_PULL_REQUEST_TITLE
			varName := "INPUT_" + strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(propPath, "github.event.", ""), ".", "_"))
			varName = strings.ReplaceAll(varName, "GITHUB_", "")

			replacedRun := strings.Replace(runText, fullExpr, "$"+varName, 1)

			var buf bytes.Buffer
			if hasDash {
				buf.WriteString(fmt.Sprintf("%s- env:\n", indent))
				buf.WriteString(fmt.Sprintf("%s    %s: %s\n", indent, varName, fullExpr))
				buf.WriteString(fmt.Sprintf("%s  run: %s", indent, replacedRun))
			} else {
				buf.WriteString(fmt.Sprintf("%senv:\n", indent))
				buf.WriteString(fmt.Sprintf("%s  %s: %s\n", indent, varName, fullExpr))
				buf.WriteString(fmt.Sprintf("%srun: %s", indent, replacedRun))
			}
			fixedCount++
			return buf.String()
		})
	}

	if fixedCount > 0 {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return 0, err
		}
	}

	return fixedCount, nil
}

package matrix

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Strategy struct {
	Dimensions  map[string][]any // base dimensions: os: [...], node: [...]
	Include     []map[string]any // include augmentations or new rows
	Exclude     []map[string]any // exclude filters
	FailFast    *bool
	MaxParallel int
	Expression  string // e.g. "${{ fromJSON(needs.setup.outputs.matrix) }}"
}

type Combination map[string]any

// Clone creates a shallow clone of the combination map.
func (c Combination) Clone() Combination {
	res := make(Combination, len(c))
	for k, v := range c {
		res[k] = v
	}
	return res
}

// KeyString returns a deterministic string representation for sorting or display.
// E.g. "node: 18, os: ubuntu-latest"
func (c Combination) KeyString() string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %v", k, c[k]))
	}
	return strings.Join(parts, ", ")
}

// ExpandWithEvaluator expands a strategy, resolving dynamic matrix expressions if present.
func ExpandWithEvaluator(strat *Strategy, evalFn func(string) (any, error)) ([]Combination, error) {
	if strat == nil {
		return []Combination{{}}, nil
	}

	if strat.Expression != "" && evalFn != nil {
		val, err := evalFn(strat.Expression)
		if err != nil {
			return nil, fmt.Errorf("evaluating dynamic matrix %q: %w", strat.Expression, err)
		}
		// Try parsing if string
		if s, ok := val.(string); ok {
			var parsed any
			if json.Unmarshal([]byte(s), &parsed) == nil {
				val = parsed
			}
		}

		switch v := val.(type) {
		case []any:
			var combos []Combination
			for _, item := range v {
				if m, ok := item.(map[string]any); ok {
					combo := Combination{}
					for k, cv := range m {
						combo[k] = cv
					}
					combos = append(combos, combo)
				} else {
					combos = append(combos, Combination{"value": item})
				}
			}
			return filterExcludes(combos, strat.Exclude), nil
		case map[string]any:
			// Dimensions map: "os": ["ubuntu", "macos"]
			subStrat := &Strategy{
				Dimensions: make(map[string][]any),
				Include:    strat.Include,
				Exclude:    strat.Exclude,
			}
			for k, valItem := range v {
				if slice, ok := valItem.([]any); ok {
					subStrat.Dimensions[k] = slice
				}
			}
			return Expand(subStrat)
		}
	}

	return Expand(strat)
}

// Expand computes the final list of matrix combinations according to GitHub Actions semantics.
func Expand(strat *Strategy) ([]Combination, error) {
	if strat == nil {
		return []Combination{{}}, nil
	}

	// 1. Gather dimension keys in deterministic sorted order
	var dimKeys []string
	for k := range strat.Dimensions {
		if k == "include" || k == "exclude" {
			continue
		}
		dimKeys = append(dimKeys, k)
	}
	sort.Strings(dimKeys)

	// 2. Cartesian product
	var combinations []Combination
	if len(dimKeys) > 0 {
		combinations = cartesian(strat.Dimensions, dimKeys, 0, Combination{})
	} else if len(strat.Include) > 0 {
		// Only include defined
		for _, inc := range strat.Include {
			combo := Combination{}
			for k, v := range inc {
				combo[k] = v
			}
			combinations = append(combinations, combo)
		}
		return filterExcludes(combinations, strat.Exclude), nil
	} else {
		// No matrix defined
		return []Combination{{}}, nil
	}

	// 3. Exclude filtering
	combinations = filterExcludes(combinations, strat.Exclude)

	// 4. Include processing
	for _, inc := range strat.Include {
		matchedAny := false
		for i, combo := range combinations {
			if matchesInclude(combo, inc, dimKeys) {
				matchedAny = true
				// Augment combo with include fields
				for k, v := range inc {
					combinations[i][k] = v
				}
			}
		}
		if !matchedAny {
			// Append as a new combination
			newCombo := Combination{}
			for k, v := range inc {
				newCombo[k] = v
			}
			combinations = append(combinations, newCombo)
		}
	}

	return combinations, nil
}

func cartesian(dims map[string][]any, keys []string, depth int, current Combination) []Combination {
	if depth >= len(keys) {
		return []Combination{current.Clone()}
	}

	key := keys[depth]
	values := dims[key]
	if len(values) == 0 {
		// Empty dimension means Cartesian product is empty
		return nil
	}

	var results []Combination
	for _, val := range values {
		current[key] = val
		sub := cartesian(dims, keys, depth+1, current)
		results = append(results, sub...)
	}
	return results
}

func filterExcludes(combinations []Combination, excludes []map[string]any) []Combination {
	if len(excludes) == 0 {
		return combinations
	}

	var filtered []Combination
	for _, combo := range combinations {
		excluded := false
		for _, exc := range excludes {
			if matchesExclude(combo, exc) {
				excluded = true
				break
			}
		}
		if !excluded {
			filtered = append(filtered, combo)
		}
	}
	return filtered
}

func matchesExclude(combo Combination, exc map[string]any) bool {
	if len(exc) == 0 {
		return false
	}
	for k, ev := range exc {
		cv, ok := combo[k]
		if !ok || !valuesEqual(cv, ev) {
			return false
		}
	}
	return true
}

func matchesInclude(combo Combination, inc map[string]any, dimKeys []string) bool {
	// Must match all keys in inc that are part of base dimensions
	hasAnyDimKey := false
	for _, k := range dimKeys {
		if ev, ok := inc[k]; ok {
			hasAnyDimKey = true
			cv, exists := combo[k]
			if !exists || !valuesEqual(cv, ev) {
				return false
			}
		}
	}
	return hasAnyDimKey
}

func valuesEqual(a, b any) bool {
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return strings.EqualFold(aStr, bStr)
}

// FormatTable formats combinations into an aligned ASCII table.
func FormatTable(combinations []Combination) string {
	if len(combinations) == 0 {
		return "Empty matrix (0 jobs)\n"
	}

	// Find all unique keys across combinations
	keySet := make(map[string]bool)
	for _, c := range combinations {
		for k := range c {
			keySet[k] = true
		}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Calculate column widths
	widths := make(map[string]int)
	widths["#"] = 3
	for _, k := range keys {
		widths[k] = len(k)
	}

	for i, c := range combinations {
		idxStr := fmt.Sprintf("%d", i+1)
		if len(idxStr) > widths["#"] {
			widths["#"] = len(idxStr)
		}
		for _, k := range keys {
			valStr := fmt.Sprintf("%v", c[k])
			if len(valStr) > widths[k] {
				widths[k] = len(valStr)
			}
		}
	}

	var buf bytes.Buffer
	// Header
	buf.WriteString(fmt.Sprintf("%-*s", widths["#"], "#"))
	for _, k := range keys {
		buf.WriteString(" | ")
		buf.WriteString(fmt.Sprintf("%-*s", widths[k], k))
	}
	buf.WriteString("\n")

	// Separator
	buf.WriteString(strings.Repeat("-", widths["#"]))
	for _, k := range keys {
		buf.WriteString("-+-")
		buf.WriteString(strings.Repeat("-", widths[k]))
	}
	buf.WriteString("\n")

	// Rows
	for i, c := range combinations {
		idxStr := fmt.Sprintf("%d", i+1)
		buf.WriteString(fmt.Sprintf("%-*s", widths["#"], idxStr))
		for _, k := range keys {
			buf.WriteString(" | ")
			valStr := fmt.Sprintf("%v", c[k])
			buf.WriteString(fmt.Sprintf("%-*s", widths[k], valStr))
		}
		buf.WriteString("\n")
	}

	buf.WriteString(fmt.Sprintf("\nTotal combinations: %d\n", len(combinations)))
	return buf.String()
}

// FormatJSON returns formatted JSON representation.
func FormatJSON(combinations []Combination) (string, error) {
	bytes, err := json.MarshalIndent(combinations, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

package matrix

import (
	"strings"
	"testing"
)

func TestMatrixExpansion(t *testing.T) {
	strat := &Strategy{
		Dimensions: map[string][]any{
			"os":   {"ubuntu-latest", "macos-latest", "windows-latest"},
			"node": {18, 20},
		},
		Exclude: []map[string]any{
			{"os": "windows-latest", "node": 18},
		},
		Include: []map[string]any{
			// Augment existing
			{"os": "ubuntu-latest", "node": 18, "npm_version": "9"},
			// New combo
			{"os": "debian-12", "node": 22},
		},
	}

	combos, err := Expand(strat)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}

	// Base 3x2 = 6
	// Exclude 1 (windows, 18) -> 5
	// Include 1 augment (ubuntu, 18) -> still 5
	// Include 1 new (debian, 22) -> 6
	if len(combos) != 6 {
		t.Fatalf("expected 6 combinations, got %d", len(combos))
	}

	// Verify windows + 18 is not present
	for _, c := range combos {
		if c["os"] == "windows-latest" && c["node"] == 18 {
			t.Errorf("found excluded combination: %v", c)
		}
	}

	// Verify ubuntu + 18 has npm_version 9
	foundAugment := false
	for _, c := range combos {
		if c["os"] == "ubuntu-latest" && c["node"] == 18 {
			if c["npm_version"] == "9" {
				foundAugment = true
			}
		}
	}
	if !foundAugment {
		t.Errorf("did not find augmented combo for ubuntu-latest, 18")
	}

	// Verify debian-12 combo was added
	foundDebian := false
	for _, c := range combos {
		if c["os"] == "debian-12" && c["node"] == 22 {
			foundDebian = true
		}
	}
	if !foundDebian {
		t.Errorf("did not find debian-12 include combo")
	}

	// Test formatting
	tbl := FormatTable(combos)
	if !strings.Contains(tbl, "debian-12") || !strings.Contains(tbl, "Total combinations: 6") {
		t.Errorf("table output missing expected content:\n%s", tbl)
	}

	jsonStr, err := FormatJSON(combos)
	if err != nil || !strings.Contains(jsonStr, "ubuntu-latest") {
		t.Errorf("FormatJSON failed or missing content: %s (err=%v)", jsonStr, err)
	}
}

func TestMatrixOnlyInclude(t *testing.T) {
	strat := &Strategy{
		Include: []map[string]any{
			{"color": "red", "animal": "cat"},
			{"color": "blue", "animal": "dog"},
		},
	}

	combos, err := Expand(strat)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if len(combos) != 2 {
		t.Fatalf("expected 2 combinations, got %d", len(combos))
	}
}

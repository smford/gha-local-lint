package matrix

import (
	"testing"
)

func BenchmarkMatrixCartesianExpand(b *testing.B) {
	strat := &Strategy{
		Dimensions: map[string][]any{
			"os":   {"ubuntu-latest", "macos-latest", "windows-latest"},
			"go":   {"1.21", "1.22", "1.23"},
			"arch": {"amd64", "arm64"},
		},
		Exclude: []map[string]any{
			{"os": "windows-latest", "arch": "arm64"},
		},
		Include: []map[string]any{
			{"os": "ubuntu-latest", "go": "1.23", "arch": "amd64", "race": true},
			{"os": "debian-12", "go": "1.23", "arch": "amd64"},
		},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Expand(strat)
	}
}

func BenchmarkMatrixDynamicExpand(b *testing.B) {
	strat := &Strategy{
		Expression: `${{ fromJSON('["svc-1", "svc-2", "svc-3", "svc-4"]') }}`,
	}
	evalFn := func(expr string) (any, error) {
		return []any{"svc-1", "svc-2", "svc-3", "svc-4"}, nil
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExpandWithEvaluator(strat, evalFn)
	}
}

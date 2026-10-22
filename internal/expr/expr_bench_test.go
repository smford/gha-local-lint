package expr

import (
	"testing"
)

func BenchmarkLexer(b *testing.B) {
	input := "github.ref == 'refs/heads/main' && !contains(github.event.head_commit.message, '[skip ci]')"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Tokenize(input)
	}
}

func BenchmarkParser(b *testing.B) {
	input := "github.ref == 'refs/heads/main' && !contains(github.event.head_commit.message, '[skip ci]')"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(input)
	}
}

func BenchmarkEvaluateCondition(b *testing.B) {
	input := "github.ref == 'refs/heads/main' && !contains(github.event.head_commit.message, '[skip ci]')"
	ctx := NewContext(map[string]any{
		"github": map[string]any{
			"ref": "refs/heads/main",
			"event": map[string]any{
				"head_commit": map[string]any{
					"message": "feat: amazing improvement",
				},
			},
		},
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = EvaluateCondition(input, ctx)
	}
}

func BenchmarkInterpolate(b *testing.B) {
	input := "Image tag: v${{ matrix.version }}-${{ github.actor }}"
	ctx := NewContext(map[string]any{
		"matrix": map[string]any{"version": "1.23"},
		"github": map[string]any{"actor": "octocat"},
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Interpolate(input, ctx)
	}
}

package expr

import (
	"testing"
)

func TestLexerAndParser(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"true"},
		{"'hello world'"},
		{"'it''s cool'"},
		{"42"},
		{"-3.14"},
		{"github.event.pull_request.title"},
		{"steps['build'].outputs.artifact_id"},
		{"contains(github.ref, 'main')"},
		{"!cancelled() && (success() || failure())"},
		{"github.ref == 'refs/heads/main' && !contains(github.event.head_commit.message, '[skip ci]')"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ast, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) failed: %v", tt.input, err)
			}
			if ast == nil {
				t.Fatalf("Parse(%q) returned nil AST", tt.input)
			}
		})
	}
}

func TestEvaluator(t *testing.T) {
	ctx := NewContext(map[string]any{
		"github": map[string]any{
			"ref":        "refs/heads/main",
			"actor":      "octocat",
			"repository": "octocat/hello-world",
			"event": map[string]any{
				"head_commit": map[string]any{
					"message": "feat: great feature",
				},
				"pull_request": map[string]any{
					"title": "Fix bug",
					"draft": false,
				},
			},
		},
		"matrix": map[string]any{
			"os":   "ubuntu-latest",
			"node": 20,
		},
		"env": map[string]any{
			"ENABLE_FEATURE": "true",
			"COUNT":          5,
		},
	})

	tests := []struct {
		expr     string
		expected any
	}{
		// Literals & Booleans
		{"true", true},
		{"false", false},
		{"!true", false},
		{"!false", true},
		{"42 == 42", true},
		{"42 != 43", true},
		{"'foo' == 'FOO'", true}, // case-insensitive string equality
		{"'foo' != 'bar'", true},
		{"10 > 5", true},
		{"10 <= 10", true},
		{"5 < 3", false},

		// Context lookups
		{"github.ref == 'refs/heads/main'", true},
		{"github.actor == 'OCTOCAT'", true},
		{"matrix.node == 20", true},
		{"matrix['os'] == 'ubuntu-latest'", true},

		// Complex boolean expressions
		{"github.ref == 'refs/heads/main' && !contains(github.event.head_commit.message, '[skip ci]')", true},
		{"github.ref == 'refs/heads/dev' || github.actor == 'octocat'", true},

		// String functions
		{"contains('Hello World', 'world')", true},
		{"contains('Hello World', 'xyz')", false},
		{"startsWith('refs/heads/main', 'refs/heads/')", true},
		{"endsWith('bundle.tar.gz', '.tar.gz')", true},
		{"format('Hello {0}, {1}!', 'Alice', 'Bob')", "Hello Alice, Bob!"},
		{"join(fromJSON('[\"a\", \"b\", \"c\"]'), '-')", "a-b-c"},

		// JSON functions
		{"fromJSON('{\"foo\": \"bar\"}').foo == 'bar'", true},
		{"fromJSON('42') == 42", true},

		// Null & NaN
		{"null == null", true},
		{"null == ''", false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			ast, err := Parse(tt.expr)
			if err != nil {
				t.Fatalf("Parse(%q) failed: %v", tt.expr, err)
			}
			val, err := ctx.Evaluate(ast)
			if err != nil {
				t.Fatalf("Evaluate(%q) failed: %v", tt.expr, err)
			}
			if val != tt.expected {
				t.Errorf("Evaluate(%q) = %v (%T), want %v (%T)", tt.expr, val, val, tt.expected, tt.expected)
			}
		})
	}
}

func TestEvaluateConditionStatusChecks(t *testing.T) {
	ctx := NewContext(map[string]any{
		"flag": true,
	})

	// Case 1: Job succeeded -> should evaluate condition normally
	ctx.JobStatus = StatusSuccess
	res, err := EvaluateCondition("flag == true", ctx)
	if err != nil || !res {
		t.Errorf("expected true, got %v, err=%v", res, err)
	}

	// Case 2: Job failed, condition has NO status check -> implicit && success() causes it to be false
	ctx.JobStatus = StatusFailure
	res, err = EvaluateCondition("flag == true", ctx)
	if err != nil || res {
		t.Errorf("expected false when job failed without explicit status check, got %v, err=%v", res, err)
	}

	// Case 3: Job failed, but condition has always() -> should evaluate to true
	res, err = EvaluateCondition("always() && flag == true", ctx)
	if err != nil || !res {
		t.Errorf("expected true with always(), got %v, err=%v", res, err)
	}

	// Case 4: Job failed, condition has failure() -> should evaluate to true
	res, err = EvaluateCondition("failure()", ctx)
	if err != nil || !res {
		t.Errorf("expected true with failure(), got %v, err=%v", res, err)
	}
}

func TestInterpolate(t *testing.T) {
	ctx := NewContext(map[string]any{
		"github": map[string]any{
			"actor": "octocat",
		},
		"matrix": map[string]any{
			"version": "1.20",
		},
	})

	// Full typed expression
	val, err := Interpolate("${{ matrix.version == '1.20' }}", ctx)
	if err != nil || val != true {
		t.Fatalf("expected bool true, got %v (err=%v)", val, err)
	}

	// String interpolation
	valStr, err := Interpolate("Image tag: v${{ matrix.version }} by ${{ github.actor }}", ctx)
	if err != nil || valStr != "Image tag: v1.20 by octocat" {
		t.Fatalf("expected interpolated string, got %v (err=%v)", valStr, err)
	}
}

func TestExtractReferences(t *testing.T) {
	expr := "echo '${{ github.event.pull_request.title }}' and ${{ github.head_ref }}"
	refs := ExtractReferences(expr)
	expected := []string{"github.event.pull_request.title", "github.head_ref"}
	if len(refs) != len(expected) {
		t.Fatalf("expected %d refs, got %d: %v", len(expected), len(refs), refs)
	}
	for i, ref := range expected {
		if refs[i] != ref {
			t.Errorf("refs[%d] = %q, want %q", i, refs[i], ref)
		}
	}
}

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/asc/gha-local-lint/internal/expr"
)

func runExpr(args []string) int {
	fs := flag.NewFlagSet("expr", flag.ExitOnError)
	contextRaw := fs.String("context", "", "Mock context JSON string or file path")
	interactive := fs.Bool("interactive", false, "Launch interactive expression REPL")
	format := fs.String("format", "text", "Output format: text or json")
	parsedArgs, err := parseFlagsAndArgs(fs, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Flag error: %v\n", err)
		return 1
	}

	evalCtxData := make(map[string]any)
	if *contextRaw != "" {
		rawBytes := []byte(*contextRaw)
		if _, statErr := os.Stat(*contextRaw); statErr == nil {
			if fileData, readErr := os.ReadFile(*contextRaw); readErr == nil {
				rawBytes = fileData
			}
		}
		if unmarshalErr := json.Unmarshal(rawBytes, &evalCtxData); unmarshalErr != nil {
			fmt.Fprintf(os.Stderr, "Error parsing context JSON: %v\n", unmarshalErr)
			return 1
		}
	}
	ctx := expr.NewContext(evalCtxData)

	if *interactive || len(parsedArgs) == 0 {
		runExprREPL(ctx)
		return 0
	}

	exprStr := strings.Join(parsedArgs, " ")
	ast, err := expr.Parse(exprStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		return 1
	}

	val, err := ctx.Evaluate(ast)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Evaluation error: %v\n", err)
		return 1
	}

	if *format == "json" {
		out, _ := json.MarshalIndent(map[string]any{
			"expression": exprStr,
			"type":       fmt.Sprintf("%T", val),
			"value":      val,
		}, "", "  ")
		fmt.Println(string(out))
	} else {
		fmt.Printf("=> %v (%T)\n", val, val)
	}

	return 0
}

func runExprREPL(ctx *expr.Context) {
	fmt.Println("gha-local-lint Expression REPL")
	fmt.Println("Type expressions like: contains('main', 'm'), fromJSON('[\"a\",\"b\"]')[0], or 'exit' to quit.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("gha-expr> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" || line == "q" {
			break
		}

		ast, err := expr.Parse(line)
		if err != nil {
			fmt.Printf("Parse error: %v\n", err)
			continue
		}

		val, err := ctx.Evaluate(ast)
		if err != nil {
			fmt.Printf("Evaluation error: %v\n", err)
			continue
		}

		fmt.Printf("=> %v (%T)\n", val, val)
	}
}

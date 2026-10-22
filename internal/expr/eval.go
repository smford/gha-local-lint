package expr

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type JobStatus string

const (
	StatusSuccess   JobStatus = "success"
	StatusFailure   JobStatus = "failure"
	StatusCancelled JobStatus = "cancelled"
)

type Context struct {
	Data      map[string]any
	JobStatus JobStatus
}

func NewContext(data map[string]any) *Context {
	if data == nil {
		data = make(map[string]any)
	}
	return &Context{
		Data:      data,
		JobStatus: StatusSuccess,
	}
}

func (c *Context) Set(key string, val any) {
	c.Data[key] = val
}

func (c *Context) Clone() *Context {
	newData := make(map[string]any, len(c.Data))
	for k, v := range c.Data {
		newData[k] = v
	}
	return &Context{
		Data:      newData,
		JobStatus: c.JobStatus,
	}
}

// Evaluate evaluates an expression AST node in the given context.
func (c *Context) Evaluate(node Node) (any, error) {
	if node == nil {
		return nil, nil
	}

	switch n := node.(type) {
	case *LiteralNode:
		return n.Value, nil

	case *IdentNode:
		val, _ := c.lookupIdent(n.Name)
		return val, nil

	case *MemberNode:
		obj, err := c.Evaluate(n.Object)
		if err != nil {
			return nil, err
		}
		if obj == nil {
			return nil, nil
		}
		if n.Property == "*" {
			// Star wildcard filter over array or map values
			return filterStar(obj), nil
		}
		return getProperty(obj, n.Property), nil

	case *IndexNode:
		obj, err := c.Evaluate(n.Object)
		if err != nil {
			return nil, err
		}
		if obj == nil {
			return nil, nil
		}
		idx, err := c.Evaluate(n.Index)
		if err != nil {
			return nil, err
		}
		return getIndex(obj, idx), nil

	case *UnaryNode:
		val, err := c.Evaluate(n.Expr)
		if err != nil {
			return nil, err
		}
		if n.Op == "!" {
			return !IsTruthy(val), nil
		}
		return nil, fmt.Errorf("unknown unary operator %q", n.Op)

	case *BinaryNode:
		return c.evalBinary(n)

	case *CallNode:
		return c.evalCall(n)

	case *StarNode:
		return "*", nil

	default:
		return nil, fmt.Errorf("unhandled AST node type: %T", node)
	}
}

func (c *Context) lookupIdent(name string) (any, bool) {
	lower := strings.ToLower(name)
	switch lower {
	case "true":
		return true, true
	case "false":
		return false, true
	case "null":
		return nil, true
	case "nan":
		return math.NaN(), true
	case "infinity":
		return math.Inf(1), true
	}

	for k, v := range c.Data {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return nil, false
}

func (c *Context) evalBinary(n *BinaryNode) (any, error) {
	// Short-circuiting for logical operators
	if n.Op == "&&" {
		left, err := c.Evaluate(n.Left)
		if err != nil {
			return nil, err
		}
		if !IsTruthy(left) {
			return false, nil
		}
		right, err := c.Evaluate(n.Right)
		if err != nil {
			return nil, err
		}
		return IsTruthy(right), nil
	}

	if n.Op == "||" {
		left, err := c.Evaluate(n.Left)
		if err != nil {
			return nil, err
		}
		if IsTruthy(left) {
			return true, nil
		}
		right, err := c.Evaluate(n.Right)
		if err != nil {
			return nil, err
		}
		return IsTruthy(right), nil
	}

	left, err := c.Evaluate(n.Left)
	if err != nil {
		return nil, err
	}
	right, err := c.Evaluate(n.Right)
	if err != nil {
		return nil, err
	}

	switch n.Op {
	case "==":
		return compareEqual(left, right), nil
	case "!=":
		return !compareEqual(left, right), nil
	case "<":
		return compareRelational(left, right) < 0, nil
	case "<=":
		return compareRelational(left, right) <= 0, nil
	case ">":
		return compareRelational(left, right) > 0, nil
	case ">=":
		return compareRelational(left, right) >= 0, nil
	default:
		return nil, fmt.Errorf("unknown binary operator %q", n.Op)
	}
}

func (c *Context) evalCall(n *CallNode) (any, error) {
	fn := strings.ToLower(n.Name)
	switch fn {
	case "contains":
		if len(n.Args) != 2 {
			return nil, fmt.Errorf("contains() expects 2 arguments, got %d", len(n.Args))
		}
		search, err := c.Evaluate(n.Args[0])
		if err != nil {
			return nil, err
		}
		item, err := c.Evaluate(n.Args[1])
		if err != nil {
			return nil, err
		}
		return fnContains(search, item), nil

	case "startswith":
		if len(n.Args) != 2 {
			return nil, fmt.Errorf("startsWith() expects 2 arguments, got %d", len(n.Args))
		}
		s1, err := c.Evaluate(n.Args[0])
		if err != nil {
			return nil, err
		}
		s2, err := c.Evaluate(n.Args[1])
		if err != nil {
			return nil, err
		}
		str1 := strings.ToLower(fmt.Sprintf("%v", s1))
		str2 := strings.ToLower(fmt.Sprintf("%v", s2))
		return strings.HasPrefix(str1, str2), nil

	case "endswith":
		if len(n.Args) != 2 {
			return nil, fmt.Errorf("endsWith() expects 2 arguments, got %d", len(n.Args))
		}
		s1, err := c.Evaluate(n.Args[0])
		if err != nil {
			return nil, err
		}
		s2, err := c.Evaluate(n.Args[1])
		if err != nil {
			return nil, err
		}
		str1 := strings.ToLower(fmt.Sprintf("%v", s1))
		str2 := strings.ToLower(fmt.Sprintf("%v", s2))
		return strings.HasSuffix(str1, str2), nil

	case "format":
		if len(n.Args) < 1 {
			return nil, fmt.Errorf("format() expects at least 1 argument")
		}
		fmtStrVal, err := c.Evaluate(n.Args[0])
		if err != nil {
			return nil, err
		}
		var args []any
		for i := 1; i < len(n.Args); i++ {
			argVal, err := c.Evaluate(n.Args[i])
			if err != nil {
				return nil, err
			}
			args = append(args, argVal)
		}
		return fnFormat(fmt.Sprintf("%v", fmtStrVal), args), nil

	case "join":
		if len(n.Args) < 1 || len(n.Args) > 2 {
			return nil, fmt.Errorf("join() expects 1 or 2 arguments, got %d", len(n.Args))
		}
		arrVal, err := c.Evaluate(n.Args[0])
		if err != nil {
			return nil, err
		}
		sep := ","
		if len(n.Args) == 2 {
			sepVal, err := c.Evaluate(n.Args[1])
			if err != nil {
				return nil, err
			}
			sep = fmt.Sprintf("%v", sepVal)
		}
		return fnJoin(arrVal, sep), nil

	case "tojson":
		if len(n.Args) != 1 {
			return nil, fmt.Errorf("toJSON() expects 1 argument")
		}
		val, err := c.Evaluate(n.Args[0])
		if err != nil {
			return nil, err
		}
		bytes, err := json.MarshalIndent(val, "", "  ")
		if err != nil {
			return "", nil
		}
		return string(bytes), nil

	case "fromjson":
		if len(n.Args) != 1 {
			return nil, fmt.Errorf("fromJSON() expects 1 argument")
		}
		val, err := c.Evaluate(n.Args[0])
		if err != nil {
			return nil, err
		}
		str := fmt.Sprintf("%v", val)
		var out any
		err = json.Unmarshal([]byte(str), &out)
		if err != nil {
			return nil, fmt.Errorf("fromJSON() parse error: %w", err)
		}
		return out, nil

	case "hashfiles":
		var patterns []string
		for _, arg := range n.Args {
			val, err := c.Evaluate(arg)
			if err != nil {
				return nil, err
			}
			patterns = append(patterns, fmt.Sprintf("%v", val))
		}
		return fnHashFiles(patterns), nil

	case "success":
		return c.JobStatus == StatusSuccess, nil

	case "failure":
		return c.JobStatus == StatusFailure, nil

	case "always":
		return true, nil

	case "cancelled":
		return c.JobStatus == StatusCancelled, nil

	default:
		return nil, fmt.Errorf("unknown function %q", n.Name)
	}
}

func fnContains(search, item any) bool {
	if search == nil {
		return false
	}
	itemStr := fmt.Sprintf("%v", item)
	val := reflect.ValueOf(search)
	if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
		for i := 0; i < val.Len(); i++ {
			elem := val.Index(i).Interface()
			if compareEqual(elem, item) {
				return true
			}
		}
		return false
	}
	// String containment
	searchStr := strings.ToLower(fmt.Sprintf("%v", search))
	return strings.Contains(searchStr, strings.ToLower(itemStr))
}

func fnFormat(formatStr string, args []any) string {
	var sb strings.Builder
	for i := 0; i < len(formatStr); i++ {
		if formatStr[i] == '{' {
			if i+1 < len(formatStr) && formatStr[i+1] == '{' {
				sb.WriteByte('{')
				i++
				continue
			}
			// Find closing }
			closeIdx := strings.IndexByte(formatStr[i:], '}')
			if closeIdx > 0 {
				numStr := formatStr[i+1 : i+closeIdx]
				idx, err := strconv.Atoi(numStr)
				if err == nil && idx >= 0 && idx < len(args) {
					sb.WriteString(fmt.Sprintf("%v", args[idx]))
					i += closeIdx
					continue
				}
			}
		}
		if formatStr[i] == '}' && i+1 < len(formatStr) && formatStr[i+1] == '}' {
			sb.WriteByte('}')
			i++
			continue
		}
		sb.WriteByte(formatStr[i])
	}
	return sb.String()
}

func fnJoin(arr any, sep string) string {
	if arr == nil {
		return ""
	}
	val := reflect.ValueOf(arr)
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return fmt.Sprintf("%v", arr)
	}
	var parts []string
	for i := 0; i < val.Len(); i++ {
		parts = append(parts, fmt.Sprintf("%v", val.Index(i).Interface()))
	}
	return strings.Join(parts, sep)
}

func fnHashFiles(patterns []string) string {
	h := sha256.New()
	matchedAny := false
	for _, pat := range patterns {
		matches, err := filepath.Glob(pat)
		if err != nil {
			continue
		}
		for _, m := range matches {
			f, err := os.Open(m)
			if err != nil {
				continue
			}
			matchedAny = true
			_, _ = io.Copy(h, f)
			_ = f.Close()
		}
	}
	if !matchedAny {
		// Return deterministic hash of patterns if offline/no files
		for _, pat := range patterns {
			h.Write([]byte(pat))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func IsTruthy(val any) bool {
	if val == nil {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		return len(v) > 0
	case int, int8, int16, int32, int64:
		return reflect.ValueOf(v).Int() != 0
	case uint, uint8, uint16, uint32, uint64:
		return reflect.ValueOf(v).Uint() != 0
	case float32, float64:
		f := reflect.ValueOf(v).Float()
		return f != 0 && !math.IsNaN(f)
	default:
		return true
	}
}

func compareEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Boolean coercion: in GHA, if one operand is a boolean, the other is coerced to boolean
	_, aIsBool := toBool(a)
	_, bIsBool := toBool(b)
	if aIsBool || bIsBool {
		return IsTruthy(a) == IsTruthy(b)
	}

	// Number coercion: if one operand is a number, coerce other to number
	aNum, aIsNum := toNumber(a)
	bNum, bIsNum := toNumber(b)
	if aIsNum && bIsNum {
		if math.IsNaN(aNum) || math.IsNaN(bNum) {
			return false
		}
		return math.Abs(aNum-bNum) < 1e-9
	}

	// Strings: case-insensitive
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return strings.EqualFold(aStr, bStr)
}

func toBool(v any) (bool, bool) {
	if b, ok := v.(bool); ok {
		return b, true
	}
	return false, false
}

func toNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}

func compareRelational(a, b any) int {
	aNum, aIsNum := toNumber(a)
	bNum, bIsNum := toNumber(b)
	if aIsNum && bIsNum {
		if aNum < bNum {
			return -1
		} else if aNum > bNum {
			return 1
		}
		return 0
	}
	aStr := strings.ToLower(fmt.Sprintf("%v", a))
	bStr := strings.ToLower(fmt.Sprintf("%v", b))
	if aStr < bStr {
		return -1
	} else if aStr > bStr {
		return 1
	}
	return 0
}

func getProperty(obj any, prop string) any {
	if obj == nil {
		return nil
	}

	val := reflect.ValueOf(obj)
	for val.Kind() == reflect.Pointer || val.Kind() == reflect.Interface {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Map:
		for _, key := range val.MapKeys() {
			kStr := fmt.Sprintf("%v", key.Interface())
			if strings.EqualFold(kStr, prop) {
				return val.MapIndex(key).Interface()
			}
		}
	case reflect.Struct:
		t := val.Type()
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if strings.EqualFold(field.Name, prop) {
				return val.Field(i).Interface()
			}
		}
	}
	return nil
}

func getIndex(obj any, idx any) any {
	if obj == nil {
		return nil
	}
	val := reflect.ValueOf(obj)
	for val.Kind() == reflect.Pointer || val.Kind() == reflect.Interface {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Slice, reflect.Array:
		var intIdx int
		switch i := idx.(type) {
		case int:
			intIdx = i
		case int64:
			intIdx = int(i)
		case float64:
			intIdx = int(i)
		case string:
			parsed, err := strconv.Atoi(i)
			if err != nil {
				return nil
			}
			intIdx = parsed
		default:
			return nil
		}
		if intIdx >= 0 && intIdx < val.Len() {
			return val.Index(intIdx).Interface()
		}
	case reflect.Map:
		keyStr := fmt.Sprintf("%v", idx)
		for _, key := range val.MapKeys() {
			kStr := fmt.Sprintf("%v", key.Interface())
			if strings.EqualFold(kStr, keyStr) {
				return val.MapIndex(key).Interface()
			}
		}
	}
	return nil
}

func filterStar(obj any) any {
	if obj == nil {
		return nil
	}
	val := reflect.ValueOf(obj)
	for val.Kind() == reflect.Pointer || val.Kind() == reflect.Interface {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
		var res []any
		for i := 0; i < val.Len(); i++ {
			res = append(res, val.Index(i).Interface())
		}
		return res
	}
	if val.Kind() == reflect.Map {
		var res []any
		for _, k := range val.MapKeys() {
			res = append(res, val.MapIndex(k).Interface())
		}
		return res
	}
	return obj
}

// HasStatusCheck checks if expression contains any status check function.
func HasStatusCheck(node Node) bool {
	if node == nil {
		return false
	}
	switch n := node.(type) {
	case *CallNode:
		lower := strings.ToLower(n.Name)
		if lower == "success" || lower == "failure" || lower == "always" || lower == "cancelled" {
			return true
		}
		for _, arg := range n.Args {
			if HasStatusCheck(arg) {
				return true
			}
		}
	case *UnaryNode:
		return HasStatusCheck(n.Expr)
	case *BinaryNode:
		return HasStatusCheck(n.Left) || HasStatusCheck(n.Right)
	case *MemberNode:
		return HasStatusCheck(n.Object)
	case *IndexNode:
		return HasStatusCheck(n.Object) || HasStatusCheck(n.Index)
	}
	return false
}

// EvaluateCondition evaluates an expression in a conditional (if:) context.
// In GitHub Actions, if no status check function is present, && success() is implicitly added.
func EvaluateCondition(rawExpr string, ctx *Context) (bool, error) {
	raw := strings.TrimSpace(rawExpr)
	if raw == "" {
		return true, nil
	}
	// Check if expression is wrapped in ${{ ... }}
	if strings.HasPrefix(raw, "${{") && strings.HasSuffix(raw, "}}") {
		raw = strings.TrimSpace(raw[3 : len(raw)-2])
	}

	ast, err := Parse(raw)
	if err != nil {
		return false, err
	}

	if !HasStatusCheck(ast) {
		// Implicitly append && success()
		if ctx.JobStatus != StatusSuccess {
			return false, nil
		}
	}

	res, err := ctx.Evaluate(ast)
	if err != nil {
		return false, err
	}
	return IsTruthy(res), nil
}

var exprRegex = regexp.MustCompile(`\$\{\{\s*(.*?)\s*\}\}`)

// Interpolate evaluates all ${{ ... }} expressions in a string.
// If the whole string is ${{ ... }}, it returns the evaluated typed value.
func Interpolate(input string, ctx *Context) (any, error) {
	trimmed := strings.TrimSpace(input)
	// If input is purely ${{ ... }} without surrounding text:
	if strings.HasPrefix(trimmed, "${{") && strings.HasSuffix(trimmed, "}}") {
		inner := trimmed[3 : len(trimmed)-2]
		// Ensure there isn't another ${{ }} inside (e.g. not "${{ foo }} bar ${{ baz }}")
		if !strings.Contains(inner, "${{") && !strings.Contains(inner, "}}") {
			ast, err := Parse(inner)
			if err != nil {
				return nil, err
			}
			return ctx.Evaluate(ast)
		}
	}

	// Otherwise, replace occurrences of ${{ ... }} with string representation
	var evalErr error
	replaced := exprRegex.ReplaceAllStringFunc(input, func(m string) string {
		submatches := exprRegex.FindStringSubmatch(m)
		if len(submatches) < 2 {
			return m
		}
		exprStr := submatches[1]
		ast, err := Parse(exprStr)
		if err != nil {
			evalErr = err
			return m
		}
		val, err := ctx.Evaluate(ast)
		if err != nil {
			evalErr = err
			return m
		}
		if val == nil {
			return ""
		}
		return fmt.Sprintf("%v", val)
	})

	if evalErr != nil {
		return nil, evalErr
	}
	return replaced, nil
}

// ExtractReferences finds all dereference paths (e.g., "github.event.pull_request.title") in an expression or interpolated string.
func ExtractReferences(input string) []string {
	var results []string

	// Check if input contains ${{ ... }}
	matches := exprRegex.FindAllStringSubmatch(input, -1)
	if len(matches) > 0 {
		for _, m := range matches {
			if len(m) >= 2 {
				results = append(results, extractFromExpr(m[1])...)
			}
		}
	} else {
		// Could be a raw expression (like inside if:)
		results = append(results, extractFromExpr(input)...)
	}

	return uniqueStrings(results)
}

func extractFromExpr(exprStr string) []string {
	ast, err := Parse(exprStr)
	if err != nil {
		return nil
	}
	var refs []string
	walkReferenceRoots(ast, func(path string) {
		refs = append(refs, path)
	})
	return refs
}

func walkReferenceRoots(node Node, emit func(string)) {
	if node == nil {
		return
	}
	// If this node is a member or index access, extract the full path and don't recurse into its object
	if isRefNode(node) {
		emit(nodeToPath(node))
		return
	}
	switch n := node.(type) {
	case *UnaryNode:
		walkReferenceRoots(n.Expr, emit)
	case *BinaryNode:
		walkReferenceRoots(n.Left, emit)
		walkReferenceRoots(n.Right, emit)
	case *CallNode:
		for _, a := range n.Args {
			walkReferenceRoots(a, emit)
		}
	}
}

func isRefNode(node Node) bool {
	switch n := node.(type) {
	case *IdentNode:
		// Don't treat standalone literals as refs
		lower := strings.ToLower(n.Name)
		return lower != "true" && lower != "false" && lower != "null" && lower != "nan" && lower != "infinity"
	case *MemberNode:
		return true
	case *IndexNode:
		return true
	default:
		return false
	}
}

func nodeToPath(node Node) string {
	switch n := node.(type) {
	case *IdentNode:
		return n.Name
	case *MemberNode:
		parent := nodeToPath(n.Object)
		if parent != "" {
			return parent + "." + n.Property
		}
		return n.Property
	case *IndexNode:
		parent := nodeToPath(n.Object)
		if lit, ok := n.Index.(*LiteralNode); ok {
			if parent != "" {
				return fmt.Sprintf("%s.%v", parent, lit.Value)
			}
			return fmt.Sprintf("%v", lit.Value)
		}
		return parent
	default:
		return ""
	}
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

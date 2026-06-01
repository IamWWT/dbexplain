package filequery

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Value represents a runtime value in the query engine.
// All values are stored as strings (CSV-native) with type inference on demand.
type Value string

// Bool returns the boolean interpretation of a value.
func (v Value) Bool() bool {
	s := strings.TrimSpace(string(v))
	if s == "" {
		return false
	}
	// Check explicit boolean strings
	lower := strings.ToLower(s)
	if lower == "false" || lower == "no" || lower == "off" || lower == "0" {
		return false
	}
	if lower == "true" || lower == "yes" || lower == "on" {
		return true
	}
	// Try numeric interpretation
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		// Non-empty, non-false string → treat as true
		return true
	}
	return f != 0
}

// Float parses the value as float64.
func (v Value) Float() (float64, bool) {
	s := strings.TrimSpace(string(v))
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// Int parses the value as int64.
func (v Value) Int() (int64, bool) {
	s := strings.TrimSpace(string(v))
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// String returns the raw string value.
func (v Value) String() string {
	return string(v)
}

// --- Column mapping ---

// ColMap maps column names (optionally table-qualified) to column indices.
type ColMap map[string]int

// BuildColMap builds a column index map from a header row.
// Keys: bare column names + "table.col" (if table name provided).
func BuildColMap(header []string, tableAlias string) ColMap {
	cm := make(ColMap, len(header))
	for i, col := range header {
		cm[col] = i
		if tableAlias != "" {
			cm[tableAlias+"."+col] = i
		}
	}
	return cm
}

// JoinColMaps merges multiple column maps into one.
// prefix1 and prefix2 are used for disambiguation.
func JoinColMaps(leftHeader []string, leftAlias string, rightHeader []string, rightAlias string) (ColMap, []string) {
	combined := make(ColMap)
	var fullHeader []string

	for i, col := range leftHeader {
		key := col
		if leftAlias != "" {
			key = leftAlias + "." + col
		}
		combined[key] = i
		combined[col] = i // bare name too (risk of collision but acceptable)
		fullHeader = append(fullHeader, col)
	}

	offset := len(leftHeader)
	for i, col := range rightHeader {
		key := col
		if rightAlias != "" {
			key = rightAlias + "." + col
		}
		combined[key] = offset + i
		// Only add bare name if it doesn't collide
		if _, exists := combined[col]; !exists {
			combined[col] = offset + i
		}
		fullHeader = append(fullHeader, col)
	}

	return combined, fullHeader
}

// --- Expression evaluator ---

// Row is a list of string values.
type Row []Value

// SubqueryCache stores pre-computed result sets for subqueries.
type SubqueryCache map[*SubqueryExpr]map[string]bool

// Eval evaluates an expression against a given row.
func Eval(expr Expr, row Row, colMap ColMap) (Value, error) {
	return evalCached(expr, row, colMap, nil)
}

// EvalWithSubqueries evaluates an expression with subquery support.
func EvalWithSubqueries(expr Expr, row Row, colMap ColMap, cache SubqueryCache) (Value, error) {
	return evalCached(expr, row, colMap, cache)
}

// evalCached is the internal evaluator with optional subquery cache.
func evalCached(expr Expr, row Row, colMap ColMap, cache SubqueryCache) (Value, error) {
	switch e := expr.(type) {
	case *ColumnRef:
		if e.Col == "*" {
			// Used in COUNT(*) — return a non-null sentinel
			return Value("1"), nil
		}
		idx, ok := colMap[e.Table+"."+e.Col]
		if !ok && e.Table != "" {
			// Try table alias from the FROM clause directly
			idx, ok = colMap[e.Table+"."+e.Col]
		}
		if !ok {
			// Try bare column name (might have been added during join)
			idx, ok = colMap[e.Col]
		}
		if !ok {
			return "", fmt.Errorf("column %q not found", e.String())
		}
		if idx < 0 || idx >= len(row) {
			return "", fmt.Errorf("column index %d out of range for row length %d", idx, len(row))
		}
		return row[idx], nil

	case *NumberLit:
		return Value(e.Value), nil

	case *StringLit:
		return Value(e.Value), nil

	case *BinaryExpr:
		return evalBinaryCached(e, row, colMap, cache)

	case *UnaryExpr:
		right, err := evalCached(e.Right, row, colMap, cache)
		if err != nil {
			return "", err
		}
		switch e.Op {
		case "NOT":
			return Value(boolVal(!right.Bool())), nil
		case "-":
			f, ok := right.Float()
			if !ok {
				return Value("-" + string(right)), nil
			}
			return Value(fmt.Sprintf("%v", -f)), nil
		default:
			return "", fmt.Errorf("unknown unary operator %q", e.Op)
		}

	case *BetweenExpr:
		val, err := evalCached(e.Expr, row, colMap, cache)
		if err != nil {
			return "", err
		}
		low, err := evalCached(e.Low, row, colMap, cache)
		if err != nil {
			return "", err
		}
		high, err := evalCached(e.High, row, colMap, cache)
		if err != nil {
			return "", err
		}

		vf, vOK := val.Float()
		lf, lOK := low.Float()
		hf, hOK := high.Float()
		if vOK && lOK && hOK {
			return Value(boolVal(vf >= lf && vf <= hf)), nil
		}
		// String comparison fallback
		vs := strings.TrimSpace(string(val))
		ls := strings.TrimSpace(string(low))
		hs := strings.TrimSpace(string(high))
		return Value(boolVal(vs >= ls && vs <= hs)), nil

	case *FuncCall:
		return evalFuncCall(e, row, colMap)

	case *SubqueryExpr:
		if cache != nil {
			return Value("1"), nil // presence in cache means non-empty result
		}
		return "", fmt.Errorf("subquery not pre-evaluated")

	default:
		return "", fmt.Errorf("unknown expression type %T", expr)
	}
}

// evalBinaryCached evaluates a binary expression with optional subquery cache.
func evalBinaryCached(e *BinaryExpr, row Row, colMap ColMap, cache SubqueryCache) (Value, error) {
	switch strings.ToUpper(e.Op) {
	case "AND":
		left, err := evalCached(e.Left, row, colMap, cache)
		if err != nil {
			return "", err
		}
		if !left.Bool() {
			return Value("false"), nil
		}
		right, err := evalCached(e.Right, row, colMap, cache)
		if err != nil {
			return "", err
		}
		return Value(boolVal(right.Bool())), nil

	case "OR":
		left, err := evalCached(e.Left, row, colMap, cache)
		if err != nil {
			return "", err
		}
		if left.Bool() {
			return Value("true"), nil
		}
		right, err := evalCached(e.Right, row, colMap, cache)
		if err != nil {
			return "", err
		}
		return Value(boolVal(right.Bool())), nil

	case "LIKE", "NOT LIKE":
		left, err := evalCached(e.Left, row, colMap, cache)
		if err != nil {
			return "", err
		}
		right, err := evalCached(e.Right, row, colMap, cache)
		if err != nil {
			return "", err
		}
		match := likeMatch(string(left), string(right))
		if e.Op == "NOT LIKE" {
			return Value(boolVal(!match)), nil
		}
		return Value(boolVal(match)), nil

	case "IS NULL", "IS NOT NULL":
		left, err := evalCached(e.Left, row, colMap, cache)
		if err != nil {
			return "", err
		}
		isNull := string(left) == ""
		if e.Op == "IS NOT NULL" {
			return Value(boolVal(!isNull)), nil
		}
		return Value(boolVal(isNull)), nil

	case "IN", "NOT IN":
		left, err := evalCached(e.Left, row, colMap, cache)
		if err != nil {
			return "", err
		}

		// Handle subquery IN (SELECT ...)
		if sub, ok := e.Right.(*SubqueryExpr); ok {
			found := false
			if cache != nil {
				if valSet, ok := cache[sub]; ok {
					_, found = valSet[string(left)]
				}
			}
			if e.Op == "NOT IN" {
				return Value(boolVal(!found)), nil
			}
			return Value(boolVal(found)), nil
		}

		// Right side is an OR chain of values
		right, err := evalCached(e.Right, row, colMap, cache)
		if err != nil {
			return "", err
		}
		found := false
		if be, ok := e.Right.(*BinaryExpr); ok && be.Op == "OR" {
			found = walkOrChainCached(left, be, row, colMap, cache)
		} else {
			rv := string(right)
			found = string(left) == rv
		}
		if e.Op == "NOT IN" {
			return Value(boolVal(!found)), nil
		}
		return Value(boolVal(found)), nil

	default:
		// Comparison or arithmetic operators
		left, err := evalCached(e.Left, row, colMap, cache)
		if err != nil {
			return "", err
		}
		right, err := evalCached(e.Right, row, colMap, cache)
		if err != nil {
			return "", err
		}

		return evalBinaryOp(left, right, e.Op)
	}
}


// walkOrChainCached walks an OR chain to find if left matches any element.
func walkOrChainCached(left Value, expr *BinaryExpr, row Row, colMap ColMap, cache SubqueryCache) bool {
	if expr.Op != "OR" {
		// Leaf: evaluate and compare
		right, err := evalCached(expr, row, colMap, cache)
		if err != nil {
			return false
		}
		return string(left) == string(right)
	}

	// Check left side
	if leftBe, ok := expr.Left.(*BinaryExpr); ok && leftBe.Op == "OR" {
		if walkOrChainCached(left, leftBe, row, colMap, cache) {
			return true
		}
	} else {
		v, err := evalCached(expr.Left, row, colMap, cache)
		if err == nil && string(left) == string(v) {
			return true
		}
	}

	// Check right side
	if rightBe, ok := expr.Right.(*BinaryExpr); ok && rightBe.Op == "OR" {
		return walkOrChainCached(left, rightBe, row, colMap, cache)
	}
	v, err := evalCached(expr.Right, row, colMap, cache)
	if err == nil && string(left) == string(v) {
		return true
	}
	return false
}

// evalBinaryOp performs arithmetic and comparison operations.
func evalBinaryOp(left, right Value, op string) (Value, error) {
	lf, lOK := left.Float()
	rf, rOK := right.Float()

	switch op {
	case "+":
		if lOK && rOK {
			return Value(fmt.Sprintf("%v", lf+rf)), nil
		}
		return Value(string(left) + string(right)), nil
	case "-":
		if lOK && rOK {
			return Value(fmt.Sprintf("%v", lf-rf)), nil
		}
		return Value(""), fmt.Errorf("cannot subtract non-numeric values: %q - %q", left, right)
	case "*":
		if lOK && rOK {
			return Value(fmt.Sprintf("%v", lf*rf)), nil
		}
		return Value(""), fmt.Errorf("cannot multiply non-numeric values: %q * %q", left, right)
	case "/":
		if lOK && rOK {
			if rf == 0 {
				return Value(""), fmt.Errorf("division by zero")
			}
			return Value(fmt.Sprintf("%v", lf/rf)), nil
		}
		return Value(""), fmt.Errorf("cannot divide non-numeric values: %q / %q", left, right)
	case "=":
		if lOK && rOK {
			return Value(boolVal(lf == rf)), nil
		}
		return Value(boolVal(string(left) == string(right))), nil
	case "!=", "<>":
		if lOK && rOK {
			return Value(boolVal(lf != rf)), nil
		}
		return Value(boolVal(string(left) != string(right))), nil
	case "<":
		if lOK && rOK {
			return Value(boolVal(lf < rf)), nil
		}
		return Value(boolVal(string(left) < string(right))), nil
	case ">":
		if lOK && rOK {
			return Value(boolVal(lf > rf)), nil
		}
		return Value(boolVal(string(left) > string(right))), nil
	case "<=":
		if lOK && rOK {
			return Value(boolVal(lf <= rf)), nil
		}
		return Value(boolVal(string(left) <= string(right))), nil
	case ">=":
		if lOK && rOK {
			return Value(boolVal(lf >= rf)), nil
		}
		return Value(boolVal(string(left) >= string(right))), nil
	default:
		return Value(""), fmt.Errorf("unknown operator %q", op)
	}
}

// --- Function evaluation ---

func evalFuncCall(fn *FuncCall, row Row, colMap ColMap) (Value, error) {
	switch fn.Name {
	case "ABS":
		if len(fn.Args) != 1 {
			return "", fmt.Errorf("ABS requires 1 argument, got %d", len(fn.Args))
		}
		val, err := Eval(fn.Args[0], row, colMap)
		if err != nil {
			return "", err
		}
		f, ok := val.Float()
		if !ok {
			return Value(""), fmt.Errorf("ABS requires numeric argument, got %q", val)
		}
		return Value(fmt.Sprintf("%v", math.Abs(f))), nil

	case "CAST":
		if len(fn.Args) != 1 {
			return "", fmt.Errorf("CAST requires 1 argument, got %d", len(fn.Args))
		}
		val, err := Eval(fn.Args[0], row, colMap)
		if err != nil {
			return "", err
		}
		// CAST as FLOAT / INTEGER / TEXT
		switch fn.CastType {
		case "FLOAT", "DOUBLE", "REAL", "DECIMAL":
			f, ok := val.Float()
			if !ok {
				// Conversion failed — return original value
				return val, nil
			}
			return Value(fmt.Sprintf("%v", f)), nil
		case "INT", "INTEGER", "BIGINT", "SMALLINT":
			n, ok := val.Int()
			if !ok {
				// Conversion failed — return original value
				return val, nil
			}
			return Value(fmt.Sprintf("%d", n)), nil
		default:
			return val, nil // TEXT or unknown — return raw string
		}

	case "ROUND":
		if len(fn.Args) < 1 {
			return "", fmt.Errorf("ROUND requires at least 1 argument")
		}
		val, err := Eval(fn.Args[0], row, colMap)
		if err != nil {
			return "", err
		}
		f, ok := val.Float()
		if !ok {
			return val, nil
		}
		decimals := 0
		if len(fn.Args) >= 2 {
			dv, err := Eval(fn.Args[1], row, colMap)
			if err == nil {
				if d, ok := dv.Int(); ok {
					decimals = int(d)
				}
			}
		}
		pow := math.Pow(10, float64(decimals))
		return Value(fmt.Sprintf("%v", math.Round(f*pow)/pow)), nil

	case "SUM", "AVG", "COUNT", "MAX", "MIN":
		// These are handled by the aggregate engine, not the row evaluator.
		// But for non-aggregate queries, evaluate the argument.
		if len(fn.Args) == 1 {
			return Eval(fn.Args[0], row, colMap) // uses Eval (no cache needed for scalar)
		}
		return Value(""), nil

	default:
		// Unknown function — try evaluating args
		if len(fn.Args) == 1 {
			return Eval(fn.Args[0], row, colMap)
		}
		return Value(""), fmt.Errorf("unknown function %q", fn.Name)
	}
}

// --- LIKE matching ---

// likeMatch implements SQL LIKE pattern matching.
// % matches any sequence, _ matches any single character.
// Backslash is the default escape character.
func likeMatch(s, pattern string) bool {
	s = strings.ToLower(s)
	pattern = strings.ToLower(pattern)
	return likeMatchRecurse(s, pattern)
}

func likeMatchRecurse(s, pattern string) bool {
	pi := 0 // pattern index
	si := 0 // string index

	for pi < len(pattern) {
		pc := pattern[pi]

		switch pc {
		case '%':
			// % matches any sequence (including empty)
			// Try matching the rest of the pattern at each position
			pi++
			if pi >= len(pattern) {
				return true // trailing % matches everything
			}
			for si <= len(s) {
				if likeMatchRecurse(s[si:], pattern[pi:]) {
					return true
				}
				si++
			}
			return false

		case '_':
			// _ matches exactly one character
			if si >= len(s) {
				return false
			}
			si++
			pi++

		case '\\':
			// Escape: next character is literal
			pi++
			if pi >= len(pattern) {
				return false
			}
			if si >= len(s) || s[si] != pattern[pi] {
				return false
			}
			si++
			pi++

		default:
			if si >= len(s) || s[si] != pc {
				return false
			}
			si++
			pi++
		}
	}

	return si >= len(s)
}

// --- Helpers ---

func boolVal(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// FloatOrZero safely converts a Value to float64, returning 0 on failure.
func FloatOrZero(v Value) float64 {
	f, ok := v.Float()
	if !ok {
		return 0
	}
	return f
}

// CountString safely converts a Value to its string representation.
func CountString(v Value) string {
	return string(v)
}

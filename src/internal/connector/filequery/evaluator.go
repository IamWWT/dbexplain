package filequery

import (
	"fmt"
	"math"
	"sort"
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

// ── Window function computation ──

// WindowValues stores pre-computed window function values per SELECT column.
// Key is the column index in the SELECT list, value is per-row computed values.
type WindowValues map[int][]Value

// HasWindowFunctions returns true if any SELECT expression is a window function.
func HasWindowFunctions(stmt *SelectStmt) bool {
	for _, col := range stmt.Columns {
		if fc, ok := col.Expr.(*FuncCall); ok && IsWindowFunc(fc) {
			return true
		}
	}
	return false
}

// computeWindowFunctions computes all window functions in the SELECT list.
// Must be called with the full dataset (after WHERE/GROUP BY/HAVING, before ORDER BY).
// Returns nil if no window functions are present.
func computeWindowFunctions(stmt *SelectStmt, data []Row, colMap ColMap, alias string) WindowValues {
	results := make(WindowValues)

	for colIdx, sel := range stmt.Columns {
		fc, ok := sel.Expr.(*FuncCall)
		if !ok || !IsWindowFunc(fc) {
			continue
		}

		values := computeWindowFunc(fc, data, colMap, alias)
		if values != nil {
			results[colIdx] = values
		}
	}

	if len(results) == 0 {
		return nil
	}
	return results
}

// computeWindowFunc computes a single window function for all rows.
func computeWindowFunc(fc *FuncCall, data []Row, colMap ColMap, alias string) []Value {
	wd := fc.Over
	if wd == nil {
		return nil
	}

	// Determine partition column indices
	partColIdxs := resolveColIndices(wd.PartitionBy, colMap, alias)

	// Build partition map
	partitions := make(map[string][]int)
	var partitionKeys []string
	for i, row := range data {
		key := makePartitionKey(row, partColIdxs)
		if _, exists := partitions[key]; !exists {
			partitionKeys = append(partitionKeys, key)
		}
		partitions[key] = append(partitions[key], i)
	}

	values := make([]Value, len(data))
	switch fc.Name {
	case "ROW_NUMBER":
		computeRowNumber(values, data, partitions, partitionKeys, wd, colMap, alias)
	case "RANK":
		computeRank(values, data, partitions, partitionKeys, wd, colMap, alias, false)
	case "DENSE_RANK":
		computeRank(values, data, partitions, partitionKeys, wd, colMap, alias, true)
	case "NTILE":
		if len(fc.Args) >= 1 {
			computeNtile(values, data, partitions, partitionKeys, wd, colMap, alias, fc.Args[0])
		}
	case "LAG":
		computeLagLead(values, data, partitions, partitionKeys, wd, colMap, alias, fc.Args, false)
	case "LEAD":
		computeLagLead(values, data, partitions, partitionKeys, wd, colMap, alias, fc.Args, true)
	case "FIRST_VALUE":
		computeFirstLastValue(values, data, partitions, partitionKeys, wd, colMap, alias, fc.Args, true)
	case "LAST_VALUE":
		computeFirstLastValue(values, data, partitions, partitionKeys, wd, colMap, alias, fc.Args, false)
	case "SUM", "AVG", "COUNT", "MAX", "MIN":
		computeAggWindow(values, data, partitions, partitionKeys, wd, colMap, alias, fc.Args, fc.Name)
	}

	return values
}

// resolveColIndices resolves ColumnRefs to column indices.
func resolveColIndices(cols []ColumnRef, colMap ColMap, alias string) []int {
	idxs := make([]int, 0, len(cols))
	for _, c := range cols {
		idx, ok := colMap[c.Col]
		if !ok && alias != "" {
			idx, ok = colMap[alias+"."+c.Col]
		}
		if ok {
			idxs = append(idxs, idx)
		}
	}
	return idxs
}

// makePartitionKey creates a partition key from a row.
func makePartitionKey(row Row, colIdxs []int) string {
	if len(colIdxs) == 0 {
		return "[[single]]"
	}
	parts := make([]string, len(colIdxs))
	for i, ci := range colIdxs {
		if ci >= 0 && ci < len(row) {
			parts[i] = string(row[ci])
		}
	}
	return strings.Join(parts, "\x00")
}

// computeRowNumber assigns ROW_NUMBER() values within each partition.
func computeRowNumber(values []Value, data []Row, partitions map[string][]int,
	partitionKeys []string, wd *WindowDef, colMap ColMap, alias string) {

	for _, pk := range partitionKeys {
		indices := partitions[pk]
		sorted := sortPartitionIndices(indices, data, wd.OrderBy, colMap, alias)
		for i, idx := range sorted {
			values[idx] = Value(fmt.Sprintf("%d", i+1))
		}
	}
}

// computeRank assigns RANK() or DENSE_RANK() values within each partition.
func computeRank(values []Value, data []Row, partitions map[string][]int,
	partitionKeys []string, wd *WindowDef, colMap ColMap, alias string, dense bool) {

	for _, pk := range partitionKeys {
		indices := partitions[pk]
		sorted := sortPartitionIndices(indices, data, wd.OrderBy, colMap, alias)

		if len(sorted) == 0 {
			continue
		}

		currentDenseRank := 1
		for i, idx := range sorted {
			if i == 0 {
				values[idx] = Value("1")
				continue
			}

			// Check if this row is tied with the previous row
			tied := len(wd.OrderBy) > 0 && rowsEqual(data, idx, sorted[i-1], wd.OrderBy, colMap, alias)

			if tied {
				// Same rank as previous row
				if dense {
					values[idx] = Value(fmt.Sprintf("%d", currentDenseRank))
				} else {
					values[idx] = Value(fmt.Sprintf("%d", currentDenseRank))
				}
			} else {
				currentDenseRank++
				if dense {
					values[idx] = Value(fmt.Sprintf("%d", currentDenseRank))
				} else {
					// RANK: position+1 (allows gaps for ties)
					values[idx] = Value(fmt.Sprintf("%d", i+1))
				}
			}
		}
	}
}

// sortPartitionIndices sorts row indices within a partition by OVER ORDER BY.
func sortPartitionIndices(indices []int, data []Row, orderBy []OrderExpr, colMap ColMap, alias string) []int {
	if len(orderBy) == 0 {
		// No ORDER BY: use natural order (preserve input order)
		return indices
	}
	sorted := make([]int, len(indices))
	copy(sorted, indices)
	sort.SliceStable(sorted, func(a, b int) bool {
		for _, ob := range orderBy {
			va := getRowValue(data[sorted[a]], ob.Expr, colMap, alias)
			vb := getRowValue(data[sorted[b]], ob.Expr, colMap, alias)
			if va == "" && vb == "" {
				continue
			}
			if va == "" {
				return ob.NullsDir == "FIRST" || (ob.NullsDir == "" && ob.Dir == "DESC")
			}
			if vb == "" {
				return ob.NullsDir == "LAST" || (ob.NullsDir == "" && ob.Dir == "ASC")
			}
			fa, errA := strconv.ParseFloat(va, 64)
			fb, errB := strconv.ParseFloat(vb, 64)
			if errA == nil && errB == nil {
				if fa != fb {
					if ob.Dir == "DESC" {
						return fa > fb
					}
					return fa < fb
				}
				continue
			}
			if va != vb {
				if ob.Dir == "DESC" {
					return va > vb
				}
				return va < vb
			}
		}
		return false
	})
	return sorted
}

// getRowValue extracts the value of a column from a row.
func getRowValue(row Row, col ColumnRef, colMap ColMap, alias string) string {
	idx, ok := colMap[col.Col]
	if !ok && alias != "" {
		idx, ok = colMap[alias+"."+col.Col]
	}
	if ok && idx >= 0 && idx < len(row) {
		return string(row[idx])
	}
	return ""
}

// rowsEqual checks if two rows have the same values for the ORDER BY columns.
func rowsEqual(data []Row, i, j int, orderBy []OrderExpr, colMap ColMap, alias string) bool {
	for _, ob := range orderBy {
		vi := getRowValue(data[i], ob.Expr, colMap, alias)
		vj := getRowValue(data[j], ob.Expr, colMap, alias)
		if vi != vj {
			return false
		}
	}
	return true
}

// computeNtile assigns NTILE(n) values within each partition.
func computeNtile(values []Value, data []Row, partitions map[string][]int,
	partitionKeys []string, wd *WindowDef, colMap ColMap, alias string, arg Expr) {

	n := 1 // default
	if nl, ok := arg.(*NumberLit); ok {
		if v, err := strconv.Atoi(nl.Value); err == nil && v > 0 {
			n = v
		}
	}

	for _, pk := range partitionKeys {
		indices := partitions[pk]
		sorted := sortPartitionIndices(indices, data, wd.OrderBy, colMap, alias)
		size := len(sorted)
		if size == 0 {
			continue
		}

		// NTILE algorithm: first (size % n) buckets get size/n + 1 rows
		baseSize := size / n
		remainder := size % n

		rowIdx := 0
		for bucket := 1; bucket <= n && rowIdx < size; bucket++ {
			bucketSize := baseSize
			if bucket <= remainder {
				bucketSize++
			}
			for j := 0; j < bucketSize && rowIdx < size; j++ {
				values[sorted[rowIdx]] = Value(fmt.Sprintf("%d", bucket))
				rowIdx++
			}
		}
	}
}

// computeLagLead computes LAG(expr, offset, default) or LEAD(expr, offset, default).
func computeLagLead(values []Value, data []Row, partitions map[string][]int,
	partitionKeys []string, wd *WindowDef, colMap ColMap, alias string, args []Expr, lead bool) {

	if len(args) < 1 {
		return
	}

	for _, pk := range partitionKeys {
		indices := partitions[pk]
		sorted := sortPartitionIndices(indices, data, wd.OrderBy, colMap, alias)

		for pos, idx := range sorted {
			targetPos := pos - 1 // LAG: previous row
			if lead {
				targetPos = pos + 1 // LEAD: next row
			}

			// Check bounds
			if targetPos < 0 || targetPos >= len(sorted) {
				// Out of bounds: use default (3rd arg) or empty string
				if len(args) >= 3 {
					v, err := evalCached(args[2], data[idx], colMap, nil)
					if err == nil {
						values[idx] = v
					} else {
						values[idx] = Value("")
					}
				} else {
					values[idx] = Value("")
				}
				continue
			}

			// Evaluate the expression against the target row
			targetIdx := sorted[targetPos]
			v, err := evalCached(args[0], data[targetIdx], colMap, nil)
			if err != nil {
				// If evaluation fails, try default
				if len(args) >= 3 {
					dv, derr := evalCached(args[2], data[idx], colMap, nil)
					if derr == nil {
						values[idx] = dv
					} else {
						values[idx] = Value("")
					}
				} else {
					values[idx] = Value("")
				}
				continue
			}
			values[idx] = v
		}
	}
}

// computeFirstLastValue computes FIRST_VALUE(expr) or LAST_VALUE(expr).
// With frame support, the value is evaluated per row based on the frame boundaries.
func computeFirstLastValue(values []Value, data []Row, partitions map[string][]int,
	partitionKeys []string, wd *WindowDef, colMap ColMap, alias string, args []Expr, first bool) {

	if len(args) < 1 {
		return
	}

	for _, pk := range partitionKeys {
		indices := partitions[pk]
		sorted := sortPartitionIndices(indices, data, wd.OrderBy, colMap, alias)
		if len(sorted) == 0 {
			continue
		}

		for pos, idx := range sorted {
			start, end := getFrameStartEnd(wd, pos, len(sorted), data, sorted, colMap, alias)
			if start > end || start < 0 {
				values[idx] = Value("")
				continue
			}

			targetIdx := sorted[start] // FIRST_VALUE: first row in frame
			if !first {
				targetIdx = sorted[end] // LAST_VALUE: last row in frame
			}

			v, err := evalCached(args[0], data[targetIdx], colMap, nil)
			if err != nil {
				v = Value("")
			}
			values[idx] = v
		}
	}
}

// computeAggWindow computes aggregate-as-window functions: SUM/AVG/COUNT/MAX/MIN OVER (...).
// Frame-aware: aggregates only over rows within the frame boundaries.
func computeAggWindow(values []Value, data []Row, partitions map[string][]int,
	partitionKeys []string, wd *WindowDef, colMap ColMap, alias string, args []Expr, funcName string) {

	if len(args) < 1 {
		return
	}

	for _, pk := range partitionKeys {
		indices := partitions[pk]
		sorted := sortPartitionIndices(indices, data, wd.OrderBy, colMap, alias)
		if len(sorted) == 0 {
			continue
		}

		for pos, idx := range sorted {
			start, end := getFrameStartEnd(wd, pos, len(sorted), data, sorted, colMap, alias)
			if start > end || start < 0 {
				values[idx] = Value("")
				continue
			}

			// Aggregate over frame range [start, end]
			var sum float64
			count := 0
			hasVal := false
			var maxVal, minVal float64
			hasMinMax := false

			for fi := start; fi <= end; fi++ {
				frameIdx := sorted[fi]
				v, err := evalCached(args[0], data[frameIdx], colMap, nil)
				if err != nil {
					continue
				}
				vs := strings.TrimSpace(string(v))
				if vs == "" {
					continue
				}
				count++
				f, ok := strconv.ParseFloat(vs, 64)
				if ok == nil {
					sum += f
					hasVal = true
					if !hasMinMax {
						maxVal = f
						minVal = f
						hasMinMax = true
					} else {
						if f > maxVal {
							maxVal = f
						}
						if f < minVal {
							minVal = f
						}
					}
				}
			}

			switch funcName {
			case "SUM":
				if hasVal {
					values[idx] = Value(formatFloat(sum))
				} else {
					values[idx] = Value("")
				}
			case "AVG":
				if count > 0 && hasVal {
					values[idx] = Value(formatFloat(sum / float64(count)))
				} else {
					values[idx] = Value("")
				}
			case "COUNT":
				values[idx] = Value(fmt.Sprintf("%d", count))
			case "MAX":
				if hasMinMax {
					values[idx] = Value(formatFloat(maxVal))
				} else {
					values[idx] = Value("")
				}
			case "MIN":
				if hasMinMax {
					values[idx] = Value(formatFloat(minVal))
				} else {
					values[idx] = Value("")
				}
			}
		}
	}
}

// getFrameStartEnd returns the start and end indices (inclusive) for the window frame
// at the given position within a sorted partition of the given size.
// For RANGE mode with offset boundaries, uses the first ORDER BY column for value comparison.
func getFrameStartEnd(wd *WindowDef, pos, size int, data []Row, sorted []int, colMap ColMap, alias string) (int, int) {
	if wd.Frame == nil {
		// Default frame behavior
		if len(wd.OrderBy) == 0 {
			// No ORDER BY: frame is entire partition
			return 0, size - 1
		}
		// ORDER BY present: RANGE BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
		return 0, pos
	}

	frame := wd.Frame
	if frame.Type == "RANGE" {
		return getRangeFrameBounds(frame, pos, size, data, sorted, colMap, alias, wd)
	}
	// ROWS mode: positional offsets
	start := calcRowBound(frame.Start, pos, size)
	end := calcRowBound(frame.End, pos, size)
	if start > end {
		return 0, -1 // empty frame
	}
	return start, end
}

// calcRowBound computes the actual index for a ROWS frame boundary.
func calcRowBound(bound FrameBound, pos, size int) int {
	switch bound.Type {
	case FrameUnboundedPreceding:
		return 0
	case FrameUnboundedFollowing:
		return size - 1
	case FrameCurrentRow:
		return pos
	case FrameOffsetPreceding:
		if pos-bound.Offset < 0 {
			return 0
		}
		return pos - bound.Offset
	case FrameOffsetFollowing:
		if pos+bound.Offset >= size {
			return size - 1
		}
		return pos + bound.Offset
	default:
		return pos
	}
}

// getRangeFrameBounds computes frame bounds for RANGE mode.
// Uses the first ORDER BY column for value-based comparison.
func getRangeFrameBounds(frame *WindowFrame, pos, size int, data []Row, sorted []int, colMap ColMap, alias string, wd *WindowDef) (int, int) {
	startPos := 0
	endPos := size - 1

	// Determine start boundary
	switch frame.Start.Type {
	case FrameUnboundedPreceding:
		startPos = 0
	case FrameCurrentRow:
		startPos = pos
	case FrameOffsetPreceding:
		if len(data) > 0 && len(wd.OrderBy) > 0 {
			startPos = findRangeStart(pos, size, frame.Start.Offset, data, sorted, colMap, alias, wd.OrderBy[0].Expr)
		} else {
			startPos = pos
		}
	default:
		startPos = pos
	}

	// Determine end boundary
	switch frame.End.Type {
	case FrameUnboundedFollowing:
		endPos = size - 1
	case FrameCurrentRow:
		endPos = pos
	case FrameOffsetFollowing:
		if len(data) > 0 && len(wd.OrderBy) > 0 {
			endPos = findRangeEnd(pos, size, frame.End.Offset, data, sorted, colMap, alias, wd.OrderBy[0].Expr)
		} else {
			endPos = pos
		}
	default:
		endPos = pos
	}

	if startPos > endPos {
		return 0, -1
	}
	return startPos, endPos
}

// findRangeStart finds the first row whose ORDER BY value is within the offset range.
func findRangeStart(pos, size, offset int, data []Row, sorted []int, colMap ColMap, alias string, orderCol ColumnRef) int {
	currentVal := getRowValue(data[sorted[pos]], orderCol, colMap, alias)
	if currentVal == "" {
		return 0
	}
	currentFloat, err := strconv.ParseFloat(currentVal, 64)
	if err != nil {
		return 0
	}
	start := 0
	for i := pos - 1; i >= 0; i-- {
		val := getRowValue(data[sorted[i]], orderCol, colMap, alias)
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			start = i + 1
			break
		}
		if currentFloat-f > float64(offset) {
			start = i + 1
			break
		}
	}
	return start
}

// findRangeEnd finds the last row whose ORDER BY value is within the offset range.
func findRangeEnd(pos, size, offset int, data []Row, sorted []int, colMap ColMap, alias string, orderCol ColumnRef) int {
	currentVal := getRowValue(data[sorted[pos]], orderCol, colMap, alias)
	if currentVal == "" {
		return pos
	}
	currentFloat, err := strconv.ParseFloat(currentVal, 64)
	if err != nil {
		return pos
	}
	end := size - 1
	for i := pos + 1; i < size; i++ {
		val := getRowValue(data[sorted[i]], orderCol, colMap, alias)
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			end = i - 1
			break
		}
		if f-currentFloat > float64(offset) {
			end = i - 1
			break
		}
	}
	return end
}
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

package filequery

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IamWWT/dbexplain/query"
)

// Execute runs a SQL query against in-memory CSV data and returns a QueryResult.
//
// Parameters:
//   - sql: the SQL query string
//   - header: column names from the primary table
//   - rows: data rows (without header)
//   - extras: additional named datasets for JOIN queries (nil for single-table)
//   - maxRows: maximum rows to return (from --limit flag)
func Execute(sql string, header []string, rows [][]string, extras []NamedData, maxRows int) (*query.QueryResult, error) {
	start := time.Now()

	// Parse SQL
	stmt, err := Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Convert data to Row slice
	data := make([]Row, len(rows))
	for i, row := range rows {
		r := make(Row, len(row))
		for j, val := range row {
			r[j] = Value(val)
		}
		data[i] = r
	}

	// Build data sources map (table alias → data)
	sources := map[string]struct {
		header []string
		rows   []Row
	}{
		stmt.From: {header: header, rows: data},
	}
	if stmt.FromAlias != "" {
		sources[stmt.FromAlias] = sources[stmt.From]
	}

	// Add extra JOIN sources
	for _, extra := range extras {
		erows := make([]Row, len(extra.Rows))
		for i, row := range extra.Rows {
			r := make(Row, len(row))
			for j, val := range row {
				r[j] = Value(val)
			}
			erows[i] = r
		}
		sources[extra.Alias] = struct {
			header []string
			rows   []Row
		}{header: extra.Header, rows: erows}
	}

	// Get primary data
	primaryHeader := header
	currentData := data

	// Execute JOINs (if any)
	if len(stmt.Joins) > 0 {
		for _, join := range stmt.Joins {
			joinSrc, ok := sources[join.Table]
			if !ok && join.Alias != "" {
				joinSrc, ok = sources[join.Alias]
			}
			if !ok {
				return nil, fmt.Errorf("JOIN: table %q not found in data sources", join.Table)
			}
			if join.Alias != "" {
				if aliasSrc, ok := sources[join.Alias]; ok {
					joinSrc = aliasSrc
				}
			}
			joinData, joinHeader, err := executeHashJoin(
				currentData, primaryHeader, stmt.FromAlias,
				joinSrc.rows, joinSrc.header, join.Alias,
				join.On,
			)
			if err != nil {
				return nil, fmt.Errorf("JOIN error: %w", err)
			}
			currentData = joinData
			primaryHeader = joinHeader
		}
		// Rebuild colmap after JOIN
	}

	// Build column map
	colMap := BuildColMap(primaryHeader, stmt.FromAlias)
	for _, extra := range extras {
		extraColMap := BuildColMap(extra.Header, extra.Alias)
		for k, v := range extraColMap {
			if _, exists := colMap[k]; !exists {
				colMap[k] = v + len(primaryHeader)
			}
		}
	}

	// Apply WHERE filter
	if stmt.Where != nil {
		var filtered []Row
		for _, row := range currentData {
			result, err := Eval(stmt.Where, row, colMap)
			if err != nil {
				// If evaluation fails, skip the row (lenient)
				continue
			}
			if result.Bool() {
				filtered = append(filtered, row)
			}
		}
		currentData = filtered
	}

	// Handle GROUP BY + aggregates
	if HasAggregates(stmt) {
		result, err := executeAggregation(stmt, currentData, primaryHeader, colMap)
		if err != nil {
			return nil, err
		}
		// Apply ORDER BY to aggregation results
		if len(stmt.OrderBy) > 0 {
			sortAggResults(result, stmt.OrderBy)
		}
		result.ExecutionTime = time.Since(start).String()
		return result, nil
	}

	// Handle ORDER BY
	if len(stmt.OrderBy) > 0 {
		sort.SliceStable(currentData, func(i, j int) bool {
			for _, ob := range stmt.OrderBy {
				idx, ok := colMap[ob.Expr.Col]
				if !ok {
					// Try with table prefix
					idx, ok = colMap[stmt.FromAlias+"."+ob.Expr.Col]
					if !ok {
						continue
					}
				}
				if idx >= len(currentData[i]) || idx >= len(currentData[j]) {
					continue
				}
				vi := string(currentData[i][idx])
				vj := string(currentData[j][idx])

				// Try numeric comparison
				fi, errI := strconv.ParseFloat(vi, 64)
				fj, errJ := strconv.ParseFloat(vj, 64)
				if errI == nil && errJ == nil {
					if fi != fj {
						if ob.Dir == "DESC" {
							return fi > fj
						}
						return fi < fj
					}
					continue
				}

				// String comparison
				if vi != vj {
					if ob.Dir == "DESC" {
						return vi > vj
					}
					return vi < vj
				}
			}
			return false
		})
	}

	// Apply LIMIT/OFFSET
	limit := stmt.Limit
	if limit <= 0 {
		limit = maxRows
	}
	offset := stmt.Offset
	if offset > len(currentData) {
		offset = len(currentData)
	}
	end := offset + limit
	if end > len(currentData) {
		end = len(currentData)
	}
	currentData = currentData[offset:end]

	// Build SELECT columns (projection)
	result, err := buildResult(stmt, currentData, primaryHeader, colMap)
	if err != nil {
		return nil, err
	}

	result.ExecutionTime = time.Since(start).String()
	return result, nil
}

// executeHashJoin performs a hash join between two datasets.
func executeHashJoin(
	leftData []Row, leftHeader []string, leftAlias string,
	rightData []Row, rightHeader []string, rightAlias string,
	onExpr Expr,
) ([]Row, []string, error) {
	// Build combined column map
	colMap, combinedHeader := JoinColMaps(leftHeader, leftAlias, rightHeader, rightAlias)

	// Parse ON condition to extract join keys
	// Strategy: for each right row, build hash on the ON expression value
	// Since ON expr involves both tables, we need to evaluate it per left row.

	// Hash build phase: index right rows by the ON expression components
	// For simple "left.key = right.key" ON conditions, we extract the right column
	rightColIdx := -1
	if be, ok := onExpr.(*BinaryExpr); ok && be.Op == "=" {
		// Try to find which side references right table
		if colRef, ok := be.Right.(*ColumnRef); ok {
			if colRef.Table == rightAlias || colRef.Table == "" {
				if idx := findColIndex(colRef.Col, rightHeader, rightAlias); idx >= 0 {
					rightColIdx = idx
				}
			}
		}
	}

	if rightColIdx >= 0 {
		// Optimized hash join with known key
		hash := make(map[string][]Row)
		for _, rrow := range rightData {
			var key string
			if rightColIdx < len(rrow) {
				key = string(rrow[rightColIdx])
			}
			hash[key] = append(hash[key], rrow)
		}

		// Probe phase
		// Find the left column index
		var leftColIdx int = -1
		if be, ok := onExpr.(*BinaryExpr); ok && be.Op == "=" {
			if colRef, ok := be.Left.(*ColumnRef); ok {
				if idx := findColIndex(colRef.Col, leftHeader, leftAlias); idx >= 0 {
					leftColIdx = idx
				}
			}
		}

		var result []Row
		for _, lrow := range leftData {
			var key string
			if leftColIdx >= 0 && leftColIdx < len(lrow) {
				key = string(lrow[leftColIdx])
			}
			if matches, ok := hash[key]; ok {
				for _, rrow := range matches {
					combined := make(Row, len(combinedHeader))
					// Copy left row
					for j := range leftHeader {
						if j < len(lrow) {
							combined[j] = lrow[j]
						}
					}
					// Copy right row
					for j := range rightHeader {
						if j < len(rrow) {
							combined[len(leftHeader)+j] = rrow[j]
						}
					}
					result = append(result, combined)
				}
			}
		}
		return result, combinedHeader, nil
	}

	// Fallback: nested loop join for complex ON conditions
	var result []Row
	for _, lrow := range leftData {
		for _, rrow := range rightData {
			// Build combined row for ON evaluation
			combined := make(Row, len(combinedHeader))
			for j := range leftHeader {
				if j < len(lrow) {
					combined[j] = lrow[j]
				}
			}
			for j := range rightHeader {
				if j < len(rrow) {
					combined[len(leftHeader)+j] = rrow[j]
				}
			}
			val, err := Eval(onExpr, combined, colMap)
			if err == nil && val.Bool() {
				result = append(result, combined)
			}
		}
	}
	return result, combinedHeader, nil
}

// findColIndex finds a column index in a header, trying both bare name and qualified name.
func findColIndex(colName string, header []string, tableAlias string) int {
	for i, h := range header {
		if h == colName {
			return i
		}
	}
	return -1
}

// executeAggregation handles GROUP BY queries.
func executeAggregation(stmt *SelectStmt, data []Row, header []string, colMap ColMap) (*query.QueryResult, error) {
	// Determine group columns
	var groupCols []int
	for _, gb := range stmt.GroupBy {
		idx, ok := colMap[gb.Col]
		if !ok {
			idx, ok = colMap[stmt.FromAlias+"."+gb.Col]
		}
		if !ok {
			return nil, fmt.Errorf("GROUP BY column %q not found", gb.Col)
		}
		groupCols = append(groupCols, idx)
	}

	// Build aggregator
	agg := NewAggregator(groupCols)

	// Determine which columns in SELECT are aggregate vs group columns
	var resultCols []query.ColumnInfo
	var aggCols []AggCol
	var groupOutCols []struct {
		name  string
		index int
	}

	for _, sel := range stmt.Columns {
		colName := sel.Alias
		if colName == "" {
			colName = exprName(sel.Expr)
		}

		if fc, ok := sel.Expr.(*FuncCall); ok && IsAggregateFunc(fc.Name) {
			// Aggregate function
			argIdx := -1
			if len(fc.Args) == 1 {
				if cr, ok := fc.Args[0].(*ColumnRef); ok {
					idx, found := colMap[cr.Col]
					if !found {
						idx, found = colMap[stmt.FromAlias+"."+cr.Col]
					}
					if found {
						argIdx = idx
					}
				}
			}
			aggCols = append(aggCols, AggCol{
				Index: argIdx,
				Func:  fc.Name,
				Alias: colName,
			})
			resultCols = append(resultCols, query.ColumnInfo{Name: colName, Type: "FLOAT"})
		} else if cr, ok := sel.Expr.(*ColumnRef); ok && cr.Col != "*" {
			// Group column or regular column
			idx, found := colMap[cr.Col]
			if !found {
				idx, found = colMap[stmt.FromAlias+"."+cr.Col]
			}
			if found {
				groupOutCols = append(groupOutCols, struct {
					name  string
					index int
				}{colName, idx})
			}
			resultCols = append(resultCols, query.ColumnInfo{Name: colName, Type: "TEXT"})
		} else if cr, ok := sel.Expr.(*ColumnRef); ok && cr.Col == "*" && len(stmt.GroupBy) == 0 {
			// COUNT(*) without GROUP BY
			// Skip it here, handled below
			aggCols = append(aggCols, AggCol{Index: -1, Func: "COUNT", Alias: "count"})
			resultCols = append(resultCols, query.ColumnInfo{Name: "count", Type: "INTEGER"})
		} else {
			// Expression — evaluate per row, then aggregate
			// For now, treat as group column if it resolves to a col ref
			resultCols = append(resultCols, query.ColumnInfo{Name: colName, Type: "TEXT"})
		}
	}

	// If no explicit agg cols but HasAggregates is true, add them from SELECT
	for _, ac := range aggCols {
		agg.AddAggCol(ac)
	}

	// Feed data
	agg.Feed(data)

	// Compute
	aggResults := agg.Compute()

	// If no GROUP BY but has aggregates, still get one row
	if len(aggResults) == 0 && len(aggCols) > 0 {
		aggResults = []AggResult{{
			GroupKey: "[[single]]",
			Values:   make(map[string]string),
		}}
		// Compute single group
		var indices []int
		for i := range data {
			indices = append(indices, i)
		}
		for _, ac := range aggCols {
			agg2 := NewAggregator(nil)
			agg2.AddAggCol(ac)
			agg2.Feed(data)
			singleResults := agg2.Compute()
			if len(singleResults) > 0 {
				aggResults[0].Values[ac.Alias] = singleResults[0].Values[ac.Alias]
			}
		}
	}

	// Build result rows
	var outRows [][]*string
	for _, ar := range aggResults {
		var row []*string
		for _, sel := range stmt.Columns {
			colName := sel.Alias
			if colName == "" {
				colName = exprName(sel.Expr)
			}

			if fc, ok := sel.Expr.(*FuncCall); ok && IsAggregateFunc(fc.Name) {
				v := ar.Values[colName]
				row = append(row, &v)
			} else if cr, ok := sel.Expr.(*ColumnRef); ok && cr.Col != "*" {
				// Group column — take from first row in group
				idx, found := colMap[cr.Col]
				if !found {
					idx, found = colMap[stmt.FromAlias+"."+cr.Col]
				}
				if found && idx < len(ar.GroupRow) {
					v := string(ar.GroupRow[idx])
					row = append(row, &v)
				} else {
					v := ""
					row = append(row, &v)
				}
			} else {
				// Expression or computed
				if cr, ok := sel.Expr.(*ColumnRef); ok && cr.Col == "*" && len(stmt.GroupBy) == 0 {
					v := ar.Values["count"]
					if v == "" {
						v = fmt.Sprintf("%d", len(data))
					}
					row = append(row, &v)
				} else {
					// Try evaluating against first group row
					val, err := Eval(sel.Expr, ar.GroupRow, colMap)
					if err != nil {
						v := ""
						row = append(row, &v)
					} else {
						v := string(val)
						row = append(row, &v)
					}
				}
			}
		}
		outRows = append(outRows, row)
	}

	return &query.QueryResult{
		Columns:  resultCols,
		Rows:     outRows,
		RowCount: len(outRows),
	}, nil
}

// buildResult builds a QueryResult from the SELECT columns and data.
func buildResult(stmt *SelectStmt, data []Row, header []string, colMap ColMap) (*query.QueryResult, error) {
	if len(stmt.Columns) == 0 {
		return nil, fmt.Errorf("no columns in SELECT")
	}

	// If SELECT *, use all columns
	if len(stmt.Columns) == 1 {
		if cr, ok := stmt.Columns[0].Expr.(*ColumnRef); ok && cr.Col == "*" {
			cols := make([]query.ColumnInfo, len(header))
			for i, h := range header {
				cols[i] = query.ColumnInfo{Name: h, Type: "TEXT"}
			}
			rows := make([][]*string, len(data))
			for i, row := range data {
				r := make([]*string, len(header))
				for j := range header {
					if j < len(row) {
						v := string(row[j])
						r[j] = &v
					}
				}
				rows[i] = r
			}
			return &query.QueryResult{
				Columns:  cols,
				Rows:     rows,
				RowCount: len(rows),
			}, nil
		}
	}

	// Column projection
	var cols []query.ColumnInfo
	var colExprs []struct {
		expr Expr
		alias string
	}
	for _, sel := range stmt.Columns {
		name := sel.Alias
		if name == "" {
			name = exprName(sel.Expr)
		}
		cols = append(cols, query.ColumnInfo{Name: name, Type: "TEXT"})
		colExprs = append(colExprs, struct {
			expr  Expr
			alias string
		}{sel.Expr, name})
	}

	rows := make([][]*string, len(data))
	for i, row := range data {
		out := make([]*string, len(colExprs))
		for j, ce := range colExprs {
			val, err := Eval(ce.expr, row, colMap)
			if err != nil {
				v := ""
				out[j] = &v
			} else {
				v := string(val)
				out[j] = &v
			}
		}
		rows[i] = out
	}

	return &query.QueryResult{
		Columns:  cols,
		Rows:     rows,
		RowCount: len(rows),
	}, nil
}

// sortAggResults sorts aggregation results by ORDER BY clauses.
func sortAggResults(result *query.QueryResult, orderBy []OrderExpr) {
	if len(orderBy) == 0 || len(result.Rows) == 0 {
		return
	}

	// Build column index map from result columns
	colIdx := make(map[string]int)
	for i, col := range result.Columns {
		colIdx[col.Name] = i
	}

	sort.SliceStable(result.Rows, func(i, j int) bool {
		for _, ob := range orderBy {
			idx, ok := colIdx[ob.Expr.Col]
			if !ok {
				continue
			}
			if idx >= len(result.Rows[i]) || idx >= len(result.Rows[j]) {
				continue
			}
			vi := *result.Rows[i][idx]
			vj := *result.Rows[j][idx]

			// Try numeric comparison
			fi, errI := strconv.ParseFloat(vi, 64)
			fj, errJ := strconv.ParseFloat(vj, 64)
			if errI == nil && errJ == nil {
				if fi != fj {
					if ob.Dir == "DESC" {
						return fi > fj
					}
					return fi < fj
				}
				continue
			}

			// String comparison
			if vi != vj {
				if ob.Dir == "DESC" {
					return vi > vj
				}
				return vi < vj
			}
		}
		return false
	})
}

// exprName returns a human-readable name for an expression (for use as column header).
func exprName(e Expr) string {
	switch ex := e.(type) {
	case *ColumnRef:
		return ex.Col
	case *FuncCall:
		args := make([]string, len(ex.Args))
		for i, a := range ex.Args {
			args[i] = exprName(a)
		}
		return strings.ToUpper(ex.Name) + "(" + strings.Join(args, ",") + ")"
	case *BinaryExpr:
		return exprName(ex.Left) + "_" + exprName(ex.Right)
	case *NumberLit:
		return ex.Value
	case *StringLit:
		return "'" + ex.Value + "'"
	default:
		return "expr"
	}
}

// EnsureRows converts a 2D string slice to a Row slice.
func EnsureRows(data [][]string) []Row {
	rows := make([]Row, len(data))
	for i, r := range data {
		row := make(Row, len(r))
		for j, v := range r {
			row[j] = Value(v)
		}
		rows[i] = row
	}
	return rows
}

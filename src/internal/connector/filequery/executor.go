package filequery

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/IamWWT/dbexplain/internal/query"
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

	switch s := stmt.(type) {
	case *SelectStmt:
		result, err := executeSelect(s, header, rows, extras, maxRows)
		if result != nil {
			result.ExecutionTime = time.Since(start).String()
		}
		return result, err
	case *UnionStmt:
		return executeUnion(s, header, rows, extras, maxRows, start)
	default:
		return nil, fmt.Errorf("unsupported statement type %T", stmt)
	}
}

// executeSelect runs a single SELECT query against in-memory data.
func executeSelect(stmt *SelectStmt, header []string, rows [][]string, extras []NamedData, maxRows int) (*query.QueryResult, error) {
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
		var joinData []Row
		var joinHeader []string
		var joinErr error
		if join.JoinType == "RIGHT" {
				// RIGHT JOIN: swap left and right, then do LEFT JOIN
				joinData, joinHeader, joinErr = executeHashJoin(
					joinSrc.rows, joinSrc.header, join.Alias,
					currentData, primaryHeader, stmt.FromAlias,
					join.On, "LEFT",
				)
			} else {
				joinData, joinHeader, joinErr = executeHashJoin(
					currentData, primaryHeader, stmt.FromAlias,
					joinSrc.rows, joinSrc.header, join.Alias,
					join.On, join.JoinType,
				)
			}
			if joinErr != nil {
				return nil, fmt.Errorf("JOIN error: %w", joinErr)
			}
			currentData = joinData
			primaryHeader = joinHeader
		}
	}

	// Build column map
	var colMap ColMap
	if len(stmt.Joins) > 0 {
		// After JOIN, primaryHeader is the concatenation of primary + each JOIN's right table columns.
		// Build colMap with both aliases at the correct offsets.
		colMap = make(ColMap)
		// Primary table columns at offset 0..len(header)-1
		for i, col := range header {
			if stmt.FromAlias != "" {
				colMap[stmt.FromAlias+"."+col] = i
			}
			colMap[col] = i
		}
		// Each JOIN's right table columns at subsequent offsets
		offset := len(header)
		for _, join := range stmt.Joins {
			src, ok := sources[join.Table]
			if !ok && join.Alias != "" {
				src, ok = sources[join.Alias]
			}
			if ok {
				for i, col := range src.header {
					idx := offset + i
					if join.Alias != "" {
						colMap[join.Alias+"."+col] = idx
					}
					if _, exists := colMap[col]; !exists {
						colMap[col] = idx
					}
				}
				offset += len(src.header)
			}
		}
	} else {
		colMap = BuildColMap(primaryHeader, stmt.FromAlias)
		for _, extra := range extras {
			extraColMap := BuildColMap(extra.Header, extra.Alias)
			for k, v := range extraColMap {
				if _, exists := colMap[k]; !exists {
					colMap[k] = v + len(primaryHeader)
				}
			}
		}
	}

	// Pre-evaluate subqueries in WHERE clause
	var subqueryCache map[*SubqueryExpr]map[string]bool
	if stmt.Where != nil {
		subqueryCache = make(map[*SubqueryExpr]map[string]bool)
		collectSubqueries(stmt.Where, subqueryCache)
		// Execute each subquery and build result sets
		for subExpr := range subqueryCache {
			valSet := make(map[string]bool)
			subResult, err := executeSelect(subExpr.Stmt, header, rows, extras, maxRows)
			if err == nil && subResult != nil {
				for _, row := range subResult.Rows {
					if len(row) > 0 && row[0] != nil {
						valSet[*row[0]] = true
					}
			}
			subqueryCache[subExpr] = valSet
		}
	}
}

	// Apply WHERE filter with hash index optimization
	if stmt.Where != nil {
		currentData = applyWhereFilter(stmt, currentData, colMap, subqueryCache)
	}

	// Window function evaluation (after WHERE, before ORDER BY/projection)
	var winVals WindowValues
	if HasWindowFunctions(stmt) {
		winVals = computeWindowFunctions(stmt, currentData, colMap, stmt.FromAlias)
	}

	// Handle GROUP BY + aggregates
	if HasAggregates(stmt) {
		result, err := executeAggregation(stmt, currentData, primaryHeader, colMap)
		if err != nil {
			return nil, err
		}
		// Apply HAVING filter to aggregation results
		if stmt.Having != nil {
			result, err = applyHaving(result, stmt.Having)
			if err != nil {
				return nil, fmt.Errorf("HAVING: %w", err)
			}
		}
		// Apply ORDER BY to aggregation results
		if len(stmt.OrderBy) > 0 && result != nil {
			sortAggResults(result, stmt.OrderBy)
		}
		return result, nil
	}

	// Handle ORDER BY (with NULLS FIRST/LAST support)
	// Use index permutation to preserve original row positions for window value lookup
	var rowOrder []int // maps sorted position → original position (nil = no reorder)
	if len(stmt.OrderBy) > 0 {
		perm := make([]int, len(currentData))
		for i := range perm {
			perm[i] = i
		}
		sort.SliceStable(perm, func(a, b int) bool {
			i, j := perm[a], perm[b]
			for _, ob := range stmt.OrderBy {
				var vi, vj string
				idx, ok := colMap[ob.Expr.Col]
				if !ok {
					idx, ok = colMap[stmt.FromAlias+"."+ob.Expr.Col]
				}
				if ok {
					if idx >= len(currentData[i]) || idx >= len(currentData[j]) {
						continue
					}
					vi = string(currentData[i][idx])
					vj = string(currentData[j][idx])
				} else {
					// Resolve as SELECT alias (computed expression)
					var found bool
					for _, sel := range stmt.Columns {
						if sel.Alias != "" && sel.Alias == ob.Expr.Col {
							vv1, err1 := Eval(sel.Expr, Row(currentData[i]), colMap)
							vv2, err2 := Eval(sel.Expr, Row(currentData[j]), colMap)
							if err1 == nil && err2 == nil {
								vi = string(vv1)
								vj = string(vv2)
								found = true
							}
							break
						}
					}
					if !found {
						continue
					}
				}

				// NULLS FIRST/LAST handling
				viIsNull := vi == ""
				vjIsNull := vj == ""
				if viIsNull && vjIsNull {
					continue
				}
				if viIsNull {
					return ob.NullsDir == "FIRST" || (ob.NullsDir == "" && ob.Dir == "DESC")
				}
				if vjIsNull {
					return ob.NullsDir == "LAST" || (ob.NullsDir == "" && ob.Dir == "ASC")
				}

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
		// Reorder data according to permutation
		reordered := make([]Row, len(currentData))
		rowOrder = make([]int, len(currentData))
		for i, p := range perm {
			reordered[i] = currentData[p]
			rowOrder[i] = p
		}
		currentData = reordered
	}

	// Apply DISTINCT ON (after ORDER BY, keeps first row per group)
	if len(stmt.DistinctOn) > 0 {
		currentData = dedupDistinctOn(currentData, colMap, stmt.FromAlias, stmt.DistinctOn)
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
	result, err := buildResult(stmt, currentData, primaryHeader, colMap, winVals, rowOrder)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// executeUnion executes a UNION [ALL] of two SELECT statements.
func executeUnion(stmt *UnionStmt, header []string, rows [][]string, extras []NamedData, maxRows int, start time.Time) (*query.QueryResult, error) {
	left, err := executeSelect(stmt.Left, header, rows, extras, maxRows)
	if err != nil {
		return nil, fmt.Errorf("UNION left: %w", err)
	}

	right, err := executeSelect(stmt.Right, header, rows, extras, maxRows)
	if err != nil {
		return nil, fmt.Errorf("UNION right: %w", err)
	}

	merged := mergeResults(left, right, stmt.All, maxRows)
	merged.ExecutionTime = time.Since(start).String()
	return merged, nil
}

// mergeResults combines two query results for UNION [ALL].
// For UNION ALL, rows are concatenated. For UNION, duplicates are removed.
func mergeResults(left, right *query.QueryResult, all bool, maxRows int) *query.QueryResult {
	// Use left column names
	cols := left.Columns

	// Count total rows
	totalRows := len(left.Rows) + len(right.Rows)
	merged := make([][]*string, 0, totalRows)

	if all {
		// UNION ALL: simple concatenation
		merged = append(merged, left.Rows...)
		merged = append(merged, right.Rows...)
	} else {
		// UNION: concatenate and dedup
		seen := make(map[string]bool)
		addUnique := func(rows [][]*string) {
			for _, row := range rows {
				key := joinRowValues(row)
				if !seen[key] {
					seen[key] = true
					merged = append(merged, row)
				}
			}
		}
		addUnique(left.Rows)
		addUnique(right.Rows)
	}

	// Apply maxRows
	if maxRows > 0 && len(merged) > maxRows {
		merged = merged[:maxRows]
	}

	return &query.QueryResult{
		Columns:  cols,
		Rows:     merged,
		RowCount: len(merged),
	}
}

// joinRowValues creates a string key for a row (for dedup).
func joinRowValues(row []*string) string {
	var b strings.Builder
	for i, v := range row {
		if i > 0 {
			b.WriteByte('\x00')
		}
		if v != nil {
			b.WriteString(*v)
		}
	}
	return b.String()
}

// dedupDistinctOn keeps only the first row for each distinct value of the ON columns.
// Data must already be sorted by ORDER BY for correct "first row" semantics.
func dedupDistinctOn(data []Row, colMap ColMap, fromAlias string, distinctOn []ColumnRef) []Row {
	seen := make(map[string]bool)
	var result []Row
	for _, row := range data {
		var key strings.Builder
		for i, dcol := range distinctOn {
			if i > 0 {
				key.WriteByte('\x00')
			}
			idx, ok := colMap[dcol.Col]
			if !ok && fromAlias != "" {
				idx, ok = colMap[fromAlias+"."+dcol.Col]
			}
			if ok && idx < len(row) {
				key.WriteString(string(row[idx]))
			}
		}
		k := key.String()
		if !seen[k] {
			seen[k] = true
			result = append(result, row)
		}
	}
	return result
}

// collectSubqueries traverses an expression tree and collects all SubqueryExpr nodes.
func collectSubqueries(expr Expr, cache map[*SubqueryExpr]map[string]bool) {
	switch e := expr.(type) {
	case *BinaryExpr:
		collectSubqueries(e.Left, cache)
		collectSubqueries(e.Right, cache)
	case *SubqueryExpr:
		cache[e] = nil
	}
}

// executeHashJoin performs a hash join between two datasets.
// joinType: "INNER" (default) or "LEFT".
func executeHashJoin(
	leftData []Row, leftHeader []string, leftAlias string,
	rightData []Row, rightHeader []string, rightAlias string,
	onExpr Expr, joinType string,
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
			} else if joinType == "LEFT" {
				// LEFT JOIN: keep unmatched left rows, right columns empty
				combined := make(Row, len(combinedHeader))
				for j := range leftHeader {
					if j < len(lrow) {
						combined[j] = lrow[j]
					}
				}
				result = append(result, combined)
			}
		}
		return result, combinedHeader, nil
	}

	// Fallback: nested loop join for complex ON conditions
	var result []Row
	for _, lrow := range leftData {
		matched := false
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
				matched = true
			}
		}
		if !matched && joinType == "LEFT" {
			// LEFT JOIN: keep unmatched left rows, right columns empty
			combined := make(Row, len(combinedHeader))
			for j := range leftHeader {
				if j < len(lrow) {
					combined[j] = lrow[j]
				}
			}
			result = append(result, combined)
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
						return nil, fmt.Errorf("evaluate expression: %w", err)
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
func buildResult(stmt *SelectStmt, data []Row, header []string, colMap ColMap, winVals WindowValues, rowOrder []int) (*query.QueryResult, error) {
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
			// Window function: use pre-computed value instead of per-row Eval
			if winVals != nil {
				if vals, ok := winVals[j]; ok {
					origIdx := i
					if rowOrder != nil && i < len(rowOrder) {
						origIdx = rowOrder[i]
					}
					if origIdx < len(vals) {
						v := string(vals[origIdx])
						out[j] = &v
						continue
					}
				}
			}
			val, err := Eval(ce.expr, row, colMap)
			if err != nil {
				return nil, fmt.Errorf("evaluate column %q: %w", ce.alias, err)
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

			// NULLS FIRST/LAST handling
			viIsNull := vi == ""
			vjIsNull := vj == ""
			if viIsNull && vjIsNull {
				continue
			}
			if viIsNull {
				return ob.NullsDir == "FIRST" || (ob.NullsDir == "" && ob.Dir == "DESC")
			}
			if vjIsNull {
				return ob.NullsDir == "LAST" || (ob.NullsDir == "" && ob.Dir == "ASC")
			}

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

// applyHaving filters aggregation result rows by a HAVING expression.
// Rows that don't satisfy the HAVING condition are removed.
// Rows where HAVING evaluation errors out are also removed (defensive).
func applyHaving(result *query.QueryResult, having Expr) (*query.QueryResult, error) {
	if result == nil || len(result.Rows) == 0 {
		return result, nil
	}

	// Build column index map from result column names
	colMap := make(ColMap)
	for i, col := range result.Columns {
		colMap[col.Name] = i
	}

	// Filter rows
	var filtered [][]*string
	for _, row := range result.Rows {
		// Convert []*string to Row ([]Value) for the evaluator
		rowVals := make(Row, len(row))
		for j, cell := range row {
			if cell != nil {
				rowVals[j] = Value(*cell)
			}
		}

		val, err := Eval(having, rowVals, colMap)
		if err != nil {
			// Skip rows where HAVING evaluation fails (defensive)
			continue
		}
		if val.Bool() {
			filtered = append(filtered, row)
		}
	}

	result.Rows = filtered
	result.RowCount = len(filtered)
	return result, nil
}

// ── Hash index WHERE optimization ──────────────────────────────────────────

// applyWhereFilter filters rows using WHERE conditions with hash index optimization.
// For simple ColumnRef = LiteralValue equality chains, builds a hash index to
// avoid O(n) full scan. Falls back to full WHERE evaluation for complex expressions.
func applyWhereFilter(stmt *SelectStmt, data []Row, colMap ColMap, subqueryCache SubqueryCache) []Row {
	if stmt.Where == nil {
		return data
	}

	// Try hash index optimization for simple ColumnRef = LiteralValue patterns
	if colName, literalVal, ok := extractEqualityCondition(stmt.Where); ok {
		if colIdx, found := colMap[colName]; found {
			// Build hash index: value → row indices
			hash := make(map[string][]int)
			for i, row := range data {
				if colIdx < len(row) {
					key := string(row[colIdx])
					hash[key] = append(hash[key], i)
				}
			}

			// Probe with the literal value
			var filtered []Row
			if indices, hit := hash[literalVal]; hit {
				for _, i := range indices {
					filtered = append(filtered, data[i])
				}
			}

			// If WHERE has additional AND conditions, apply full eval on the subset
			if hasAdditionalConditions(stmt.Where) {
				return filterRows(stmt.Where, filtered, colMap, subqueryCache)
			}
			return filtered
		}
	}

	// Fallback: full scan
	return filterRows(stmt.Where, data, colMap, subqueryCache)
}

// filterRows evaluates the WHERE expression against every row.
func filterRows(where Expr, data []Row, colMap ColMap, cache SubqueryCache) []Row {
	var filtered []Row
	for _, row := range data {
		result, err := EvalWithSubqueries(where, row, colMap, cache)
		if err != nil {
			continue
		}
		if result.Bool() {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

// extractEqualityCondition walks the WHERE expression tree and extracts the
// first simple ColumnRef = LiteralValue (or LiteralValue = ColumnRef) pattern.
// For AND-chained conditions, finds the first equality for hash index probing.
// Returns ("", "", false) for complex expressions (OR, LIKE, BETWEEN, etc.).
func extractEqualityCondition(expr Expr) (colName string, literalValue string, ok bool) {
	be, ok := expr.(*BinaryExpr)
	if !ok {
		return "", "", false
	}
	if strings.EqualFold(be.Op, "AND") {
		// Walk AND chain, return first equality found
		if cn, lv, found := extractEqualityCondition(be.Left); found {
			return cn, lv, true
		}
		return extractEqualityCondition(be.Right)
	}
	if be.Op == "=" {
		// col = literal
		if cr, isCol := be.Left.(*ColumnRef); isCol {
			if lit, isLit := be.Right.(*StringLit); isLit {
				return cr.Col, lit.Value, true
			}
			if lit, isLit := be.Right.(*NumberLit); isLit {
				return cr.Col, lit.Value, true
			}
		}
		// literal = col (symmetric)
		if cr, isCol := be.Right.(*ColumnRef); isCol {
			if lit, isLit := be.Left.(*StringLit); isLit {
				return cr.Col, lit.Value, true
			}
			if lit, isLit := be.Left.(*NumberLit); isLit {
				return cr.Col, lit.Value, true
			}
		}
	}
	return "", "", false
}

// hasAdditionalConditions returns true if the WHERE root is an AND chain,
// meaning the extracted equality is a pre-filter and full WHERE re-evaluation is needed.
func hasAdditionalConditions(expr Expr) bool {
	be, ok := expr.(*BinaryExpr)
	return ok && strings.EqualFold(be.Op, "AND")
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

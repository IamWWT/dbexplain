package dsl

import (
	"fmt"
	"strings"
)

// CompileToPromQL compiles a QueryIR into a PromQL string.
//
// Supports:
//   - Metric name → PromQL metric selector
//   - String WHERE conditions (IsStr=true) → label matchers {k="v"}
//   - Aggregation + GROUP BY → avg by (col) (metric{matchers})
//   - ORDER BY, LIMIT, OFFSET, explicit column SELECT → post-processing
//
// Rejects:
//   - JOIN — Prometheus single-metric model
//   - Numeric WHERE (IsStr=false) — sample filters need binary expressions
//   - GROUP BY without aggregation — requires aggregation function
//   - SELECT * with GROUP BY — requires explicit aggregation
func CompileToPromQL(ir *QueryIR) (string, error) {
	if ir == nil {
		return "", &ErrCompile{"nil IR"}
	}

	// Raw PromQL: pass through directly without compilation.
	// The user specifies the complete PromQL expression via promql() syntax.
	// WHERE and GROUP BY are rejected because they cannot be safely composed
	// with arbitrary PromQL expressions — users should embed filters and
	// aggregation directly in the promql() expression.
	if ir.IsRawPromQL {
		if len(ir.Where) > 0 {
			return "", &ErrCompile{"WHERE is not supported with promql() — include filters in the expression"}
		}
		if len(ir.GroupBy) > 0 {
			return "", &ErrCompile{"GROUP BY is not supported with promql() — include aggregation in the expression"}
		}
		return ir.From, nil
	}

	// Reject JOINs
	if ir.HasJoins {
		return "", &ErrCompile{"JOIN is not supported for Prometheus DSL"}
	}

	// ── GROUP BY + aggregation handling ──
	if len(ir.GroupBy) > 0 {
		return compileGroupByPromQL(ir)
	}

	// Reject aggregations without GROUP BY
	if !ir.AllColumns {
		for _, col := range ir.Columns {
			if col.Func != "" {
				return "", &ErrCompile{
					fmt.Sprintf("aggregation %s without GROUP BY is not supported for Prometheus DSL", col.Func),
				}
			}
		}
	}

	// Note: ORDER BY, LIMIT, OFFSET, and explicit column SELECT are accepted
	// here and handled as post-processing by dslExecPromQL (execute.go).

	// Build PromQL
	base := ir.From

	// Label matchers from WHERE (IsStr=true only)
	var labelWhere []WhereIR
	for _, w := range ir.Where {
		if w.IsStr {
			labelWhere = append(labelWhere, w)
		}
	}

	// Reject sample filters
	for _, w := range ir.Where {
		if !w.IsStr {
			return "", &ErrCompile{
				fmt.Sprintf("numeric WHERE conditions are not supported for Prometheus DSL (column %q)", w.Column),
			}
		}
	}

	// Add label matchers
	if len(labelWhere) > 0 {
		matchers, err := whereToPromQLLabelMatchers(labelWhere)
		if err != nil {
			return "", &ErrCompile{err.Error()}
		}
		base += "{" + matchers + "}"
	}

	return base, nil
}

// compileGroupByPromQL compiles a GROUP BY + aggregation query to PromQL.
func compileGroupByPromQL(ir *QueryIR) (string, error) {
	// Find the aggregation function from SELECT columns
	var aggFunc string
	var groupCols []string

	for _, col := range ir.Columns {
		if col.Func != "" {
			if aggFunc != "" {
				return "", &ErrCompile{"multiple aggregation functions not supported for Prometheus DSL"}
			}
			aggFunc = col.Func
		}
	}

	if aggFunc == "" {
		return "", &ErrCompile{"GROUP BY requires an aggregation function (e.g. COUNT, AVG, SUM)"}
	}

	if ir.AllColumns {
		return "", &ErrCompile{"SELECT * with GROUP BY is not supported — specify aggregation columns explicitly"}
	}

	for _, g := range ir.GroupBy {
		groupCols = append(groupCols, g.Column)
	}

	// Map SQL aggregation names to PromQL
	promQLAgg := promQLAggFunc(aggFunc)
	if promQLAgg == "" {
		return "", &ErrCompile{fmt.Sprintf("aggregation function %q is not supported for Prometheus DSL", aggFunc)}
	}

	// Build base metric selector
	base := ir.From

	// Label matchers from WHERE
	var labelWhere []WhereIR
	for _, w := range ir.Where {
		if w.IsStr {
			labelWhere = append(labelWhere, w)
		}
	}
	for _, w := range ir.Where {
		if !w.IsStr {
			return "", &ErrCompile{
				fmt.Sprintf("numeric WHERE conditions are not supported for Prometheus DSL (column %q)", w.Column),
			}
		}
	}

	if len(labelWhere) > 0 {
		matchers, err := whereToPromQLLabelMatchers(labelWhere)
		if err != nil {
			return "", &ErrCompile{err.Error()}
		}
		base += "{" + matchers + "}"
	}

	// Build: aggFunc by (groupCols) (metric{matchers})
	promQL := fmt.Sprintf("%s by (%s) (%s)", promQLAgg, strings.Join(groupCols, ","), base)
	return promQL, nil
}

// promQLAggFunc maps SQL aggregation function names to PromQL equivalents.
func promQLAggFunc(sqlFunc string) string {
	switch strings.ToUpper(sqlFunc) {
	case "COUNT":
		return "count"
	case "SUM":
		return "sum"
	case "AVG":
		return "avg"
	case "MIN":
		return "min"
	case "MAX":
		return "max"
	case "GROUP":
		return "group"
	case "STDDEV":
		return "stddev"
	case "STDVAR":
		return "stdvar"
	default:
		return ""
	}
}

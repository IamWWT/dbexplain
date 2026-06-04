package dsl

import (
	"fmt"
)

// CompileToPromQL compiles a QueryIR into a PromQL string.
//
// Phase 1 supports:
//   - Metric name → PromQL metric selector
//   - String WHERE conditions (IsStr=true) → label matchers {k="v"}
//
// Phase 1 rejects (returns clear error):
//   - Aggregation functions (COUNT, SUM, AVG, etc.) — PromQL semantics differ
//   - GROUP BY — requires specific aggregation function binding
//   - ORDER BY, LIMIT, OFFSET — PromQL has no equivalent
//   - JOIN — Prometheus single-metric model
//   - Numeric WHERE (IsStr=false) — sample filters need binary expressions
func CompileToPromQL(ir *QueryIR) (string, error) {
	if ir == nil {
		return "", &ErrCompile{"nil IR"}
	}

	// Reject JOINs
	if ir.HasJoins {
		return "", &ErrCompile{"JOIN is not supported for Prometheus DSL"}
	}

	// Reject aggregations
	if !ir.AllColumns {
		for _, col := range ir.Columns {
			if col.Func != "" {
				return "", &ErrCompile{
					fmt.Sprintf("aggregation %s is not supported for Prometheus DSL", col.Func),
				}
			}
		}
	}

	// Reject GROUP BY
	if len(ir.GroupBy) > 0 {
		return "", &ErrCompile{"GROUP BY is not supported for Prometheus DSL"}
	}

	// Reject ORDER BY
	if len(ir.OrderBy) > 0 {
		return "", &ErrCompile{"ORDER BY is not supported for Prometheus DSL"}
	}

	// Reject LIMIT / OFFSET
	if ir.Limit > 0 {
		return "", &ErrCompile{"LIMIT is not supported for Prometheus DSL"}
	}
	if ir.Offset > 0 {
		return "", &ErrCompile{"OFFSET is not supported for Prometheus DSL"}
	}

	// Verify SELECT * (Phase 1 requirement)
	if !ir.AllColumns {
		return "", &ErrCompile{"only SELECT * is supported for Prometheus DSL"}
	}

	// Build PromQL
	base := ir.From

	// Label matchers from WHERE (IsStr=true only)
	var labelWhere []WhereIR
	var sampleWhere []WhereIR
	for _, w := range ir.Where {
		if w.IsStr {
			labelWhere = append(labelWhere, w)
		} else {
			sampleWhere = append(sampleWhere, w)
		}
	}

	// Reject sample filters in Phase 1
	if len(sampleWhere) > 0 {
		return "", &ErrCompile{
			fmt.Sprintf("numeric WHERE conditions are not supported for Prometheus DSL (column %q)", sampleWhere[0].Column),
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

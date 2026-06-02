package filequery

import (
	"fmt"
	"math"
	"strings"
)

// Aggregator performs hash-based aggregation (GROUP BY).
type Aggregator struct {
	groupCols []int       // column indices for GROUP BY keys
	aggCols   []AggCol    // aggregation columns
	groups    map[string][]int // hash key → row indices
	groupKeys []string    // ordered group keys
	rows      []Row       // all input rows
}

// AggCol defines an aggregation target column.
type AggCol struct {
	Index int    // column index in input rows
	Func  string // SUM, AVG, COUNT, MAX, MIN
	Alias string // output column alias
}

// NewAggregator creates a new Aggregator.
func NewAggregator(groupCols []int) *Aggregator {
	return &Aggregator{
		groupCols: groupCols,
		groups:    make(map[string][]int),
	}
}

// AddAggCol adds an aggregation column.
func (a *Aggregator) AddAggCol(col AggCol) {
	a.aggCols = append(a.aggCols, col)
}

// Feed processes all rows through the aggregator.
func (a *Aggregator) Feed(rows []Row) {
	a.rows = rows
	a.groupKeys = nil
	a.groups = make(map[string][]int)

	for i, row := range rows {
		key := a.groupKey(row)
		if _, exists := a.groups[key]; !exists {
			a.groups[key] = []int{i}
			a.groupKeys = append(a.groupKeys, key)
		} else {
			a.groups[key] = append(a.groups[key], i)
		}
	}
}

// groupKey computes the GROUP BY key for a row.
func (a *Aggregator) groupKey(row Row) string {
	if len(a.groupCols) == 0 {
		return "[[single]]" // single group for no GROUP BY
	}
	parts := make([]string, len(a.groupCols))
	for i, ci := range a.groupCols {
		if ci >= 0 && ci < len(row) {
			parts[i] = string(row[ci])
		}
	}
	return strings.Join(parts, "\x00")
}

// Result holds a single group's aggregation results.
type AggResult struct {
	GroupKey string            // the GROUP BY key
	Values   map[string]string // agg col alias → computed value
	GroupRow Row               // the first row in the group (for non-aggregate cols)
}

// Compute processes all groups and returns results.
func (a *Aggregator) Compute() []AggResult {
	if len(a.groupKeys) == 0 {
		return nil
	}

	results := make([]AggResult, 0, len(a.groupKeys))
	for _, key := range a.groupKeys {
		indices := a.groups[key]
		res := AggResult{
			GroupKey: key,
			Values:   make(map[string]string),
			GroupRow: a.rows[indices[0]],
		}
		for _, ac := range a.aggCols {
			res.Values[ac.Alias] = a.computeAgg(indices, ac)
		}
		results = append(results, res)
	}
	return results
}

// computeAgg computes a single aggregation for a group.
func (a *Aggregator) computeAgg(indices []int, ac AggCol) string {
	switch ac.Func {
	case "COUNT":
		return fmt.Sprintf("%d", len(indices))
	case "SUM":
		var sum float64
		hasVal := false
		for _, idx := range indices {
			if ac.Index >= 0 && ac.Index < len(a.rows[idx]) {
				f, ok := a.rows[idx][ac.Index].Float()
				if ok {
					sum += f
					hasVal = true
				}
			}
		}
		if !hasVal {
			return ""
		}
		return formatFloat(sum)
	case "AVG":
		var sum float64
		count := 0
		for _, idx := range indices {
			if ac.Index >= 0 && ac.Index < len(a.rows[idx]) {
				f, ok := a.rows[idx][ac.Index].Float()
				if ok {
					sum += f
					count++
				}
			}
		}
		if count == 0 {
			return ""
		}
		return formatFloat(sum / float64(count))
	case "MAX":
		var max float64
		hasVal := false
		for _, idx := range indices {
			if ac.Index >= 0 && ac.Index < len(a.rows[idx]) {
				f, ok := a.rows[idx][ac.Index].Float()
				if ok {
					if !hasVal || f > max {
						max = f
						hasVal = true
					}
				}
			}
		}
		if !hasVal {
			return ""
		}
		return formatFloat(max)
	case "MIN":
		var min float64
		hasVal := false
		for _, idx := range indices {
			if ac.Index >= 0 && ac.Index < len(a.rows[idx]) {
				f, ok := a.rows[idx][ac.Index].Float()
				if ok {
					if !hasVal || f < min {
						min = f
						hasVal = true
					}
				}
			}
		}
		if !hasVal {
			return ""
		}
		return formatFloat(min)
	default:
		return ""
	}
}

// formatFloat formats a float64, removing trailing zeros.
func formatFloat(f float64) string {
	if f == math.Trunc(f) {
		return fmt.Sprintf("%.0f", f)
	}
	return fmt.Sprintf("%.4f", f)
}

// IsAggregate returns true if the expression uses aggregate functions.
// Used by the executor to decide whether to use the aggregator path.
func IsAggregate(expr Expr) bool {
	switch e := expr.(type) {
	case *FuncCall:
		// Window functions (with OVER clause) are NOT GROUP BY aggregates
		if IsWindowFunc(e) {
			return false
		}
		if IsAggregateFunc(e.Name) {
			return true
		}
		for _, arg := range e.Args {
			if IsAggregate(arg) {
				return true
			}
		}
	case *BinaryExpr:
		return IsAggregate(e.Left) || IsAggregate(e.Right)
	case *UnaryExpr:
		return IsAggregate(e.Right)
	case *BetweenExpr:
		return IsAggregate(e.Expr) || IsAggregate(e.Low) || IsAggregate(e.High)
	}
	return false
}

// HasAggregates checks if any of the SELECT expressions contain aggregates.
func HasAggregates(stmt *SelectStmt) bool {
	for _, col := range stmt.Columns {
		if IsAggregate(col.Expr) {
			return true
		}
	}
	return false
}

package dsl

import (
	"fmt"
	"strings"

	"github.com/IamWWT/dbexplain/internal/sqlast"
)

// Vendor identifies the query language family of a data source.
type Vendor int

const (
	VendorSQL    Vendor = iota // SQL databases (mysql, postgres, sqlite, etc.)
	VendorPromQL               // Prometheus (PromQL)
	VendorFile                 // File-based (CSV/XLSX)
)

// String returns the human-readable name of the vendor.
func (v Vendor) String() string {
	switch v {
	case VendorSQL:
		return "SQL"
	case VendorPromQL:
		return "PromQL"
	case VendorFile:
		return "file"
	default:
		return "unknown"
	}
}

// QueryIR is a vendor-agnostic intermediate representation of a DSL query.
// It is produced by parsing SQL syntax with sqlast, then converting the AST
// to this IR. Compilers (CompileToSQL, CompileToPromQL, etc.) translate the
// IR to the target query language.
type QueryIR struct {
	AllColumns  bool        // true = SELECT *
	Columns     []ColumnIR  // explicit column list (ignored if AllColumns)
	From        string      // primary table or metric name
	Alias       string      // table alias (empty if none)
	Where       []WhereIR   // WHERE conditions (AND flattened, OR rejected)
	GroupBy     []GroupByIR // GROUP BY columns
	OrderBy     []OrderByIR // ORDER BY entries
	Limit       int         // LIMIT (0 = no limit)
	Offset      int         // OFFSET (0 = no offset)
	HasJoins    bool        // true if query has JOIN clauses (not supported for PromQL)
	IsRawPromQL bool        // true = From is raw PromQL, skip compilation
}

// ColumnIR represents a selected expression, optionally with aggregation.
type ColumnIR struct {
	Name   string     // column or metric name
	Alias  string     // AS alias
	Func   string     // aggregate: "COUNT", "SUM", "AVG", "RATE", "" for plain
	Args   []ColumnIR // function arguments (for Func != "")
	Window string     // PromQL range: "5m", "1h", "" for instant
}

// WhereIR represents a single WHERE condition.
type WhereIR struct {
	Column string // label or field name
	Op     string // =, !=, =~, !~, >, <, >=, <=
	Value  string // comparison value
	IsStr  bool   // true = string literal (label matcher), false = numeric (sample filter)
}

// GroupByIR represents a GROUP BY column.
type GroupByIR struct {
	Column string
}

// OrderByIR represents an ORDER BY entry.
type OrderByIR struct {
	Column string
	Desc   bool
}

// SelectStmtToIR converts a parsed SQL SELECT statement to a vendor-agnostic IR.
// It extracts the structural elements (SELECT/FROM/WHERE/GROUP BY/ORDER BY)
// while leaving semantic interpretation to the compiler.
func SelectStmtToIR(stmt *sqlast.SelectStmt) (*QueryIR, error) {
	if stmt == nil {
		return nil, fmt.Errorf("nil SELECT statement")
	}

	ir := &QueryIR{
		From:    stmt.From,
		Alias:   stmt.FromAlias,
		Limit:   stmt.Limit,
		Offset:  stmt.Offset,
		HasJoins: len(stmt.Joins) > 0,
	}

	// SELECT clause
	if len(stmt.Columns) == 1 && isStarColumn(stmt.Columns[0]) {
		ir.AllColumns = true
	} else {
		for _, col := range stmt.Columns {
			columnIR, err := selectExprToColumnIR(col)
			if err != nil {
				return nil, err
			}
			ir.Columns = append(ir.Columns, columnIR)
		}
	}

	// WHERE clause
	if stmt.Where != nil {
		where, err := whereToIR(stmt.Where)
		if err != nil {
			return nil, fmt.Errorf("WHERE: %w", err)
		}
		ir.Where = where
	}

	// GROUP BY
	for _, g := range stmt.GroupBy {
		ir.GroupBy = append(ir.GroupBy, GroupByIR{Column: g.Col})
	}

	// ORDER BY
	for _, o := range stmt.OrderBy {
		ir.OrderBy = append(ir.OrderBy, OrderByIR{
			Column: o.Expr.Col,
			Desc:   o.Dir == "DESC",
		})
	}

	return ir, nil
}

// isStarColumn checks if a SelectExpr represents SELECT *.
func isStarColumn(se sqlast.SelectExpr) bool {
	if cr, ok := se.Expr.(*sqlast.ColumnRef); ok && cr.Col == "*" && cr.Table == "" {
		return true
	}
	return false
}

// selectExprToColumnIR converts a single SELECT expression to ColumnIR.
func selectExprToColumnIR(se sqlast.SelectExpr) (ColumnIR, error) {
	if fc, ok := se.Expr.(*sqlast.FuncCall); ok {
		// Function call: COUNT(*), SUM(col), RATE(metric, '5m'), etc.
		args := make([]ColumnIR, len(fc.Args))
		for i, a := range fc.Args {
			switch v := a.(type) {
			case *sqlast.ColumnRef:
				args[i] = ColumnIR{Name: v.Col}
			case *sqlast.StringLit:
				args[i] = ColumnIR{Name: v.Value}
			case *sqlast.NumberLit:
				args[i] = ColumnIR{Name: v.Value}
			default:
				args[i] = ColumnIR{Name: a.String()}
			}
		}
		return ColumnIR{
			Name:  se.Alias, // Use alias as display name if set
			Alias: se.Alias,
			Func:  fc.Name,
			Args:  args,
		}, nil
	}

	if cr, ok := se.Expr.(*sqlast.ColumnRef); ok {
		return ColumnIR{
			Name:  cr.Col,
			Alias: se.Alias,
		}, nil
	}

	// Fallback: string representation
	return ColumnIR{
		Name:  se.Expr.String(),
		Alias: se.Alias,
	}, nil
}

// whereToIR recursively converts a WHERE expression tree to a flat list of WhereIR.
// AND chains are flattened into multiple conditions. OR is rejected.
func whereToIR(expr sqlast.Expr) ([]WhereIR, error) {
	bin, ok := expr.(*sqlast.BinaryExpr)
	if !ok {
		return nil, fmt.Errorf("unsupported WHERE expression type %T", expr)
	}

	// AND: flatten into multiple conditions
	if strings.EqualFold(bin.Op, "AND") {
		left, err := whereToIR(bin.Left)
		if err != nil {
			return nil, err
		}
		right, err := whereToIR(bin.Right)
		if err != nil {
			return nil, err
		}
		return append(left, right...), nil
	}

	// OR: reject (PromQL label matchers don't support OR)
	if strings.EqualFold(bin.Op, "OR") {
		return nil, fmt.Errorf("OR conditions are not supported")
	}

	// NOT: reject
	if bin.Op == "NOT" {
		return nil, fmt.Errorf("NOT conditions are not supported")
	}

	// Leaf condition
	return leafConditionToIR(bin)
}

// leafConditionToIR converts a single BinaryExpr leaf to WhereIR.
func leafConditionToIR(bin *sqlast.BinaryExpr) ([]WhereIR, error) {
	// Left must be a column reference
	colRef, ok := bin.Left.(*sqlast.ColumnRef)
	if !ok {
		return nil, fmt.Errorf("left side of WHERE must be a column reference")
	}

	op := bin.Op

	// Normalize operators
	switch op {
	case "=", "!=", "=~", "!~", ">", "<", ">=", "<=", "LIKE", "NOT LIKE", "IN", "NOT IN":
		// valid
	default:
		return nil, fmt.Errorf("unsupported WHERE operator: %s", op)
	}

	switch right := bin.Right.(type) {
	case *sqlast.StringLit:
		return []WhereIR{{
			Column: colRef.Col,
			Op:     op,
			Value:  right.Value,
			IsStr:  true,
		}}, nil
	case *sqlast.NumberLit:
		return []WhereIR{{
			Column: colRef.Col,
			Op:     op,
			Value:  right.Value,
			IsStr:  false,
		}}, nil
	default:
		return nil, fmt.Errorf("unsupported WHERE value type for column %q", colRef.Col)
	}
}

// whereToPromQLLabelMatchers converts WHERE conditions to PromQL label matcher string.
// Only IsStr=true conditions are included (label matchers).
func whereToPromQLLabelMatchers(where []WhereIR) (string, error) {
	var matchers []string
	for _, w := range where {
		if !w.IsStr {
			continue // skip numeric conditions in Phase 1
		}
		switch w.Op {
		case "=", "!=", "=~", "!~":
			matchers = append(matchers, fmt.Sprintf("%s%q", w.Column+w.Op, w.Value))
		case "LIKE":
			matchers = append(matchers, fmt.Sprintf("%s=~%q", w.Column, w.Value))
		case "NOT LIKE":
			matchers = append(matchers, fmt.Sprintf("%s!~%q", w.Column, w.Value))
		default:
			return "", fmt.Errorf("unsupported label matcher operator: %s", w.Op)
		}
	}
	return strings.Join(matchers, ","), nil
}

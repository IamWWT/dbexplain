// Package sqlast provides shared SQL AST types, lexer, and parser.
// Extracted from connector/filequery for reuse by sqlguard, policy, and DSL.
package sqlast

import (
	"fmt"
	"strings"
)

// --- Expression types ---

// Expr is implemented by all expression nodes.
type Expr interface {
	exprNode()
	String() string
}

// ColumnRef references a column, optionally qualified by table alias (e.g., "t.col").
type ColumnRef struct {
	Table string // table alias (empty if unqualified)
	Col   string // column name
}

func (c *ColumnRef) exprNode()       {}
func (c *ColumnRef) String() string {
	if c.Table != "" {
		return c.Table + "." + c.Col
	}
	return c.Col
}

// NumberLit is a numeric literal.
type NumberLit struct {
	Value string // raw string form
}

func (n *NumberLit) exprNode()       {}
func (n *NumberLit) String() string { return n.Value }

// StringLit is a string literal (single-quoted in SQL).
type StringLit struct {
	Value string
}

func (s *StringLit) exprNode()       {}
func (s *StringLit) String() string { return "'" + s.Value + "'" }

// BinaryExpr represents a binary operation (comparison, arithmetic, logical).
type BinaryExpr struct {
	Left  Expr
	Op    string // =, !=, <, >, <=, >=, +, -, *, /, AND, OR, LIKE, IN, NOT IN
	Right Expr
}

func (b *BinaryExpr) exprNode()       {}
func (b *BinaryExpr) String() string { return fmt.Sprintf("(%s %s %s)", b.Left, b.Op, b.Right) }

// UnaryExpr represents a unary operation (e.g., NOT).
type UnaryExpr struct {
	Op    string // NOT, -
	Right Expr
}

func (u *UnaryExpr) exprNode()       {}
func (u *UnaryExpr) String() string { return fmt.Sprintf("%s(%s)", u.Op, u.Right) }

// FrameBoundType represents a window frame boundary type.
type FrameBoundType string

const (
	FrameUnboundedPreceding FrameBoundType = "UNBOUNDED PRECEDING"
	FrameOffsetPreceding    FrameBoundType = "OFFSET PRECEDING"
	FrameCurrentRow         FrameBoundType = "CURRENT ROW"
	FrameOffsetFollowing    FrameBoundType = "OFFSET FOLLOWING"
	FrameUnboundedFollowing FrameBoundType = "UNBOUNDED FOLLOWING"
)

// FrameBound defines one endpoint of a window frame.
type FrameBound struct {
	Type   FrameBoundType // boundary type
	Offset int            // numeric offset (0 for CURRENT ROW and UNBOUNDED)
}

// WindowFrame defines a ROWS/RANGE frame specification.
type WindowFrame struct {
	Type  string     // "ROWS" or "RANGE"
	Start FrameBound // frame start
	End   FrameBound // frame end
}

// String returns the SQL representation of a window frame.
func (wf *WindowFrame) String() string {
	if wf == nil {
		return ""
	}
	startStr := frameBoundToString(wf.Start)
	endStr := frameBoundToString(wf.End)
	return wf.Type + " BETWEEN " + startStr + " AND " + endStr
}

func frameBoundToString(b FrameBound) string {
	switch b.Type {
	case FrameUnboundedPreceding:
		return "UNBOUNDED PRECEDING"
	case FrameUnboundedFollowing:
		return "UNBOUNDED FOLLOWING"
	case FrameCurrentRow:
		return "CURRENT ROW"
	case FrameOffsetPreceding:
		return fmt.Sprintf("%d PRECEDING", b.Offset)
	case FrameOffsetFollowing:
		return fmt.Sprintf("%d FOLLOWING", b.Offset)
	default:
		return "UNKNOWN"
	}
}

// WindowDef defines an OVER clause for window functions.
type WindowDef struct {
	PartitionBy []ColumnRef  // PARTITION BY columns (nil if none)
	OrderBy     []OrderExpr  // ORDER BY within window (nil if none)
	Frame       *WindowFrame // nil = default frame (full partition if no ORDER BY, RANGE UNBOUNDED PRECEDING TO CURRENT ROW if ORDER BY)
}

// FuncCall represents a function call (SUM, AVG, COUNT, MAX, MIN, CAST, ABS, ROW_NUMBER, etc.).
type FuncCall struct {
	Name       string     // function name (uppercase)
	Args       []Expr     // function arguments
	CastType   string     // for CAST: target type name
	IsDistinct bool       // true if DISTINCT keyword present
	Over       *WindowDef // OVER clause for window functions (nil if not a window function)
}

func (f *FuncCall) exprNode() {}
func (f *FuncCall) String() string {
	if f.Name == "CAST" && len(f.Args) == 1 {
		return fmt.Sprintf("CAST(%s AS %s)", f.Args[0], f.CastType)
	}
	args := make([]string, len(f.Args))
	for i, a := range f.Args {
		args[i] = a.String()
	}
	s := fmt.Sprintf("%s(%s)", f.Name, strings.Join(args, ", "))
	if f.Over != nil {
		parts := make([]string, 0)
		if len(f.Over.PartitionBy) > 0 {
			pb := make([]string, len(f.Over.PartitionBy))
			for i, c := range f.Over.PartitionBy {
				pb[i] = c.String()
			}
			parts = append(parts, "PARTITION BY "+strings.Join(pb, ", "))
		}
		if len(f.Over.OrderBy) > 0 {
			ob := make([]string, len(f.Over.OrderBy))
			for i, o := range f.Over.OrderBy {
				s := o.Expr.String()
				if o.Dir == "DESC" {
					s += " DESC"
				}
				if o.NullsDir != "" {
					s += " NULLS " + o.NullsDir
				}
				ob[i] = s
			}
			parts = append(parts, "ORDER BY "+strings.Join(ob, ", "))
		}
		if f.Over.Frame != nil {
			parts = append(parts, f.Over.Frame.String())
		}
		s += " OVER (" + strings.Join(parts, " ") + ")"
	}
	return s
}

// BetweenExpr represents x BETWEEN a AND b.
type BetweenExpr struct {
	Expr Expr
	Low  Expr
	High Expr
}

func (b *BetweenExpr) exprNode()       {}
func (b *BetweenExpr) String() string { return fmt.Sprintf("%s BETWEEN %s AND %s", b.Expr, b.Low, b.High) }

// --- Statement types ---

// Stmt is the interface implemented by all statement types.
type Stmt interface {
	stmtNode()
}

// SelectExpr is a single expression in the SELECT list, optionally aliased.
type SelectExpr struct {
	Expr  Expr
	Alias string // AS alias (empty if no alias)
}

// SelectStmt is the parsed representation of a SELECT query.
type SelectStmt struct {
	Columns    []SelectExpr // SELECT list
	From       string       // primary table name
	FromAlias  string       // primary table alias (empty if none)
	Joins      []JoinClause // JOIN clauses
	Where      Expr         // WHERE expression (nil if none)
	GroupBy    []ColumnRef  // GROUP BY columns
	OrderBy    []OrderExpr  // ORDER BY clauses
	Limit      int          // LIMIT (0 = no limit)
	Offset     int          // OFFSET (0 = no offset)
	Having     Expr         // HAVING expression (nil if none)
	DistinctOn []ColumnRef  // DISTINCT ON columns (nil if none)
}

func (s *SelectStmt) stmtNode() {}

// UnionStmt represents a UNION [ALL] of two SELECT statements.
type UnionStmt struct {
	Left  *SelectStmt
	Right *SelectStmt
	All   bool // true = UNION ALL, false = UNION (distinct)
}

func (u *UnionStmt) stmtNode() {}

// JoinClause represents a single JOIN clause.
type JoinClause struct {
	Table    string // table name
	Alias    string // table alias (empty if none)
	On       Expr   // ON condition
	JoinType string // "INNER" or "LEFT" (default "INNER")
}

// OrderExpr represents a single ORDER BY entry.
type OrderExpr struct {
	Expr     ColumnRef // the column to order by
	Dir      string    // "ASC" or "DESC"
	NullsDir string    // "FIRST", "LAST", or "" (default)
}

// AggInfo holds aggregation information for a column.
type AggInfo struct {
	Func     string // SUM, AVG, COUNT, MAX, MIN ("" for non-aggregate)
	Col      string // column name (may include table qualifier)
	Distinct bool
}

// SubqueryExpr represents a scalar subquery: (SELECT ...)
type SubqueryExpr struct {
	Stmt *SelectStmt // the inner SELECT statement
}

func (s *SubqueryExpr) exprNode()       {}
func (s *SubqueryExpr) String() string { return "(SELECT ...)" }

// --- NamedData holds a named dataset for JOIN resolution ---
type NamedData struct {
	Alias  string
	Header []string
	Rows   [][]string
}

// ResolveTableAlias determines the effective table alias for a table reference.
func ResolveTableAlias(table, alias string) string {
	if alias != "" {
		return alias
	}
	return table
}

// IsAggregateFunc returns true if the function name is an aggregate.
func IsAggregateFunc(name string) bool {
	switch strings.ToUpper(name) {
	case "SUM", "AVG", "COUNT", "MAX", "MIN":
		return true
	}
	return false
}

// IsWindowFunc returns true if the FuncCall is a window function (has OVER clause).
func IsWindowFunc(fc *FuncCall) bool {
	return fc != nil && fc.Over != nil
}

// IsRankingFunc returns true for ranking window functions.
func IsRankingFunc(name string) bool {
	switch strings.ToUpper(name) {
	case "ROW_NUMBER", "RANK", "DENSE_RANK", "NTILE":
		return true
	}
	return false
}

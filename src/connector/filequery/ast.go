// Package filequery provides an in-memory SQL query engine for CSV/XLSX data.
// It supports SELECT, WHERE, GROUP BY, ORDER BY, JOIN, aggregations, and expressions.
// No external dependencies — pure Go standard library.
package filequery

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

// FuncCall represents a function call (SUM, AVG, COUNT, MAX, MIN, CAST, ABS).
type FuncCall struct {
	Name       string // function name (uppercase)
	Args       []Expr // function arguments
	CastType   string // for CAST: target type name
	IsDistinct bool   // true if DISTINCT keyword present
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
	return fmt.Sprintf("%s(%s)", f.Name, strings.Join(args, ", "))
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
	Table string // table name
	Alias string // table alias (empty if none)
	On    Expr   // ON condition
}

// OrderExpr represents a single ORDER BY entry.
type OrderExpr struct {
	Expr     ColumnRef // the column to order by
	Dir      string    // "ASC" or "DESC"
	NullsDir string    // "FIRST", "LAST", or "" (default)
}

// AggInfo holds aggregation information for a column.
type AggInfo struct {
	Func string // SUM, AVG, COUNT, MAX, MIN ("" for non-aggregate)
	Col  string // column name (may include table qualifier)
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

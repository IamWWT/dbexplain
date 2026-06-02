// Package filequery provides an in-memory SQL query engine for CSV/XLSX data.
// It supports SELECT, WHERE, GROUP BY, ORDER BY, JOIN, aggregations, and expressions.
// No external dependencies — pure Go standard library.
//
// This file re-exports types from internal/sqlast via type aliases.
package filequery

import (
	"github.com/IamWWT/dbexplain/internal/sqlast"
)

// --- Expression types (aliased from sqlast) ---

// Expr is implemented by all expression nodes.
type Expr = sqlast.Expr

// ColumnRef references a column, optionally qualified by table alias.
type ColumnRef = sqlast.ColumnRef

// NumberLit is a numeric literal.
type NumberLit = sqlast.NumberLit

// StringLit is a string literal.
type StringLit = sqlast.StringLit

// BinaryExpr represents a binary operation.
type BinaryExpr = sqlast.BinaryExpr

// UnaryExpr represents a unary operation.
type UnaryExpr = sqlast.UnaryExpr

// FuncCall represents a function call.
type FuncCall = sqlast.FuncCall

// BetweenExpr represents x BETWEEN a AND b.
type BetweenExpr = sqlast.BetweenExpr

// SubqueryExpr represents a scalar subquery: (SELECT ...).
type SubqueryExpr = sqlast.SubqueryExpr

// --- Statement types (aliased from sqlast) ---

// Stmt is the interface implemented by all statement types.
type Stmt = sqlast.Stmt

// SelectExpr is a single expression in the SELECT list, optionally aliased.
type SelectExpr = sqlast.SelectExpr

// SelectStmt is the parsed representation of a SELECT query.
type SelectStmt = sqlast.SelectStmt

// UnionStmt represents a UNION [ALL] of two SELECT statements.
type UnionStmt = sqlast.UnionStmt

// JoinClause represents a single JOIN clause.
type JoinClause = sqlast.JoinClause

// OrderExpr represents a single ORDER BY entry.
type OrderExpr = sqlast.OrderExpr

// AggInfo holds aggregation information for a column.
type AggInfo = sqlast.AggInfo

// NamedData holds a named dataset for JOIN resolution.
type NamedData = sqlast.NamedData

// ResolveTableAlias determines the effective table alias for a table reference.
func ResolveTableAlias(table, alias string) string {
	return sqlast.ResolveTableAlias(table, alias)
}

// IsAggregateFunc returns true if the function name is an aggregate.
func IsAggregateFunc(name string) bool {
	return sqlast.IsAggregateFunc(name)
}

// WindowDef defines an OVER clause for window functions.
type WindowDef = sqlast.WindowDef

// WindowFrame defines a ROWS/RANGE frame specification.
type WindowFrame = sqlast.WindowFrame

// FrameBound defines one endpoint of a window frame.
type FrameBound = sqlast.FrameBound

// FrameBoundType represents a window frame boundary type.
type FrameBoundType = sqlast.FrameBoundType

// Frame boundary constants.
const (
	FrameUnboundedPreceding = sqlast.FrameUnboundedPreceding
	FrameOffsetPreceding    = sqlast.FrameOffsetPreceding
	FrameCurrentRow         = sqlast.FrameCurrentRow
	FrameOffsetFollowing    = sqlast.FrameOffsetFollowing
	FrameUnboundedFollowing = sqlast.FrameUnboundedFollowing
)

// IsWindowFunc returns true if the FuncCall is a window function (has OVER clause).
func IsWindowFunc(fc *FuncCall) bool {
	return sqlast.IsWindowFunc(fc)
}

// IsRankingFunc returns true for ranking window functions.
func IsRankingFunc(name string) bool {
	return sqlast.IsRankingFunc(name)
}

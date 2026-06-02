package filequery

import (
	"github.com/IamWWT/dbexplain/internal/sqlast"
)

// Parser implements a recursive descent SQL parser.
type Parser = sqlast.Parser

// NewParser creates a new Parser.
func NewParser(tokens []Token) *Parser {
	return sqlast.NewParser(tokens)
}

// Parse parses a full SQL statement and returns a Stmt.
// The returned statement can be *SelectStmt or *UnionStmt.
func Parse(sql string) (Stmt, error) {
	return sqlast.Parse(sql)
}

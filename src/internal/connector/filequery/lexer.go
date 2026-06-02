package filequery

import (
	"github.com/IamWWT/dbexplain/internal/sqlast"
)

// TokenType represents a SQL token type.
type TokenType = sqlast.TokenType

// Token represents a single lexical token.
type Token = sqlast.Token

// Token constants re-exported from sqlast.
const (
	TOKEN_EOF       = sqlast.TOKEN_EOF
	TOKEN_ERROR     = sqlast.TOKEN_ERROR
	TOKEN_SELECT    = sqlast.TOKEN_SELECT
	TOKEN_FROM      = sqlast.TOKEN_FROM
	TOKEN_WHERE     = sqlast.TOKEN_WHERE
	TOKEN_JOIN      = sqlast.TOKEN_JOIN
	TOKEN_ON        = sqlast.TOKEN_ON
	TOKEN_GROUP     = sqlast.TOKEN_GROUP
	TOKEN_BY        = sqlast.TOKEN_BY
	TOKEN_ORDER     = sqlast.TOKEN_ORDER
	TOKEN_ASC       = sqlast.TOKEN_ASC
	TOKEN_DESC      = sqlast.TOKEN_DESC
	TOKEN_LIMIT     = sqlast.TOKEN_LIMIT
	TOKEN_OFFSET    = sqlast.TOKEN_OFFSET
	TOKEN_AS        = sqlast.TOKEN_AS
	TOKEN_AND       = sqlast.TOKEN_AND
	TOKEN_OR        = sqlast.TOKEN_OR
	TOKEN_NOT       = sqlast.TOKEN_NOT
	TOKEN_IN        = sqlast.TOKEN_IN
	TOKEN_LIKE      = sqlast.TOKEN_LIKE
	TOKEN_BETWEEN   = sqlast.TOKEN_BETWEEN
	TOKEN_DISTINCT  = sqlast.TOKEN_DISTINCT
	TOKEN_CAST      = sqlast.TOKEN_CAST
	TOKEN_TRUE      = sqlast.TOKEN_TRUE
	TOKEN_FALSE     = sqlast.TOKEN_FALSE
	TOKEN_NULL      = sqlast.TOKEN_NULL
	TOKEN_UNION     = sqlast.TOKEN_UNION
	TOKEN_ALL       = sqlast.TOKEN_ALL
	TOKEN_NULLS     = sqlast.TOKEN_NULLS
	TOKEN_FIRST     = sqlast.TOKEN_FIRST
	TOKEN_LAST      = sqlast.TOKEN_LAST
	TOKEN_IS        = sqlast.TOKEN_IS
	TOKEN_HAVING    = sqlast.TOKEN_HAVING
	TOKEN_LEFT      = sqlast.TOKEN_LEFT
	TOKEN_RIGHT     = sqlast.TOKEN_RIGHT
	TOKEN_OUTER     = sqlast.TOKEN_OUTER
	TOKEN_OVER      = sqlast.TOKEN_OVER
	TOKEN_PARTITION = sqlast.TOKEN_PARTITION
	TOKEN_ROWS      = sqlast.TOKEN_ROWS
	TOKEN_RANGE     = sqlast.TOKEN_RANGE
	TOKEN_UNBOUNDED = sqlast.TOKEN_UNBOUNDED
	TOKEN_PRECEDING = sqlast.TOKEN_PRECEDING
	TOKEN_FOLLOWING = sqlast.TOKEN_FOLLOWING
	TOKEN_CURRENT   = sqlast.TOKEN_CURRENT
	TOKEN_ROW       = sqlast.TOKEN_ROW
	TOKEN_IDENT     = sqlast.TOKEN_IDENT
	TOKEN_NUMBER    = sqlast.TOKEN_NUMBER
	TOKEN_STRING    = sqlast.TOKEN_STRING
	TOKEN_EQ        = sqlast.TOKEN_EQ
	TOKEN_NE        = sqlast.TOKEN_NE
	TOKEN_LT        = sqlast.TOKEN_LT
	TOKEN_GT        = sqlast.TOKEN_GT
	TOKEN_LE        = sqlast.TOKEN_LE
	TOKEN_GE        = sqlast.TOKEN_GE
	TOKEN_PLUS      = sqlast.TOKEN_PLUS
	TOKEN_MINUS     = sqlast.TOKEN_MINUS
	TOKEN_STAR      = sqlast.TOKEN_STAR
	TOKEN_SLASH     = sqlast.TOKEN_SLASH
	TOKEN_LPAREN    = sqlast.TOKEN_LPAREN
	TOKEN_RPAREN    = sqlast.TOKEN_RPAREN
	TOKEN_COMMA     = sqlast.TOKEN_COMMA
	TOKEN_DOT       = sqlast.TOKEN_DOT
	TOKEN_SEMICOLON = sqlast.TOKEN_SEMICOLON
)

// Lexer tokenizes SQL input.
type Lexer = sqlast.Lexer

// NewLexer creates a new Lexer.
func NewLexer(sql string) *Lexer {
	return sqlast.NewLexer(sql)
}

// Tokenize is a convenience function to tokenize an entire SQL string.
func Tokenize(sql string) ([]Token, error) {
	return sqlast.Tokenize(sql)
}

// tokenName returns the human-readable name of a token type.
// Re-exported for tests and parse error messages.
func tokenName(tt TokenType) string {
	return sqlast.TokenName(tt)
}

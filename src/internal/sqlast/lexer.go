package sqlast

import (
	"fmt"
	"strings"
	"unicode"
)

// TokenType represents a SQL token type.
type TokenType int

const (
	TOKEN_EOF TokenType = iota
	TOKEN_ERROR

	// Keywords
	TOKEN_SELECT
	TOKEN_FROM
	TOKEN_WHERE
	TOKEN_JOIN
	TOKEN_ON
	TOKEN_GROUP
	TOKEN_BY
	TOKEN_ORDER
	TOKEN_ASC
	TOKEN_DESC
	TOKEN_LIMIT
	TOKEN_OFFSET
	TOKEN_AS
	TOKEN_AND
	TOKEN_OR
	TOKEN_NOT
	TOKEN_IN
	TOKEN_LIKE
	TOKEN_BETWEEN
	TOKEN_DISTINCT
	TOKEN_CAST
	TOKEN_TRUE
	TOKEN_FALSE
	TOKEN_NULL

	// Phase I enhancements
	TOKEN_UNION
	TOKEN_ALL
	TOKEN_NULLS
	TOKEN_FIRST
	TOKEN_LAST

	// Phase II — new keywords
	TOKEN_IS
	TOKEN_HAVING
	TOKEN_LEFT
	TOKEN_RIGHT
	TOKEN_OUTER

	// Window function keywords
	TOKEN_OVER
	TOKEN_PARTITION

	// Window frame keywords (Phase 4)
	TOKEN_ROWS
	TOKEN_RANGE
	TOKEN_UNBOUNDED
	TOKEN_PRECEDING
	TOKEN_FOLLOWING
	TOKEN_CURRENT
	TOKEN_ROW

	// Identifiers & literals
	TOKEN_IDENT  // column/table name
	TOKEN_NUMBER // integer or float
	TOKEN_STRING // single-quoted string

	// Operators
	TOKEN_EQ       // =
	TOKEN_NE       // != or <>
	TOKEN_LT       // <
	TOKEN_GT       // >
	TOKEN_LE       // <=
	TOKEN_GE       // >=
	TOKEN_PLUS     // +
	TOKEN_MINUS    // -
	TOKEN_STAR     // *
	TOKEN_SLASH    // /
	TOKEN_LPAREN   // (
	TOKEN_RPAREN   // )
	TOKEN_COMMA    // ,
	TOKEN_DOT      // .
	TOKEN_SEMICOLON // ;
)

// Token represents a single lexical token.
type Token struct {
	Type  TokenType
	Value string
	Pos   int // byte position in input
}

func (t Token) String() string {
	if t.Value != "" {
		return t.Value
	}
	return tokenName(t.Type)
}

func tokenName(tt TokenType) string {
	return TokenName(tt)
}

// keywords maps uppercase SQL keywords to token types.
var keywords = map[string]TokenType{
	"SELECT":   TOKEN_SELECT,
	"FROM":     TOKEN_FROM,
	"WHERE":    TOKEN_WHERE,
	"JOIN":     TOKEN_JOIN,
	"ON":       TOKEN_ON,
	"GROUP":    TOKEN_GROUP,
	"BY":       TOKEN_BY,
	"ORDER":    TOKEN_ORDER,
	"ASC":      TOKEN_ASC,
	"DESC":     TOKEN_DESC,
	"LIMIT":    TOKEN_LIMIT,
	"OFFSET":   TOKEN_OFFSET,
	"AS":       TOKEN_AS,
	"AND":      TOKEN_AND,
	"OR":       TOKEN_OR,
	"NOT":      TOKEN_NOT,
	"IN":       TOKEN_IN,
	"LIKE":     TOKEN_LIKE,
	"BETWEEN":  TOKEN_BETWEEN,
	"DISTINCT": TOKEN_DISTINCT,
	"CAST":     TOKEN_CAST,
	"TRUE":     TOKEN_TRUE,
	"FALSE":    TOKEN_FALSE,
	"NULL":     TOKEN_NULL,
	"UNION":    TOKEN_UNION,
	"ALL":      TOKEN_ALL,
	"NULLS":    TOKEN_NULLS,
	"FIRST":    TOKEN_FIRST,
	"LAST":     TOKEN_LAST,
	"IS":       TOKEN_IS,
	"HAVING":   TOKEN_HAVING,
	"LEFT":      TOKEN_LEFT,
	"RIGHT":     TOKEN_RIGHT,
	"OUTER":     TOKEN_OUTER,
	"OVER":       TOKEN_OVER,
	"PARTITION":  TOKEN_PARTITION,
	"ROWS":       TOKEN_ROWS,
	"RANGE":      TOKEN_RANGE,
	"UNBOUNDED":  TOKEN_UNBOUNDED,
	"PRECEDING":  TOKEN_PRECEDING,
	"FOLLOWING":  TOKEN_FOLLOWING,
	"CURRENT":    TOKEN_CURRENT,
	"ROW":        TOKEN_ROW,
}

// Lexer tokenizes SQL input.
type Lexer struct {
	input []rune
	pos   int
}

// NewLexer creates a new Lexer.
func NewLexer(sql string) *Lexer {
	return &Lexer{input: []rune(sql), pos: 0}
}

// Next returns the next token from the input.
func (l *Lexer) Next() Token {
	l.skipWhitespace()
	if l.pos >= len(l.input) {
		return Token{Type: TOKEN_EOF, Pos: l.pos}
	}

	pos := l.pos
	ch := l.input[pos]

	// Single-character operators and punctuation
	switch ch {
	case '=':
		l.pos++
		return Token{Type: TOKEN_EQ, Value: "=", Pos: pos}
	case '<':
		if l.peek() == '=' {
			l.pos += 2
			return Token{Type: TOKEN_LE, Value: "<=", Pos: pos}
		}
		if l.peek() == '>' {
			l.pos += 2
			return Token{Type: TOKEN_NE, Value: "<>", Pos: pos}
		}
		l.pos++
		return Token{Type: TOKEN_LT, Value: "<", Pos: pos}
	case '>':
		if l.peek() == '=' {
			l.pos += 2
			return Token{Type: TOKEN_GE, Value: ">=", Pos: pos}
		}
		l.pos++
		return Token{Type: TOKEN_GT, Value: ">", Pos: pos}
	case '!':
		if l.peek() == '=' {
			l.pos += 2
			return Token{Type: TOKEN_NE, Value: "!=", Pos: pos}
		}
		return Token{Type: TOKEN_ERROR, Value: "unexpected '!'", Pos: pos}
	case '+':
		l.pos++
		return Token{Type: TOKEN_PLUS, Value: "+", Pos: pos}
	case '-':
		l.pos++
		return Token{Type: TOKEN_MINUS, Value: "-", Pos: pos}
	case '*':
		l.pos++
		return Token{Type: TOKEN_STAR, Value: "*", Pos: pos}
	case '/':
		l.pos++
		return Token{Type: TOKEN_SLASH, Value: "/", Pos: pos}
	case '(':
		l.pos++
		return Token{Type: TOKEN_LPAREN, Value: "(", Pos: pos}
	case ')':
		l.pos++
		return Token{Type: TOKEN_RPAREN, Value: ")", Pos: pos}
	case ',':
		l.pos++
		return Token{Type: TOKEN_COMMA, Value: ",", Pos: pos}
	case '.':
		l.pos++
		return Token{Type: TOKEN_DOT, Value: ".", Pos: pos}
	case ';':
		l.pos++
		return Token{Type: TOKEN_SEMICOLON, Value: ";", Pos: pos}
	case '\'':
		return l.readString()
	case '"':
		return l.readDoubleQuotedString()
	}

	// Number
	if unicode.IsDigit(ch) {
		return l.readNumber()
	}

	// Identifier or keyword (starts with letter or underscore)
	if unicode.IsLetter(ch) || ch == '_' {
		return l.readIdent()
	}

	l.pos++
	// Specific hint for double quotes (legacy or old-version fallback)
	if ch == '"' {
		return Token{Type: TOKEN_ERROR, Value: "use single quotes (') for string literals", Pos: pos}
	}
	return Token{Type: TOKEN_ERROR, Value: fmt.Sprintf("unexpected character %q", ch), Pos: pos}
}

// peek returns the next character without advancing, or 0 if at end.
func (l *Lexer) peek() rune {
	if l.pos+1 < len(l.input) {
		return l.input[l.pos+1]
	}
	return 0
}

// skipWhitespace advances past spaces, tabs, newlines.
func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			l.pos++
		} else {
			break
		}
	}
}

// readIdent reads an identifier or keyword.
func (l *Lexer) readIdent() Token {
	pos := l.pos
	start := l.pos
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
			l.pos++
		} else {
			break
		}
	}
	word := string(l.input[start:l.pos])
	upper := strings.ToUpper(word)

	// Check if it's a keyword
	if tt, ok := keywords[upper]; ok {
		return Token{Type: tt, Value: upper, Pos: pos}
	}

	// Check for aggregate functions (also identifiers, but we mark them)
	return Token{Type: TOKEN_IDENT, Value: word, Pos: pos}
}

// readNumber reads a numeric literal.
func (l *Lexer) readNumber() Token {
	pos := l.pos
	start := l.pos
	isFloat := false

	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if unicode.IsDigit(ch) {
			l.pos++
		} else if ch == '.' && !isFloat {
			// Check next char is digit (to distinguish from dot operator)
			next := l.peekAfterOne()
			if next != -1 && unicode.IsDigit(rune(next)) {
				isFloat = true
				l.pos++
			} else {
				break
			}
		} else {
			break
		}
	}

	return Token{Type: TOKEN_NUMBER, Value: string(l.input[start:l.pos]), Pos: pos}
}

// readString reads a single-quoted string literal.
func (l *Lexer) readString() Token {
	pos := l.pos
	l.pos++ // skip opening quote
	var buf strings.Builder
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '\'' {
			// Check for escaped quote ''
			if l.peek() == '\'' {
				buf.WriteByte('\'')
				l.pos += 2
				continue
			}
			l.pos++ // skip closing quote
			return Token{Type: TOKEN_STRING, Value: buf.String(), Pos: pos}
		}
		buf.WriteRune(ch)
		l.pos++
	}
	return Token{Type: TOKEN_ERROR, Value: "unterminated string", Pos: pos}
}

// readDoubleQuotedString reads a double-quoted string literal.
func (l *Lexer) readDoubleQuotedString() Token {
	pos := l.pos
	l.pos++ // skip opening quote
	var buf strings.Builder
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '"' {
			// Check for escaped double quote ""
			if l.peek() == '"' {
				buf.WriteByte('"')
				l.pos += 2
				continue
			}
			l.pos++ // skip closing quote
			return Token{Type: TOKEN_STRING, Value: buf.String(), Pos: pos}
		}
		buf.WriteRune(ch)
		l.pos++
	}
	return Token{Type: TOKEN_ERROR, Value: "unterminated string", Pos: pos}
}

// peekAfterOne returns the character after the current position + 1, or -1.
func (l *Lexer) peekAfterOne() int {
	if l.pos+1 < len(l.input) {
		return int(l.input[l.pos+1])
	}
	return -1
}

// TokenName returns the human-readable name of a token type.
func TokenName(tt TokenType) string {
	switch tt {
	case TOKEN_EOF:
		return "EOF"
	case TOKEN_ERROR:
		return "ERROR"
	case TOKEN_SELECT:
		return "SELECT"
	case TOKEN_FROM:
		return "FROM"
	case TOKEN_WHERE:
		return "WHERE"
	case TOKEN_JOIN:
		return "JOIN"
	case TOKEN_ON:
		return "ON"
	case TOKEN_GROUP:
		return "GROUP"
	case TOKEN_BY:
		return "BY"
	case TOKEN_ORDER:
		return "ORDER"
	case TOKEN_ASC:
		return "ASC"
	case TOKEN_DESC:
		return "DESC"
	case TOKEN_LIMIT:
		return "LIMIT"
	case TOKEN_OFFSET:
		return "OFFSET"
	case TOKEN_AS:
		return "AS"
	case TOKEN_AND:
		return "AND"
	case TOKEN_OR:
		return "OR"
	case TOKEN_NOT:
		return "NOT"
	case TOKEN_IN:
		return "IN"
	case TOKEN_LIKE:
		return "LIKE"
	case TOKEN_BETWEEN:
		return "BETWEEN"
	case TOKEN_DISTINCT:
		return "DISTINCT"
	case TOKEN_CAST:
		return "CAST"
	case TOKEN_TRUE:
		return "TRUE"
	case TOKEN_FALSE:
		return "FALSE"
	case TOKEN_NULL:
		return "NULL"
	case TOKEN_UNION:
		return "UNION"
	case TOKEN_ALL:
		return "ALL"
	case TOKEN_NULLS:
		return "NULLS"
	case TOKEN_FIRST:
		return "FIRST"
	case TOKEN_LAST:
		return "LAST"
	case TOKEN_IS:
		return "IS"
	case TOKEN_HAVING:
		return "HAVING"
	case TOKEN_LEFT:
		return "LEFT"
	case TOKEN_RIGHT:
		return "RIGHT"
	case TOKEN_OUTER:
		return "OUTER"
	case TOKEN_OVER:
		return "OVER"
	case TOKEN_PARTITION:
		return "PARTITION"
	case TOKEN_ROWS:
		return "ROWS"
	case TOKEN_RANGE:
		return "RANGE"
	case TOKEN_UNBOUNDED:
		return "UNBOUNDED"
	case TOKEN_PRECEDING:
		return "PRECEDING"
	case TOKEN_FOLLOWING:
		return "FOLLOWING"
	case TOKEN_CURRENT:
		return "CURRENT"
	case TOKEN_ROW:
		return "ROW"
	case TOKEN_IDENT:
		return "IDENT"
	case TOKEN_NUMBER:
		return "NUMBER"
	case TOKEN_STRING:
		return "STRING"
	case TOKEN_EQ:
		return "="
	case TOKEN_NE:
		return "!="
	case TOKEN_LT:
		return "<"
	case TOKEN_GT:
		return ">"
	case TOKEN_LE:
		return "<="
	case TOKEN_GE:
		return ">="
	case TOKEN_PLUS:
		return "+"
	case TOKEN_MINUS:
		return "-"
	case TOKEN_STAR:
		return "*"
	case TOKEN_SLASH:
		return "/"
	case TOKEN_LPAREN:
		return "("
	case TOKEN_RPAREN:
		return ")"
	case TOKEN_COMMA:
		return ","
	case TOKEN_DOT:
		return "."
	case TOKEN_SEMICOLON:
		return ";"
	}
	return fmt.Sprintf("UNKNOWN(%d)", tt)
}

// Tokenize is a convenience function to tokenize an entire SQL string.
func Tokenize(sql string) ([]Token, error) {
	l := NewLexer(sql)
	var tokens []Token
	for {
		tok := l.Next()
		tokens = append(tokens, tok)
		if tok.Type == TOKEN_EOF {
			break
		}
		if tok.Type == TOKEN_ERROR {
			return tokens, fmt.Errorf("lex error at position %d: %s", tok.Pos, tok.Value)
		}
	}
	return tokens, nil
}

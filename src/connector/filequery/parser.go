package filequery

import (
	"fmt"
	"strconv"
	"strings"
)

// Parser implements a recursive descent SQL parser.
type Parser struct {
	tokens []Token
	pos    int
}

// NewParser creates a new Parser.
func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens, pos: 0}
}

// Parse parses a full SQL statement and returns a SelectStmt.
func Parse(sql string) (*SelectStmt, error) {
	tokens, err := Tokenize(sql)
	if err != nil {
		return nil, err
	}
	p := NewParser(tokens)
	stmt, err := p.parseSelect()
	if err != nil {
		return nil, err
	}
	return stmt, nil
}

// peek returns the current token without consuming it.
func (p *Parser) peek() Token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return Token{Type: TOKEN_EOF}
}

// next consumes and returns the current token.
func (p *Parser) next() Token {
	tok := p.peek()
	p.pos++
	return tok
}

// expect checks that the current token is of the expected type and consumes it.
func (p *Parser) expect(tt TokenType) (Token, error) {
	tok := p.peek()
	if tok.Type != tt {
		return tok, fmt.Errorf("expected %s, got %s at position %d", tokenName(tt), tok, tok.Pos)
	}
	return p.next(), nil
}

// skipSemicolons skips any semicolons (for simple SQL without multiple statements).
func (p *Parser) skipSemicolons() {
	for p.peek().Type == TOKEN_SEMICOLON {
		p.next()
	}
}

// parseSelect parses: SELECT select_list [FROM table_ref] [WHERE expr] [GROUP BY col,...] [ORDER BY col,...] [LIMIT N] [OFFSET M]
func (p *Parser) parseSelect() (*SelectStmt, error) {
	stmt := &SelectStmt{Limit: 0, Offset: 0}

	// SELECT
	if _, err := p.expect(TOKEN_SELECT); err != nil {
		return nil, err
	}

	// select_list
	columns, err := p.parseSelectList()
	if err != nil {
		return nil, err
	}
	stmt.Columns = columns

	// Optional DISTINCT (parse but we don't enforce it in-memory)
	if p.peek().Type == TOKEN_DISTINCT {
		p.next()
	}

	p.skipSemicolons()

	// FROM
	if p.peek().Type == TOKEN_FROM {
		p.next()
		fromName, fromAlias, err := p.parseTableRef()
		if err != nil {
			return nil, err
		}
		stmt.From = fromName
		stmt.FromAlias = fromAlias
	}

	// JOIN clauses
	for p.peek().Type == TOKEN_JOIN {
		p.next()
		joinTable, joinAlias, err := p.parseTableRef()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TOKEN_ON); err != nil {
			return nil, err
		}
		onExpr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		stmt.Joins = append(stmt.Joins, JoinClause{
			Table: joinTable,
			Alias: joinAlias,
			On:    onExpr,
		})
	}

	// WHERE
	if p.peek().Type == TOKEN_WHERE {
		p.next()
		where, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	// GROUP BY
	if p.peek().Type == TOKEN_GROUP {
		p.next() // GROUP
		if _, err := p.expect(TOKEN_BY); err != nil {
			return nil, err
		}
		for {
			col, err := p.parseColumnRef()
			if err != nil {
				return nil, err
			}
			stmt.GroupBy = append(stmt.GroupBy, col)
			if p.peek().Type == TOKEN_COMMA {
				p.next()
			} else {
				break
			}
		}
	}

	// ORDER BY
	if p.peek().Type == TOKEN_ORDER {
		p.next() // ORDER
		if _, err := p.expect(TOKEN_BY); err != nil {
			return nil, err
		}
		for {
			col, err := p.parseColumnRef()
			if err != nil {
				return nil, err
			}
			dir := "ASC"
			if p.peek().Type == TOKEN_ASC {
				p.next()
			} else if p.peek().Type == TOKEN_DESC {
				p.next()
				dir = "DESC"
			}
			stmt.OrderBy = append(stmt.OrderBy, OrderExpr{Expr: col, Dir: dir})
			if p.peek().Type == TOKEN_COMMA {
				p.next()
			} else {
				break
			}
		}
	}

	// LIMIT
	if p.peek().Type == TOKEN_LIMIT {
		p.next()
		tok := p.next()
		if tok.Type != TOKEN_NUMBER {
			return nil, fmt.Errorf("expected number after LIMIT, got %s", tok)
		}
		n, err := strconv.Atoi(tok.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid LIMIT value %q: %w", tok.Value, err)
		}
		stmt.Limit = n
	}

	// OFFSET
	if p.peek().Type == TOKEN_OFFSET {
		p.next()
		tok := p.next()
		if tok.Type != TOKEN_NUMBER {
			return nil, fmt.Errorf("expected number after OFFSET, got %s", tok)
		}
		n, err := strconv.Atoi(tok.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid OFFSET value %q: %w", tok.Value, err)
		}
		stmt.Offset = n
	}

	p.skipSemicolons()

	if p.peek().Type != TOKEN_EOF {
		return nil, fmt.Errorf("unexpected token %s after complete statement", p.peek())
	}

	return stmt, nil
}

// parseSelectList parses the column list after SELECT.
func (p *Parser) parseSelectList() ([]SelectExpr, error) {
	var columns []SelectExpr

	// SELECT *
	if p.peek().Type == TOKEN_STAR {
		p.next()
		columns = append(columns, SelectExpr{
			Expr: &ColumnRef{Col: "*"},
		})
		return columns, nil
	}

	for {
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}

		sel := SelectExpr{Expr: expr}

		// Optional AS alias
		if p.peek().Type == TOKEN_AS {
			p.next() // AS
			aliasTok := p.next()
			if aliasTok.Type != TOKEN_IDENT {
				return nil, fmt.Errorf("expected identifier after AS, got %s", aliasTok)
			}
			sel.Alias = aliasTok.Value
		} else if p.peek().Type == TOKEN_IDENT {
			// Implicit alias (no AS keyword)
			// But be careful: IDENT could be part of next expression
			// Only treat as alias if followed by comma or end of select list
			nextTok := p.peek()
			// We already peeked, check if it's a keyword that ends a clause
			switch nextTok.Type {
			case TOKEN_FROM, TOKEN_WHERE, TOKEN_GROUP, TOKEN_ORDER, TOKEN_LIMIT, TOKEN_JOIN:
				// not an alias
			default:
				// Could be alias, peek ahead carefully
				// For simplicity, only handle implicit alias when followed by comma or FROM
				saved := p.pos
				p.next() // consume the ident
				after := p.peek()
				if after.Type == TOKEN_COMMA || after.Type == TOKEN_FROM || after.Type == TOKEN_WHERE ||
					after.Type == TOKEN_GROUP || after.Type == TOKEN_ORDER || after.Type == TOKEN_LIMIT ||
					after.Type == TOKEN_JOIN || after.Type == TOKEN_EOF || after.Type == TOKEN_SEMICOLON {
					// The ident was an alias
					sel.Alias = nextTok.Value
					// pos already advanced
				} else {
					// Not an alias, restore position
					p.pos = saved
				}
			}
		}

		columns = append(columns, sel)

		if p.peek().Type == TOKEN_COMMA {
			p.next()
		} else {
			break
		}
	}

	return columns, nil
}

// parseTableRef parses: table_name [alias]
func (p *Parser) parseTableRef() (string, string, error) {
	tok := p.next()
	if tok.Type != TOKEN_IDENT {
		return "", "", fmt.Errorf("expected table name, got %s", tok)
	}
	name := tok.Value
	alias := ""

	// Check for alias (next token is an identifier not followed by ON, WHERE, etc.)
	if p.peek().Type == TOKEN_IDENT {
		aliasTok := p.peek()
		// Check if it's actually a keyword masquerading
		switch aliasTok.Value {
		case "ON", "WHERE", "ORDER", "GROUP", "LIMIT", "JOIN", "AS":
			// not an alias
		default:
			p.next()
			alias = aliasTok.Value
		}
	}

	return name, alias, nil
}

// parseColumnRef parses: [table.]column
func (p *Parser) parseColumnRef() (ColumnRef, error) {
	tok := p.next()
	if tok.Type != TOKEN_IDENT && tok.Type != TOKEN_STAR {
		return ColumnRef{}, fmt.Errorf("expected column name, got %s", tok)
	}

	// Check for qualified reference: table.col
	if tok.Type == TOKEN_IDENT && p.peek().Type == TOKEN_DOT {
		tableName := tok.Value
		p.next() // consume dot
		colTok := p.next()
		if colTok.Type != TOKEN_IDENT && colTok.Type != TOKEN_STAR {
			return ColumnRef{}, fmt.Errorf("expected column name after '.', got %s", colTok)
		}
		return ColumnRef{Table: tableName, Col: colTok.Value}, nil
	}

	return ColumnRef{Col: tok.Value}, nil
}

// --- Expression parsing ---

// parseExpr parses an expression (AND/OR have lowest precedence).
func (p *Parser) parseExpr() (Expr, error) {
	return p.parseLogical()
}

// parseLogical handles: expr AND expr, expr OR expr
func (p *Parser) parseLogical() (Expr, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}

	for {
		if p.peek().Type == TOKEN_AND {
			p.next()
			right, err := p.parseComparison()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Left: left, Op: "AND", Right: right}
		} else if p.peek().Type == TOKEN_OR {
			p.next()
			right, err := p.parseComparison()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Left: left, Op: "OR", Right: right}
		} else {
			break
		}
	}
	return left, nil
}

// parseComparison handles: expr = expr, expr != expr, expr < expr, etc.
// Also: expr LIKE expr, expr IN (list), expr NOT IN (list), expr BETWEEN expr AND expr
func (p *Parser) parseComparison() (Expr, error) {
	// Check for NOT prefix (NOT IN, NOT LIKE)
	negate := false
	if p.peek().Type == TOKEN_NOT {
		p.next()
		negate = true
	}

	left, err := p.parseAddSub()
	if err != nil {
		return nil, err
	}

	switch p.peek().Type {
	case TOKEN_EQ:
		p.next()
		right, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: "=", Right: right}
	case TOKEN_NE:
		p.next()
		right, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: "!=", Right: right}
	case TOKEN_LT:
		p.next()
		right, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: "<", Right: right}
	case TOKEN_GT:
		p.next()
		right, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: ">", Right: right}
	case TOKEN_LE:
		p.next()
		right, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: "<=", Right: right}
	case TOKEN_GE:
		p.next()
		right, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: ">=", Right: right}
	case TOKEN_LIKE:
		p.next()
		right, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: "LIKE", Right: right}
	case TOKEN_IN:
		p.next()
		if _, err := p.expect(TOKEN_LPAREN); err != nil {
			return nil, err
		}
		var list []Expr
		for {
			elem, err := p.parseAddSub()
			if err != nil {
				return nil, err
			}
			list = append(list, elem)
			if p.peek().Type == TOKEN_COMMA {
				p.next()
			} else {
				break
			}
		}
		if _, err := p.expect(TOKEN_RPAREN); err != nil {
			return nil, err
		}
		op := "IN"
		if negate {
			op = "NOT IN"
		}
		left = &BinaryExpr{Left: left, Op: op, Right: p.listToExpr(list)}
	case TOKEN_BETWEEN:
		p.next()
		low, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TOKEN_AND); err != nil {
			return nil, err
		}
		high, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		if negate {
			// NOT BETWEEN → expr < low OR expr > high
			left = &BinaryExpr{
				Left:  &BinaryExpr{Left: left, Op: "<", Right: low},
				Op:    "OR",
				Right: &BinaryExpr{Left: left, Op: ">", Right: high},
			}
		} else {
			left = &BetweenExpr{Expr: left, Low: low, High: high}
		}
	default:
		if negate {
			// NOT followed by something else — likely NOT (expr) or NOT expr
			left = &UnaryExpr{Op: "NOT", Right: left}
		}
	}

	return left, nil
}

// parseAddSub handles: expr + expr, expr - expr
func (p *Parser) parseAddSub() (Expr, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return nil, err
	}

	for {
		if p.peek().Type == TOKEN_PLUS {
			p.next()
			right, err := p.parseMulDiv()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Left: left, Op: "+", Right: right}
		} else if p.peek().Type == TOKEN_MINUS {
			p.next()
			right, err := p.parseMulDiv()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Left: left, Op: "-", Right: right}
		} else {
			break
		}
	}
	return left, nil
}

// parseMulDiv handles: expr * expr, expr / expr
func (p *Parser) parseMulDiv() (Expr, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	for {
		if p.peek().Type == TOKEN_STAR {
			p.next()
			right, err := p.parsePrimary()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Left: left, Op: "*", Right: right}
		} else if p.peek().Type == TOKEN_SLASH {
			p.next()
			right, err := p.parsePrimary()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Left: left, Op: "/", Right: right}
		} else {
			break
		}
	}
	return left, nil
}

// parsePrimary handles: literal, column ref, function call, (expr), CAST
func (p *Parser) parsePrimary() (Expr, error) {
	tok := p.peek()

	switch tok.Type {
	case TOKEN_NUMBER:
		p.next()
		return &NumberLit{Value: tok.Value}, nil

	case TOKEN_STRING:
		p.next()
		return &StringLit{Value: tok.Value}, nil

	case TOKEN_NULL:
		p.next()
		return &NumberLit{Value: ""}, nil // NULL as empty

	case TOKEN_TRUE:
		p.next()
		return &NumberLit{Value: "1"}, nil

	case TOKEN_FALSE:
		p.next()
		return &NumberLit{Value: "0"}, nil

	case TOKEN_LPAREN:
		p.next()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TOKEN_RPAREN); err != nil {
			return nil, err
		}
		return expr, nil

	case TOKEN_CAST:
		p.next()
		return p.parseCast()

	case TOKEN_IDENT:
		// Could be: function call, CAST(..), or column reference
		name := tok.Value
		p.next()

		// Check if it's a function call
		if p.peek().Type == TOKEN_LPAREN {
			return p.parseFuncCall(name)
		}

		// Check for qualified reference: table.col
		if p.peek().Type == TOKEN_DOT {
			p.next() // consume dot
			colTok := p.next()
			if colTok.Type != TOKEN_IDENT && colTok.Type != TOKEN_STAR {
				return nil, fmt.Errorf("expected column name after '.', got %s", colTok)
			}
			return &ColumnRef{Table: name, Col: colTok.Value}, nil
		}

		return &ColumnRef{Col: name}, nil

	case TOKEN_STAR:
		p.next()
		return &ColumnRef{Col: "*"}, nil

	case TOKEN_MINUS:
		p.next()
		expr, err := parsePrimary(p)
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "-", Right: expr}, nil

	default:
		return nil, fmt.Errorf("unexpected token %s in expression", tok)
	}
}

// parseFuncCall parses: func_name(arg1, arg2, ...)
func (p *Parser) parseFuncCall(name string) (Expr, error) {
	p.next() // consume (
	fn := &FuncCall{Name: strings.ToUpper(name)}

	// Check for empty args (e.g., COUNT(*))
	if p.peek().Type == TOKEN_RPAREN {
		p.next()
		return fn, nil
	}

	// Parse arguments
	for {
		// Check for DISTINCT inside function
		if p.peek().Type == TOKEN_DISTINCT {
			p.next()
			fn.IsDistinct = true
		}

		arg, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		fn.Args = append(fn.Args, arg)

		if p.peek().Type == TOKEN_COMMA {
			p.next()
		} else {
			break
		}
	}

	if _, err := p.expect(TOKEN_RPAREN); err != nil {
		return nil, err
	}

	return fn, nil
}

// parseCast parses: CAST(expr AS type)
func (p *Parser) parseCast() (Expr, error) {
	if _, err := p.expect(TOKEN_LPAREN); err != nil {
		return nil, err
	}
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TOKEN_AS); err != nil {
		return nil, err
	}
	typeTok := p.next()
	if typeTok.Type != TOKEN_IDENT {
		return nil, fmt.Errorf("expected type name in CAST, got %s", typeTok)
	}
	if _, err := p.expect(TOKEN_RPAREN); err != nil {
		return nil, err
	}
	return &FuncCall{Name: "CAST", Args: []Expr{expr}, CastType: strings.ToUpper(typeTok.Value)}, nil
}

// listToExpr wraps a list of expressions into a single expr for IN/NOT IN.
// We use a BinaryExpr with left being the target and right being a special list marker.
func (p *Parser) listToExpr(list []Expr) Expr {
	if len(list) == 0 {
		return &NumberLit{Value: ""}
	}
	// Wrap list as a chain of ORs for simplicity
	result := list[0]
	for i := 1; i < len(list); i++ {
		result = &BinaryExpr{Left: result, Op: "OR", Right: list[i]}
	}
	return result
}

// parseExpr is a thin wrapper that calls p.parseExpr for the unexported method.
func parsePrimary(p *Parser) (Expr, error) {
	return p.parsePrimary()
}

// --- Utility functions ---

// ResolveTableAlias determines the effective table alias for a table reference.
// Returns alias if given, otherwise the table name itself.
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

// DetectJoinQuick does a quick string-level check for JOIN in SQL.
// This is used by execute.go before full parsing.
func DetectJoinQuick(sql string) bool {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	// Check for JOIN keyword (simple, not foolproof but good enough)
	words := strings.Fields(upper)
	for _, w := range words {
		if w == "JOIN" {
			return true
		}
	}
	return false
}

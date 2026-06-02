package filequery

import (
	"fmt"
	"testing"
)

// --- Lexer tests ---

func TestLexerBasic(t *testing.T) {
	tests := []struct {
		sql    string
		tokens []TokenType
	}{
		{"SELECT * FROM t", []TokenType{TOKEN_SELECT, TOKEN_STAR, TOKEN_FROM, TOKEN_IDENT, TOKEN_EOF}},
		{"WHERE a = 1", []TokenType{TOKEN_WHERE, TOKEN_IDENT, TOKEN_EQ, TOKEN_NUMBER, TOKEN_EOF}},
		{"a AND b OR NOT c", []TokenType{TOKEN_IDENT, TOKEN_AND, TOKEN_IDENT, TOKEN_OR, TOKEN_NOT, TOKEN_IDENT, TOKEN_EOF}},
		{"GROUP BY a, b", []TokenType{TOKEN_GROUP, TOKEN_BY, TOKEN_IDENT, TOKEN_COMMA, TOKEN_IDENT, TOKEN_EOF}},
		{"ORDER BY a DESC", []TokenType{TOKEN_ORDER, TOKEN_BY, TOKEN_IDENT, TOKEN_DESC, TOKEN_EOF}},
		{"LIMIT 10 OFFSET 5", []TokenType{TOKEN_LIMIT, TOKEN_NUMBER, TOKEN_OFFSET, TOKEN_NUMBER, TOKEN_EOF}},
		{"a != b", []TokenType{TOKEN_IDENT, TOKEN_NE, TOKEN_IDENT, TOKEN_EOF}},
		{"a <> b", []TokenType{TOKEN_IDENT, TOKEN_NE, TOKEN_IDENT, TOKEN_EOF}},
		{"a <= b", []TokenType{TOKEN_IDENT, TOKEN_LE, TOKEN_IDENT, TOKEN_EOF}},
		{"a >= b", []TokenType{TOKEN_IDENT, TOKEN_GE, TOKEN_IDENT, TOKEN_EOF}},
		{"'hello world'", []TokenType{TOKEN_STRING, TOKEN_EOF}},
		{"t.col", []TokenType{TOKEN_IDENT, TOKEN_DOT, TOKEN_IDENT, TOKEN_EOF}},
		{"SUM(a)", []TokenType{TOKEN_IDENT, TOKEN_LPAREN, TOKEN_IDENT, TOKEN_RPAREN, TOKEN_EOF}},
		{"a LIKE '%test%'", []TokenType{TOKEN_IDENT, TOKEN_LIKE, TOKEN_STRING, TOKEN_EOF}},
		{"a BETWEEN 1 AND 10", []TokenType{TOKEN_IDENT, TOKEN_BETWEEN, TOKEN_NUMBER, TOKEN_AND, TOKEN_NUMBER, TOKEN_EOF}},
		{"a IN (1, 2, 3)", []TokenType{TOKEN_IDENT, TOKEN_IN, TOKEN_LPAREN, TOKEN_NUMBER, TOKEN_COMMA, TOKEN_NUMBER, TOKEN_COMMA, TOKEN_NUMBER, TOKEN_RPAREN, TOKEN_EOF}},
		{"CAST(a AS FLOAT)", []TokenType{TOKEN_CAST, TOKEN_LPAREN, TOKEN_IDENT, TOKEN_AS, TOKEN_IDENT, TOKEN_RPAREN, TOKEN_EOF}},
	}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			tokens, err := Tokenize(tt.sql)
			if err != nil {
				t.Fatalf("Tokenize(%q) error: %v", tt.sql, err)
			}
			if len(tokens) != len(tt.tokens) {
				t.Fatalf("Tokenize(%q) got %d tokens, want %d\ngot: %v", tt.sql, len(tokens), len(tt.tokens), tokenTypes(tokens))
			}
			for i, tok := range tokens {
				if tok.Type != tt.tokens[i] {
					t.Errorf("Tokenize(%q) token[%d] = %s, want %s", tt.sql, i, tok, tokenName(tt.tokens[i]))
				}
			}
		})
	}
}

func tokenTypes(tokens []Token) []TokenType {
	types := make([]TokenType, len(tokens))
	for i, tok := range tokens {
		types[i] = tok.Type
	}
	return types
}

// parseTestStmt is a helper that parses SQL and asserts it returns a *SelectStmt.
func parseTestStmt(t *testing.T, sql string) *SelectStmt {
	t.Helper()
	parsed, err := Parse(sql)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	stmt, ok := parsed.(*SelectStmt)
	if !ok {
		t.Fatalf("expected *SelectStmt, got %T", parsed)
	}
	return stmt
}

// --- Parser tests ---

func TestParseSelectStar(t *testing.T) {
	stmt := parseTestStmt(t,"SELECT * FROM test_table")
	if len(stmt.Columns) != 1 {
		t.Fatalf("expected 1 column, got %d", len(stmt.Columns))
	}
	cr, ok := stmt.Columns[0].Expr.(*ColumnRef)
	if !ok || cr.Col != "*" {
		t.Fatalf("expected SELECT *, got %v", stmt.Columns[0].Expr)
	}
	if stmt.From != "test_table" {
		t.Fatalf("expected FROM test_table, got %q", stmt.From)
	}
}

func TestParseSelectColumns(t *testing.T) {
	stmt := parseTestStmt(t,"SELECT col1, col2 FROM my_table")
	if len(stmt.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(stmt.Columns))
	}
	if cr, ok := stmt.Columns[0].Expr.(*ColumnRef); !ok || cr.Col != "col1" {
		t.Fatalf("expected col1, got %v", stmt.Columns[0].Expr)
	}
}

func TestParseSelectWithAlias(t *testing.T) {
	stmt := parseTestStmt(t,"SELECT col1 AS c1, col2 c2 FROM t")
	if stmt.Columns[0].Alias != "c1" {
		t.Fatalf("expected alias c1, got %q", stmt.Columns[0].Alias)
	}
}

func TestParseWhere(t *testing.T) {
	stmt := parseTestStmt(t,"SELECT * FROM t WHERE col1 > 100")
	if stmt.Where == nil {
		t.Fatal("expected WHERE clause")
	}
	be, ok := stmt.Where.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", stmt.Where)
	}
	if be.Op != ">" {
		t.Fatalf("expected >, got %q", be.Op)
	}
}

func TestParseWhereAndOr(t *testing.T) {
	stmt := parseTestStmt(t,"SELECT * FROM t WHERE a > 1 AND b < 10 OR c = 5")
	if stmt.Where == nil {
		t.Fatal("expected WHERE clause")
	}
	// Top level should be OR
	be, ok := stmt.Where.(*BinaryExpr)
	if !ok || be.Op != "OR" {
		t.Fatalf("expected OR at top, got %v", stmt.Where)
	}
}

func TestParseGroupBy(t *testing.T) {
	stmt := parseTestStmt(t,"SELECT col1, COUNT(*) FROM t GROUP BY col1")
	if len(stmt.GroupBy) != 1 {
		t.Fatalf("expected 1 GROUP BY column, got %d", len(stmt.GroupBy))
	}
	if stmt.GroupBy[0].Col != "col1" {
		t.Fatalf("expected GROUP BY col1, got %q", stmt.GroupBy[0].Col)
	}
}

func TestParseOrderBy(t *testing.T) {
	stmt := parseTestStmt(t,"SELECT * FROM t ORDER BY col1 DESC, col2 ASC")
	if len(stmt.OrderBy) != 2 {
		t.Fatalf("expected 2 ORDER BY columns, got %d", len(stmt.OrderBy))
	}
	if stmt.OrderBy[0].Dir != "DESC" || stmt.OrderBy[1].Dir != "ASC" {
		t.Fatalf("unexpected ORDER BY directions: %v", stmt.OrderBy)
	}
}

func TestParseLimitOffset(t *testing.T) {
	stmt := parseTestStmt(t,"SELECT * FROM t LIMIT 10 OFFSET 5")
	if stmt.Limit != 10 {
		t.Fatalf("expected LIMIT 10, got %d", stmt.Limit)
	}
	if stmt.Offset != 5 {
		t.Fatalf("expected OFFSET 5, got %d", stmt.Offset)
	}
}

func TestParseJoin(t *testing.T) {
	stmt := parseTestStmt(t,"SELECT t.col1, o.col2 FROM t1 t JOIN t2 o ON t.key = o.key")
	if len(stmt.Joins) != 1 {
		t.Fatalf("expected 1 JOIN, got %d", len(stmt.Joins))
	}
	if stmt.Joins[0].Table != "t2" {
		t.Fatalf("expected JOIN t2, got %q", stmt.Joins[0].Table)
	}
	if stmt.Joins[0].Alias != "o" {
		t.Fatalf("expected JOIN alias o, got %q", stmt.Joins[0].Alias)
	}
	// Check ON clause
	if stmt.Joins[0].On == nil {
		t.Fatal("expected ON clause")
	}
}

func TestParseQualifiedColumns(t *testing.T) {
	stmt := parseTestStmt(t,"SELECT t.reach_rate, o.org_name FROM t1 t JOIN t2 o ON t.id = o.id")
	cr, ok := stmt.Columns[0].Expr.(*ColumnRef)
	if !ok {
		t.Fatalf("expected ColumnRef, got %T", stmt.Columns[0].Expr)
	}
	if cr.Table != "t" || cr.Col != "reach_rate" {
		t.Fatalf("expected t.reach_rate, got %s.%s", cr.Table, cr.Col)
	}
}

func TestParseAggregateFunc(t *testing.T) {
	stmt := parseTestStmt(t,"SELECT AVG(reach_rate) AS avg_rate FROM t")
	fc, ok := stmt.Columns[0].Expr.(*FuncCall)
	if !ok {
		t.Fatalf("expected FuncCall, got %T", stmt.Columns[0].Expr)
	}
	if fc.Name != "AVG" {
		t.Fatalf("expected AVG, got %q", fc.Name)
	}
}

func TestParseCast(t *testing.T) {
	stmt := parseTestStmt(t,"SELECT CAST(col1 AS FLOAT) FROM t")
	fc, ok := stmt.Columns[0].Expr.(*FuncCall)
	if !ok || fc.Name != "CAST" {
		t.Fatalf("expected CAST, got %T", stmt.Columns[0].Expr)
	}
	if fc.CastType != "FLOAT" {
		t.Fatalf("expected CAST AS FLOAT, got %q", fc.CastType)
	}
}

func TestParseArithmetic(t *testing.T) {
	stmt := parseTestStmt(t,"SELECT (a + b) * c FROM t")
	if len(stmt.Columns) != 1 {
		t.Fatalf("expected 1 column, got %d", len(stmt.Columns))
	}
}

func TestParseFullQuery(t *testing.T) {
	sql := `SELECT pnbrn_org_name, AVG(reach_rate) AS avg_reach_rate
FROM pb_touch_ops_sample_2000
WHERE reach_rate > 0
GROUP BY pnbrn_org_name
ORDER BY avg_reach_rate DESC
LIMIT 10`

	stmt := parseTestStmt(t,sql)
	if len(stmt.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(stmt.Columns))
	}
	if len(stmt.GroupBy) != 1 {
		t.Fatalf("expected 1 GROUP BY, got %d", len(stmt.GroupBy))
	}
	if len(stmt.OrderBy) != 1 {
		t.Fatalf("expected 1 ORDER BY, got %d", len(stmt.OrderBy))
	}
	if stmt.Limit != 10 {
		t.Fatalf("expected LIMIT 10, got %d", stmt.Limit)
	}
}

// --- Evaluator tests ---

func TestEvalComparison(t *testing.T) {
	header := []string{"name", "age", "salary"}
	cm := BuildColMap(header, "")
	row := Row{Value("Alice"), Value("30"), Value("5000.50")}

	tests := []struct {
		expr Expr
		want string
	}{
		{&BinaryExpr{Left: &ColumnRef{Col: "age"}, Op: "=", Right: &NumberLit{Value: "30"}}, "true"},
		{&BinaryExpr{Left: &ColumnRef{Col: "age"}, Op: "!=", Right: &NumberLit{Value: "25"}}, "true"},
		{&BinaryExpr{Left: &ColumnRef{Col: "age"}, Op: "<", Right: &NumberLit{Value: "40"}}, "true"},
		{&BinaryExpr{Left: &ColumnRef{Col: "age"}, Op: ">", Right: &NumberLit{Value: "20"}}, "true"},
		{&BinaryExpr{Left: &ColumnRef{Col: "age"}, Op: "<=", Right: &NumberLit{Value: "30"}}, "true"},
		{&BinaryExpr{Left: &ColumnRef{Col: "age"}, Op: ">=", Right: &NumberLit{Value: "30"}}, "true"},
		{&BinaryExpr{Left: &ColumnRef{Col: "age"}, Op: "=", Right: &NumberLit{Value: "31"}}, "false"},
		{&BinaryExpr{Left: &ColumnRef{Col: "name"}, Op: "=", Right: &StringLit{Value: "Alice"}}, "true"},
		{&BinaryExpr{Left: &ColumnRef{Col: "name"}, Op: "=", Right: &StringLit{Value: "Bob"}}, "false"},
	}

	for _, tt := range tests {
		t.Run(tt.expr.String(), func(t *testing.T) {
			got, err := Eval(tt.expr, row, cm)
			if err != nil {
				t.Fatalf("Eval error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Eval = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEvalArithmetic(t *testing.T) {
	header := []string{"a", "b"}
	cm := BuildColMap(header, "")
	row := Row{Value("10"), Value("3")}

	tests := []struct {
		expr Expr
		want string
	}{
		{&BinaryExpr{Left: &ColumnRef{Col: "a"}, Op: "+", Right: &ColumnRef{Col: "b"}}, "13"},
		{&BinaryExpr{Left: &ColumnRef{Col: "a"}, Op: "-", Right: &ColumnRef{Col: "b"}}, "7"},
		{&BinaryExpr{Left: &ColumnRef{Col: "a"}, Op: "*", Right: &ColumnRef{Col: "b"}}, "30"},
		{&BinaryExpr{Left: &ColumnRef{Col: "a"}, Op: "/", Right: &NumberLit{Value: "4"}}, "2.5"},
	}

	for _, tt := range tests {
		t.Run(tt.expr.String(), func(t *testing.T) {
			got, err := Eval(tt.expr, row, cm)
			if err != nil {
				t.Fatalf("Eval error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Eval = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEvalAndOr(t *testing.T) {
	header := []string{"a", "b"}
	cm := BuildColMap(header, "")
	row := Row{Value("1"), Value("0")}

	tests := []struct {
		expr Expr
		want string
	}{
		{&BinaryExpr{Left: &ColumnRef{Col: "a"}, Op: "AND", Right: &ColumnRef{Col: "b"}}, "false"},
		{&BinaryExpr{Left: &ColumnRef{Col: "a"}, Op: "OR", Right: &ColumnRef{Col: "b"}}, "true"},
		{&BinaryExpr{Left: &NumberLit{Value: "1"}, Op: "AND", Right: &NumberLit{Value: "1"}}, "true"},
	}

	for _, tt := range tests {
		t.Run(tt.expr.String(), func(t *testing.T) {
			got, err := Eval(tt.expr, row, cm)
			if err != nil {
				t.Fatalf("Eval error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Eval = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEvalLike(t *testing.T) {
	header := []string{"name"}
	cm := BuildColMap(header, "")
	row := Row{Value("Shanghai Branch")}

	tests := []struct {
		pattern string
		want    string
	}{
		{"%Shanghai%", "true"},
		{"%Branch", "true"},
		{"Shanghai%", "true"},
		{"%BeiJing%", "false"},
		{"%shanghai%", "true"}, // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			expr := &BinaryExpr{
				Left:  &ColumnRef{Col: "name"},
				Op:    "LIKE",
				Right: &StringLit{Value: tt.pattern},
			}
			got, err := Eval(expr, row, cm)
			if err != nil {
				t.Fatalf("Eval error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Eval LIKE %q = %q, want %q", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestEvalCast(t *testing.T) {
	header := []string{"val"}
	cm := BuildColMap(header, "")
	row := Row{Value("42.7")}

	expr := &FuncCall{Name: "CAST", Args: []Expr{&ColumnRef{Col: "val"}}, CastType: "FLOAT"}
	got, err := Eval(expr, row, cm)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	f, ok := got.Float()
	if !ok || f != 42.7 {
		t.Fatalf("Eval CAST = %v, want 42.7", got)
	}
}

func TestEvalAbs(t *testing.T) {
	header := []string{"val"}
	cm := BuildColMap(header, "")
	row := Row{Value("-10.5")}

	expr := &FuncCall{Name: "ABS", Args: []Expr{&ColumnRef{Col: "val"}}}
	got, err := Eval(expr, row, cm)
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}
	if string(got) != "10.5" {
		t.Fatalf("Eval ABS = %q, want 10.5", got)
	}
}

// --- Executor tests ---

func TestExecuteSelectStar(t *testing.T) {
	header := []string{"name", "value"}
	rows := [][]string{
		{"a", "1"},
		{"b", "2"},
		{"c", "3"},
	}

	result, err := Execute("SELECT * FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 3 {
		t.Fatalf("expected 3 rows, got %d", result.RowCount)
	}
}

func TestExecuteWhere(t *testing.T) {
	header := []string{"name", "value"}
	rows := [][]string{
		{"a", "1"},
		{"b", "2"},
		{"c", "3"},
		{"d", "4"},
		{"e", "5"},
	}

	result, err := Execute("SELECT * FROM t WHERE value > 3", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows (value>3), got %d", result.RowCount)
	}
}

func TestExecuteOrderBy(t *testing.T) {
	header := []string{"name", "value"}
	rows := [][]string{
		{"a", "3"},
		{"b", "1"},
		{"c", "2"},
	}

	result, err := Execute("SELECT * FROM t ORDER BY value ASC", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 3 {
		t.Fatalf("expected 3 rows, got %d", result.RowCount)
	}
	// Check order
	if *result.Rows[1][1] != "2" {
		t.Fatalf("expected second row value=2, got %q", *result.Rows[1][1])
	}
}

func TestExecuteOrderByDesc(t *testing.T) {
	header := []string{"name", "value"}
	rows := [][]string{
		{"a", "1"},
		{"b", "5"},
		{"c", "3"},
	}

	result, err := Execute("SELECT * FROM t ORDER BY value DESC", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if *result.Rows[0][1] != "5" {
		t.Fatalf("expected first row value=5, got %q", *result.Rows[0][1])
	}
}

func TestExecuteLimit(t *testing.T) {
	header := []string{"name"}
	rows := [][]string{{"a"}, {"b"}, {"c"}, {"d"}, {"e"}}

	result, err := Execute("SELECT * FROM t LIMIT 3", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 3 {
		t.Fatalf("expected 3 rows, got %d", result.RowCount)
	}
}

func TestExecuteLimitOffset(t *testing.T) {
	header := []string{"name"}
	rows := [][]string{{"a"}, {"b"}, {"c"}, {"d"}, {"e"}}

	result, err := Execute("SELECT * FROM t LIMIT 2 OFFSET 2", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows, got %d", result.RowCount)
	}
	if *result.Rows[0][0] != "c" {
		t.Fatalf("expected first row 'c', got %q", *result.Rows[0][0])
	}
}

func TestExecuteProjection(t *testing.T) {
	header := []string{"name", "value", "extra"}
	rows := [][]string{{"a", "10", "x"}}

	result, err := Execute("SELECT name, value FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if len(result.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(result.Columns))
	}
}

func TestExecuteColumnExpr(t *testing.T) {
	header := []string{"a", "b"}
	rows := [][]string{{"10", "20"}}

	result, err := Execute("SELECT a + b AS sum FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 1 {
		t.Fatalf("expected 1 row, got %d", result.RowCount)
	}
	if *result.Rows[0][0] != "30" {
		t.Fatalf("expected a+b=30, got %q", *result.Rows[0][0])
	}
}

// --- Aggregate tests ---

func TestExecuteGroupByCount(t *testing.T) {
	header := []string{"dept", "name"}
	rows := [][]string{
		{"eng", "a"},
		{"eng", "b"},
		{"sales", "c"},
		{"eng", "d"},
		{"sales", "e"},
	}

	result, err := Execute("SELECT dept, COUNT(*) AS cnt FROM t GROUP BY dept", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 groups, got %d", result.RowCount)
	}
}

func TestExecuteGroupBySum(t *testing.T) {
	header := []string{"dept", "salary"}
	rows := [][]string{
		{"eng", "100"},
		{"eng", "200"},
		{"sales", "300"},
	}

	result, err := Execute("SELECT dept, SUM(salary) AS total FROM t GROUP BY dept", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 groups, got %d", result.RowCount)
	}
	// Find eng total
	for _, row := range result.Rows {
		if *row[0] == "eng" {
			if *row[1] != "300" {
				t.Fatalf("expected eng sum=300, got %q", *row[1])
			}
		}
	}
}

func TestExecuteGroupByAvg(t *testing.T) {
	header := []string{"dept", "salary"}
	rows := [][]string{
		{"eng", "100"},
		{"eng", "200"},
		{"sales", "300"},
	}

	result, err := Execute("SELECT dept, AVG(salary) AS avg FROM t GROUP BY dept", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	for _, row := range result.Rows {
		if *row[0] == "eng" {
			if *row[1] != "150" {
				t.Fatalf("expected eng avg=150, got %q", *row[1])
			}
		}
	}
}

func TestExecuteGroupByMaxMin(t *testing.T) {
	header := []string{"dept", "salary"}
	rows := [][]string{
		{"eng", "100"},
		{"eng", "300"},
		{"sales", "200"},
	}

	result, err := Execute("SELECT dept, MAX(salary) AS max_sal, MIN(salary) AS min_sal FROM t GROUP BY dept", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	for _, row := range result.Rows {
		if *row[0] == "eng" {
			if *row[1] != "300" {
				t.Fatalf("expected eng max=300, got %q", *row[1])
			}
			if *row[2] != "100" {
				t.Fatalf("expected eng min=100, got %q", *row[2])
			}
		}
	}
}

func TestExecuteAggregateNoGroup(t *testing.T) {
	header := []string{"salary"}
	rows := [][]string{{"100"}, {"200"}, {"300"}}

	result, err := Execute("SELECT SUM(salary) AS total FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 1 {
		t.Fatalf("expected 1 row, got %d", result.RowCount)
	}
	if *result.Rows[0][0] != "600" {
		t.Fatalf("expected total=600, got %q", *result.Rows[0][0])
	}
}

func TestExecuteCountStar(t *testing.T) {
	header := []string{"name"}
	rows := [][]string{{"a"}, {"b"}, {"c"}}

	result, err := Execute("SELECT COUNT(*) AS cnt FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 1 {
		t.Fatalf("expected 1 row, got %d", result.RowCount)
	}
	if *result.Rows[0][0] != "3" {
		t.Fatalf("expected count=3, got %q", *result.Rows[0][0])
	}
}

// --- JOIN tests ---

func TestExecuteHashJoin(t *testing.T) {
	header1 := []string{"id", "name"}
	rows1 := [][]string{
		{"1", "Alice"},
		{"2", "Bob"},
		{"3", "Charlie"},
	}

	header2 := []string{"id", "dept"}
	rows2 := [][]string{
		{"1", "eng"},
		{"2", "sales"},
		{"4", "hr"},
	}

	extras := []NamedData{
		{Alias: "o", Header: header2, Rows: rows2},
	}

	result, err := Execute("SELECT t.name, o.dept FROM t1 t JOIN t2 o ON t.id = o.id", header1, rows1, extras, 100)
	if err != nil {
		t.Fatalf("Execute JOIN error: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 joined rows, got %d", result.RowCount)
	}
}

func TestExecuteJoinWithOrderBy(t *testing.T) {
	header1 := []string{"id", "name", "score"}
	rows1 := [][]string{
		{"1", "Alice", "90"},
		{"2", "Bob", "80"},
	}

	header2 := []string{"id", "grade"}
	rows2 := [][]string{
		{"1", "A"},
		{"2", "B"},
	}

	extras := []NamedData{
		{Alias: "g", Header: header2, Rows: rows2},
	}

	result, err := Execute("SELECT t.name, t.score, g.grade FROM t1 t JOIN t2 g ON t.id = g.id ORDER BY t.score DESC", header1, rows1, extras, 100)
	if err != nil {
		t.Fatalf("Execute JOIN error: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows, got %d", result.RowCount)
	}
	// First row should be Alice (score 90, DESC)
	if *result.Rows[0][0] != "Alice" {
		t.Fatalf("expected first row Alice (DESC), got %q", *result.Rows[0][0])
	}
}

func TestExecuteJoinGroupBy(t *testing.T) {
	header1 := []string{"id", "name", "dept_id"}
	rows1 := [][]string{
		{"1", "Alice", "10"},
		{"2", "Bob", "10"},
		{"3", "Charlie", "20"},
	}

	header2 := []string{"dept_id", "dept_name"}
	rows2 := [][]string{
		{"10", "Engineering"},
		{"20", "Sales"},
	}

	extras := []NamedData{
		{Alias: "d", Header: header2, Rows: rows2},
	}

	result, err := Execute("SELECT d.dept_name, COUNT(*) AS cnt FROM t1 t JOIN t2 d ON t.dept_id = d.dept_id GROUP BY d.dept_name", header1, rows1, extras, 100)
	if err != nil {
		t.Fatalf("Execute JOIN+GROUP BY error: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 groups, got %d", result.RowCount)
	}
}

// --- Complex business scenario tests ---

func TestExecuteComplexArithmetic(t *testing.T) {
	header := []string{"reach_rate", "total_reach_cnt", "tol_cnt"}
	rows := [][]string{
		{"85.0", "170", "200"},
		{"60.0", "60", "100"},
	}

	result, err := Execute("SELECT reach_rate, total_reach_cnt, tol_cnt, ABS(reach_rate - (CAST(total_reach_cnt AS FLOAT) / tol_cnt * 100)) AS diff FROM t WHERE tol_cnt > 0", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows, got %d", result.RowCount)
	}
}

func TestExecuteWhereBetween(t *testing.T) {
	header := []string{"val"}
	rows := [][]string{{"5"}, {"15"}, {"25"}}

	result, err := Execute("SELECT * FROM t WHERE val BETWEEN 10 AND 20", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 1 {
		t.Fatalf("expected 1 row (BETWEEN), got %d", result.RowCount)
	}
}

func TestExecuteWhereIn(t *testing.T) {
	header := []string{"name"}
	rows := [][]string{{"a"}, {"b"}, {"c"}, {"d"}}

	result, err := Execute("SELECT * FROM t WHERE name IN ('a', 'c')", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows (IN), got %d", result.RowCount)
	}
}

func TestExecuteRealWorld(t *testing.T) {
	// Simulate the Q09 scenario: province reach rate ranking
	header := []string{"csmgr_refno", "pnbrn_org_name", "reach_rate", "tol_cnt"}
	rows := [][]string{
		{"RM001", "上海分行", "85.0", "200"},
		{"RM002", "上海分行", "75.0", "150"},
		{"RM003", "江苏分行", "60.0", "100"},
		{"RM004", "江苏分行", "70.0", "120"},
		{"RM005", "广东分行", "90.0", "300"},
		{"RM006", "广东分行", "80.0", "250"},
	}

	result, err := Execute("SELECT pnbrn_org_name, AVG(reach_rate) AS avg_reach_rate FROM t GROUP BY pnbrn_org_name ORDER BY avg_reach_rate DESC", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute real-world error: %v", err)
	}
	if result.RowCount != 3 {
		t.Fatalf("expected 3 provinces, got %d", result.RowCount)
	}
	// First should be 广东 (highest avg: 85)
	if *result.Rows[0][0] != "广东分行" {
		t.Fatalf("expected first 广东分行, got %q", *result.Rows[0][0])
	}
}

// --- NULLS FIRST / LAST ---

func TestLexerNullsFirstLast(t *testing.T) {
	sql := "SELECT * FROM t ORDER BY col DESC NULLS LAST"
	tokens, err := Tokenize(sql)
	if err != nil {
		t.Fatalf("Tokenize error: %v", err)
	}
	// Find NULLS and LAST tokens
	hasNulls, hasLast := false, false
	for _, tok := range tokens {
		if tok.Type == TOKEN_NULLS {
			hasNulls = true
		}
		if tok.Type == TOKEN_LAST {
			hasLast = true
		}
	}
	if !hasNulls {
		t.Fatal("expected NULLS token")
	}
	if !hasLast {
		t.Fatal("expected LAST token")
	}
}

func TestParseNullsFirstLast(t *testing.T) {
	stmt := parseTestStmt(t, "SELECT * FROM t ORDER BY col DESC NULLS LAST")

	if len(stmt.OrderBy) != 1 {
		t.Fatalf("expected 1 ORDER BY, got %d", len(stmt.OrderBy))
	}
	if stmt.OrderBy[0].Dir != "DESC" {
		t.Fatalf("expected DESC, got %q", stmt.OrderBy[0].Dir)
	}
	if stmt.OrderBy[0].NullsDir != "LAST" {
		t.Fatalf("expected NULLS LAST, got %q", stmt.OrderBy[0].NullsDir)
	}
}

func TestParseNullsFirstDefault(t *testing.T) {
	stmt := parseTestStmt(t, "SELECT * FROM t ORDER BY col ASC NULLS FIRST")
	if stmt.OrderBy[0].NullsDir != "FIRST" {
		t.Fatalf("expected NULLS FIRST, got %q", stmt.OrderBy[0].NullsDir)
	}
}

func TestExecuteOrderByNullsLast(t *testing.T) {
	header := []string{"val"}
	rows := [][]string{{"30"}, {""}, {"10"}, {""}, {"20"}}

	// ORDER BY val DESC NULLS LAST: non-null desc first, then nulls
	result, err := Execute("SELECT * FROM t ORDER BY val DESC NULLS LAST", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 5 {
		t.Fatalf("expected 5 rows, got %d", result.RowCount)
	}
	// First row should be "30" (highest non-null)
	if *result.Rows[0][0] != "30" {
		t.Fatalf("expected first row '30', got %q", *result.Rows[0][0])
	}
	// Last rows should be empty (NULLS LAST)
	if *result.Rows[3][0] != "" {
		t.Fatalf("expected row 4 empty, got %q", *result.Rows[3][0])
	}
}

func TestExecuteOrderByNullsFirst(t *testing.T) {
	header := []string{"val"}
	rows := [][]string{{"30"}, {""}, {"10"}, {""}, {"20"}}

	// ORDER BY val ASC NULLS FIRST: nulls first, then ascending
	result, err := Execute("SELECT * FROM t ORDER BY val ASC NULLS FIRST", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 5 {
		t.Fatalf("expected 5 rows, got %d", result.RowCount)
	}
	// First rows should be empty (NULLS FIRST)
	if *result.Rows[0][0] != "" {
		t.Fatalf("expected first row empty, got %q", *result.Rows[0][0])
	}
	// Last row should be "30" (highest)
	if *result.Rows[4][0] != "30" {
		t.Fatalf("expected last row '30', got %q", *result.Rows[4][0])
	}
}

// --- UNION ALL ---

func TestParseUnionAll(t *testing.T) {
	stmt, err := Parse("SELECT a FROM t1 UNION ALL SELECT b FROM t2")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	u, ok := stmt.(*UnionStmt)
	if !ok {
		t.Fatalf("expected *UnionStmt, got %T", stmt)
	}
	if !u.All {
		t.Fatal("expected UNION ALL")
	}
	if u.Left == nil || u.Right == nil {
		t.Fatal("expected left and right SELECTs")
	}
}

func TestExecuteUnionAll(t *testing.T) {
	header := []string{"name"}
	rows := [][]string{{"a"}, {"b"}}

	// The same data appears for both left and right
	result, err := Execute("SELECT name FROM t UNION ALL SELECT name FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	// Should be 4 rows (2 from left + 2 from right)
	if result.RowCount != 4 {
		t.Fatalf("expected 4 rows, got %d", result.RowCount)
	}
}

func TestExecuteUnionAllDifferentData(t *testing.T) {
	// UNION ALL with different WHERE filters effectively gives different subsets
	header := []string{"name", "val"}
	rows := [][]string{{"a", "1"}, {"b", "2"}, {"c", "3"}}

	result, err := Execute("SELECT name FROM t WHERE val >= '2' UNION ALL SELECT name FROM t WHERE val < '2'", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	// 2 rows from first SELECT + 1 row from second SELECT = 3
	if result.RowCount != 3 {
		t.Fatalf("expected 3 rows, got %d", result.RowCount)
	}
}

// --- UNION (distinct) ---

func TestExecuteUnionDistinct(t *testing.T) {
	header := []string{"name"}
	rows := [][]string{{"a"}, {"b"}, {"a"}}

	result, err := Execute("SELECT name FROM t UNION SELECT name FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	// Only 2 distinct values: a, b (but rows run twice so more input)
	// UNION dedup across left + right results
	if result.RowCount != 2 {
		t.Fatalf("expected 2 distinct rows, got %d", result.RowCount)
	}
}

func TestExecuteUnionNoDuplicates(t *testing.T) {
	header := []string{"val"}
	rows := [][]string{{"x"}, {"y"}}

	// UNION of identical queries should still dedup but data is already distinct
	result, err := Execute("SELECT val FROM t WHERE val = 'x' UNION SELECT val FROM t WHERE val = 'y'", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows, got %d", result.RowCount)
	}
}

// --- DISTINCT ON ---

func TestParseDistinctOn(t *testing.T) {
	stmt := parseTestStmt(t, "SELECT DISTINCT ON (dept) dept, name FROM t ORDER BY dept, name DESC")
	if len(stmt.DistinctOn) != 1 {
		t.Fatalf("expected 1 DISTINCT ON column, got %d", len(stmt.DistinctOn))
	}
	if stmt.DistinctOn[0].Col != "dept" {
		t.Fatalf("expected DISTINCT ON dept, got %q", stmt.DistinctOn[0].Col)
	}
}

func TestParseDistinctOnMultiple(t *testing.T) {
	stmt := parseTestStmt(t, "SELECT DISTINCT ON (a, b) a, b, c FROM t ORDER BY a, b")
	if len(stmt.DistinctOn) != 2 {
		t.Fatalf("expected 2 DISTINCT ON columns, got %d", len(stmt.DistinctOn))
	}
}

func TestExecuteDistinctOn(t *testing.T) {
	header := []string{"dept", "name", "score"}
	rows := [][]string{
		{"eng", "alice", "90"},
		{"eng", "bob", "85"},
		{"sales", "carol", "95"},
		{"sales", "dave", "80"},
	}

	// For each dept, keep the first row (highest score due to ORDER BY)
	result, err := Execute("SELECT DISTINCT ON (dept) dept, name, score FROM t ORDER BY dept, score DESC", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows (1 per dept), got %d", result.RowCount)
	}
	// First row should be eng with highest score
}

// --- Subquery IN (SELECT ...) ---

func TestParseSubqueryIn(t *testing.T) {
	stmt := parseTestStmt(t, "SELECT * FROM t WHERE col IN (SELECT col2 FROM t WHERE col2 > 10)")
	if stmt.Where == nil {
		t.Fatal("expected WHERE clause")
	}
	be, ok := stmt.Where.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", stmt.Where)
	}
	if be.Op != "IN" {
		t.Fatalf("expected IN, got %q", be.Op)
	}
	if _, ok := be.Right.(*SubqueryExpr); !ok {
		t.Fatalf("expected SubqueryExpr on right side, got %T", be.Right)
	}
}

func TestExecuteSubqueryIn(t *testing.T) {
	header := []string{"id", "name"}
	rows := [][]string{
		{"1", "alice"},
		{"2", "bob"},
		{"3", "carol"},
		{"4", "dave"},
	}

	result, err := Execute("SELECT * FROM t WHERE id IN (SELECT id FROM t WHERE name >= 'c')", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	// Subquery returns carol(3) and dave(4) → main query should return those 2
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows, got %d", result.RowCount)
	}
}

func TestExecuteSubqueryNotIn(t *testing.T) {
	header := []string{"id", "name"}
	rows := [][]string{
		{"1", "alice"},
		{"2", "bob"},
		{"3", "carol"},
	}

	result, err := Execute("SELECT name FROM t WHERE name NOT IN (SELECT name FROM t WHERE id = '2')", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	// Subquery returns 'bob', main should return 'alice' and 'carol'
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows, got %d", result.RowCount)
	}
}

// --- Edge cases ---

func TestExecuteEmptyTable(t *testing.T) {
	header := []string{"name"}
	var rows [][]string

	_, err := Execute("SELECT * FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute empty error: %v", err)
	}
}

func TestExecuteNoMatchWhere(t *testing.T) {
	header := []string{"name"}
	rows := [][]string{{"a"}, {"b"}}

	result, err := Execute("SELECT * FROM t WHERE name = 'nonexistent'", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 0 {
		t.Fatalf("expected 0 rows, got %d", result.RowCount)
	}
}

// --- Double-quoted strings (v2) ---

func TestLexerDoubleQuotedString(t *testing.T) {
	tokens, err := Tokenize(`"hello world"`)
	if err != nil {
		t.Fatalf("Tokenize double-quoted error: %v", err)
	}
	found := false
	for _, tok := range tokens {
		if tok.Type == TOKEN_STRING && tok.Value == "hello world" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected TOKEN_STRING for double-quoted literal")
	}
}

func TestExecuteDoubleQuotedString(t *testing.T) {
	header := []string{"name", "city"}
	rows := [][]string{
		{"alice", "上海"},
		{"bob", "北京"},
		{"carol", "上海"},
	}

	result, err := Execute(`SELECT name FROM t WHERE city = "上海"`, header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute double-quote error: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows (city=上海), got %d", result.RowCount)
	}
}

// --- IS NULL / IS NOT NULL (v2) ---

func TestParseIsNull(t *testing.T) {
	stmt := parseTestStmt(t, "SELECT * FROM t WHERE col IS NULL")
	be, ok := stmt.Where.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", stmt.Where)
	}
	if be.Op != "IS NULL" {
		t.Fatalf("expected IS NULL, got %q", be.Op)
	}
}

func TestParseIsNotNull(t *testing.T) {
	stmt := parseTestStmt(t, "SELECT * FROM t WHERE col IS NOT NULL")
	be, ok := stmt.Where.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", stmt.Where)
	}
	if be.Op != "IS NOT NULL" {
		t.Fatalf("expected IS NOT NULL, got %q", be.Op)
	}
}

func TestExecuteIsNull(t *testing.T) {
	header := []string{"name", "remark"}
	rows := [][]string{
		{"a", "ok"},
		{"b", ""},
		{"c", "done"},
		{"d", ""},
	}

	result, err := Execute("SELECT * FROM t WHERE remark IS NULL", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute IS NULL error: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows (remark IS NULL), got %d", result.RowCount)
	}
}

func TestExecuteIsNotNull(t *testing.T) {
	header := []string{"name", "remark"}
	rows := [][]string{
		{"a", "ok"},
		{"b", ""},
		{"c", "done"},
		{"d", ""},
	}

	result, err := Execute("SELECT * FROM t WHERE remark IS NOT NULL", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute IS NOT NULL error: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows (remark IS NOT NULL), got %d", result.RowCount)
	}
}

// --- HAVING (v2) ---

func TestParseHaving(t *testing.T) {
	stmt := parseTestStmt(t, "SELECT dept, AVG(score) AS avg_score FROM t GROUP BY dept HAVING avg_score > 50")
	if stmt.Having == nil {
		t.Fatal("expected HAVING clause")
	}
	be, ok := stmt.Having.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr for HAVING, got %T", stmt.Having)
	}
	if be.Op != ">" {
		t.Fatalf("expected > in HAVING, got %q", be.Op)
	}
}

func TestExecuteHaving(t *testing.T) {
	header := []string{"dept", "score"}
	rows := [][]string{
		{"eng", "90"},
		{"eng", "70"},
		{"sales", "40"},
		{"sales", "60"},
		{"hr", "50"},
	}

	result, err := Execute("SELECT dept, AVG(score) AS avg_score FROM t GROUP BY dept HAVING avg_score > 50", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute HAVING error: %v", err)
	}
	// eng avg=80, sales avg=50, hr avg=50 — only eng > 50
	if result.RowCount != 1 {
		t.Fatalf("expected 1 group (HAVING avg > 50), got %d", result.RowCount)
	}
	if *result.Rows[0][0] != "eng" {
		t.Fatalf("expected 'eng' as surviving group, got %q", *result.Rows[0][0])
	}
}

// --- LEFT JOIN (v2) ---

func TestParseLeftJoin(t *testing.T) {
	stmt := parseTestStmt(t, "SELECT t.*, o.name FROM t1 t LEFT JOIN t2 o ON t.id = o.id")
	if len(stmt.Joins) != 1 {
		t.Fatalf("expected 1 JOIN, got %d", len(stmt.Joins))
	}
	if stmt.Joins[0].JoinType != "LEFT" {
		t.Fatalf("expected LEFT JOIN, got %q", stmt.Joins[0].JoinType)
	}
}

func TestParseRightJoin(t *testing.T) {
	stmt := parseTestStmt(t, "SELECT * FROM t1 RIGHT JOIN t2 ON t1.id = t2.id")
	if stmt.Joins[0].JoinType != "RIGHT" {
		t.Fatalf("expected RIGHT JOIN, got %q", stmt.Joins[0].JoinType)
	}
}

func TestExecuteLeftJoinMatchAll(t *testing.T) {
	header1 := []string{"id", "name"}
	rows1 := [][]string{
		{"1", "Alice"},
		{"2", "Bob"},
		{"3", "Charlie"},
	}

	header2 := []string{"id", "dept"}
	rows2 := [][]string{
		{"1", "eng"},
		{"2", "sales"},
	}

	extras := []NamedData{{Alias: "o", Header: header2, Rows: rows2}}

	result, err := Execute("SELECT t.name, o.dept FROM t1 t LEFT JOIN t2 o ON t.id = o.id", header1, rows1, extras, 100)
	if err != nil {
		t.Fatalf("Execute LEFT JOIN error: %v", err)
	}
	// Charlie has no match but should still appear (3 rows)
	if result.RowCount != 3 {
		t.Fatalf("expected 3 rows (LEFT JOIN includes Charlie), got %d", result.RowCount)
	}
	// Last row (Charlie) should have empty dept
	if *result.Rows[2][1] != "" {
		t.Fatalf("expected Charlie's dept to be empty, got %q", *result.Rows[2][1])
	}
}

func TestExecuteLeftJoinNoMatch(t *testing.T) {
	header1 := []string{"id", "name"}
	rows1 := [][]string{
		{"1", "Alice"},
		{"2", "Bob"},
	}

	header2 := []string{"id", "dept"}
	rows2 := [][]string{
		{"3", "hr"},
		{"4", "finance"},
	}

	extras := []NamedData{{Alias: "o", Header: header2, Rows: rows2}}

	result, err := Execute("SELECT t.name, o.dept FROM t1 t LEFT JOIN t2 o ON t.id = o.id", header1, rows1, extras, 100)
	if err != nil {
		t.Fatalf("Execute LEFT JOIN no-match error: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows (all left rows retained), got %d", result.RowCount)
	}
	// Both should have empty dept
	for i, row := range result.Rows {
		if *row[1] != "" {
			t.Fatalf("expected row %d dept empty, got %q", i, *row[1])
		}
	}
}

// --- ROUND single-arg (v2) ---

func TestExecuteRoundSingleArg(t *testing.T) {
	header := []string{"val"}
	rows := [][]string{{"42.7"}, {"3.14"}, {"99.9"}}

	result, err := Execute("SELECT ROUND(val) AS r FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute ROUND single-arg error: %v", err)
	}
	if result.RowCount != 3 {
		t.Fatalf("expected 3 rows, got %d", result.RowCount)
	}
	if *result.Rows[0][0] != "43" {
		t.Fatalf("expected ROUND(42.7)=43, got %q", *result.Rows[0][0])
	}
	if *result.Rows[1][0] != "3" {
		t.Fatalf("expected ROUND(3.14)=3, got %q", *result.Rows[1][0])
	}
}

func TestExecuteRoundTwoArg(t *testing.T) {
	header := []string{"val"}
	rows := [][]string{{"42.777"}, {"3.14159"}}

	result, err := Execute("SELECT ROUND(val, 2) AS r FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute ROUND two-arg error: %v", err)
	}
	if *result.Rows[0][0] != "42.78" {
		t.Fatalf("expected ROUND(42.777, 2)=42.78, got %q", *result.Rows[0][0])
	}
}

func TestExecuteOrderByAlias(t *testing.T) {
	header := []string{"name", "total", "cnt"}
	rows := [][]string{
		{"a", "100", "10"},
		{"b", "200", "5"},
		{"c", "50", "10"},
		{"d", "300", "5"},
	}
	// ORDER BY computed expression alias DESC: ratio = total/cnt
	// Expected: d=60, b=40, a=10, c=5
	result, err := Execute("SELECT name, CAST(total AS FLOAT) / cnt * 100 AS ratio FROM t ORDER BY ratio DESC", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 4 {
		t.Fatalf("expected 4 rows, got %d", result.RowCount)
	}
	// Verify DESC order by ratio
	expected := []string{"d", "b", "a", "c"}
	for i, exp := range expected {
		if *result.Rows[i][0] != exp {
			t.Fatalf("position %d: expected name=%q, got %q", i, exp, *result.Rows[i][0])
		}
	}
	// Also test ASC
	result, err = Execute("SELECT name, CAST(total AS FLOAT) / cnt * 100 AS ratio FROM t ORDER BY ratio ASC", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	expectedAsc := []string{"c", "a", "b", "d"}
	for i, exp := range expectedAsc {
		if *result.Rows[i][0] != exp {
			t.Fatalf("ASC position %d: expected name=%q, got %q", i, exp, *result.Rows[i][0])
		}
	}
}

// ── Window function tests ──

func TestLexerOverKeyword(t *testing.T) {
	tokens, err := Tokenize("OVER")
	if err != nil {
		t.Fatalf("Tokenize error: %v", err)
	}
	if len(tokens) < 1 || tokens[0].Type != TOKEN_OVER {
		t.Fatalf("expected TOKEN_OVER, got %v", tokens[0])
	}
}

func TestLexerPartitionKeyword(t *testing.T) {
	tokens, err := Tokenize("PARTITION")
	if err != nil {
		t.Fatalf("Tokenize error: %v", err)
	}
	if len(tokens) < 1 || tokens[0].Type != TOKEN_PARTITION {
		t.Fatalf("expected TOKEN_PARTITION, got %v", tokens[0])
	}
}

func TestParseRowNumberOverEmpty(t *testing.T) {
	stmt := parseTestStmt(t, "SELECT ROW_NUMBER() OVER () AS rn FROM t")
	if len(stmt.Columns) != 1 {
		t.Fatalf("expected 1 column, got %d", len(stmt.Columns))
	}
	fc, ok := stmt.Columns[0].Expr.(*FuncCall)
	if !ok {
		t.Fatalf("expected FuncCall, got %T", stmt.Columns[0].Expr)
	}
	if fc.Name != "ROW_NUMBER" {
		t.Fatalf("expected ROW_NUMBER, got %q", fc.Name)
	}
	if fc.Over == nil {
		t.Fatal("expected Over clause")
	}
	if len(fc.Over.PartitionBy) != 0 {
		t.Fatalf("expected empty PARTITION BY, got %d columns", len(fc.Over.PartitionBy))
	}
	if len(fc.Over.OrderBy) != 0 {
		t.Fatalf("expected empty ORDER BY, got %d columns", len(fc.Over.OrderBy))
	}
	if stmt.Columns[0].Alias != "rn" {
		t.Fatalf("expected alias rn, got %q", stmt.Columns[0].Alias)
	}
}

func TestParseRowNumberOverPartitionOrder(t *testing.T) {
	stmt := parseTestStmt(t, "SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY salary DESC) AS rn FROM t")
	fc, ok := stmt.Columns[0].Expr.(*FuncCall)
	if !ok {
		t.Fatalf("expected FuncCall, got %T", stmt.Columns[0].Expr)
	}
	if fc.Over == nil {
		t.Fatal("expected Over clause")
	}
	if len(fc.Over.PartitionBy) != 1 {
		t.Fatalf("expected 1 PARTITION BY column, got %d", len(fc.Over.PartitionBy))
	}
	if fc.Over.PartitionBy[0].Col != "dept" {
		t.Fatalf("expected PARTITION BY dept, got %q", fc.Over.PartitionBy[0].Col)
	}
	if len(fc.Over.OrderBy) != 1 {
		t.Fatalf("expected 1 ORDER BY column, got %d", len(fc.Over.OrderBy))
	}
	if fc.Over.OrderBy[0].Expr.Col != "salary" {
		t.Fatalf("expected ORDER BY salary, got %q", fc.Over.OrderBy[0].Expr.Col)
	}
	if fc.Over.OrderBy[0].Dir != "DESC" {
		t.Fatalf("expected DESC, got %q", fc.Over.OrderBy[0].Dir)
	}
}

func TestParseRankDenseRankNtile(t *testing.T) {
	stmt := parseTestStmt(t, "SELECT RANK() OVER (ORDER BY score), DENSE_RANK() OVER (ORDER BY score), NTILE(4) OVER (ORDER BY id) FROM t")
	if len(stmt.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(stmt.Columns))
	}
	for i, name := range []string{"RANK", "DENSE_RANK", "NTILE"} {
		fc, ok := stmt.Columns[i].Expr.(*FuncCall)
		if !ok {
			t.Fatalf("column[%d]: expected FuncCall, got %T", i, stmt.Columns[i].Expr)
		}
		if fc.Name != name {
			t.Fatalf("column[%d]: expected %s, got %q", i, name, fc.Name)
		}
		if fc.Over == nil {
			t.Fatalf("column[%d]: expected Over clause", i)
		}
	}
}

func TestExecuteRowNumberNoPartition(t *testing.T) {
	header := []string{"name", "score"}
	rows := [][]string{
		{"alice", "90"},
		{"bob", "85"},
		{"carol", "95"},
	}

	result, err := Execute("SELECT name, score, ROW_NUMBER() OVER (ORDER BY score DESC) AS rn FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 3 {
		t.Fatalf("expected 3 rows, got %d", result.RowCount)
	}
	// ROW_NUMBER should be 1,2,3 ordered by score DESC: carol(95)=1, alice(90)=2, bob(85)=3
	expectedRN := []string{"2", "3", "1"}
	for i, rn := range expectedRN {
		if *result.Rows[i][2] != rn {
			t.Fatalf("row %d: expected rn=%s, got %s (name=%s)", i, rn, *result.Rows[i][2], *result.Rows[i][0])
		}
	}
}

func TestExecuteRowNumberPartitionBy(t *testing.T) {
	header := []string{"dept", "name", "salary"}
	rows := [][]string{
		{"eng", "alice", "100"},
		{"eng", "bob", "80"},
		{"sales", "carol", "90"},
		{"sales", "dave", "70"},
	}

	result, err := Execute("SELECT dept, name, ROW_NUMBER() OVER (PARTITION BY dept ORDER BY salary DESC) AS rn FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 4 {
		t.Fatalf("expected 4 rows, got %d", result.RowCount)
	}

	expected := map[string]string{
		"alice": "1",
		"bob":   "2",
		"carol": "1",
		"dave":  "2",
	}
	for _, row := range result.Rows {
		name := *row[1]
		rn := *row[2]
		if expected[name] != rn {
			t.Errorf("name=%s: expected rn=%s, got rn=%s", name, expected[name], rn)
		}
	}
}

func TestExecuteRowNumberOrderByWindow(t *testing.T) {
	header := []string{"name", "salary"}
	rows := [][]string{
		{"bob", "80"},
		{"alice", "100"},
		{"carol", "90"},
	}

	result, err := Execute("SELECT name, salary, ROW_NUMBER() OVER (ORDER BY name) AS rn FROM t ORDER BY salary DESC", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 3 {
		t.Fatalf("expected 3 rows, got %d", result.RowCount)
	}
	// Final ORDER BY salary DESC: alice(100)=rn1, carol(90)=rn3, bob(80)=rn2
	expectedRN := []string{"1", "3", "2"}
	for i, rn := range expectedRN {
		if *result.Rows[i][2] != rn {
			t.Fatalf("row %d: expected rn=%s, got %s", i, rn, *result.Rows[i][2])
		}
	}
}

func TestExecuteRankWithTies(t *testing.T) {
	header := []string{"name", "score"}
	rows := [][]string{
		{"alice", "90"},
		{"bob", "85"},
		{"carol", "90"},
		{"dave", "70"},
	}

	result, err := Execute("SELECT name, score, RANK() OVER (ORDER BY score DESC) AS rk FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 4 {
		t.Fatalf("expected 4 rows, got %d", result.RowCount)
	}

	expected := map[string]string{
		"alice": "1",
		"carol": "1",
		"bob":   "3",
		"dave":  "4",
	}
	for _, row := range result.Rows {
		name := *row[0]
		rk := *row[2]
		if expected[name] != rk {
			t.Errorf("name=%s: expected rank=%s, got %s", name, expected[name], rk)
		}
	}
}

func TestExecuteDenseRankNoGaps(t *testing.T) {
	header := []string{"name", "score"}
	rows := [][]string{
		{"alice", "90"},
		{"bob", "85"},
		{"carol", "90"},
		{"dave", "70"},
	}

	result, err := Execute("SELECT name, score, DENSE_RANK() OVER (ORDER BY score DESC) AS dr FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 4 {
		t.Fatalf("expected 4 rows, got %d", result.RowCount)
	}

	expected := map[string]string{
		"alice": "1",
		"carol": "1",
		"bob":   "2",
		"dave":  "3",
	}
	for _, row := range result.Rows {
		name := *row[0]
		dr := *row[2]
		if expected[name] != dr {
			t.Errorf("name=%s: expected denserank=%s, got %s", name, expected[name], dr)
		}
	}
}

func TestExecuteNtileBucketing(t *testing.T) {
	header := []string{"name", "score"}
	rows := make([][]string, 10)
	for i := 0; i < 10; i++ {
		rows[i] = []string{fmt.Sprintf("p%d", i+1), fmt.Sprintf("%d", 100-i*10)}
	}

	result, err := Execute("SELECT name, score, NTILE(3) OVER (ORDER BY score DESC) AS bucket FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 10 {
		t.Fatalf("expected 10 rows, got %d", result.RowCount)
	}

	expectedBuckets := []string{"1", "1", "1", "1", "2", "2", "2", "3", "3", "3"}
	for i, bucket := range expectedBuckets {
		if *result.Rows[i][2] != bucket {
			t.Errorf("row %d (name=%s): expected bucket=%s, got %s", i, *result.Rows[i][0], bucket, *result.Rows[i][2])
		}
	}
}

func TestExecuteWindowWithPartitionTies(t *testing.T) {
	header := []string{"dept", "name", "score"}
	rows := [][]string{
		{"eng", "alice", "90"},
		{"eng", "bob", "85"},
		{"eng", "carol", "90"},
		{"sales", "dave", "95"},
		{"sales", "eve", "80"},
	}

	result, err := Execute("SELECT dept, name, score, RANK() OVER (PARTITION BY dept ORDER BY score DESC) AS rk FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 5 {
		t.Fatalf("expected 5 rows, got %d", result.RowCount)
	}

	expected := map[string]string{
		"alice": "1",
		"carol": "1",
		"bob":   "3",
		"dave":  "1",
		"eve":   "2",
	}
	for _, row := range result.Rows {
		name := *row[1]
		rk := *row[3]
		if expected[name] != rk {
			t.Errorf("name=%s: expected rk=%s, got %s", name, expected[name], rk)
		}
	}
}

func TestExecuteWindowColumnAlias(t *testing.T) {
	header := []string{"name", "salary"}
	rows := [][]string{
		{"alice", "100"},
		{"bob", "80"},
	}

	result, err := Execute("SELECT name, ROW_NUMBER() OVER (ORDER BY salary DESC) AS row_num FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows, got %d", result.RowCount)
	}
	if result.Columns[1].Name != "row_num" {
		t.Fatalf("expected column name 'row_num', got %q", result.Columns[1].Name)
	}
}

// ── Phase 2: Value-reference window functions ──

func TestExecuteLagDefault(t *testing.T) {
	header := []string{"name", "salary"}
	rows := [][]string{
		{"alice", "100"},
		{"bob", "80"},
		{"carol", "90"},
	}

	result, err := Execute("SELECT name, salary, LAG(salary, 1, '0') OVER (ORDER BY name) AS prev_salary FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 3 {
		t.Fatalf("expected 3 rows, got %d", result.RowCount)
	}
	// ORDER BY name: alice(100), bob(80), carol(90)
	// LAG: alice→default 0, bob→prev=100, carol→prev=80
	for _, row := range result.Rows {
		name := *row[0]
		prev := *row[2]
		switch name {
		case "alice":
			if prev != "0" {
				t.Errorf("alice: expected prev='0', got %q", prev)
			}
		case "bob":
			if prev != "100" {
				t.Errorf("bob: expected prev='100', got %q", prev)
			}
		case "carol":
			if prev != "80" {
				t.Errorf("carol: expected prev='80', got %q", prev)
			}
		}
	}
}

func TestExecuteLeadDefaultEmpty(t *testing.T) {
	header := []string{"name", "salary"}
	rows := [][]string{
		{"alice", "100"},
		{"bob", "80"},
	}

	result, err := Execute("SELECT name, salary, LEAD(salary) OVER (ORDER BY name) AS next_salary FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows, got %d", result.RowCount)
	}
	// ORDER BY name: alice(100), bob(80)
	// LEAD: alice→next=80, bob→next='' (default empty)
	for _, row := range result.Rows {
		name := *row[0]
		next := *row[2]
		switch name {
		case "alice":
			if next != "80" {
				t.Errorf("alice: expected next='80', got %q", next)
			}
		case "bob":
			if next != "" {
				t.Errorf("bob: expected next='', got %q", next)
			}
		}
	}
}

func TestExecuteFirstValue(t *testing.T) {
	header := []string{"dept", "name", "salary"}
	rows := [][]string{
		{"eng", "bob", "80"},
		{"eng", "alice", "100"},
		{"sales", "dave", "70"},
		{"sales", "carol", "90"},
	}

	result, err := Execute("SELECT dept, name, salary, FIRST_VALUE(name) OVER (PARTITION BY dept ORDER BY salary DESC) AS top_earner FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 4 {
		t.Fatalf("expected 4 rows, got %d", result.RowCount)
	}
	// eng partition: alice(100)=first, bob(80)=last
	// sales partition: carol(90)=first, dave(70)=last
	// FIRST_VALUE: all eng rows get "alice", all sales rows get "carol"
	for _, row := range result.Rows {
		dept := *row[0]
		top := *row[3]
		switch dept {
		case "eng":
			if top != "alice" {
				t.Errorf("eng: expected top='alice', got %q", top)
			}
		case "sales":
			if top != "carol" {
				t.Errorf("sales: expected top='carol', got %q", top)
			}
		}
	}
}

func TestExecuteLastValue(t *testing.T) {
	header := []string{"dept", "name", "score"}
	rows := [][]string{
		{"eng", "alice", "90"},
		{"eng", "bob", "85"},
		{"eng", "carol", "95"},
	}

	result, err := Execute("SELECT dept, name, score, LAST_VALUE(name) OVER (PARTITION BY dept ORDER BY score ASC) AS lowest FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 3 {
		t.Fatalf("expected 3 rows, got %d", result.RowCount)
	}
	// Default frame: RANGE BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
	// eng partition, ORDER BY score ASC (sorted): bob(85), alice(90), carol(95)
	// LAST_VALUE with default frame: each row sees the current row's value
	// alice at sorted pos 1 → frame [0..1] → LAST_VALUE = "alice"
	for _, row := range result.Rows {
		lowest := *row[3]
		name := *row[1]
		if lowest != name {
			t.Errorf("row %q: expected lowest=%q (current row with default frame), got %q", name, name, lowest)
		}
	}
}

func TestExecuteLagExprColumn(t *testing.T) {
	header := []string{"name", "qty", "price"}
	rows := [][]string{
		{"a", "10", "5"},
		{"b", "20", "3"},
		{"c", "15", "4"},
	}

	result, err := Execute("SELECT name, qty * price AS val, LAG(qty * price, 1, '0') OVER (ORDER BY name) AS prev_val FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 3 {
		t.Fatalf("expected 3 rows, got %d", result.RowCount)
	}
	// ORDER BY name: a(10*5=50), b(20*3=60), c(15*4=60)
	// LAG: a→0, b→50, c→60
	for _, row := range result.Rows {
		name := *row[0]
		prev := *row[2]
		switch name {
		case "a":
			if prev != "0" {
				t.Errorf("a: expected prev='0', got %q", prev)
			}
		case "b":
			if prev != "50" {
				t.Errorf("b: expected prev='50', got %q", prev)
			}
		case "c":
			if prev != "60" {
				t.Errorf("c: expected prev='60', got %q", prev)
			}
		}
	}
}

// ── Phase 3: Aggregate-as-window functions ──

func TestExecuteSumOverPartition(t *testing.T) {
	header := []string{"dept", "name", "salary"}
	rows := [][]string{
		{"eng", "alice", "100"},
		{"eng", "bob", "80"},
		{"sales", "carol", "90"},
	}

	result, err := Execute("SELECT dept, name, salary, SUM(salary) OVER (PARTITION BY dept) AS dept_total FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 3 {
		t.Fatalf("expected 3 rows, got %d", result.RowCount)
	}
	// eng total: 100+80=180, sales total: 90
	for _, row := range result.Rows {
		dept := *row[0]
		total := *row[3]
		switch dept {
		case "eng":
			if total != "180" {
				t.Errorf("eng: expected total='180', got %q", total)
			}
		case "sales":
			if total != "90" {
				t.Errorf("sales: expected total='90', got %q", total)
			}
		}
	}
}

func TestExecuteAvgOverPartition(t *testing.T) {
	header := []string{"dept", "name", "salary"}
	rows := [][]string{
		{"eng", "alice", "100"},
		{"eng", "bob", "80"},
		{"sales", "carol", "90"},
	}

	result, err := Execute("SELECT dept, name, AVG(salary) OVER (PARTITION BY dept) AS dept_avg FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	// eng avg: (100+80)/2=90, sales avg: 90/1=90
	for _, row := range result.Rows {
		avg := *row[2]
		if avg != "90" {
			t.Errorf("expected avg='90', got %q", avg)
		}
	}
}

func TestExecuteCountOverPartition(t *testing.T) {
	header := []string{"dept", "name"}
	rows := [][]string{
		{"eng", "alice"},
		{"eng", "bob"},
		{"sales", "carol"},
	}

	result, err := Execute("SELECT dept, name, COUNT(*) OVER (PARTITION BY dept) AS dept_count FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	for _, row := range result.Rows {
		dept := *row[0]
		cnt := *row[2]
		switch dept {
		case "eng":
			if cnt != "2" {
				t.Errorf("eng: expected count='2', got %q", cnt)
			}
		case "sales":
			if cnt != "1" {
				t.Errorf("sales: expected count='1', got %q", cnt)
			}
		}
	}
}

func TestExecuteMaxMinOverPartition(t *testing.T) {
	header := []string{"dept", "name", "score"}
	rows := [][]string{
		{"eng", "alice", "90"},
		{"eng", "bob", "80"},
		{"eng", "carol", "95"},
		{"sales", "dave", "85"},
	}

	result, err := Execute("SELECT dept, name, score, MAX(score) OVER (PARTITION BY dept) AS max_score, MIN(score) OVER (PARTITION BY dept) AS min_score FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	// eng: max=95, min=80; sales: max=85, min=85
	for _, row := range result.Rows {
		dept := *row[0]
		maxS := *row[3]
		minS := *row[4]
		switch dept {
		case "eng":
			if maxS != "95" {
				t.Errorf("eng: expected max='95', got %q", maxS)
			}
			if minS != "80" {
				t.Errorf("eng: expected min='80', got %q", minS)
			}
		case "sales":
			if maxS != "85" {
				t.Errorf("sales: expected max='85', got %q", maxS)
			}
			if minS != "85" {
				t.Errorf("sales: expected min='85', got %q", minS)
			}
		}
	}
}

func TestExecuteAggOverNoArg(t *testing.T) {
	header := []string{"name", "val"}
	rows := [][]string{
		{"a", "10"},
		{"b", "20"},
		{"c", "30"},
	}

	result, err := Execute("SELECT name, val, SUM(val) OVER () AS total FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 3 {
		t.Fatalf("expected 3 rows, got %d", result.RowCount)
	}
	// No PARTITION BY → single partition, total = 10+20+30 = 60
	for _, row := range result.Rows {
		total := *row[2]
		if total != "60" {
			t.Errorf("expected total='60', got %q", total)
		}
	}
}

// ── Phase 4: Window Frame Spec Tests ──

func TestLexerFrameKeywords(t *testing.T) {
	tests := []struct {
		input string
		typ   TokenType
	}{
		{"ROWS", TOKEN_ROWS},
		{"RANGE", TOKEN_RANGE},
		{"UNBOUNDED", TOKEN_UNBOUNDED},
		{"PRECEDING", TOKEN_PRECEDING},
		{"FOLLOWING", TOKEN_FOLLOWING},
		{"CURRENT", TOKEN_CURRENT},
		{"ROW", TOKEN_ROW},
	}
	for _, tc := range tests {
		tokens, err := Tokenize(tc.input)
		if err != nil {
			t.Fatalf("Tokenize(%q) error: %v", tc.input, err)
		}
		if len(tokens) < 1 || tokens[0].Type != tc.typ {
			t.Errorf("Tokenize(%q): expected type %d, got %d (%s)", tc.input, tc.typ, tokens[0].Type, tokens[0].Value)
		}
	}
}

func TestParseRowNumberOverRowsFrame(t *testing.T) {
	stmt, err := Parse("SELECT ROW_NUMBER() OVER (ORDER BY x ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS rn FROM t")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	sel, ok := stmt.(*SelectStmt)
	if !ok {
		t.Fatalf("expected *SelectStmt, got %T", stmt)
	}
	if len(sel.Columns) != 1 {
		t.Fatalf("expected 1 column, got %d", len(sel.Columns))
	}
	fc, ok := sel.Columns[0].Expr.(*FuncCall)
	if !ok || fc.Name != "ROW_NUMBER" {
		t.Fatalf("expected ROW_NUMBER, got %v", sel.Columns[0].Expr)
	}
	if fc.Over == nil {
		t.Fatal("expected OVER clause")
	}
	if fc.Over.Frame == nil {
		t.Fatal("expected frame clause")
	}
	if fc.Over.Frame.Type != "ROWS" {
		t.Fatalf("expected ROWS frame, got %s", fc.Over.Frame.Type)
	}
	if fc.Over.Frame.Start.Type != FrameUnboundedPreceding {
		t.Fatalf("expected UNBOUNDED PRECEDING start, got %s", fc.Over.Frame.Start.Type)
	}
	if fc.Over.Frame.End.Type != FrameCurrentRow {
		t.Fatalf("expected CURRENT ROW end, got %s", fc.Over.Frame.End.Type)
	}
}

func TestParseRangeFrame(t *testing.T) {
	stmt, err := Parse("SELECT SUM(x) OVER (ORDER BY y RANGE BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING) FROM t")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	sel, ok := stmt.(*SelectStmt)
	if !ok {
		t.Fatalf("expected *SelectStmt, got %T", stmt)
	}
	fc, ok := sel.Columns[0].Expr.(*FuncCall)
	if !ok {
		t.Fatalf("expected FuncCall, got %T", sel.Columns[0].Expr)
	}
	if fc.Over.Frame == nil {
		t.Fatal("expected frame clause")
	}
	if fc.Over.Frame.Type != "RANGE" {
		t.Fatalf("expected RANGE frame, got %s", fc.Over.Frame.Type)
	}
	if fc.Over.Frame.End.Type != FrameUnboundedFollowing {
		t.Fatalf("expected UNBOUNDED FOLLOWING end, got %s", fc.Over.Frame.End.Type)
	}
}

func TestParseRowsShorthandFrame(t *testing.T) {
	// Shorthand: ROWS UNBOUNDED PRECEDING (without BETWEEN ... AND)
	stmt, err := Parse("SELECT ROW_NUMBER() OVER (ORDER BY x ROWS UNBOUNDED PRECEDING) AS rn FROM t")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	sel, ok := stmt.(*SelectStmt)
	if !ok {
		t.Fatalf("expected *SelectStmt, got %T", stmt)
	}
	fc, ok := sel.Columns[0].Expr.(*FuncCall)
	if !ok {
		t.Fatalf("expected FuncCall, got %T", sel.Columns[0].Expr)
	}
	if fc.Over.Frame == nil {
		t.Fatal("expected frame clause")
	}
	if fc.Over.Frame.Start.Type != FrameUnboundedPreceding {
		t.Fatalf("expected UNBOUNDED PRECEDING start, got %s", fc.Over.Frame.Start.Type)
	}
	if fc.Over.Frame.End.Type != FrameCurrentRow {
		t.Fatalf("expected CURRENT ROW end (default), got %s", fc.Over.Frame.End.Type)
	}
}

func TestExecuteLastValueFullPartitionFrame(t *testing.T) {
	header := []string{"dept", "name", "score"}
	rows := [][]string{
		{"eng", "alice", "90"},
		{"eng", "bob", "85"},
		{"eng", "carol", "95"},
	}

	// ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING = partition-wide
	result, err := Execute("SELECT dept, name, score, LAST_VALUE(name) OVER (PARTITION BY dept ORDER BY score ASC ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING) AS lowest FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 3 {
		t.Fatalf("expected 3 rows, got %d", result.RowCount)
	}
	// ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING: all rows see the partition-wide last value
	for _, row := range result.Rows {
		lowest := *row[3]
		if lowest != "carol" {
			t.Errorf("expected lowest='carol', got %q", lowest)
		}
	}
}

func TestExecuteSlidingWindowSum(t *testing.T) {
	header := []string{"id", "val"}
	rows := [][]string{
		{"1", "10"},
		{"2", "20"},
		{"3", "30"},
		{"4", "40"},
		{"5", "50"},
	}

	// ROWS BETWEEN 1 PRECEDING AND 1 FOLLOWING (moving average window)
	result, err := Execute("SELECT id, val, SUM(CAST(val AS INTEGER)) OVER (ORDER BY id ROWS BETWEEN 1 PRECEDING AND 1 FOLLOWING) AS window_sum FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 5 {
		t.Fatalf("expected 5 rows, got %d", result.RowCount)
	}
	// Window: 1 preceding to 1 following
	// id=1: 10+20          = 30
	// id=2: 10+20+30       = 60
	// id=3: 20+30+40       = 90
	// id=4: 30+40+50       = 120
	// id=5: 40+50          = 90
	expected := map[string]string{
		"1": "30",
		"2": "60",
		"3": "90",
		"4": "120",
		"5": "90",
	}
	for _, row := range result.Rows {
		id := *row[0]
		ws := *row[2]
		if exp, ok := expected[id]; ok {
			if ws != exp {
				t.Errorf("id=%s: expected window_sum=%s, got %s", id, exp, ws)
			}
		}
	}
}

func TestExecuteRowsFrameUnboundedFollowing(t *testing.T) {
	header := []string{"id", "val"}
	rows := [][]string{
		{"1", "10"},
		{"2", "20"},
		{"3", "30"},
	}

	// ROWS BETWEEN CURRENT ROW AND UNBOUNDED FOLLOWING: sum of current and all remaining rows
	result, err := Execute("SELECT id, val, SUM(CAST(val AS INTEGER)) OVER (ORDER BY id ROWS BETWEEN CURRENT ROW AND UNBOUNDED FOLLOWING) AS remaining FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 3 {
		t.Fatalf("expected 3 rows, got %d", result.RowCount)
	}
	// id=1: 10+20+30 = 60
	// id=2: 20+30    = 50
	// id=3: 30       = 30
	expected := map[string]string{
		"1": "60",
		"2": "50",
		"3": "30",
	}
	for _, row := range result.Rows {
		id := *row[0]
		rem := *row[2]
		if exp, ok := expected[id]; ok {
			if rem != exp {
				t.Errorf("id=%s: expected remaining=%s, got %s", id, exp, rem)
			}
		}
	}
}

func TestExecuteFirstValueWithFrame(t *testing.T) {
	header := []string{"id", "val"}
	rows := [][]string{
		{"1", "10"},
		{"2", "20"},
		{"3", "30"},
		{"4", "40"},
	}

	// ROWS BETWEEN 1 PRECEDING AND CURRENT ROW: FIRST_VALUE sees the first row in frame
	result, err := Execute("SELECT id, val, FIRST_VALUE(CAST(val AS INTEGER)) OVER (ORDER BY id ROWS BETWEEN 1 PRECEDING AND CURRENT ROW) AS fv FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 4 {
		t.Fatalf("expected 4 rows, got %d", result.RowCount)
	}
	// frame: [max(0,pos-1), pos]
	// id=1: frame [0,0] -> fv=10
	// id=2: frame [0,1] -> fv=10
	// id=3: frame [1,2] -> fv=20
	// id=4: frame [2,3] -> fv=30
	expected := map[string]string{
		"1": "10",
		"2": "10",
		"3": "20",
		"4": "30",
	}
	for _, row := range result.Rows {
		id := *row[0]
		fv := *row[2]
		if exp, ok := expected[id]; ok {
			if fv != exp {
				t.Errorf("id=%s: expected fv=%s, got %s", id, exp, fv)
			}
		}
	}
}

func TestExecuteCountOverRowsFrame(t *testing.T) {
	header := []string{"id", "cat", "val"}
	rows := [][]string{
		{"1", "A", "10"},
		{"2", "A", "20"},
		{"3", "A", "30"},
		{"4", "B", "40"},
		{"5", "B", "50"},
	}

	// COUNT with ROWS BETWEEN 1 PRECEDING AND 1 FOLLOWING
	result, err := Execute("SELECT id, cat, COUNT(*) OVER (PARTITION BY cat ORDER BY id ROWS BETWEEN 1 PRECEDING AND 1 FOLLOWING) AS cnt FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 5 {
		t.Fatalf("expected 5 rows, got %d", result.RowCount)
	}
	// cat=A partition, sorted by id: 1,2,3
	// id=1: frame [0,1] -> count=2
	// id=2: frame [0,2] -> count=3
	// id=3: frame [1,2] -> count=2
	// cat=B partition, sorted by id: 4,5
	// id=4: frame [0,1] -> count=2
	// id=5: frame [0,1] -> count=2
	expected := map[string]string{
		"1": "2",
		"2": "3",
		"3": "2",
		"4": "2",
		"5": "2",
	}
	for _, row := range result.Rows {
		id := *row[0]
		cnt := *row[2]
		if exp, ok := expected[id]; ok {
			if cnt != exp {
				t.Errorf("id=%s: expected cnt=%s, got %s", id, exp, cnt)
			}
		}
	}
}

func TestExecuteRowFrameWithOffsetFollowingOnly(t *testing.T) {
	header := []string{"id", "val"}
	rows := [][]string{
		{"1", "100"},
		{"2", "200"},
		{"3", "300"},
		{"4", "400"},
	}

	// ROWS BETWEEN 2 PRECEDING AND 1 PRECEDING (offset range excluding current row)
	// At pos=0: start=max(0,-2)=0, end=max(0,-1)=0 -> [0,0] sum=100
	// At pos=1: start=max(0,-1)=0, end=max(0,0)=0 -> [0,0] sum=100
	// At pos=2: start=0, end=1          -> [0,1] sum=100+200=300
	// At pos=3: start=1, end=2          -> [1,2] sum=200+300=500
	result, err := Execute("SELECT id, val, SUM(CAST(val AS INTEGER)) OVER (ORDER BY id ROWS BETWEEN 2 PRECEDING AND 1 PRECEDING) AS sum_prev FROM t", header, rows, nil, 100)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.RowCount != 4 {
		t.Fatalf("expected 4 rows, got %d", result.RowCount)
	}
	expected := map[string]string{
		"1": "100",
		"2": "100",
		"3": "300",
		"4": "500",
	}
	for _, row := range result.Rows {
		id := *row[0]
		s := *row[2]
		if exp, ok := expected[id]; ok {
			if s != exp {
				t.Errorf("id=%s: expected sum=%s, got %s", id, exp, s)
			}
		}
	}
}

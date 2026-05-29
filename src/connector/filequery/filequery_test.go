package filequery

import (
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

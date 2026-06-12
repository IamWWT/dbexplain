package dsl

import (
	"fmt"
	"strings"
	"testing"

	"github.com/IamWWT/dbexplain/internal/dsn"
	"github.com/IamWWT/dbexplain/internal/config"
	"github.com/IamWWT/dbexplain/internal/sqlast"
)

// ── Preprocessor tests ──

func TestPreprocess_NoRefs(t *testing.T) {
	input := "SELECT * FROM users WHERE id = 1"
	out, refs, err := preprocess(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != input {
		t.Errorf("expected output %q, got %q", input, out)
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 refs, got %d", len(refs))
	}
}

func TestPreprocess_SingleRef(t *testing.T) {
	input := "SELECT * FROM @mydb.users"
	out, refs, err := preprocess(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == input {
		t.Errorf("expected output to differ from input")
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].Label != "mydb" {
		t.Errorf("expected label 'mydb', got %q", refs[0].Label)
	}
	if refs[0].Table != "users" {
		t.Errorf("expected table 'users', got %q", refs[0].Table)
	}
	// Verify placeholder looks right
	if refs[0].Placeholder == "" {
		t.Errorf("expected non-empty placeholder")
	}
}

func TestPreprocess_MultipleRefs(t *testing.T) {
	input := "SELECT u.name, o.total FROM @mydb.users u JOIN @other.orders o ON u.id = o.user_id"
	out, refs, err := preprocess(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == input {
		t.Errorf("expected output to differ from input")
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].Label != "mydb" || refs[0].Table != "users" {
		t.Errorf("ref[0] = %+v, want {mydb users}", refs[0])
	}
	if refs[1].Label != "other" || refs[1].Table != "orders" {
		t.Errorf("ref[1] = %+v, want {other orders}", refs[1])
	}
}

func TestPreprocess_DuplicateRef(t *testing.T) {
	input := "SELECT * FROM @mydb.users u JOIN @mydb.users v ON u.id = v.id"
	out, refs, err := preprocess(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == input {
		t.Errorf("expected output to differ from input")
	}
	// Same @mydb.users should only produce one ref and reuse same placeholder
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref (deduplicated), got %d", len(refs))
	}
}

func TestPreprocess_EmptyInput(t *testing.T) {
	out, refs, err := preprocess("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output, got %q", out)
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 refs, got %d", len(refs))
	}
}

func TestPreprocess_NoAtSign(t *testing.T) {
	input := "SELECT * FROM users"
	out, refs, err := preprocess(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != input {
		t.Errorf("expected %q, got %q", input, out)
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 refs, got %d", len(refs))
	}
}

// ── Parser tests ──

func TestParse_Simple(t *testing.T) {
	query, err := Parse("SELECT * FROM @mydb.users")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if query == nil {
		t.Fatal("expected non-nil query")
	}
	if query.Raw != "SELECT * FROM @mydb.users" {
		t.Errorf("Raw = %q", query.Raw)
	}
	if len(query.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(query.Sources))
	}
	ref := query.Sources["__dsl_0__"]
	if ref.Label != "mydb" || ref.Table != "users" {
		t.Errorf("unexpected ref: %+v", ref)
	}
	// Verify the placeholder is in the preprocessed SQL
	if query.SQL == query.Raw {
		t.Errorf("expected SQL to differ from Raw (should have placeholder)")
	}
}

func TestParse_WithJoin(t *testing.T) {
	query, err := Parse("SELECT * FROM @mydb.users u JOIN @other.orders o ON u.id = o.user_id")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(query.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(query.Sources))
	}
	stmt, ok := query.Stmt.(*sqlast.SelectStmt)
	if !ok {
		t.Fatalf("expected *SelectStmt, got %T", query.Stmt)
	}
	if stmt.From == "" {
		t.Error("expected non-empty From")
	}
	if len(stmt.Joins) != 1 {
		t.Errorf("expected 1 JOIN, got %d", len(stmt.Joins))
	}
}

func TestParse_PlainSQL(t *testing.T) {
	query, err := Parse("SELECT id, name FROM users WHERE age > 18")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(query.Sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(query.Sources))
	}
	if query.HasSourceRefs() {
		t.Error("HasSourceRefs should be false for plain SQL")
	}
}

func TestParse_Empty(t *testing.T) {
	_, err := Parse("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if _, ok := err.(*ErrSyntax); !ok {
		t.Errorf("expected *ErrSyntax, got %T", err)
	}
}

func TestParse_InvalidSQL(t *testing.T) {
	_, err := Parse("SLECT * FOM users")
	if err == nil {
		t.Fatal("expected error for invalid SQL")
	}
}

func TestParse_Union(t *testing.T) {
	query, err := Parse("SELECT * FROM @a.t1 UNION ALL SELECT * FROM @b.t2")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(query.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(query.Sources))
	}
	_, ok := query.Stmt.(*sqlast.UnionStmt)
	if !ok {
		t.Fatalf("expected *UnionStmt, got %T", query.Stmt)
	}
}

// ── Binder tests ──

func TestBind_Basic(t *testing.T) {
	query, err := Parse("SELECT * FROM @mydb.users")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	entries := []config.DSNEntry{
		{Raw: "mysql://user:pass@localhost:3306/testdb?label=mydb"},
		{Raw: "csv:///data/file.csv?label=other"},
	}

	bound, err := Bind(query, entries)
	if err != nil {
		t.Fatalf("Bind() error: %v", err)
	}
	if bound == nil {
		t.Fatal("expected non-nil bound query")
	}
	if len(bound.Sources) != 1 {
		t.Fatalf("expected 1 bound source, got %d", len(bound.Sources))
	}
	src := bound.Sources["__dsl_0__"]
	if src.DSN.Label != "mydb" {
		t.Errorf("expected label 'mydb', got %q", src.DSN.Label)
	}
	if src.Kind != SourceSQL {
		t.Errorf("expected SourceSQL, got %v", src.Kind)
	}
}

func TestBind_MultipleSources(t *testing.T) {
	query, err := Parse("SELECT * FROM @db1.users u JOIN @db2.orders o ON u.id = o.user_id")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	entries := []config.DSNEntry{
		{Raw: "postgres://user:pass@host1:5432/db1?label=db1"},
		{Raw: "postgres://user:pass@host2:5432/db2?label=db2"},
	}

	bound, err := Bind(query, entries)
	if err != nil {
		t.Fatalf("Bind() error: %v", err)
	}
	if len(bound.Sources) != 2 {
		t.Fatalf("expected 2 bound sources, got %d", len(bound.Sources))
	}
}

func TestBind_Unresolved(t *testing.T) {
	query, err := Parse("SELECT * FROM @unknown.table")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	entries := []config.DSNEntry{
		{Raw: "mysql://user:pass@host/db?label=mydb"},
	}

	_, err = Bind(query, entries)
	if err == nil {
		t.Fatal("expected error for unresolved reference")
	}
	if _, ok := err.(*ErrUnresolved); !ok {
		t.Errorf("expected *ErrUnresolved, got %T", err)
	}
}

func TestBind_EmptyEntries(t *testing.T) {
	query, err := Parse("SELECT * FROM @mydb.users")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	_, err = Bind(query, nil)
	if err == nil {
		t.Fatal("expected error for empty entries")
	}
}

func TestBind_FileSource(t *testing.T) {
	query, err := Parse("SELECT * FROM @data.staff")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	entries := []config.DSNEntry{
		{Raw: "csv:///data/staff.csv?label=data"},
	}

	bound, err := Bind(query, entries)
	if err != nil {
		t.Fatalf("Bind() error: %v", err)
	}
	if len(bound.Sources) != 1 {
		t.Fatalf("expected 1 bound source, got %d", len(bound.Sources))
	}
	src := bound.Sources["__dsl_0__"]
	if src.Kind != SourceFile {
		t.Errorf("expected SourceFile, got %v", src.Kind)
	}
}

func TestBind_NativeSource(t *testing.T) {
	query, err := Parse("SELECT * FROM @cache.data")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	entries := []config.DSNEntry{
		{Raw: "redis://localhost:6379/0?label=cache"},
	}

	bound, err := Bind(query, entries)
	if err != nil {
		t.Fatalf("Bind() error: %v", err)
	}
	src := bound.Sources["__dsl_0__"]
	if src.Kind != SourceNative {
		t.Errorf("expected SourceNative, got %v", src.Kind)
	}
}

func TestBind_NilQuery(t *testing.T) {
	_, err := Bind(nil, []config.DSNEntry{{Raw: "mysql://host/db?label=x"}})
	if err == nil {
		t.Fatal("expected error for nil query")
	}
}

// ── Integration: full parse + bind ──

func TestIntegration_ParseAndBind(t *testing.T) {
	input := "SELECT name, age FROM @mydb.staff WHERE age > 30 ORDER BY name"
	query, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	entries := []config.DSNEntry{
		{Raw: "postgres://user:pass@pg:5432/hr?label=mydb"},
	}

	bound, err := Bind(query, entries)
	if err != nil {
		t.Fatalf("Bind() error: %v", err)
	}

	// Verify the bound source
	src := bound.Sources["__dsl_0__"]
	if src.Ref.Label != "mydb" || src.Ref.Table != "staff" {
		t.Errorf("ref = %+v", src.Ref)
	}
	if src.DSN.Label != "mydb" {
		t.Errorf("dsn label = %q", src.DSN.Label)
	}
	if src.Kind != SourceSQL {
		t.Errorf("kind = %v", src.Kind)
	}

	// Verify the underlying AST is intact
	stmt, ok := query.Stmt.(*sqlast.SelectStmt)
	if !ok {
		t.Fatalf("expected *SelectStmt, got %T", query.Stmt)
	}
	if len(stmt.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(stmt.Columns))
	}
}

// ── classifySource unit tests ──

func TestClassifySource(t *testing.T) {
	tests := []struct {
		kind string
		want SourceKind
	}{
		{"mysql", SourceSQL},
		{"postgres", SourceSQL},
		{"sqlite", SourceSQL},
		{"clickhouse", SourceSQL},
		{"gaussdb", SourceSQL},
		{"csv", SourceFile},
		{"xlsx", SourceFile},
		{"redis", SourceNative},
		{"mongodb", SourceNative},
		{"qdrant", SourceNative},
		{"elasticsearch", SourceNative},
	}
	for _, tt := range tests {
		got := classifySource(tt.kind)
		if got != tt.want {
			t.Errorf("classifySource(%q) = %v, want %v", tt.kind, got, tt.want)
		}
	}
}

// Helper to verify DSN parsing works for bind tests.
func TestDSN(t *testing.T) {
	_, err := dsn.ParseDSN("mysql://user:pass@localhost:3306/testdb?label=mydb")
	if err != nil {
		t.Fatalf("ParseDSN error: %v", err)
	}
}

// ── CompileToSQL tests ──

func TestCompileToSQL_Basic(t *testing.T) {
	query, err := Parse("SELECT * FROM @mydb.users WHERE id = 1")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	entries := []config.DSNEntry{
		{Raw: "mysql://user:pass@host:3306/testdb?label=mydb"},
	}
	bound, err := Bind(query, entries)
	if err != nil {
		t.Fatalf("Bind() error: %v", err)
	}
	sql, err := CompileToSQL(query, bound)
	if err != nil {
		t.Fatalf("CompileToSQL() error: %v", err)
	}
	if !strings.Contains(sql, "users") {
		t.Errorf("expected compiled SQL to contain 'users', got %q", sql)
	}
	if strings.Contains(sql, "__dsl_") {
		t.Errorf("expected compiled SQL to have no placeholders, got %q", sql)
	}
}

func TestCompileToSQL_NoRefs(t *testing.T) {
	sql, err := CompileToSQL(&DSLQuery{Raw: "SELECT 1"}, &BoundQuery{})
	if err != nil {
		t.Fatalf("CompileToSQL() error: %v", err)
	}
	if sql != "SELECT 1" {
		t.Errorf("expected 'SELECT 1', got %q", sql)
	}
}

func TestCompileToSQL_NilQuery(t *testing.T) {
	_, err := CompileToSQL(nil, &BoundQuery{})
	if err == nil {
		t.Fatal("expected error for nil query")
	}
	if _, ok := err.(*ErrCompile); !ok {
		t.Errorf("expected *ErrCompile, got %T", err)
	}
}

func TestCompileToSQL_NilBound(t *testing.T) {
	_, err := CompileToSQL(&DSLQuery{Raw: "SELECT 1"}, nil)
	if err == nil {
		t.Fatal("expected error for nil bound")
	}
}

func TestCompileToSQL_NonSQLSource(t *testing.T) {
	query, err := Parse("SELECT * FROM @cache.data")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	entries := []config.DSNEntry{
		{Raw: "redis://localhost:6379/0?label=cache"},
	}
	bound, err := Bind(query, entries)
	if err != nil {
		t.Fatalf("Bind() error: %v", err)
	}
	_, err = CompileToSQL(query, bound)
	if err == nil {
		t.Fatal("expected error for non-SQL source")
	}
}

// ── BoundQuery utility tests ──

func TestSourceKinds_Single(t *testing.T) {
	bq := &BoundQuery{
		Sources: map[string]BoundSource{
			"p1": {Kind: SourceSQL},
		},
	}
	kinds := bq.SourceKinds()
	if len(kinds) != 1 || kinds[0] != SourceSQL {
		t.Errorf("expected [SourceSQL], got %v", kinds)
	}
}

func TestSourceKinds_Multiple(t *testing.T) {
	bq := &BoundQuery{
		Sources: map[string]BoundSource{
			"p1": {Kind: SourceSQL},
			"p2": {Kind: SourceFile},
		},
	}
	kinds := bq.SourceKinds()
	if len(kinds) != 2 {
		t.Errorf("expected 2 kinds, got %d", len(kinds))
	}
}

func TestPrimarySource(t *testing.T) {
	bq := &BoundQuery{
		Sources: map[string]BoundSource{
			"p1": {Kind: SourceSQL, Ref: SourceRef{Label: "a", Table: "b"}},
		},
	}
	ps := bq.PrimarySource()
	if ps == nil {
		t.Fatal("expected non-nil primary source")
	}
	if ps.Ref.Label != "a" {
		t.Errorf("expected label 'a', got %q", ps.Ref.Label)
	}
}

func TestPrimarySource_Empty(t *testing.T) {
	bq := &BoundQuery{}
	ps := bq.PrimarySource()
	if ps != nil {
		t.Errorf("expected nil for empty bound query, got %+v", ps)
	}
}

// ── Full pipeline end-to-end tests ──

func TestPipeline_Full(t *testing.T) {
	input := "SELECT name, age FROM @pg.staff WHERE age > 30 ORDER BY name"
	query, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	entries := []config.DSNEntry{
		{Raw: "postgres://user:pass@pg:5432/hr?label=pg"},
	}
	bound, err := Bind(query, entries)
	if err != nil {
		t.Fatalf("Bind() error: %v", err)
	}
	sql, err := CompileToSQL(query, bound)
	if err != nil {
		t.Fatalf("CompileToSQL() error: %v", err)
	}
	// Verify the compiled SQL is valid and has no placeholders
	if !strings.Contains(sql, "staff") {
		t.Errorf("expected SQL to contain 'staff', got %q", sql)
	}
	if strings.Contains(sql, "__dsl_") {
		t.Errorf("expected no placeholders in compiled SQL, got %q", sql)
	}
	// Verify AST is still intact after full pipeline
	stmt, ok := query.Stmt.(*sqlast.SelectStmt)
	if !ok {
		t.Fatalf("expected *SelectStmt, got %T", query.Stmt)
	}
	if len(stmt.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(stmt.Columns))
	}
}

func TestPipeline_FileSource(t *testing.T) {
	input := "SELECT * FROM @data.staff"
	query, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	entries := []config.DSNEntry{
		{Raw: "csv:///data/staff.csv?label=data"},
	}
	bound, err := Bind(query, entries)
	if err != nil {
		t.Fatalf("Bind() error: %v", err)
	}
	kinds := bound.SourceKinds()
	if len(kinds) != 1 || kinds[0] != SourceFile {
		t.Errorf("expected [SourceFile], got %v", kinds)
	}
	ps := bound.PrimarySource()
	if ps == nil || ps.Kind != SourceFile {
		t.Errorf("expected SourceFile primary source")
	}
	// CompileToSQL should fail for non-SQL sources
	_, err = CompileToSQL(query, bound)
	if err == nil {
		t.Error("expected CompileToSQL to fail for file source")
	}
}

func TestPipeline_Determinism(t *testing.T) {
	// Same input + same entries → same output every time
	input := "SELECT * FROM @mydb.users"
	entries := []config.DSNEntry{
		{Raw: "mysql://u:p@h/db?label=mydb"},
	}

	var results []string
	for i := 0; i < 5; i++ {
		query, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		bound, err := Bind(query, entries)
		if err != nil {
			t.Fatalf("Bind() error: %v", err)
		}
		sql, err := CompileToSQL(query, bound)
		if err != nil {
			t.Fatalf("CompileToSQL() error: %v", err)
		}
		results = append(results, sql)
	}
	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Errorf("run %d produced different result: %q vs %q", i, results[i], results[0])
		}
	}
}

// ── classifySource Prometheus ──

func TestClassifySource_Prometheus(t *testing.T) {
	if got := classifySource("prometheus"); got != SourceNative {
		t.Errorf("classifySource(\"prometheus\") = %v, want SourceNative", got)
	}
}

// ── classifyVendor tests ──

func TestClassifyVendor(t *testing.T) {
	tests := []struct {
		kind string
		want Vendor
	}{
		{"mysql", VendorSQL},
		{"postgres", VendorSQL},
		{"sqlite", VendorSQL},
		{"clickhouse", VendorSQL},
		{"gaussdb", VendorSQL},
		{"duckdb", VendorSQL},
		{"csv", VendorFile},
		{"xlsx", VendorFile},
		{"tsv", VendorFile},
		{"prometheus", VendorPromQL},
		{"redis", VendorSQL},
		{"mongodb", VendorSQL},
		{"qdrant", VendorSQL},
		{"elasticsearch", VendorSQL},
	}
	for _, tt := range tests {
		got := classifyVendor(tt.kind)
		if got != tt.want {
			t.Errorf("classifyVendor(%q) = %v, want %v", tt.kind, got, tt.want)
		}
	}
}

// ── SelectStmtToIR tests ──

func TestSelectStmtToIR_AllColumns(t *testing.T) {
	query, err := Parse("SELECT * FROM @prom.up")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	stmt, ok := query.Stmt.(*sqlast.SelectStmt)
	if !ok {
		t.Fatalf("expected *SelectStmt, got %T", query.Stmt)
	}

	ir, err := SelectStmtToIR(stmt)
	if err != nil {
		t.Fatalf("SelectStmtToIR() error: %v", err)
	}

	if !ir.AllColumns {
		t.Error("expected AllColumns=true")
	}
	if ir.From != "__dsl_0__" {
		t.Errorf("expected From=__dsl_0__, got %q", ir.From)
	}
}

func TestSelectStmtToIR_Where(t *testing.T) {
	query, err := Parse("SELECT * FROM @prom.up WHERE job = 'node' AND instance = 'local'")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	stmt, ok := query.Stmt.(*sqlast.SelectStmt)
	if !ok {
		t.Fatalf("expected *SelectStmt, got %T", query.Stmt)
	}

	ir, err := SelectStmtToIR(stmt)
	if err != nil {
		t.Fatalf("SelectStmtToIR() error: %v", err)
	}

	if len(ir.Where) != 2 {
		t.Fatalf("expected 2 WHERE conditions, got %d", len(ir.Where))
	}

	if ir.Where[0].Column != "job" || ir.Where[0].Op != "=" || ir.Where[0].Value != "node" || !ir.Where[0].IsStr {
		t.Errorf("unexpected Where[0]: %+v", ir.Where[0])
	}
	if ir.Where[1].Column != "instance" || ir.Where[1].Op != "=" || ir.Where[1].Value != "local" || !ir.Where[1].IsStr {
		t.Errorf("unexpected Where[1]: %+v", ir.Where[1])
	}
}

func TestSelectStmtToIR_RejectOR(t *testing.T) {
	query, err := Parse("SELECT * FROM @prom.up WHERE job = 'x' OR job = 'y'")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	stmt, ok := query.Stmt.(*sqlast.SelectStmt)
	if !ok {
		t.Fatalf("expected *SelectStmt, got %T", query.Stmt)
	}

	_, err = SelectStmtToIR(stmt)
	if err == nil || !strings.Contains(err.Error(), "OR") {
		t.Errorf("expected OR rejection error, got %v", err)
	}
}

func TestSelectStmtToIR_Joins(t *testing.T) {
	query, err := Parse("SELECT * FROM @prom.up JOIN @prom.down ON up.id = down.id")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	stmt, ok := query.Stmt.(*sqlast.SelectStmt)
	if !ok {
		t.Fatalf("expected *SelectStmt, got %T", query.Stmt)
	}

	ir, err := SelectStmtToIR(stmt)
	if err != nil {
		t.Fatalf("SelectStmtToIR() error: %v", err)
	}
	if !ir.HasJoins {
		t.Error("expected HasJoins=true")
	}
}

// ── CompileToPromQL tests ──

func parseAndBuildIR(dslInput string) (*QueryIR, error) {
	query, err := Parse(dslInput)
	if err != nil {
		return nil, err
	}
	stmt, ok := query.Stmt.(*sqlast.SelectStmt)
	if !ok {
		return nil, fmt.Errorf("not a SELECT")
	}
	return SelectStmtToIR(stmt)
}

func TestCompileToPromQL_Basic(t *testing.T) {
	ir, err := parseAndBuildIR("SELECT * FROM @prom.up")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ir.From = "up" // resolve placeholder

	promql, err := CompileToPromQL(ir)
	if err != nil {
		t.Fatalf("CompileToPromQL() error: %v", err)
	}
	if promql != "up" {
		t.Errorf("expected %q, got %q", "up", promql)
	}
}

func TestCompileToPromQL_Where(t *testing.T) {
	ir, err := parseAndBuildIR("SELECT * FROM @prom.up WHERE job='node'")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ir.From = "up"

	promql, err := CompileToPromQL(ir)
	if err != nil {
		t.Fatalf("CompileToPromQL() error: %v", err)
	}
	expected := `up{job="node"}`
	if promql != expected {
		t.Errorf("expected %q, got %q", expected, promql)
	}
}

func TestCompileToPromQL_WhereMulti(t *testing.T) {
	ir, err := parseAndBuildIR("SELECT * FROM @prom.up WHERE job='node' AND instance='local'")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ir.From = "up"

	promql, err := CompileToPromQL(ir)
	if err != nil {
		t.Fatalf("CompileToPromQL() error: %v", err)
	}
	expected := `up{job="node",instance="local"}`
	if promql != expected {
		t.Errorf("expected %q, got %q", expected, promql)
	}
}

func TestCompileToPromQL_WhereNotEq(t *testing.T) {
	ir, err := parseAndBuildIR("SELECT * FROM @prom.up WHERE job!='node'")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ir.From = "up"

	promql, err := CompileToPromQL(ir)
	if err != nil {
		t.Fatalf("CompileToPromQL() error: %v", err)
	}
	expected := `up{job!="node"}`
	if promql != expected {
		t.Errorf("expected %q, got %q", expected, promql)
	}
}

func TestCompileToPromQL_RejectCount(t *testing.T) {
	ir, err := parseAndBuildIR("SELECT COUNT(*) FROM @prom.up")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ir.From = "up"

	_, err = CompileToPromQL(ir)
	if err == nil || !strings.Contains(err.Error(), "COUNT") {
		t.Errorf("expected COUNT rejection error, got %v", err)
	}
}

func TestCompileToPromQL_RejectGroupByNoAgg(t *testing.T) {
	ir, err := parseAndBuildIR("SELECT * FROM @prom.up GROUP BY job")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ir.From = "up"

	_, err = CompileToPromQL(ir)
	if err == nil || !strings.Contains(err.Error(), "GROUP BY") {
		t.Errorf("expected GROUP BY rejection error (no aggregation), got %v", err)
	}
}

func TestCompileToPromQL_GroupByAgg(t *testing.T) {
	ir, err := parseAndBuildIR("SELECT job, count(value) FROM @prom.up GROUP BY job")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ir.From = "up"

	promQL, err := CompileToPromQL(ir)
	if err != nil {
		t.Fatalf("unexpected error for GROUP BY + COUNT: %v", err)
	}
	expected := "count by (job) (up)"
	if promQL != expected {
		t.Errorf("expected %q, got %q", expected, promQL)
	}
}

func TestCompileToPromQL_RejectJoin(t *testing.T) {
	ir, err := parseAndBuildIR("SELECT * FROM @prom.up JOIN @prom.down ON up.id=down.id")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ir.From = "up"

	_, err = CompileToPromQL(ir)
	if err == nil || !strings.Contains(err.Error(), "JOIN") {
		t.Errorf("expected JOIN rejection error, got %v", err)
	}
}

func TestCompileToPromQL_AcceptsOrderBy(t *testing.T) {
	ir, err := parseAndBuildIR("SELECT * FROM @prom.up ORDER BY val")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ir.From = "up"

	// ORDER BY is now accepted; post-processing handled by dslExecPromQL
	promQL, err := CompileToPromQL(ir)
	if err != nil {
		t.Fatalf("unexpected error for ORDER BY: %v", err)
	}
	if promQL != "up" {
		t.Errorf("expected PromQL 'up', got %q", promQL)
	}
}

func TestCompileToPromQL_RejectNumericWhere(t *testing.T) {
	ir, err := parseAndBuildIR("SELECT * FROM @prom.up WHERE value > 0")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ir.From = "up"

	_, err = CompileToPromQL(ir)
	if err == nil || !strings.Contains(err.Error(), "numeric WHERE") {
		t.Errorf("expected numeric WHERE rejection error, got %v", err)
	}
}

func TestCompileToPromQL_NilIR(t *testing.T) {
	_, err := CompileToPromQL(nil)
	if err == nil || !strings.Contains(err.Error(), "nil") {
		t.Errorf("expected nil IR error, got %v", err)
	}
}

#!/bin/bash
# ──────────────────────────────────────────────────────────────
#  dbexplain — 闭环验证测试脚本
#
#  用法: bash docs/test/test-runner.sh [variant-name]
#
#  在安装 dbexplain 后运行，测试全局二进制功能完整性。
#  测试覆盖 L1-L8 全分层 + 新增特性 + E2E 外部数据库。
#
#  示例:
#    bash docs/test/test-runner.sh "std-noupx"     # 标准版无 UPX
#    bash docs/test/test-runner.sh "std-upx"       # 标准版 UPX 压缩
#    bash docs/test/test-runner.sh "duckdb-noupx"  # DuckDB 版无 UPX
#    bash docs/test/test-runner.sh "duckdb-upx"    # DuckDB 版 UPX 压缩
#
#  环境要求:
#    - $HOME/.local/bin/dbexplain 或 PATH 中的 dbexplain
#    - src/.env.dbexplain (用于 E2E 外部数据库测试)
#    - /tmp/dbexplain-test/ 下的测试数据文件
# ──────────────────────────────────────────────────────────────

set -e

VARIANT="${1:-unknown}"
FAIL_COUNT=0; PASS_COUNT=0; SKIP_COUNT=0

PROJ_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$PROJ_ROOT/src"

export PATH="$HOME/.local/bin:$PATH"
BIN="dbexplain"
export DBPROBE_ENV_FILE=".env.dbexplain"

echo "================================================"
echo "  dbexplain ${VARIANT} — Full Test Suite"
echo "================================================"
echo ""

pass() { PASS_COUNT=$((PASS_COUNT+1)); echo "  PASS: $1"; }
fail() { FAIL_COUNT=$((FAIL_COUNT+1)); echo "  FAIL: $1"; }
skip() { SKIP_COUNT=$((SKIP_COUNT+1)); echo "  SKIP: $1"; }

# ── Helper: check grep pattern in command output ──
check_grep() {
  local d="$1" p="$2"; shift 2
  if "$@" 2>&1 | grep -q "$p"; then pass "$d"; else fail "$d (missing: $p)"; fi
}

# ── Helper: check command succeeds ──
check_ok() {
  local d="$1"; shift
  if "$@" >/dev/null 2>&1; then pass "$d"; else fail "$d"; fi
}

# ════════════════════════════════════════════════════════════
# L1: Environment & Static Analysis
# ════════════════════════════════════════════════════════════
echo "─── L1: Static Analysis ───"
check_grep "go version 1.26+" "go1.26" go version
check_ok  "go build ./..." go build ./...
check_ok  "go vet ./..."   go vet ./...

# ════════════════════════════════════════════════════════════
# L2: Unit Tests
# ════════════════════════════════════════════════════════════
echo ""; echo "─── L2: Unit Tests ───"
UNIT=$(go test ./... -count=1 2>&1) && pass "All unit tests pass" || {
  fail "Unit test failures"
  echo "$UNIT" | grep -E "FAIL|--- FAIL" | head -10
}

# ════════════════════════════════════════════════════════════
# L3: CLI Help (global binary)
# ════════════════════════════════════════════════════════════
echo ""; echo "─── L3: CLI Help ───"
check_grep "--version" "v0.1.7"                         $BIN --version
check_grep "-h help" "csv\|tsv\|xlsx"                    $BIN -h 2>&1
check_grep "all manual" "dbexplain"                      env PAGER="" $BIN all 2>&1
check_grep "manual EN" "Usage\|NAME"                     env PAGER="" $BIN all --language en 2>&1
check_grep "manual --filter" "Redis\|redis"              env PAGER="" $BIN all --filter redis 2>&1
for db in mysql postgres clickhouse sqlite redis mongodb elasticsearch qdrant csv tsv xlsx oracle hive; do
  check_grep "$db (v0.1.7)" "v0.1.7" $BIN "$db" 2>&1; done
for alias in pg postgresql ch sqlite3 es; do
  check_grep "$alias (v0.1.7)" "v0.1.7" $BIN "$alias" 2>&1; done
for cmd in execute list encrypt collect repl; do
  check_grep "$cmd -h" "Usage" $BIN "$cmd" -h 2>&1; done

# ════════════════════════════════════════════════════════════
# L4: Schema Collection (File DSNs — no external DB needed)
# ════════════════════════════════════════════════════════════
echo ""; echo "─── L4: Schema Collection ───"
check_grep "CSV schema" "csv-users" $BIN -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" --json 2>/dev/null
check_grep "TSV schema" "tsv-test" $BIN -dsn "tsv:///tmp/dbexplain-test/data.tsv?label=tsv-test" --json 2>/dev/null

# ════════════════════════════════════════════════════════════
# L5: Security (sqlguard via DSN)
# ════════════════════════════════════════════════════════════
echo ""; echo "─── L5: Security ───"
check_grep "empty query" "missing SQL query\|READ_ONLY_VIOLATION\|QUERY_ERROR" $BIN execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=t" "" 2>&1
check_grep "DROP blocked" "READ_ONLY_VIOLATION\|QUERY_ERROR" $BIN execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=t" "DROP TABLE t" 2>&1

# ════════════════════════════════════════════════════════════
# L6: File Processing (CSV/TSV)
# ════════════════════════════════════════════════════════════
echo ""; echo "─── L6: File Processing ───"
check_grep "CSV SELECT *" "row_count" $BIN execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT *" --json 2>/dev/null
check_grep "CSV LIMIT 1" "row_count"  $BIN execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT * LIMIT 1" --json 2>/dev/null
check_grep "TSV SELECT *" "row_count" $BIN execute -dsn "tsv:///tmp/dbexplain-test/data.tsv?label=tsv-test" "SELECT *" --json 2>/dev/null
check_grep "TSV WHERE" "row_count"    $BIN execute -dsn "tsv:///tmp/dbexplain-test/data.tsv?label=tsv-test" "SELECT * FROM data WHERE name = 'Alice'" --json 2>/dev/null
check_grep "TSV ORDER BY" "row_count" $BIN execute -dsn "tsv:///tmp/dbexplain-test/data.tsv?label=tsv-test" "SELECT * FROM data ORDER BY value DESC LIMIT 2" --json 2>/dev/null
check_grep "CSV GROUP BY" "row_count" $BIN execute -dsn "csv:///tmp/dbexplain-test/users.csv?label=csv-users" "SELECT org_scope_type, COUNT(*) AS cnt FROM users GROUP BY org_scope_type LIMIT 5" --json 2>/dev/null

# ════════════════════════════════════════════════════════════
# New Feature 1: TSV Kind fix
# ════════════════════════════════════════════════════════════
echo ""; echo "─── TSV Kind ───"
K=$($BIN -dsn "tsv:///tmp/dbexplain-test/data.tsv?label=tsv-test" --json 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin)['instances'][0]['kind'])" 2>/dev/null)
[ "$K" = "tsv" ] && pass "TSV kind is 'tsv'" || fail "TSV kind expected 'tsv', got: $K"

# ════════════════════════════════════════════════════════════
# New Feature 2: REPL
# ════════════════════════════════════════════════════════════
echo ""; echo "─── REPL ───"
check_grep "REPL starts" "REPL" $BIN repl --dsn "sqlite:////tmp/test_repl.db?label=test" <<< "" 2>&1
HELP=$(printf '.help\n.exit\n' | $BIN repl --dsn "sqlite:////tmp/test_repl.db?label=test" 2>&1)
check_grep ".help commands" ".connect" echo "$HELP"
check_grep ".help .list"    ".list"    echo "$HELP"
check_grep ".help .exit"   ".exit"    echo "$HELP"
check_grep ".exit" "Goodbye"  $BIN repl --dsn "sqlite:////tmp/test_repl.db?label=test" <<< ".exit" 2>&1
check_grep ".quit" "Goodbye"  $BIN repl --dsn "sqlite:////tmp/test_repl.db?label=test" <<< ".quit" 2>&1
check_grep "unknown cmd" "Unknown" $BIN repl --dsn "sqlite:////tmp/test_repl.db?label=test" <<< ".unknown" 2>&1
check_grep "SQL query" "x"       $BIN repl --dsn "sqlite:////tmp/test_repl.db?label=test" <<< "SELECT 1 AS x" 2>&1
check_grep ".list" "test"       $BIN repl --dsn "sqlite:////tmp/test_repl.db?label=test" <<< ".list" 2>&1

# ════════════════════════════════════════════════════════════
# New Feature 3: Hash Index
# ════════════════════════════════════════════════════════════
echo ""; echo "─── Hash Index ───"
check_grep "WHERE col='val'" "row_count" $BIN execute -dsn "tsv:///tmp/dbexplain-test/data.tsv?label=tsv-test" "SELECT * FROM data WHERE name = 'Bob'" --json 2>/dev/null
check_grep "WHERE num=val"   "row_count" $BIN execute -dsn "tsv:///tmp/dbexplain-test/data.tsv?label=tsv-test" "SELECT * FROM data WHERE id = 1" --json 2>/dev/null

# ════════════════════════════════════════════════════════════
# L7: Version Consistency & Security
# ════════════════════════════════════════════════════════════
echo ""; echo "─── L7: Git Security ───"
cd "$PROJ_ROOT"
GIT_ENV=$(git ls-files '*.env' 2>/dev/null | wc -l)
GIT_ENC=$(git ls-files '*.enc' 2>/dev/null | wc -l)
TMPL=$(git ls-files '*.env' 2>/dev/null | grep -c "example\|sample" || true)
[ $((GIT_ENV - TMPL)) -eq 0 ] && pass "No real .env in git" || fail ".env in git ($GIT_ENV total, $TMPL templates)"
[ "$GIT_ENC" -eq 0 ] && pass "No .enc in git" || fail ".enc in git"

check_ok "install.sh syntax"       bash -n dbexplain-skill/scripts/install.sh
check_ok "uninstall.sh syntax"     bash -n dbexplain-skill/scripts/uninstall.sh
check_ok "install-skill.sh syntax" bash -n dbexplain-skill/scripts/install-skill.sh

# ════════════════════════════════════════════════════════════
# L8: Capability Routing
# ════════════════════════════════════════════════════════════
echo ""; echo "─── L8: Capability Routing ───"
check_grep "csv ref" "csv"  $BIN csv 2>&1
check_grep "xlsx ref" "xlsx" $BIN xlsx 2>&1

# ════════════════════════════════════════════════════════════
# L0: Binary Verification
# ════════════════════════════════════════════════════════════
echo ""; echo "─── L0: Binary Verification ───"
BIN_PATH=$(which $BIN); check_ok "binary in PATH" test -f "$BIN_PATH"
if echo "$VARIANT" | grep -q "std"; then
  check_grep "statically linked" "statically linked" file "$BIN_PATH"
  LDD=$(ldd "$BIN_PATH" 2>&1 || true)
  check_grep "no dynamic deps" "not a dynamic\|不是动态\|statically linked" echo "$LDD"
fi

# ════════════════════════════════════════════════════════════
# E2E: External DB Tests (requires .env.dbexplain)
# ════════════════════════════════════════════════════════════
echo ""; echo "─── E2E: External DB Tests ───"
cd "$PROJ_ROOT/src"
SCHEMA=$(timeout 60 $BIN --json 2>/dev/null) || true
CNT=$(echo "$SCHEMA" | python3 -c "import json,sys;d=json.load(sys.stdin);print(len(d.get('instances',[])))" 2>/dev/null || echo "0")
[ "$CNT" -gt 0 ] && pass "Schema collection: $CNT instances" || fail "No schema collected"
check_grep "MySQL SELECT 1" "row_count\|test"  $BIN execute --db 1 "SELECT 1 AS test" --json 2>/dev/null
check_grep "SQLite SELECT 1" "row_count\|val"  $BIN execute --db 3 "SELECT 1 AS val" --json 2>/dev/null
check_grep "Redis PING" "PONG\|row_count"     $BIN execute --db 7 "PING" --json 2>/dev/null
R=$(printf '.conn aiops-mysql\nSELECT 1 AS x\n.exit\n' | $BIN repl 2>&1) || true
check_grep "REPL .conn switch" "Switched to\|x" echo "$R"

# ════════════════════════════════════════════════════════════
# Summary
# ════════════════════════════════════════════════════════════
echo ""
echo "================================================"
echo "  ${VARIANT} Test Summary"
echo "  Passed: ${PASS_COUNT}"
echo "  Failed: ${FAIL_COUNT}"
echo "  Skip:   ${SKIP_COUNT}"
echo "  Total:  $((PASS_COUNT + FAIL_COUNT + SKIP_COUNT))"
echo "================================================"
exit $FAIL_COUNT

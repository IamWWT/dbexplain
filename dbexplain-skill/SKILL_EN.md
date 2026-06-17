name: dbexplain-skill
description: >
  Use this skill when you need to explore database schemas, analyze cross-DB relationships,
  execute read-only queries, or check database health.
  Supports 16+ data sources (MySQL/PG/ClickHouse/Redis/MongoDB/ES/Prometheus/CSV/etc).
  Input: DSN connection string or .env.dbexplain config file (auto-discovered). Output: table structures/field types/health score/topology JSON or table.
user-invocable: true
trigger:
  - "explain table structure"
  - "analyze database relationships"
  - "cross-DB dependencies"
  - "cross-DB query"
  - "generate ER diagram"
  - "understand database schema"
  - "database health check"
  - "database inspection"
  - "execute read-only query"
  - "check table structure"
  - "data source overview"
  - "database troubleshooting"
  - "schema analysis"
  - "field query"
  - "data quality check"
  - "connectivity check"
---
## 1. Tool Overview

`dbexplain` is a Go binary CLI installed to system PATH. Two independent modes:

- **Schema Collection** (`dbexplain` or `dbexplain collect`): inspects table structures/field types/comments/cross-DB foreign keys/health score, outputs `instances[]` + `refs[]` (JSON)
- **Read-Only Query** (`dbexplain execute`): runs read-only SELECT on SQL/file/Mongo/Redis datasources after collection, outputs `columns[]` + `rows[]`

Also supports: incremental change detection (`--cache`), DSL mode (`--dsl`), config encryption (`encrypt`), connectivity check (`check`).

## 2. Input Definition

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| DSN config | file path or string | ✅ | `.env.dbexplain` (auto-discovered), or direct DSN via `-dsn 'scheme://...'` |
| Query | string | on-demand | SELECT query for `execute` mode. Omit for schema-only collection |
| `--human` | flag | ❌ | Table output (default JSON), for user-facing display |
| `--label name` | string | ❌ | execute: select DSN by label; collect: alias for `-include` (filter by kind/label) |
| `--db N` | int | ❌ | execute: select DSN by numeric index (DB1=1, DB2=2) |

> If any parameter is missing, the Agent must ask the user — never guess default values.

## 3. Core Principles

- **Read-Only Safe**: Only SELECT/SHOW/SCAN. Never writes data to any database.
- **Privacy Protection**: Agent must **never** view/log/ask for plaintext passwords. Let users configure in `.env`; tool auto-redacts.
- **Boundary**: Agent only invokes commands. **Never** create/modify/read user config file content.
- **Config Not Found**: `findConfigFile()` searches 7 priority paths. If none found → prints "No config file found" → exit 1. Guide user to create `.env.dbexplain` or use `-dsn` directly.
- **Bare Command Behavior**: `dbexplain` with no args prints help manual (not an error). Same for `dbexplain collect` with no DSN.
- **Error Handling**: On command failure, report the error message verbatim. **Never guess** alternative parameters or pretend a query succeeded. When `--label` is missing with multiple datasources, guide the user to add it.

## 4. User Intent → Command Quick Reference

User requests are non-linear. Map intent to first command. **P0 scenarios (most common) first:**

### P0 — Inspection / Schema / Connectivity

| User says | AI executes | Reference |
|-----------|-------------|-----------|
| "inspect DB / find issues" | `dbexplain --context ./ctx` → **read diagnostics.json** issues[] | §5.4 |
| "show table structure / what tables" | `dbexplain collect` (default) or `collect --tables` (compact) | §5.4 |
| "can't connect / check connection" | `dbexplain check` to verify connectivity | §5.3 |
| "how many tables / overview" | `dbexplain collect --tables` (compact list) | §5.4 |
| "data source overview" | `dbexplain list` to see available DSNs | §5.3 |

### P1 — Query / Statistics / Topology

| User says | AI executes | Reference |
|-----------|-------------|-----------|
| "query data / statistics" | **Collect first** to understand fields, then execute query | §5.5 |
| "count last month's orders" | collect fields → `SELECT COUNT(*) FROM t WHERE month = ?` | §5.5 |
| "cross-DB topology" | `dbexplain --context ./ctx` → read **topology.json** refs[] | §5.4 |
| "data quality check" | `dbexplain --context ./ctx` → read **diagnostics.json** | §5.4 |

### P2 — Advanced Scenarios

| User says | AI executes | Reference |
|-----------|-------------|-----------|
| "what changed since last week" | `dbexplain diff --cache cache.json --since v1.0 --human` | §5.8 |
| "cross-DB federated query" | `dbexplain execute --dsl 'SELECT * FROM @a.t JOIN @b.t ON ...'` | §5.6 |
| "analyze CSV/XLSX file" | `dbexplain execute -dsn 'csv:///path/file.csv?label=d' 'SELECT ...'` | §5.7 |

> Running bare `dbexplain` (no args) shows the help manual, not an error.

## 5. Standard Workflow

### Before you start — ALWAYS ask
1. Confirm whether user has a `.env.dbexplain` config file
2. Confirm target datasource type (SQL / NoSQL / File)
3. Confirm analysis goal (Schema preview / data query / health check / connectivity)

### 5.1 Verify Installation + Get Help

```bash
dbexplain --version                  # check if installed
dbexplain all                        # view full help manual
dbexplain all --filter execute       # filter help by keyword
```
If not installed → run `bash dbexplain-skill/scripts/install.sh` (from project root).

### 5.2 Configure DB Connection

Ask the user: do you already have a `.env.dbexplain` config file?

- **Yes** (recommended) → proceed directly. `.env.dbexplain` is auto-discovered, no flags needed.
- **No, but have connection info** → guide user to create `~/.config/dbexplain/.env.dbexplain`. Agent **must not** edit this file — tell the path and format, wait for user.
- **Direct DSN** → `dbexplain -dsn 'scheme://u:pass@host:port/db?label=x'`

### 5.3 Connectivity Check (P0 — do before collection when user says "can't connect")

```bash
dbexplain check                        # verify all DSN connectivity
dbexplain check --label mysql          # check specific datasource only
dbexplain list                         # list available DSNs
```
→ verify: exit code 0, no ACCESS_DENIED or dial tcp errors
Fail → guide user to check DSN config or network connectivity

### 5.4 Schema Collection + DB Inspection

```bash
# Recommended: output AI context directory
dbexplain --context ./ctx
```

#### --context output file consumption

| File | Content | AI Usage |
|------|---------|----------|
| `summary.json` | All DSN instances with table_count, field_count, health_score | Overview: how many DBs, tables, health scores |
| `topology.json` | refs[] cross-DB foreign key relationships | Cross-DB dependency analysis, ER diagrams |
| `diagnostics.json` | issues[] problem list (tables without PK, abnormal field types, etc.) | Database inspection conclusions |
| `chunks/*.json` | Chunked context (per table/DB) | Load into Agent context on demand |

→ verify: summary.json contains all DSN results, instances non-empty

#### collect common parameters

| Parameter | Purpose |
|-----------|---------|
| `--tables` | Compact table list mode (name/engine/row_count only) |
| `--sample` | Sample row fetching for comment inference (default: off) |
| `--conn N` | Max concurrent connections (default: 10) |
| `--timeout 30s` | Per-DSN timeout (default: 20s) |
| `-include mysql,postgres` | Collect only specified types |
| `-exclude redis` | Exclude specified types |

Incremental change detection: add `--cache schema.cache` on first run; subsequent runs output `changes` field.

### 5.5 Execute Read-Only Query

Collect schema first to understand field meanings. Agent constructs queries — no user SQL needed.

```bash
# SQL databases
dbexplain execute --label mysql 'SELECT COUNT(*) FROM orders' --human

# MongoDB (JSON format)
dbexplain execute --label mongo '{"find":"users","filter":{"age":{"$gt":18}}}' --human

# Redis (native commands)
dbexplain execute --label redis 'GET user:1001' --human
```
→ verify: returns columns + rows, row_count > 0, no ACCESS_DENIED error

Auto LIMIT 1000. Explicitly rejects DROP/INSERT/UPDATE/DELETE.

NoSQL commands at [`references/nosql-commands.md`](references/nosql-commands.md), Prometheus queries at [`references/prometheus-queries.md`](references/prometheus-queries.md), full examples at [`references/examples.md`](references/examples.md).

### 5.6 DSL Mode (v0.1.1+)

```bash
dbexplain execute --dsl --label mysql 'SELECT * FROM @mysql.users WHERE status = "active"'
```
Deterministic compilation pipeline. Full DSL syntax at [`references/dsl-syntax.md`](references/dsl-syntax.md).

### 5.7 File Query Engine

CSV/XLSX files support a full SELECT subset. **Full syntax** at [`references/sql-syntax.md`](references/sql-syntax.md).

```bash
# Aggregate analysis
dbexplain execute --label my_data \
  'SELECT department, AVG(rate) AS avg_rate FROM data GROUP BY department ORDER BY avg_rate DESC' --human

# Cross-file JOIN
dbexplain execute --label my_data \
  'SELECT o.branch_name, AVG(t.rate) FROM data t JOIN org o ON t.dept_id = o.dept_id GROUP BY o.branch_name' --human
```
→ verify: aggregation includes all groups, nulls handled

> **`SELECT *` is for preview only — never use it as the source for analytical conclusions.** All statistics must be computed via aggregate queries.

**Best practices**:
- Clarify vague requests first (summary vs detail? group comparison?), ask 2-3 key questions
- Always use explicit WHERE clauses — never rely on implicit context
- Default JSON (Agent analysis), add `--human` for terminal tables

### 5.8 Schema Diff (P2 — when user asks "what changed")

```bash
# First run: establish baseline cache
dbexplain --cache schema.cache
# Subsequent: compare changes
dbexplain diff --cache schema.cache --since v1.0 --human
```
→ verify: output changes[], contains added/removed/modified field-level diffs

Full Diff syntax at `references/dsl-syntax.md`.

## 6. Boundaries & Security Policy

### boundaries
- ❌ Never execute DROP/INSERT/UPDATE/DELETE
- ❌ Never retry queries blocked by security policy
- ❌ Never read or modify user config files
- ✅ On connection failure, report the specific error verbatim

### Security Policy (on `ACCESS_DENIED`)

Admins configure sensitive data protection in `.env`. Agent **must not bypass** — inform the user.

```env
DENY_TABLES=sensitive,audit_log               # table-level block
DENY_COLUMNS=users.password_hash               # column-level block (must be table.column format)
MASK_COLUMNS=email=REDACTED,card_no=****       # non-blocking, replace column values on output
```

| Output | Meaning | Agent Action |
|--------|---------|--------------|
| `ACCESS_DENIED: table "xxx"` | Table is denied | Try a different table |
| `ACCESS_DENIED: column "xxx"` | Column is denied | Remove that column and retry |
| `READ_ONLY_VIOLATION` | Write operation | Change to SELECT |
| `CONCURRENT_LIMIT` | Concurrency conflict | Retry later |
| `QUERY_ERROR: ...` | Connection or SQL error | Fix DSN or SQL |

## 7. Traceable Analysis

### Never Fabricate Data

**Every number in your analysis output must come from an actual SQL query result.** Prohibited:
- **Fabricated statistics**: AVG/MAX/MIN/COUNT must be computed — never estimated from SELECT *
- **Fabricated features**: Do not claim results for unsupported functions (window functions, STDDEV, median). Report engine errors honestly
- **Sorting errors**: Ranking tables must be strictly ordered
- **Concept confusion**: Distinguish row counts (`COUNT(*)`) from time spans (`COUNT(DISTINCT date_col)`)

> **Golden rule**: Whatever number you need, write the SQL to compute it.

### Citation Rules

- Append source SQL and row count to quantitative conclusions
- **Cite per conclusion** — never write "all data from query XXX" at the end
- Cite query results verbatim, do not summarize into different numbers

### Good Example

> Department A has the highest average completion rate at 95.2%; Department B at 82.1%.
> Source: `SELECT department, AVG(completion_rate) AS avg_rate FROM sales_data GROUP BY department ORDER BY avg_rate DESC` (6 rows)

### Bad Examples

> Department A performs best. ← ✗ No source, no numbers
> All data from SELECT * query. ← ✗ Vague attribution

### eval
- Schema collection: all DSNs return non-empty instances → ✅
- Read-only query: returns columns + rows, row_count > 0 → ✅
- Security: DROP/INSERT/UPDATE explicitly rejected → ✅
- Analysis: every number has source SQL annotation → ✅
- Any failure → mark degraded

### fallback
- [DSN connection failed] → report exact error, do not guess
- [Empty query result] → verify WHERE clause, try different table
- [Query timeout] → suggest `--timeout` or simplify query
- [ACCESS_DENIED] → try different table / remove denied column

## 8. Notes

- **DSN with special chars**: wrap in **single quotes** on CLI; no escaping needed in `.env`
- **MongoDB**: DSN must include `authSource` (e.g. `?authSource=admin`) and specify database name
- **Config encryption**: user runs `dbexplain encrypt` (machine fingerprint). **Delete plaintext file after encryption**. Agent must never participate in encryption
- **ES query limitation**: array fields not supported, select scalar fields only
- **ClickHouse**: do not add semicolons to queries, otherwise flagged as multi-statement
- **Full help**: `dbexplain all` (supports `--filter keyword`)
- **Troubleshooting**: DB connection/file query errors at [`references/troubleshooting.md`](references/troubleshooting.md)

name: dbexplain-skill
description: >
  Use this skill when you need to explore database schemas, analyze cross-DB relationships,
  execute read-only queries, or check database health.
  Input: DSN connection string or .env config file. Output: table structures/field types/health score JSON or table.
user-invocable: true
trigger:
  - "explain table structure"
  - "analyze database relationships"
  - "cross-DB dependencies"
  - "generate ER diagram"
  - "understand database schema"
  - "database health check"
  - "execute read-only query"
---
## 1. Tool Overview

`dbexplain` is a Go binary CLI installed to system PATH. Two independent modes:

- **Schema Collection** (`dbexplain` auto-load): inspects table structures/field types/comments/cross-DB foreign keys/health score, outputs `instances[]` + `refs[]` (JSON)
- **Read-Only Query** (`dbexplain execute`): runs read-only SELECT on SQL/file/Mongo/Redis datasources after collection, outputs `columns[]` + `rows[]`

Also supports: incremental change detection (`--cache`), DSL mode (`--dsl`), config encryption (`encrypt`), help manual (`dbexplain all`).

## 2. Input Definition

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| DSN config | file path or string | ✅ | `.env` file path (auto-discovered), or direct DSN via `-dsn 'scheme://...'` |
| Query | string | on-demand | SELECT query for `execute` mode. Omit for schema-only collection |
| `--human` | flag | ❌ | Table output (default JSON), for user-facing display |
| `--label/--db N` | string/int | ❌ | Select specific datasource when multiple are configured |

> If any parameter is missing, the Agent must ask the user — never guess default values.

## 3. Core Principles

- **Read-Only Safe**: Only SELECT/SHOW/SCAN. Never writes data to any database.
- **Privacy Protection**: Agent must **never** view/log/ask for plaintext passwords. Let users configure in `.env`; tool auto-redacts.
- **Boundary**: Agent only invokes commands. **Never** create/modify/read user config file content.
- **Global PATH**: `dbexplain` is in system PATH, callable from any directory.
- **Error Handling**: On command failure, report the error message verbatim. **Never guess** alternative parameters or pretend a query succeeded. When `--label` is missing with multiple datasources, guide the user to add it.

## 4. Standard Workflow

### 4.1 Verify Installation + Get Help

```bash
dbexplain --version                  # check if installed
dbexplain all                        # view full help manual
dbexplain all --filter execute       # filter help by keyword
```
If not installed → tell user to run `bash scripts/install.sh` (from project root).

### 4.2 Configure DB Connection

Ask the user: do you already have a `.env` config file?

- **Yes** (recommended) → proceed directly.
- **No, but have connection info** → guide user to create `~/.config/dbexplain/.env.dbexplain`. Agent **must not** edit this file — tell the path and format, wait for user.
- **Direct DSN** → `dbexplain -dsn 'scheme://u:pass@host:port/db?label=x'`

### 4.3 Collect Schema

```bash
# Collect all
dbexplain

# Output AI context directory (recommended)
dbexplain --context ./ctx
```

`--context ./ctx` generated files:

| File | Usage |
|------|-------|
| `ctx/summary.json` | DB list, table count, total fields, health score |
| `ctx/topology.json` | Cross-DB foreign keys, references, data flow |
| `ctx/diagnostics.json` | Missing indexes, field type anomalies, missing comments |
| `ctx/chunks/*.json` | Detailed structure per table (field name/type/comment) |

Incremental change detection: add `--cache schema.cache` on first run; subsequent runs output `changes` field (added/deleted/changed tables and fields). Suitable for periodic inspection.

Filter by type: `-include mysql,postgres` / `-exclude redis`.

### 4.4 Execute Read-Only Query

Collect schema first to understand field meanings, then query to verify data. Agent constructs queries — no user SQL needed.

```bash
# SQL databases
dbexplain execute --label mysql 'SELECT COUNT(*) FROM orders' --human

# MongoDB (JSON format)
dbexplain execute --label mongo '{"find":"users","filter":{"age":{"$gt":18}}}' --human

# Redis (native commands)
dbexplain execute --label redis 'GET user:1001' --human
```

Auto LIMIT 1000. Explicitly rejects DROP/INSERT/UPDATE/DELETE.

### 4.5 DSL Mode (v0.1.1+)

`--dsl` enables DSL mode with `@label.table` syntax:

```bash
dbexplain execute --dsl --label mysql 'SELECT * FROM @mysql.users WHERE status = "active"'
```

Fully deterministic compilation pipeline. See `dbexplain all --filter dsl` for details.

### 4.6 File Query Engine

CSV/XLSX files support a full SELECT subset. **Full syntax** at [`references/sql-syntax.md`](references/sql-syntax.md).

```bash
# Data preview (preview only — never use as analysis source)
dbexplain execute --label my_data 'SELECT *' --limit 5 --human

# Aggregate analysis
dbexplain execute --label my_data \
  'SELECT department, AVG(rate) AS avg_rate FROM data GROUP BY department ORDER BY avg_rate DESC' --human

# Cross-file JOIN
dbexplain execute --label my_data \
  'SELECT o.branch_name, AVG(t.rate) FROM data t JOIN org o ON t.dept_id = o.dept_id GROUP BY o.branch_name' --human
```

> **`SELECT *` is for preview only — never use it as the source for analytical conclusions.** All statistics must be computed via aggregate queries.

**Best practices**:
- Clarify vague requests first (summary vs detail? group comparison? specific metric?), ask 2-3 key questions
- Always use explicit WHERE clauses — never rely on implicit context
- Default JSON (Agent analysis), add `--human` for terminal tables

## 5. Security Policy (on `ACCESS_DENIED`)

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

## 6. Traceable Analysis

### Never Fabricate Data

**Every number in your analysis output must come from an actual SQL query result.** The following are prohibited:

- **Fabricated statistics**: Averages, extremes computed via AVG/MAX/MIN/COUNT — never estimated from SELECT *
- **Fabricated features**: Do not claim results for unsupported functions (window functions, STDDEV, median). Report engine errors honestly
- **Fabricated tables**: Use original column names and values from query output
- **Sorting errors**: Ranking tables must be strictly ordered
- **Omitted output**: Cite query results verbatim
- **Concept confusion**: Distinguish row counts (`COUNT(*)`) from time spans (`COUNT(DISTINCT date_col)`)

> **Golden rule**: Whatever number you need, write the SQL to compute it.

### Citation Rules

- Append source SQL and row count to quantitative conclusions
- **Cite per conclusion** — never write "all data from query XXX" at the end

### Good Example

> Department A has the highest average completion rate at 95.2%; Department B at 82.1%.
> Source: `SELECT department, AVG(completion_rate) AS avg_rate FROM sales_data GROUP BY department ORDER BY avg_rate DESC` (6 rows)

### Bad Examples

> Department A performs best. ← ✗ No source, no numbers
> All data from SELECT * query. ← ✗ Vague attribution

## 7. Notes

- **DSN with special chars**: wrap in **single quotes** on CLI; no escaping needed in `.env`
- **MongoDB**: DSN must include `authSource` (e.g. `?authSource=admin`) and specify database name
- **Config encryption**:
  1. User runs: `dbexplain encrypt .env.dbexplain` (machine fingerprint, no password needed)
  2. **Must delete plaintext file after encryption** (otherwise tool loads plaintext first)
  3. Agent **must never** read encryption keys or participate in encryption
- **ES query limitation**: array fields not supported, select scalar fields only
- **ClickHouse**: do not add semicolons to queries, otherwise flagged as multi-statement
- **Skill directory structure**: `tools/dbexplain` is a symlink to the system-wide binary, serving as a fallback entry point for restricted environments. When `dbexplain` is in PATH, prefer the global command
- **Full help**: `dbexplain all` (supports `--filter keyword`)
- **Troubleshooting**: DB connection/file query errors at [`references/troubleshooting.md`](references/troubleshooting.md)

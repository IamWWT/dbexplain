name: dbexplain-skill
description: >
  Database schema explorer supporting MySQL/PG/ClickHouse/SQLite/Redis/MongoDB/ES/Qdrant, CSV/TSV/XLSX files.
  Auto-generates table structures, field comments, cross-DB relationship graphs, health reports.
  Supports read-only query execution (execute, with CSV/XLSX file query engine: WHERE/GROUP BY/JOIN/aggregates/expressions) and access control (policy).
user-invocable: true
trigger:
  - "explain table structure"
  - "analyze database relationships"
  - "cross-DB dependencies"
  - "generate ER diagram"
  - "database inspection"
  - "execute read-only query"
---
## Tool Overview

`dbexplain` is a Go binary CLI installed to system PATH. Two independent modes:

- **Schema Collection** (`dbexplain -env`): inspects tables/fields/types/comments/cross-DB foreign keys/health score, outputs `instances[]` + `refs[]` (JSON)
- **Read-Only Query** (`dbexplain execute`): runs SELECT to verify data after collection, outputs `columns[]` + `rows[]` (JSON)

Also supports: help manual (`dbexplain all`), config encryption (`encrypt`), incremental change detection (`--cache`).

## 1. Core Principles

- **Read-Only Safe**: Only SELECT/SHOW/SCAN. Never writes data.
- **Privacy Protection**: Agent must **never** view/log/ask for plaintext passwords. Let users configure in `.env`; tool auto-redacts.
- **Boundary**: Agent only invokes commands. **Never** create/modify/read config file content.
- **Global PATH**: `dbexplain` is in system PATH, callable from any directory.

## 2. Standard Workflow

### 2.1 Verify Installation + Get Help

```bash
dbexplain --version                  # check if installed
dbexplain all                        # view full help manual
dbexplain all --filter execute       # filter help by keyword
dbexplain execute -h                 # subcommand help
```
If not installed → tell user to run `bash scripts/install.sh` (from project root).

### 2.2 Configure DB Connection

Ask the user: do you already have a `.env` config file?

- **Yes** (recommended) → proceed directly.
- **No, but have connection info** → guide user to create `~/.config/dbexplain/.env.dbexplain`:
  ```ini
  DB1=mysql://user:pass@host:3306/mydb?label=my-mysql
  DB2=redis://:pass@host:6379/0?label=my-redis
  ```
  Agent **must not** edit this file. Tell the path and format, wait for user.
- **Direct DSN** → `dbexplain -dsn 'scheme://u:pass@host:port/db?label=x'`

### 2.3 List Configured Databases

```bash
dbexplain list -env
```
Outputs INDEX / LABEL / KIND / HOST:PORT / DATABASE mapping. Use `--db N` or `--label xxx` to select thereafter.

### 2.4 Collect Schema

```bash
# Collect all
dbexplain -env

# Filter by type (MySQL and PG only)
dbexplain -env -include mysql,postgres

# Exclude a type
dbexplain -env -exclude redis

# Output AI context directory (recommended)
dbexplain -env --context ./ctx
```

`--context ./ctx` generated file structure:

| File | Content | Usage |
|------|---------|-------------|
| `ctx/summary.json` | Overview: DB list, table count, total fields, health score | Quick understanding of overall status |
| `ctx/topology.json` | Topology: cross-DB foreign keys, references, data flow | Analyze cross-DB dependencies and relationship chains |
| `ctx/diagnostics.json` | Diagnostics: missing indexes, field type anomalies, missing comments | Output inspection issues and risks |
| `ctx/chunks/*.json` | Detailed structure per table (field name/type/comment) | Analyze field semantics per table |

Agent should present analysis to user: tables, field meanings, foreign key relationships, health score, and risk items.

### 2.5 Schema Incremental Change Detection

First collection generates a fingerprint cache; subsequent runs auto-detect changes:

```bash
# First: build cache
dbexplain -env --context ./ctx --cache ./schema.cache

# Later: detect differences from last run
dbexplain -env --context ./ctx --cache ./schema.cache
```
Output includes `changes` field (added/deleted/changed tables and fields). Suitable for periodic inspection, version comparison.

### 2.6 Execute Read-Only Query (collect first, then query)

Use after schema collection when field meanings are unclear or you need to verify data. Agent constructs queries — no need for user to provide SQL.

**SQL Databases (MySQL/PG/ClickHouse/SQLite/ES):**
```bash
dbexplain execute -env --label mysql 'SELECT COUNT(*) FROM orders'
dbexplain execute -env --label pg --explain 'SELECT * FROM users WHERE id=42'
dbexplain execute -env --db 3 --human "SELECT * FROM events LIMIT 5"
```
Auto LIMIT 1000. Rejects DROP/INSERT/UPDATE/DELETE. ES uses standard SQL via `_sql` endpoint.

**MongoDB (JSON format):**
```bash
dbexplain execute -env --label mongo '{"find":"users","filter":{"age":{"$gt":18}}}'
dbexplain execute -env --label mongo '{"aggregate":"orders","pipeline":[{"$match":{"status":"done"}}]}'
```

**Redis (native commands):**
```bash
dbexplain execute -env --label redis 'GET user:1001'
dbexplain execute -env --label redis 'HGETALL session:abc'
```

**CSV/XLSX files (v0.1.0+ file query engine):**
File datasources support the full SELECT subset for business analysis without external tools:
```bash
# WHERE filter + column projection
dbexplain execute -env --label touch-ops 'SELECT csmgr_refno, reach_rate FROM data WHERE reach_rate < 60' --human

# GROUP BY + aggregation
dbexplain execute -env --label touch-ops 'SELECT dept, AVG(rate) AS avg_rate FROM data GROUP BY dept ORDER BY avg_rate DESC' --human

# Cross-file JOIN
dbexplain execute -env --label touch-ops \
  'SELECT o.org_name, AVG(t.reach_rate) FROM data t JOIN org o ON t.org_id = o.id GROUP BY o.org_name' --human

# Column arithmetic + type cast
dbexplain execute -env --label touch-ops \
  'SELECT rm, CAST(channel_cnt AS FLOAT) / total_cnt * 100 AS pct FROM data WHERE total_cnt > 0' --human
```
File datasources are read-only (SELECT only); DROP/INSERT returns parse error.

**Output:** Default JSON (for Agent analysis), add `--human` for terminal tables.

## 3. Security Policy (on `ACCESS_DENIED`)

Admins configure sensitive data protection in `.env`. Agent **must not bypass** — inform the user:

```env
DENY_TABLES=sensitive,audit_log               # table-level block
DENY_COLUMNS=users.password_hash               # column-level block (must be table.column format)
DENY_STATEMENTS=DROP TABLE,ALTER TABLE         # statement block
MASK_COLUMNS=email=REDACTED,card_no=****       # non-blocking, replace column values on output
```

| Output | Meaning | Agent Action |
|--------|---------|--------------|
| `ACCESS_DENIED: table "xxx"` | Table is denied | Try a different table |
| `ACCESS_DENIED: column "xxx"` | Column is denied | Remove that column and retry |
| `READ_ONLY_VIOLATION` | Write operation | Change to SELECT |
| `CONCURRENT_LIMIT` | Concurrency conflict | Retry later |
| `QUERY_ERROR: ...` | Connection or SQL error | Fix DSN or SQL |

## 4. Common Parameters

| Parameter | Scope | Description |
|-----------|-------|-------------|
| `--label/--db N` | execute | Select DB by label or index |
| `--human` | execute | Table output (default JSON) |
| `--limit/--timeout` | execute | Row limit(1000)/timeout(30s) |
| `--explain` | execute | Output query plan |
| `--context dir` | collect | AI context dir (summary/topology/diagnostics/chunks) |
| `--cache file` | collect | Schema fingerprint cache, incremental change detection |
| `-include/-exclude` | collect | Filter by DB type |
| `-json/-o file` | collect | JSON output / write to file |

## 5. Notes

- **DSN with special chars**: wrap the entire DSN in **single quotes** on CLI; no escaping needed in `.env`
- **MongoDB**: DSN must include `authSource` (e.g. `?authSource=admin`) and specify database name
- **Config encryption**:
  1. User runs: `dbexplain encrypt .env.dbexplain` (machine fingerprint, no password needed)
  2. **Must delete plaintext file after encryption** (otherwise tool loads plaintext first)
  3. After encryption, `.env` auto-discovers `.enc` file and decrypts — no command change needed
  4. Agent **must never** read encryption keys or participate in encryption
- **ES query limitation**: array fields not supported (e.g. `SELECT *` may error), select scalar fields only
- **ClickHouse**: do not add semicolons to queries, otherwise flagged as multi-statement
- **Install/Uninstall**: `bash scripts/install.sh` / `bash scripts/uninstall.sh`
- **Full help**: `dbexplain all` (supports `--filter keyword`)

# dbexplain — Database Context Compiler

> **[中文版 →](README_ZH.md)**

> **Database Context Compiler** — Deterministic ground truth for AI agents and engineering teams.

`dbexplain` is a **single-binary, zero-runtime-dependency** CLI tool that compiles database metadata and executes read-only queries across **15 heterogeneous data sources** (including optional DuckDB) — all under a unified, auditable security sandbox.

Core philosophy: **deterministic facts only — LLMs consume structured IR externally.**

---

## Architecture Hierarchy

```
┌──────────────────────────────────────────────────────────────────┐
│                     CLI Command Layer                              │
│  Schema Collect  Execute  REPL  Diff  Config Mgmt  Reference      │
│  collect        execute  repl   diff  list/encrypt  csv/xlsx      │
├──────────────────────────────────────────────────────────────────┤
│                    Query Execution Layer                            │
│  ┌────────────────┐  ┌────────────────┐  ┌───────────────────┐   │
│  │ Direct Exec    │  │ DSL Mode       │  │ Federated Query   │   │
│  │ --db 1 SELECT  │  │ @label.table   │  │ Cross-source      │   │
│  │ --label redis  │  │ auto-resolve   │  │ JOIN/UNION        │   │
│  └────────────────┘  └────────────────┘  └───────────────────┘   │
├──────────────────────────────────────────────────────────────────┤
│                    Security Layer                                   │
│  ┌──────────┐    ┌────────────┐    ┌────────────────────────┐    │
│  │ sqlguard │ →  │ AutoLimit  │ →  │ Policy Engine          │    │
│  │ AST-level │    │ LIMIT 1000 │    │ DENY_TABLES/COLUMNS   │    │
│  │ read-only │    │ full-table │    │ MASK_COLUMNS/DENY_SQL │    │
│  └──────────┘    └────────────┘    └────────────────────────┘    │
├──────────────────────────────────────────────────────────────────┤
│                 Connector Layer (15 Data Sources)                  │
│  Relational: MySQL PG GaussDB SQLite DuckDB Oracle               │
│  Analytical: ClickHouse Hive                                      │
│  Key-Value:  Redis                                                │
│  Document:   MongoDB Elasticsearch                                │
│  Vector:     Qdrant                                               │
│  File:       CSV TSV XLSX (built-in pure-Go SQL engine)          │
│  Time Series: Prometheus (PromQL instant queries)                 │
├──────────────────────────────────────────────────────────────────┤
│                  Schema / IR Data Layer                            │
│  Collect() → schema.Instance → IR → JSON / Human / Diff / Graph │
└──────────────────────────────────────────────────────────────────┘
```

### Layer Description

| Layer | Responsibility | Key Components |
|-------|---------------|----------------|
| **CLI Command Layer** | User interaction, subcommand dispatch | `cmd/dbexplain/` — `main.go`, `execute.go`, `repl.go`, `collect.go`, `diff.go` |
| **Query Execution Layer** | Three-path: Direct / DSL / Federated | `executor/`, `dsl/` (DSL compiler), `connector/filequery/` (file SQL engine) |
| **Security Layer** | AST read-only validation + LIMIT injection + policy deny | `sqlguard/`, `policy/`, `query/` (concurrency lock) |
| **Connector Layer** | Unified interface for 15 data sources | `connector/` — one file per source, `init()` auto-registers to global registry |
| **Schema/IR Layer** | Collect → Internal Representation → Output Rendering | `schema/`, `ir/`, `render/`, `output/`, `graph/`, `diff/` |

![dbexplain Architecture](docs/assets/DBEXPLAIN-ARCH.png)

> Full module mapping at [`docs/CODE_MAP.md`](docs/CODE_MAP.md).

---

---

## Capability Matrix

> Data source × capability module. ✅ supported, — N/A, ⚠️ conditional.

| Category | Source | Protocol | Collect | Query | REPL | DSL | Highlights |
|----------|--------|----------|:-------:|:-----:|:----:|:---:|------------|
| **Relational** | MySQL | `mysql://` | ✅ | ✅ SQL | ✅ | ✅ | FK, indexes, column comment inference |
| | PostgreSQL | `postgres://` | ✅ | ✅ SQL | ✅ | ✅ | Multi-schema, row counts, SSL configurable |
| | GaussDB | `gaussdb://` | ✅ | ✅ SQL | ✅ | ✅ | PostgreSQL-protocol compatible |
| | SQLite | `sqlite://` | ✅ | ✅ SQL | ✅ | ✅ | Pure Go driver, no CGO |
| | Oracle | `oracle://` `oracles://` | ✅ | ✅ SQL | ✅ | ✅ | FK/indexes/PK, TLS, 12c+ FETCH FIRST required |
| **Analytical** | ClickHouse | `clickhouse://` | ✅ | ✅ SQL | ✅ | ✅ | Sort / partition / primary keys |
| | Hive | `hive://` `hives://` | ✅ | ✅ SQL | ✅ | ✅ | DESCRIBE FORMATTED, Kerberos, TLS, no row count stats |
| | DuckDB ¹ | `duckdb://` | ✅ | ✅ SQL | ✅ | ✅ | Embedded analytical engine, requires `-tags duckdb` |
| **Key-Value** | Redis | `redis://` `rediss://` | ✅ | — | ✅ | — | Key pattern inference, cluster, TTL risk |
| **Document** | MongoDB | `mongodb://` | ✅ | — | ✅ | — | Estimated document counts |
| | Elasticsearch | `elasticsearch://` `elasticsearchs://` | ✅ | ⚠️ SQL+JSON | ✅ | — | Index mapping, TLS, native JSON _search |
| **Vector** | Qdrant | `qdrant://` | ✅ | — | ✅ | — | Vector collection metadata |
| **Time Series** | Prometheus ² | `prometheus://` | ✅ | ✅ PromQL | ✅ | ✅ | Targets/labels/metrics metadata |
| **File** | CSV / TSV | `csv://` `tsv://` | ✅ | ✅ SQL ⁵ | ✅ | ✅ | Built-in pure-Go SQL engine ³ |
| | Excel | `xlsx://` | ✅ | ✅ SQL ⁵ | ✅ | ✅ | Built-in pure-Go SQL engine ³ |

> ¹ DuckDB is an optional build: Standard edition (-std) excludes DuckDB; DuckDB edition (-duckdb) includes all drivers + DuckDB, requires CGO environment.<br>
> ² Prometheus supports both single-source DSL and cross-source federation: `SELECT * FROM @prom.up WHERE job="prometheus"`. Also supports `promql()` syntax for embedding arbitrary PromQL expressions: `FROM @prom.promql(rate(cpu[5m]) / rate(mem[5m]) * 100)`.<br>
> ³ CSV/TSV/XLSX support a full SQL subset (WHERE/GROUP BY/JOIN/window functions/UNION) with hash index optimization.
> ⁵ File-source queries execute through the built-in SQL engine, bypassing the executor path.

![Prometheus DSL vs MySQL query mapping comparison](docs/assets/promtheus-mysql-dsl-1.png)
![Prometheus DSL query examples](docs/assets/promtheus-mysql-dsl-2.png)

---

## Core Capabilities

### Schema Collection

Extract table structures, columns, indexes, foreign keys, row counts, partition keys, and engine metadata from all sources. Output formats:

| Format | Command | Use Case |
|--------|---------|----------|
| **JSON** | `dbexplain -env --json` | Machine-consumable structured data |
| **Human** | `dbexplain -env --human` | Human-readable rendered output |
| **Delta Cache** | `dbexplain -env --cache /tmp/cache.json` | Re-collect only on schema change |
| **AI Context** | `dbexplain -env --context /tmp/ctx.md` | LLM-ready context file |
| **Metrics** | `dbexplain -env --metrics` | Prometheus text format to stderr |

### Read-Only Query Execution — Three-Path Architecture

| Path | Command | Description |
|------|---------|-------------|
| **Direct Exec** | `--db 1 "SELECT ..."` / `--label redis "PING"` | SQL via native driver, NoSQL via native commands |
| **DSL Mode** | `--dsl "SELECT * FROM @label.table"` | `@label.table` references data source, auto-resolved and bound |
| **Federated Query** | `--dsl "SELECT * FROM @a.t JOIN @b.t ON ..."` | Cross-source JOIN/UNION (SQL+file+PromQL), filequery engine in-memory merge |

All three paths share the same security pipeline:

```
sqlguard(AST read-only) → Policy Engine(DENY/MASK) → AutoLimit(LIMIT 1000)
```

```bash
# Direct execution
dbexplain execute -env --db 1 "SELECT COUNT(*) FROM orders"
dbexplain execute -env --label redis "PING"

# DSL mode
dbexplain execute -env --dsl "SELECT * FROM @my-mysql.users LIMIT 10" --human

# Federated query
dbexplain execute -env --dsl "SELECT * FROM @ops-csv.data UNION ALL SELECT * FROM @xlsx.Sheet1" --human
```

### REPL Interactive Query

| Feature | Description |
|---------|-------------|
| **Startup** | `dbexplain repl -env` (from config) or `--dsn` direct connect |
| **No-Config Start** | Empty DSN enters `(disconnected)` state, `.connect <dsn>` dynamically attaches |
| **Commands** | `.conn`/`.dsn` switch source, `.list` list all, `.help`/`.exit`/`.quit` |
| **Security** | Same sqlguard + policy engine protection, all writes rejected |
| **ES JSON** | Native JSON queries via `/_search` endpoint, dynamic column resolution |
| **DSL Mode** | Single-source and federated DSL queries (including Prometheus PromQL) inside REPL |

### File Query Engine

CSV / TSV / XLSX driven by a **built-in pure-Go SQL engine** — no external database required:

| Capability | Description |
|------------|-------------|
| **Basic** | WHERE / GROUP BY / HAVING / aggregate functions (COUNT, SUM, AVG, MIN, MAX) |
| **Advanced** | Hash JOIN (INNER/LEFT/RIGHT) / ORDER BY (NULLS FIRST/LAST) / UNION / DISTINCT ON |
| **Expert** | Subquery IN / window functions (ROW_NUMBER, RANK, LEAD, LAG, NTILE, etc.) |
| **Optimization** | Hash index — `WHERE col='literal'` equality conditions O(1) lookup |
| **Cross-Format** | CSV ↔ XLSX cross-format JOIN, cross-file JOIN |

### Schema Diff

Field-level change tracking: detects column (add/remove/type/nullable/default/comment/pk), index, and FK changes at three levels.

```bash
dbexplain diff --cache schema.json --since v1.0 --human
```

### Security Architecture: Six-Layer Pipeline

All queries execute through a unified security pipeline, automatically routing to the appropriate validation path per database type.

```
                    ┌─ SQL Path ───────────────────────────────┐
                    │  sqlguard(AST read-only) → Policy Engine  │
                    │  CheckSQL → AutoLimit(1K)                 │
                    ├─ Native Path ────────────────────────────┤
                    │  Policy Engine CheckNative(cmd allowlist) │
                    ├─ File Path ──────────────────────────────┤
                    │  Policy Engine DenyTables(filename check) │
                    └──────────────────────────────────────────┘
                               ↓
                    Concurrent Lock → Exec → ApplyMask / StripDeniedColumns
```

| Layer | Component | SQL Path | Native Path | File Path |
|:-----:|-----------|:--------:|:-----------:|:---------:|
| L1 | **sqlguard** — AST read-only (8 read / 17 write verbs) | ✅ | — | — |
| L2 | **AutoLimit** — auto-inject LIMIT 1000 | ✅ | — | — |
| L3 | **Policy Engine** — DENY_TABLES/COLUMNS/STATEMENTS | ✅ CheckSQL | ✅ CheckNative | ✅ DenyTables |
| L4 | **Concurrent Lock** — per-label QueryLock | ✅ | ✅ | — ⁴ |
| L5 | **ApplyMask** — column value masking (post-exec) | ✅ | ✅ | ✅ |
| L6 | **StripDeniedColumns** — column stripping (post-exec) | ✅ | ✅ | ✅ |

#### Per-Database-Type Security Coverage

| Category | Source | Query Path | L1 sqlguard | L2 AutoLimit | L3 Policy | L4 Lock | L5 Mask | L6 Strip | Extra Protection |
|----------|--------|------------|:-----------:|:------------:|:---------:|:-------:|:-------:|:--------:|-----------------|
| **Relational** | MySQL | executor.IsSQL=true | ✅ | ✅ | ✅ SQL | ✅ | ✅ | ✅ | sqlguard |
| | PostgreSQL | executor.IsSQL=true | ✅ | ✅ | ✅ SQL | ✅ | ✅ | ✅ | sqlguard |
| | GaussDB | executor.IsSQL=true | ✅ | ✅ | ✅ SQL | ✅ | ✅ | ✅ | sqlguard |
| | SQLite | executor.IsSQL=true | ✅ | ✅ | ✅ SQL | ✅ | ✅ | ✅ | sqlguard |
| | Oracle | executor.IsSQL=true | ✅ | ✅ ¹ | ✅ SQL | ✅ | ✅ | ✅ | sqlguard |
| **Analytical** | ClickHouse | executor.IsSQL=true | ✅ | ✅ | ✅ SQL | ✅ | ✅ | ✅ | sqlguard |
| | Hive | executor.IsSQL=true | ✅ | ✅ | ✅ SQL | ✅ | ✅ | ✅ | sqlguard |
| | DuckDB ² | executor.IsSQL=true | ✅ | ✅ | ✅ SQL | ✅ | ✅ | ✅ | sqlguard + file access validation |
| **Key-Value** | Redis | executor.IsSQL=false | — | — | ✅ Native | ✅ | ✅ | ✅ | 42-command allowlist |
| **Document** | MongoDB | executor.IsSQL=false | — | — | ✅ Native | ✅ | ✅ | ✅ | find/aggregate allowlist |
| | Elasticsearch | executor.IsSQL ³ | ⚠️ SQL only | ⚠️ | ✅ | ✅ | ✅ | ✅ | _search endpoint |
| **Vector** | Qdrant | executor.IsSQL=false | — | — | ✅ Native | ✅ | ✅ | ✅ | scroll/count allowlist |
| **Time Series** | Prometheus | executor.IsSQL=false | — | — | ✅ Native | ✅ | ✅ | ✅ | PromQL read-only API |
| **File** | CSV / TSV | HandleFileExecute ⁴ | — | — | ✅ DenyTables | — | ✅ | ✅ | File read-only |
| | Excel | HandleFileExecute ⁴ | — | — | ✅ DenyTables | — | ✅ | ✅ | File read-only |

> ¹ Oracle AutoLimit: `LIMIT N` auto-converted to `FETCH FIRST N ROWS ONLY` (Oracle 12c+).
> ² DuckDB extra file access validation: `read_parquet`/`read_csv`/`read_json` restricted by `allowed_path` param.
> ³ ES dual-mode: SQL queries use IsSQL=true (full pipeline), JSON native queries use IsSQL=false (no sqlguard).
> ⁴ File path handled by `queryutil.HandleFileExecute`, bypasses executor but retains policy engine protection. L4 concurrent lock does not apply to file queries (single-threaded in-memory operation).

Non-SQL databases have their own command allow-lists or native query validators. Passwords are redacted from all output and logs.

---

## Binary Variants

| Variant | Build Command | CGO | Raw Size | After UPX |
|---------|--------------|:---:|:--------:|:---------:|
| **Standard (-std)** | `bash build.sh prod` (default) | ❌ Off | 58 MB | 11 MB (81%) |
| **DuckDB Edition (-duckdb)** | `bash build.sh minimal duckdb,...`¹ | ✅ On | 141 MB | 53 MB (62%) |

> ¹ Full DuckDB tag list: `duckdb,mysql,postgres,sqlite,clickhouse,redis,mongodb,elasticsearch,qdrant,csv,xlsx,prometheus`.
>
> **Startup speed (cold start)**: UPX editions add ~435ms of self-decompression overhead on each invocation (the executable must decompress itself into memory before any application code runs). noUPX: ~3ms. Runtime execution is **identical** after decompression.
>
> **Release**: `bash release.sh` with zero arguments produces all variants — 5-platform -std + 2-platform -duckdb, each with UPX/noUPX dual variants, 12 tarballs total.

---

## CLI Quick Reference

| Scenario | Command |
|----------|---------|
| Schema collection | `dbexplain -env / -dsn <url> / --json / --human / -o <file>` |
| Query execution | `dbexplain execute -env --db <N> / --label <name> / --dsl / --human` |
| REPL interactive | `dbexplain repl -env` / `dbexplain repl --dsn <url>` / `.connect <dsn>` |
| Federated query | `dbexplain execute -env --dsl "SELECT * FROM @a.t JOIN @b.t ON ..." --human` |
| File query | `dbexplain execute -dsn "csv://file.csv" "SELECT ..."` |
| Schema diff | `dbexplain diff --cache <file> --since <ver>` |
| List DSNs | `dbexplain list` (auto-loads -env) |
| Collection metrics | `dbexplain -env --metrics` (Prometheus format to stderr) |
| Encrypt config | `dbexplain encrypt` (auto-finds .env.dbexplain, outputs .enc) |
| Reference manual | `dbexplain mysql` / `dbexplain oracle` / `dbexplain hive` / `dbexplain all` |

---

## Quick Start

```bash
# 1. Build (single platform, all drivers, fast)
cd src && bash build.sh dev

# Or full release (5 GOOS/GOARCH + UPX compression)
cd src && bash build.sh

# 2. Configure (6-level auto-discovery)
mkdir -p ~/.config/dbexplain && cat > ~/.config/dbexplain/.env.dbexplain << EOF
DB1=mysql://root:pass@127.0.0.1:3306/mydb?label=my-mysql
DB2=redis://:pass@127.0.0.1:6379/0?label=my-redis
EOF

# 3. Verify
./release/dbexplain -env                          # Schema collection
./release/dbexplain execute --db 1 "SELECT 1" --human  # Query test
./release/dbexplain --version                     # Version check
```

---

## Documentation Navigation

| Scenario | Doc |
|----------|-----|
| Quick start guide (5 min) | [`docs/USAGE_GUIDE.md`](docs/USAGE_GUIDE.md) |
| Query examples (20+ with REPL/DSL/federated) | [`docs/CLI_EXAMPLES.md`](docs/CLI_EXAMPLES.md) |
| Deployment (source/binary/Skill) | [`docs/DEPLOY.md`](docs/DEPLOY.md) |
| Troubleshooting guide | [`dbexplain-skill/references/troubleshooting.md`](dbexplain-skill/references/troubleshooting.md) |
| Security policy config | [`docs/POLICY.md`](docs/POLICY.md) |
| Config file search rules | [`docs/CONFIG_SEARCH.md`](docs/CONFIG_SEARCH.md) |
| SQL syntax reference (file query engine) | [`dbexplain-skill/references/sql-syntax.md`](dbexplain-skill/references/sql-syntax.md) |
| Code module mapping | [`docs/CODE_MAP.md`](docs/CODE_MAP.md) |
| Database usage manuals (one per source) | [`docs/databases/`](docs/databases/) |
| Test reports (166+ items) | [`docs/test/`](docs/test/) |

---

## Development

```bash
cd src
go build ./...                              # Compile check
go vet ./...                                # Static analysis
go test ./... -count=1                      # Unit tests
bash build.sh                               # Release: 5 platforms + full + UPX
bash build.sh dev                           # Dev: current platform + all drivers
bash release.sh                             # Full release: standard + DuckDB
bash build.sh minimal mysql,postgres        # Minimal: selective drivers
bash build.sh --help                        # View all options
```

> **Naming convention**: Standard edition (pure Go, no DuckDB) uses `-std` suffix; DuckDB edition (all drivers + DuckDB) uses `-duckdb` suffix.

---

## License

Apache 2.0 © 2026 WWT

# dbexplain — Database Context Compiler

> **[中文版 →](README_ZH.md)**

> **Database Context Compiler** — Deterministic ground truth for AI agents and engineering teams.

`dbexplain` is a **single-binary, zero-runtime-dependency** CLI tool that compiles database metadata and executes read-only queries across **12 heterogeneous data sources** (including optional DuckDB) — all under a unified, auditable security sandbox.

Core philosophy: **deterministic facts only — LLMs consume structured IR externally.**

---

## Why dbexplain?

- **Unified heterogeneity** — One tool for MySQL / PG / Redis / ES / Mongo / files and more
- **Deterministic first** — Same input → same output. No AI hallucination. Inferred relationships tagged `inferred=true`
- **Zero-dependency binary** — CGO_ENABLED=0, single file, no runtime required
- **Three-layer security** — AST validation + policy engine + AutoLimit, production-ready
- **DSL mode** — Reference any data source via `@label.table`, no connection switching

---

## Deterministic First

**dbexplain does no AI reasoning or semantic guessing.** Same database, same tool version, same query → always the same result. No randomness, no black-box decisions.

All inferred relationships (e.g., naming-pattern FK matches) are tagged with `inferred=true` and a confidence score, clearly separated from DDL-declared foreign keys. The tool outputs **verifiable facts** — column names, types, constraints, index structures. Semantic understanding is left to external LLMs / agents.

---

## Capability Matrix

> Data source × capability module. ✅ supported, — N/A, ⚠️ conditional.

| Source | Protocol | Collect | SQL Query | REPL | DSL Federated | File Engine | Highlights |
|--------|----------|:-------:|:---------:|:----:|:-------------:|:-----------:|------------|
| MySQL | `mysql://` | ✅ | ✅ | ✅ | ✅ | — | FK, indexes, column comment inference |
| PostgreSQL | `postgres://` | ✅ | ✅ | ✅ | ✅ | — | Multi-schema, row counts, SSL |
| GaussDB | `gaussdb://` | ✅ | ✅ | ✅ | ✅ | — | PostgreSQL-protocol compatible |
| ClickHouse | `clickhouse://` | ✅ | ✅ | ✅ | ✅ | — | Sort / partition / primary keys |
| SQLite | `sqlite://` | ✅ | ✅ | ✅ | ✅ | — | Pure Go driver, no CGO |
| DuckDB | `duckdb://` | ✅ | ✅ | ✅ | ✅ | — | Embedded SQL engine, Parquet/CSV file analysis, optional `-tags duckdb` build |
| Redis | `redis://` | ✅ | — | ✅ | — | — | Key pattern inference, cluster, TTL |
| MongoDB | `mongodb://` | ✅ | — | ✅ | — | — | Estimated document counts |
| Elasticsearch | `elasticsearch://` | ✅ | ⚠️ SQL via `_sql` | — | — | — | Index mapping, HTTPS |
| Qdrant | `qdrant://` | ✅ | — | ✅ | — | — | Vector collection metadata |
| CSV / TSV | `csv://` `tsv://` | ✅ | — | ✅ | ✅ | ✅ | Built-in file query engine |
| Excel | `xlsx://` | ✅ | — | ✅ | ✅ | ✅ | Built-in file query engine |

> **ES REPL**: native JSON queries not supported; use `execute -env --label` command mode.<br>
> **DSL limitation**: Redis/Mongo/ES/Qdrant native queries not supported in DSL federated mode.

---

## Core Capabilities

### Schema Collection
Extract table structures, columns, indexes, foreign keys, row counts, partition keys, and engine metadata from all sources. Output formats:
- **JSON** — Machine-consumable structured data
- **Markdown / Human-readable** — Rendered with context annotations
- **Incremental fingerprint cache** `--cache` — Re-collect only on schema changes
- **AI context export** `--context` — Generate ready-to-use LLM context files

### Read-Only Query Execution — Dual-Path Architecture

Both paths share the same security pipeline: **sqlguard (AST validation) → AutoLimit → policy engine**

| Path | Usage | Description |
|------|-------|-------------|
| **Direct** | `--db 1 "SELECT ..."` | Native SQL for SQL databases, native commands for NoSQL |
| **DSL mode** | `--dsl "SELECT * FROM @label.table"` | Reference data sources via `@label.table`, auto-resolved and bound |

```bash
# Direct execution
dbexplain execute -env --db 1 "SELECT COUNT(*) FROM orders"
dbexplain execute -env --label redis "PING"

# DSL mode
dbexplain execute -env --dsl "SELECT * FROM @my-mysql.users LIMIT 10" --human
```

> DSL federated queries support cross-source JOIN/UNION (SQL ↔ File). Redis / Mongo / Qdrant / ES native sources not supported in DSL mode.

### File Query Engine
CSV / TSV / XLSX powered by a **built-in pure-Go SQL engine** — no external database required:
- **Basic** — WHERE / GROUP BY / HAVING / aggregates (COUNT, SUM, AVG, MIN, MAX)
- **Intermediate** — Hash JOIN / ORDER BY (NULLS FIRST/LAST) / UNION / DISTINCT ON
- **Advanced** — Subquery IN / window functions (ROW_NUMBER, RANK, DENSE_RANK, NTILE, LAG, LEAD, FIRST_VALUE, LAST_VALUE, aggregate OVER with ROWS/RANGE frame specs)

### Schema Diff
Field-level change tracking: detects column (add/remove/type/nullable/default/comment/pk), index, and FK changes. Supports version baseline, two-file, and current collection modes.

```bash
dbexplain diff --cache schema.json --since v1.0 --human
```

### Security Three-Layer Defense
1. **sqlguard** — AST-level read-only validation: 8 read verbs allowed, 11 write verbs rejected, multi-statement detection, CTE write detection. Falls back to string matching on parse failure
2. **Policy engine** — `DENY_TABLES` (table-level block), `DENY_COLUMNS` (column-level block, with `SELECT *` star-expansion detection), `DENY_STATEMENTS` (statement pattern block), `MASK_COLUMNS` (result redaction)
3. **AutoLimit** — Automatic `LIMIT 1000` injection for unbounded queries, no duplicate on existing LIMIT

Non-SQL databases have their own command allow-lists or native query validators. Passwords are redacted from all output and logs.

---

## Documentation

| Scenario | Doc |
|----------|-----|
| Quick start guide (5 min) | [`docs/USAGE_GUIDE.md`](docs/USAGE_GUIDE.md) |
| Query examples (15+ verified, incl. REPL) | [`docs/CLI_EXAMPLES.md`](docs/CLI_EXAMPLES.md) |
| Installation (source/binary/Skill) | [`docs/DEPLOY.md`](docs/DEPLOY.md) |
| Troubleshooting (connection/query/files) | [`dbexplain-skill/references/troubleshooting.md`](dbexplain-skill/references/troubleshooting.md) |
| Security policy config (DENY_TABLES, etc.) | [`docs/POLICY.md`](docs/POLICY.md) |
| Config file search rules | [`docs/CONFIG_SEARCH.md`](docs/CONFIG_SEARCH.md) |
| SQL syntax reference (file query engine) | [`dbexplain-skill/references/sql-syntax.md`](dbexplain-skill/references/sql-syntax.md) |
| Full docs index | [`docs/CODE_MAP.md`](docs/CODE_MAP.md) |

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
./release/dbexplain execute -env --db 1 "SELECT 1" --human  # Query test
./release/dbexplain --version                     # Version check
```

---

## CLI Quick Reference

| Scenario | Command |
|----------|---------|
| Schema collection | `dbexplain -env / -dsn <url> / -json / -human / -o <file>` |
| | `dbexplain collect -env --human` (explicit subcommand, v0.1.2+) |
| Query execution | `dbexplain execute -env --db <N> / --label <name> / --dsl / --human` |
| Interactive REPL | `dbexplain repl --dsn <url>` or `dbexplain repl -env` (v0.1.3+, all 12 sources, no DSL mode) |
| File query | `dbexplain execute -dsn "csv://file.csv" "SELECT ..."` |
| Schema diff | `dbexplain diff --cache <file> --since <ver>` |
| List DSNs | `dbexplain list -env` |
| Encrypt config | `dbexplain encrypt .env.dbexplain` |

---

## Development

```bash
cd src
go build ./...                              # Compile check
go vet ./...                                # Static analysis
go test ./... -count=1                      # Unit tests
bash build.sh                               # Release: 5 platforms + full + UPX
bash build.sh dev                           # Dev: current platform + all drivers
bash release.sh                             # Full release: standard(-std) + DuckDB(-duckdb)
bash build.sh minimal mysql,postgres        # Minimal: selective drivers
bash build.sh --help                        # View all options
```

> **Naming convention**: Standard edition (pure Go, no DuckDB) uses `-std` suffix e.g. `dbexplain-linux-amd64-std`; DuckDB edition (all drivers + DuckDB) uses `-duckdb` suffix e.g. `dbexplain-linux-amd64-duckdb`. See [`DEPLOY.md`](docs/DEPLOY.md) for details.

---

## License

Apache 2.0 © 2026 WWT

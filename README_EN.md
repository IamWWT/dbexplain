# dbexplain — Database Context Compiler

> **Database Context Compiler** — Deterministic ground truth for AI agents.

`dbexplain` is a **single binary, zero runtime dependency** CLI tool. Given database connection strings, it auto-extracts table structures, columns, indexes, and foreign keys, outputting deterministic, verifiable relationship information — with zero AI inference or semantic guessing.

---

## Supported Databases

| Database | Scheme | Highlights |
|----------|--------|------------|
| MySQL | `mysql://` | FK, indexes, column comment inference |
| PostgreSQL | `postgres://` | Multi-schema, row stats, SSL configurable |
| GaussDB | `gaussdb://` | PostgreSQL protocol compatible |
| ClickHouse | `clickhouse://` | Sort/partition/primary keys |
| SQLite | `sqlite://` | Pure Go driver, no CGO |
| Redis | `redis://` | Key pattern inference, cluster, risk diagnostics |
| Elasticsearch | `elasticsearch://` | Index mappings, HTTPS |
| MongoDB | `mongodb://` | Approximate doc count |
| Qdrant | `qdrant://` | Vector collection metadata |
| CSV/TSV | `csv://` `tsv://` | File query engine: WHERE/GROUP BY/JOIN/aggregates/expressions |
| Excel | `xlsx://` | File query engine: WHERE/GROUP BY/JOIN/aggregates/expressions |

---

## Core Principles

**Deterministic facts only.** No AI summaries, no semantic guessing, no LLM inference. See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) and [`CONSTITUTION.md`](CONSTITUTION.md).

---

## Quick Start

### Install

```bash
# Online install (binary + AI Skill)
git clone https://github.com/IamWWT/dbexplain.git
cd dbexplain && bash dbexplain-skill/scripts/install.sh --lang en

# Manual download
wget https://github.com/IamWWT/dbexplain/releases/download/v0.1.0/dbexplain-linux-amd64
chmod +x dbexplain-linux-amd64 && sudo mv dbexplain-linux-amd64 /usr/local/bin/dbexplain
```

> Full install guide (offline, Windows, source build) at [`docs/DEPLOY.md`](docs/DEPLOY.md).

### Configuration

```bash
mkdir -p ~/.config/dbexplain
cat > ~/.config/dbexplain/.env.dbexplain << 'EOF'
DB1=mysql://root:pass@127.0.0.1:3306/mydb?label=my-mysql
DB2=redis://:pass@127.0.0.1:6379/0?label=my-redis
EOF
```

> Config file search rules (6-level auto-discovery) at [`docs/CONFIG_SEARCH.md`](docs/CONFIG_SEARCH.md).

### Verify

```bash
dbexplain -env              # Schema collection
dbexplain --version         # Must show v0.1.0
dbexplain all --language en # Full manual
```

---

## DSN Format

```
scheme://[user:pass@]host[:port][/db][?label=alias&params...]
```

**Common params**: `label=<alias>` (instance ID), `cluster=true` (Redis cluster), `tls=true` (ES/Redis TLS), `sslmode=<mode>` (PostgreSQL SSL), `authSource=<db>` (MongoDB auth).

---

## Usage

### Schema Collection

```bash
dbexplain -dsn 'mysql://user:pass@localhost:3306/shop?label=shop-db'
dbexplain -env                        # All configured sources
dbexplain -env --include 'mysql,redis' # Filter by label/kind
dbexplain -env -o report.md           # File output
dbexplain -env --context ./ctx        # AI context files
dbexplain -env --cache schema.json    # Delta change detection
```

### Read-Only Query Execution

```bash
dbexplain execute -env --db 1 'SELECT COUNT(*) FROM orders'
dbexplain execute -env --label redis 'GET user:1001'
dbexplain execute -env --db 3 --human "SELECT * FROM users LIMIT 5"
```

> 13 verified examples at [`docs/CLI_EXAMPLES.md`](docs/CLI_EXAMPLES.md), security at [`docs/EXECUTE.md`](docs/EXECUTE.md).

### List Databases

```bash
dbexplain list -env   # INDEX/LABEL/KIND/HOST:PORT mapping
```

### Reference Manuals

```bash
dbexplain all --language en --filter redis
dbexplain mysql / redis / qdrant / csv
```

### Option Reference

| Flag | Description |
|------|-------------|
| `-dsn <string>` | Connection string (repeatable) |
| `-env` | Load from config file (auto-search) |
| `-config <file>` | JSON array of DSNs |
| `--include / --exclude` | Filter by label/kind/index |
| `-json` | JSON output |
| `-o <file>` | Write to file (UTF-8 BOM) |
| `-timeout <duration>` | Per-DSN timeout (default 20s) |
| `--conn N` | Max concurrent connections (default 10) |
| `--human` | Human-readable output |
| `--context <dir>` | AI context output directory |
| `--cache <file>` | Schema fingerprint cache |
| `--log-dir <dir>` | Log output directory |

---

## Safety

### Schema Collection
**Read-only** (`SELECT`/`SHOW`/`SCAN`/`PRAGMA` only). Passwords auto-redacted in output and logs. Parameterized queries prevent injection.

### Query Execution
Three-layer protection: **sqlguard** verb whitelist → **policy engine** table/column/statement deny → **AutoLimit** anti-full-scan. Non-SQL databases have native command allowlists.

> Details at [`docs/EXECUTE.md`](docs/EXECUTE.md), policy at [`docs/POLICY.md`](docs/POLICY.md), pre-release checklist at [`docs/SECURITY_CHECKLIST.md`](docs/SECURITY_CHECKLIST.md).

---

## AI Skill Integration

```bash
# Install with English Skill
bash dbexplain-skill/scripts/install.sh --lang en
bash dbexplain-skill/scripts/install-skill.sh --verify
```

Supports Claude Code, DeepSeek, AixCoding, Agents, etc. See [`docs/DEPLOY.md`](docs/DEPLOY.md).

---

## Development

```bash
cd src && go mod tidy && bash build.sh
go test ./...
```

> Test framework at [`docs/test/`](docs/test/).

---

## Documentation Index

| Doc | Content |
|-----|---------|
| [`CONSTITUTION.md`](CONSTITUTION.md) | Project constitution |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Architecture vision & roadmap |
| [`docs/ALGORITHMS.md`](docs/ALGORITHMS.md) | Algorithm reference |
| [`docs/EXECUTE.md`](docs/EXECUTE.md) | Query execution security |
| [`docs/CLI_EXAMPLES.md`](docs/CLI_EXAMPLES.md) | 13 CLI query examples |
| [`docs/POLICY.md`](docs/POLICY.md) | Access control policy |
| [`docs/DEPLOY.md`](docs/DEPLOY.md) | Installation & Skill deployment |
| [`docs/CONFIG_SEARCH.md`](docs/CONFIG_SEARCH.md) | Config file search rules |
| [`docs/SECURITY_CHECKLIST.md`](docs/SECURITY_CHECKLIST.md) | Pre-release checklist |
| [`docs/FILE_PROCESSING.md`](docs/FILE_PROCESSING.md) | CSV/TSV/XLSX processing |
| [`docs/MYSQL.md`](docs/MYSQL.md) ... | Per-database manuals |
| [`docs/test/README.md`](docs/test/README.md) | Test framework |
| [`CHANGELOG_EN.md`](CHANGELOG_EN.md) | Changelog |
| [`issues.json`](issues.json) | Issue tracking |

---

## License

Apache 2.0 © 2026 WWT

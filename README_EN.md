# dbexplain — Database Context Compiler

> **Database Context Compiler** — Deterministic ground truth for AI agents.

`dbexplain` is a **single binary, zero runtime dependency** CLI tool. Download one file and run — no Python, Node, JDK, or shared libraries needed. Given database connection strings, it auto-extracts table structures, columns, indexes, and foreign keys, outputting deterministic, verifiable relationship information — with zero AI inference or semantic guessing.

The "ground truth layer" for databases in the AI era.

---

## Table of Contents

- [Supported Databases](#supported-databases)
- [Core Principles](#core-principles)
- [Quick Start](#quick-start)
  - [Linux / macOS](#linux--macos)
  - [Windows](#windows)
  - [Build from Source](#build-from-source)
  - [Post-Install Config](#post-install-config)
  - [Encrypt Config Files](#encrypt-config-files)
- [DSN Format & Config](#dsn-format--config)
- [Usage](#usage)
  - [Schema Collection](#schema-collection)
  - [Read-Only Query Execution](#read-only-query-execution)
  - [List Databases](#list-databases)
  - [Reference Manuals](#reference-manuals)
  - [Option Reference](#option-reference)
  - [Subcommands](#subcommands)
- [Output Example](#output-example)
- [AI Skill Integration](#ai-skill-integration)
- [Safety](#safety)
- [Adding New Databases](#adding-new-databases)
- [Development](#development)
- [Documentation Index](#documentation-index)

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
| MongoDB | `mongodb://` | Approximate doc count, zero data risk |
| Qdrant | `qdrant://` | Vector collection metadata |
| CSV/TSV | `csv://` `tsv://` | Local files, single file/directory/glob, encoding detection |
| Excel | `xlsx://` | Excel files, each sheet as a table, included in standard build |

> See [`docs/`](docs/) for per-database details, safety mechanisms, and troubleshooting guides.

---

## Core Principles

**Only output verifiable facts.** Foreign keys come from DDL declarations. Relationships come from naming pattern matching. Risk diagnostics come from observable data. No AI summaries, no business semantic guessing, no LLM reasoning.

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) and [`CONSTITUTION.md`](CONSTITUTION.md) for the full architecture vision.

![dbexplain Architecture](docs/assets/architecture.drawio.png)
*4-stage pipeline: INPUT (Connection & Config) → COLLECT (9-DB + CSV/XLSX file schema extraction) → ANALYZE (FK inference/ranking/diagnostics/IR Graph) → OUTPUT (Markdown/JSON/context files)*

---

## Quick Start

### Linux / macOS

#### Online Install (Recommended)

One command for global tool install + AI Skill deployment:

```bash
git clone https://github.com/IamWWT/dbexplain.git
cd dbexplain
bash dbexplain-skill/scripts/install.sh            # Chinese skill
bash dbexplain-skill/scripts/install.sh --lang en  # English skill
```

The script auto-detects your OS and architecture (via `uname -s`/`uname -m`) and downloads the matching binary from GitHub Releases.

Available platform identifiers: `linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64`.

#### Offline Install

Pre-download the binary for your platform, then install with `--offline`:

```bash
# Download on a machine with internet (Linux amd64 example)
wget https://github.com/IamWWT/dbexplain/releases/download/v0.0.9/dbexplain-linux-amd64

# Copy to offline environment, then:
bash dbexplain-skill/scripts/install.sh --offline ./dbexplain-linux-amd64
```

Tool only, no Skill:

```bash
bash dbexplain-skill/scripts/install.sh --offline ./dbexplain-linux-amd64 --no-skill
```

#### Manual Binary Download

```bash
# Linux amd64
wget https://github.com/IamWWT/dbexplain/releases/download/v0.0.9/dbexplain-linux-amd64
chmod +x dbexplain-linux-amd64
sudo mv dbexplain-linux-amd64 /usr/local/bin/dbexplain

# macOS Apple Silicon
wget https://github.com/IamWWT/dbexplain/releases/download/v0.0.9/dbexplain-darwin-arm64
chmod +x dbexplain-darwin-arm64
sudo mv dbexplain-darwin-arm64 /usr/local/bin/dbexplain

dbexplain --version
```

### Windows

#### Online Install (Recommended)

In PowerShell:

```powershell
git clone https://github.com/IamWWT/dbexplain.git
cd dbexplain
.\dbexplain-skill\scripts\install.ps1              # Chinese skill
.\dbexplain-skill\scripts\install.ps1 -Lang en     # English skill
```

The script downloads `dbexplain-windows-amd64.exe` to `%LOCALAPPDATA%\dbexplain\` and adds it to your user PATH.

#### Offline Install

```powershell
# Download on a machine with internet
Invoke-WebRequest -Uri "https://github.com/IamWWT/dbexplain/releases/download/v0.0.9/dbexplain-windows-amd64.exe" -OutFile "dbexplain-windows-amd64.exe"

# Copy to offline environment, place at:
# %LOCALAPPDATA%\dbexplain\dbexplain.exe
# Then add that directory to your user PATH.
```

#### Manual Download

Download `dbexplain-windows-amd64.exe` from [GitHub Releases](https://github.com/IamWWT/dbexplain/releases), place in a directory of your choice, and add it to PATH.

### Build from Source

```bash
cd src && go mod tidy && bash build.sh
```

Binaries are generated in `release/` (linux/darwin/windows × amd64/arm64, 5 targets).

![dbexplain Deployment](docs/assets/deployment.drawio.png)
*3-step install: GitHub Releases → install.sh → 3 targets (binary /usr/local/bin, config ~/.config, skill ~/.agents)*

### Post-Install Config

Create a global config file (works from any directory):

```bash
mkdir -p ~/.config/dbexplain
cat > ~/.config/dbexplain/.env.dbexplain << 'EOF'
DB1=mysql://root:pass@127.0.0.1:3306/mydb?label=my-mysql
DB2=redis://:pass@127.0.0.1:6379/0?label=my-redis
DB3=postgres://user:pass@127.0.0.1:5432/mydb?label=my-pg
EOF
```

Windows users: place the config at `%USERPROFILE%\.config\dbexplain\.env.dbexplain`.

Verify:

```bash
dbexplain -env                  # Terminal formatted report
dbexplain --version             # Print version
dbexplain all --language en     # Full English manual
dbexplain mysql                 # MySQL reference manual
dbexplain redis                 # Redis reference manual
```

### Encrypt Config Files

`dbexplain` supports encrypting `.env.dbexplain` files using a machine fingerprint. The encrypted file can only be decrypted on the same machine.

```bash
# Encrypt with machine fingerprint (default, no password needed)
dbexplain encrypt

# Password + machine fingerprint double protection
dbexplain encrypt --password

# Specify input/output files
dbexplain encrypt .env.dbexplain -o config.enc
```

After encryption, place the `.env.dbexplain.enc` file in `~/.config/dbexplain/` or the current directory, and `dbexplain -env` will auto-discover and decrypt it:

```bash
# Run directly after encryption (no manual env vars needed)
dbexplain -env

# If encrypted with --password, save the password to a key file:
echo "your-password" > ~/.config/dbexplain/.encryption_key
chmod 600 ~/.config/dbexplain/.encryption_key
dbexplain -env
```

> You can also use `DBPROBE_ENV_FILE` to explicitly specify the encrypted file path (optional override), and `APP_ENCRYPTION_KEY` to provide the password (optional override).
>
> **Algorithm**: XChaCha20-Poly1305 (AEAD). Machine-only mode requires no password; the config file can only be used on the original machine.
>
> **Key Advantages**:
> - No password to remember — encrypt once, auto-decrypt transparently
> - File is useless on any other machine even if stolen (hardware fingerprint mismatch)
> - Defense-in-depth: at-rest encryption beyond firewall/ACL
> - Compliance-friendly: meets GDPR/regulatory requirements for credential encryption
>
> **Note**: Delete the plaintext config after encryption. Re-encrypt after hardware changes.

---

## DSN Format & Config

```
scheme://[user:password@]host[:port][/dbname][?label=alias&params...]
```

### Common Parameters

| Parameter | Applies To | Description |
|-----------|------------|-------------|
| `label=<alias>` | All | Instance alias, determines log file `logs/<label>.log` |
| `cluster=true` | Redis | Cluster mode, auto-scan all shards |
| `tls=true` | ES, Redis | Enable TLS |
| `sslmode=<mode>` | PostgreSQL | SSL mode: `disable`/`require`/`verify-ca`/`verify-full` |
| `tls-skip-verify=true` | ES | Skip TLS certificate verification (diagnostic use) |
| `authSource=<db>` | MongoDB | Authentication database name |

### Config File Search Order (`-env` mode)

1. `DBPROBE_ENV_FILE` environment variable (optional override)
2. `.env.dbexplain` in current directory
3. `.env.dbexplain.enc` in current directory (encrypted, auto-decrypt)
4. `~/.config/dbexplain/.env.dbexplain` (Linux/macOS) or `%USERPROFILE%\.config\dbexplain\.env.dbexplain` (Windows)
5. `~/.config/dbexplain/.env.dbexplain.enc` (encrypted, auto-decrypt)
6. `.env` in current directory (legacy backward compat)

> See [docs/CONFIG_SEARCH.md](docs/CONFIG_SEARCH.md) for details (search order is independent of binary location, CWD determines behavior).

### Config Template

```ini
# MySQL
DB1=mysql://root:password@127.0.0.1:3306/mydb?label=my-mysql

# PostgreSQL
DB2=postgres://postgres:password@127.0.0.1:5432/mydb?label=my-pg&sslmode=disable

# ClickHouse
DB3=clickhouse://default:password@127.0.0.1:8123/default?label=my-ch

# SQLite (absolute path)
DB4=sqlite:///home/user/data/app.db?label=my-sqlite

# Redis standalone / cluster
DB5=redis://:password@127.0.0.1:6379/0?label=my-redis
DB6=redis://:password@10.0.0.1:7000/0?cluster=true&label=my-redis-cluster

# Elasticsearch HTTP / HTTPS
DB7=elasticsearch://elastic:password@127.0.0.1:9200?label=my-es
# HTTPS: elasticsearchs:// or elasticsearch://...?tls=true

# MongoDB
DB8=mongodb://admin:password@127.0.0.1:27017/mydb?authSource=admin&label=my-mongo

# Qdrant
DB9=qdrant://:api-key@127.0.0.1:6334?label=my-qdrant
```

---

## Usage

### Schema Collection

```bash
# Single database
dbexplain -dsn 'mysql://user:pass@localhost:3306/shop?label=shop-db'

# Multiple heterogeneous databases
dbexplain \
  -dsn 'mysql://root:pwd@host1:3306/orders' \
  -dsn 'postgres://u:p@host2:5432/users' \
  -dsn 'redis://:pwd@host3:6379/0?label=cache'

# Load from config file, filter with include/exclude
dbexplain -env
dbexplain -env -include 'mysql,postgres'
dbexplain -env -exclude 'mongodb,qdrant'

# Write to file (Windows CN: auto GBK, others: UTF-8 BOM)
dbexplain -env -o report.md
dbexplain -env -json -o report.json

# Load DSN array from JSON config
dbexplain -config dbs.json

# Generate AI context files (for agent prompts)
dbexplain -env --context ./context
# → context/summary.json      Global summary (instance list, table ranking, importance)
# → context/topology.json      Relationship topology (cross-DB refs, clusters)
# → context/diagnostics.json   Issue checklist (severity, table, message)
# → context/chunks/*.md        Per-table retrieval-friendly Markdown

# Incremental change detection (with cron)
dbexplain -env --cache schema_cache.json
# 1st run: creates schema_cache.json (fingerprint snapshot)
# Subsequent: compares diff → outputs schema_cache_delta.json (added/removed/changed)

# Human-friendly output (with [table=] [pattern=] context markers)
dbexplain -env --human

# Custom timeout (default 20s)
dbexplain -env -timeout 60s
```

### Read-Only Query Execution

The `execute` subcommand runs sandboxed read-only queries. Default output is JSON (for AI agents); `--human` switches to ASCII table.

```bash
# SQL databases (sqlguard triple-layer: verb whitelist + multi-statement detection + auto LIMIT)
dbexplain execute -env --label shop-db 'SELECT COUNT(*) FROM orders'
dbexplain execute -env --db 1 'SHOW INDEX FROM users'
dbexplain execute -env --label my-pg --explain 'SELECT * FROM orders WHERE user_id=42'

# Native queries for non-SQL databases
dbexplain execute -env --label es-test 'SHOW TABLES'                    # ES SQL
dbexplain execute -env --label mongo '{"find":"users","filter":{}}'     # MongoDB JSON
dbexplain execute -env --label redis 'GET user:1001'                    # Redis command
dbexplain execute -env --label qdrant '{"count":"docs"}'                # Qdrant JSON

# Human-readable table output
dbexplain execute -env --db 3 --human "SELECT * FROM users LIMIT 5"
```

![list + execute --human example](docs/assets/install-offline-verify-2.png)

![dbexplain usage example](docs/assets/usages.png)

> More examples in [`docs/CLI_EXAMPLES.md`](docs/CLI_EXAMPLES.md); security architecture in [`docs/EXECUTE.md`](docs/EXECUTE.md).

### List Databases

```bash
# Zero credential exposure; encrypted .env auto-decrypted
dbexplain list -env
```

### Reference Manuals

```bash
# Full manual (with keyword filter and language switching)
dbexplain all --filter redis
dbexplain all --language en --filter "SSL mode"

# Database-specific manuals
dbexplain mysql               # MySQL
dbexplain postgres            # PostgreSQL (aliases: pg, postgresql)
dbexplain gaussdb             # GaussDB
dbexplain clickhouse          # ClickHouse (alias: ch)
dbexplain sqlite              # SQLite (alias: sqlite3)
dbexplain redis               # Redis
dbexplain elasticsearch       # Elasticsearch (alias: es)
dbexplain mongodb             # MongoDB
dbexplain qdrant              # Qdrant
dbexplain csv                 # CSV/TSV file processing (DSN format, encoding, query limits)
dbexplain xlsx                # Excel file processing (build requirements)
```

### Option Reference

| Option | Description |
|--------|-------------|
| `-dsn <string>` | Database connection string, repeatable |
| `-env` | Load DSNs from config file (auto-search 6-level paths, no manual config needed) |
| `-config <file>` | Read DSN array from JSON file |
| `-include <filter>` | Only include matching DSNs (by kind/label/index, comma-sep) |
| `-exclude <filter>` | Exclude matching DSNs |
| `-json` | Output JSON format |
| `-o <file>` | Write output to file (text mode: auto UTF-8 BOM) |
| `--log-dir <dir>` | Log output directory (default `/var/log/dbexplain`) |
| `-timeout <duration>` | Per-DSN timeout (default 20s) |
| `--conn N` | Max concurrent connections for schema collection (default 10) |
| `--version` | Print version |
| `--human` | Human-friendly output with context markers |
| `--context <dir>` | Write AI context files to directory (summary.json / topology.json / diagnostics.json / chunks/) |
| `--cache <file>` | Schema fingerprint cache. First run writes snapshot; subsequent runs output `<file>_delta.json` diff |
| `--language zh|en` | Manual language (default zh) |

### Subcommands

| Command | Description |
|---------|-------------|
| `dbexplain list` | List all configured databases with INDEX/LABEL/KIND/HOST:PORT/DATABASE mapping (zero credential exposure) |
| `dbexplain execute <SQL>` | **Read-only query execution** (sandboxed). SQL types: sqlguard validation; non-SQL: native format. `--human` for table output |
| `dbexplain encrypt <file>` | Encrypt `.env` config file (machine fingerprint / password dual mode) |
| `dbexplain all` | Full reference manual (supports `--filter`, `--language`) |
| `dbexplain <dbtype>` | Database/file-specific reference manual. 10 types: mysql, postgres/pg/postgresql, gaussdb, clickhouse/ch, sqlite/sqlite3, redis, mongodb, elasticsearch/es, qdrant, csv, xlsx |
| `dbexplain -h` | Show compact structured help overview |

---

## AI Skill Integration

`install.sh` installs both tool and skill by default, with `--lang zh|en` for language. Or run separately:

```bash
# One-click (tool + skill, online)
bash dbexplain-skill/scripts/install.sh
bash dbexplain-skill/scripts/install.sh --lang en   # English skill

# One-click (tool + skill, offline)
bash dbexplain-skill/scripts/install.sh --offline ./dbexplain-linux-amd64

# Tool only, skip skill deployment
bash dbexplain-skill/scripts/install.sh --no-skill

# Skill only (when tool is already installed)
# --lang zh for Chinese, --lang en for English
bash dbexplain-skill/scripts/install-skill.sh
bash dbexplain-skill/scripts/install-skill.sh --lang en

# Update installed skill
bash dbexplain-skill/scripts/install-skill.sh --update

# Verify installation
bash dbexplain-skill/scripts/install-skill.sh --verify

# Uninstall skill
bash dbexplain-skill/scripts/uninstall-skill.sh

# Uninstall tool
bash dbexplain-skill/scripts/uninstall.sh
```

![Skill and Tool install](docs/assets/install-offline-1.png)

![AI Agent + dbexplain Interaction](docs/assets/skill-interaction.drawio.png)
*5-step flow: ① User asks → ② AI loads SKILL.md → ③ AI invokes dbexplain to collect schema → ④ dbexplain outputs deterministic report → ⑤ AI explains to user*

> Supports Claude Code, DeepSeek, AixCoding, Agents, and more. See [`docs/DEPLOY.md`](docs/DEPLOY.md).

---

## Safety

### Schema Collection Mode
All operations are **read-only**: MySQL/PostgreSQL only `SELECT`/`SHOW`/`PRAGMA`; Redis only `SCAN`/`TYPE`/`HSCAN` (strict sampling caps); MongoDB only `ListCollectionNames`/`EstimatedDocumentCount`. Never writes, modifies, or deletes data.

- Passwords automatically redacted in output and logs (`Redacted()`)
- Per-DSN independent logging (`logs/<label>.log`)
- Filter skip records written to `logs/filter.log`, not polluting terminal output
- Parameterized queries prevent SQL injection
- Redis sampling caps: 2000 keys, 5 fields, 512 bytes, 10 stream messages

### Read-Only Query Execution (`execute`)
The `execute` subcommand runs user SQL/native queries under sandbox protection, fully separated from schema collection:

- **SQL read-only validation** (`sqlguard`): Verb whitelist + multi-statement detection + auto LIMIT injection; rejects DROP/INSERT/UPDATE/DELETE
- **Non-SQL whitelist**: Redis 30+ command whitelist, MongoDB find/aggregate whitelist, Qdrant scroll/count whitelist
- **Query routing**: `isSQLKind()` routes by database type; SQL via sqlguard, non-SQL via per-connector internal validation
- **Fine-grained access control**: Table/column/statement-level deny policies (`DENY_TABLES`/`DENY_COLUMNS`/`DENY_STATEMENTS`); column value masking (`MASK_COLUMNS`)
- **Concurrent mutex**: per-label `TryLock`, only one query at a time per label
- **Dual timeout**: Application context + database-level statement timeout
- **Output safety**: Terminal output strips ANSI escape and control chars; column width capped at 256 chars
- **Credential protection**: Query result JSON contains no connection info or passwords

> See [`docs/EXECUTE.md`](docs/EXECUTE.md) for details

---

## Adding New Databases

1. Create a new file under `src/connector/`
2. Implement the `Connector` interface: `Collect(ctx, *dsn.DSN) (*schema.Instance, error)`
3. Call `Register("kind", func() Connector { ... })` in `init()`
4. Rebuild

No core code changes needed — fully compliant with the open/closed principle.

---

## Development

- **Language**: Go 1.26+
- **Build**: `CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=v0.0.9"`
- **Test**: `go test ./...` (DSN parsing + field inference)
- **Cross-compile**: `bash build.sh` (linux/darwin/windows × amd64/arm64)

---

## Documentation Index

| Document | Content |
|----------|---------|
| [`CONSTITUTION.md`](CONSTITUTION.md) | Project constitution (core principles, dev constraints) |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Architecture vision and roadmap |
| [`docs/EXECUTE.md`](docs/EXECUTE.md) | Read-only query execution (security architecture, 9-DB verification) |
| [`docs/CLI_EXAMPLES.md`](docs/CLI_EXAMPLES.md) | CLI query examples (13 verified commands across 7 data sources) |
| [`docs/DEPLOY.md`](docs/DEPLOY.md) | Deployment guide (source build + tool install + Skill deploy) |
| [`docs/MYSQL.md`](docs/MYSQL.md) | MySQL field inference, index/FK collection |
| [`docs/POSTGRESQL.md`](docs/POSTGRESQL.md) | PostgreSQL pg_catalog, SSL, multi-schema |
| [`docs/GAUSSDB.md`](docs/GAUSSDB.md) | GaussDB PostgreSQL protocol compatible |
| [`docs/CLICKHOUSE.md`](docs/CLICKHOUSE.md) | ClickHouse HTTP, sort/partition keys |
| [`docs/SQLITE.md`](docs/SQLITE.md) | SQLite INTEGER PRIMARY KEY, CGO-free |
| [`docs/REDIS.md`](docs/REDIS.md) | Redis keyspace analysis, risk diagnostics |
| [`docs/MONGO.md`](docs/MONGO.md) | MongoDB auth troubleshooting, read-only metadata |
| [`docs/ELASTICSEARCH.md`](docs/ELASTICSEARCH.md) | Elasticsearch index mappings, HTTPS |
| [`docs/QDRANT.md`](docs/QDRANT.md) | Qdrant vector collection metadata |
| [`docs/POLICY.md`](docs/POLICY.md) | Fine-grained access control policy (table/column/statement level) |
| [`docs/FILE_PROCESSING.md`](docs/FILE_PROCESSING.md) | CSV/TSV/XLSX file processing (DSN format, encoding, type inference) |
| [`docs/ISSUE-062.md`](docs/ISSUE-062.md) | v0.0.9 policy engine fix record (wildcard query bypass) |
| [`docs/SECURITY_CHECKLIST.md`](docs/SECURITY_CHECKLIST.md) | Security checklist (pre-release must-read) |
| [`CHANGELOG.md`](CHANGELOG.md) | Changelog (Chinese) |
| [`CHANGELOG_EN.md`](CHANGELOG_EN.md) | Changelog (English) |
| [`issues.json`](issues.json) | Issue tracking |

---

## License

Apache 2.0 © 2026 WWT

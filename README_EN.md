# dbexplain — Database Context Compiler

> **Database Context Compiler** — Deterministic ground truth for AI agents.

`dbexplain` is a **zero-dependency, statically compiled** CLI tool. Given database connection strings, it auto-extracts table structures, columns, indexes, and foreign keys, outputting deterministic, verifiable relationship information — with zero AI inference or semantic guessing.

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
- [Usage](#usage)
  - [Basic Usage](#basic-usage)
  - [Option Reference](#option-reference)
  - [Per-Database Examples](#per-database-examples)
- [DSN Format & Config](#dsn-format--config)
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

> See [`docs/`](docs/) for per-database details, safety mechanisms, and troubleshooting guides.

---

## Core Principles

**Only output verifiable facts.** Foreign keys come from DDL declarations. Relationships come from naming pattern matching. Risk diagnostics come from observable data. No AI summaries, no business semantic guessing, no LLM reasoning.

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) and [`CONSTITUTION.md`](CONSTITUTION.md) for the full architecture vision.

---

## Quick Start

### Linux / macOS

#### Online Install (Recommended)

One command for global tool install + AI Skill deployment:

```bash
git clone https://github.com/IamWWT/understand_dbs_skills.git
cd understand_dbs_skills
bash db-relationship-explainer/scripts/install.sh            # Chinese skill
bash db-relationship-explainer/scripts/install.sh --lang en  # English skill
```

The script auto-detects your OS and architecture (via `uname -s`/`uname -m`) and downloads the matching binary from GitHub Releases.

Available platform identifiers: `linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64`.

#### Offline Install

Pre-download the binary for your platform, then install with `--offline`:

```bash
# Download on a machine with internet (Linux amd64 example)
wget https://github.com/IamWWT/understand_dbs_skills/releases/download/v0.0.5/dbexplain-linux-amd64

# Copy to offline environment, then:
bash db-relationship-explainer/scripts/install.sh --offline ./dbexplain-linux-amd64
```

Tool only, no Skill:

```bash
bash db-relationship-explainer/scripts/install.sh --offline ./dbexplain-linux-amd64 --no-skill
```

#### Manual Binary Download

```bash
# Linux amd64
wget https://github.com/IamWWT/understand_dbs_skills/releases/download/v0.0.5/dbexplain-linux-amd64
chmod +x dbexplain-linux-amd64
sudo mv dbexplain-linux-amd64 /usr/local/bin/dbexplain

# macOS Apple Silicon
wget https://github.com/IamWWT/understand_dbs_skills/releases/download/v0.0.5/dbexplain-darwin-arm64
chmod +x dbexplain-darwin-arm64
sudo mv dbexplain-darwin-arm64 /usr/local/bin/dbexplain

dbexplain --version
```

### Windows

#### Online Install (Recommended)

In PowerShell:

```powershell
git clone https://github.com/IamWWT/understand_dbs_skills.git
cd understand_dbs_skills
.\db-relationship-explainer\scripts\install.ps1              # Chinese skill
.\db-relationship-explainer\scripts\install.ps1 -Lang en     # English skill
```

The script downloads `dbexplain-windows-amd64.exe` to `%LOCALAPPDATA%\dbexplain\` and adds it to your user PATH.

#### Offline Install

```powershell
# Download on a machine with internet
Invoke-WebRequest -Uri "https://github.com/IamWWT/understand_dbs_skills/releases/download/v0.0.5/dbexplain-windows-amd64.exe" -OutFile "dbexplain-windows-amd64.exe"

# Copy to offline environment, place at:
# %LOCALAPPDATA%\dbexplain\dbexplain.exe
# Then add that directory to your user PATH.
```

#### Manual Download

Download `dbexplain-windows-amd64.exe` from [GitHub Releases](https://github.com/IamWWT/understand_dbs_skills/releases), place in a directory of your choice, and add it to PATH.

### Build from Source

```bash
cd src && go mod tidy && bash build.sh
```

Binaries are generated in `release/` (linux/darwin/windows × amd64/arm64, 5 targets).

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

Windows users: place the config at `%USERPROFILE%\.dbexplain\.env.dbexplain`.

Verify:

```bash
dbexplain -env                  # Terminal formatted report
dbexplain --version             # Print version
dbexplain --manual --language en  # Full English manual
```

---

## Usage

### Basic Usage

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

# Search the manual
dbexplain --manual --filter redis
dbexplain --manual --language en --filter "SSL mode"
```

### Option Reference

| Option | Description |
|--------|-------------|
| `-dsn <string>` | Database connection string, repeatable |
| `-env` | Load DSNs from config file (search: `DBPROBE_ENV_FILE` → `.env.dbexplain` → `~/.config/dbexplain/.env.dbexplain` → `.env`) |
| `-config <file>` | Read DSN array from JSON file |
| `-include <filter>` | Only include matching DSNs (by kind/label/index, comma-sep) |
| `-exclude <filter>` | Exclude matching DSNs |
| `-json` | Output JSON format |
| `-o <file>` | Write output to file (auto UTF-8 BOM) |
| `--log-dir <dir>` | Log output directory (default `/var/log/dbexplain`) |
| `-timeout <duration>` | Per-DSN timeout (default 20s) |
| `--version` | Print version |
| `--manual` | Full help manual (`--language en` for English) |
| `--filter <keyword>` | Filter `--manual` output (case-insensitive) |
| `--human` | Human-friendly output with context markers |
| `--context <dir>` | Write AI context files to directory (summary.json / topology.json / diagnostics.json / chunks/) |
| `--cache <file>` | Schema fingerprint cache. First run writes snapshot; subsequent runs output `<file>_delta.json` diff |
| `--language zh|en` | Manual language (default zh) |

### Per-Database Examples

**MySQL**
```bash
dbexplain -dsn 'mysql://root:pwd@127.0.0.1:3306/shop?label=shop-db'
```

**PostgreSQL (multi-schema / SSL)**
```bash
dbexplain -dsn 'postgres://user:pwd@127.0.0.1:5432/warehouse?label=my-pg&sslmode=disable'
```

**Redis standalone / cluster**
```bash
# Standalone
dbexplain -dsn 'redis://:pwd@127.0.0.1:6379/0?label=my-redis'
# Cluster
dbexplain -dsn 'redis://:pwd@10.0.0.1:7000/0?cluster=true&label=my-cluster'
```

**ClickHouse**
```bash
dbexplain -dsn 'clickhouse://default:pwd@127.0.0.1:8123/default?label=my-ch'
```

**SQLite (absolute path)**
```bash
dbexplain -dsn 'sqlite:///home/user/data/app.db?label=local-db'
```

**MongoDB**
```bash
dbexplain -dsn 'mongodb://admin:pwd@127.0.0.1:27017/mydb?authSource=admin&label=my-mongo'
```

**Elasticsearch HTTP / HTTPS**
```bash
# HTTP
dbexplain -dsn 'elasticsearch://elastic:pwd@127.0.0.1:9200?label=my-es'
# HTTPS
dbexplain -dsn 'elasticsearchs://elastic:pwd@127.0.0.1:9200?label=my-es'
```

**Qdrant**
```bash
dbexplain -dsn 'qdrant://:api-key@127.0.0.1:6334?label=my-qdrant'
```

**GaussDB**
```bash
dbexplain -dsn 'gaussdb://user:pwd@192.168.0.1:25308/mydb?label=my-gauss'
```

> More details: `dbexplain --manual [--filter <keyword>]`

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
| `authSource=<db>` | MongoDB | Authentication database name |

### Config File Search Order (`-env` mode)

1. `DBPROBE_ENV_FILE` environment variable
2. `.env.dbexplain` in current directory
3. `~/.config/dbexplain/.env.dbexplain` (Linux/macOS) or `%USERPROFILE%\.dbexplain\.env.dbexplain` (Windows)
4. `.env` in current directory (legacy backward compat)

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

## Output Example

```
> Instances (2)
  shop-db                    mysql    1 db(s), 5 tables
  cache                      redis    1 db(s), 3 tables

> shop-db  /  mydb
  orders [InnoDB] ~42,000 rows  Core order table
----------------------------------------------------
  name       type          flags    comment
  ---------  ------------  -------  ------------
  id         int(11)       PK NN
  user_id    int(11)       NN       identifier
  total      decimal(10,2) NN       amount/quantity
  created_at datetime      NN       timestamp
  indexes: IDX(user_id)

> Relationships (3 explicit FK, 2 inferred)
  shop-db/mydb.orders(user_id) --FK--> shop-db/mydb.users(id)

> Issues (2)
  [!] shop-db/mydb/orders  FK column "user_id" has no index
  [i] cache/db0/session:{hex}  no TTL on security-sensitive key
```

![Terminal example](docs/assets/explain-test-dsn+env.png)

---

## AI Skill Integration

`install.sh` installs both tool and skill by default, with `--lang zh|en` for language. Or run separately:

```bash
# One-click (tool + skill, online)
bash db-relationship-explainer/scripts/install.sh
bash db-relationship-explainer/scripts/install.sh --lang en   # English skill

# One-click (tool + skill, offline)
bash db-relationship-explainer/scripts/install.sh --offline ./dbexplain-linux-amd64

# Tool only, skip skill deployment
bash db-relationship-explainer/scripts/install.sh --no-skill

# Skill only (when tool is already installed)
# --lang zh for Chinese, --lang en for English
bash db-relationship-explainer/scripts/install-skill.sh
bash db-relationship-explainer/scripts/install-skill.sh --lang en

# Update installed skill
bash db-relationship-explainer/scripts/install-skill.sh --update

# Verify installation
bash db-relationship-explainer/scripts/install-skill.sh --verify

# Uninstall skill
bash db-relationship-explainer/scripts/uninstall-skill.sh

# Uninstall tool
bash db-relationship-explainer/scripts/uninstall.sh
```

![Skill install](docs/assets/skill_install_mgr.png)

> Supports Claude Code, DeepSeek, AixCoding, Agents, and more. See [`docs/DEPLOY_SKILLS.md`](docs/DEPLOY_SKILLS.md).

---

## Safety

All operations are **read-only**: MySQL/PostgreSQL only `SELECT`/`SHOW`/`PRAGMA`; Redis only `SCAN`/`TYPE`/`HSCAN` (strict sampling caps); MongoDB only `ListCollectionNames`/`EstimatedDocumentCount`. Never writes, modifies, or deletes data.

- Passwords automatically redacted in output and logs (`Redacted()`)
- Per-DSN independent logging (`logs/<label>.log`)
- Filter skip records written to `logs/filter.log`, not polluting terminal output
- Parameterized queries prevent SQL injection
- Redis sampling caps: 2000 keys, 5 fields, 512 bytes, 10 stream messages

> See per-database docs ([`docs/`](docs/)) for detailed safety reviews and permission guides.

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
- **Build**: `CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=v0.0.5"`
- **Test**: `go test ./...` (DSN parsing + field inference)
- **Cross-compile**: `bash build.sh` (linux/darwin/windows × amd64/arm64)

---

## Documentation Index

| Document | Content |
|----------|---------|
| [`CONSTITUTION.md`](CONSTITUTION.md) | Project constitution (core principles, dev constraints) |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Architecture vision and roadmap |
| [`docs/DEPLOY_SKILLS.md`](docs/DEPLOY_SKILLS.md) | Skill deployment guide (multi-platform) |
| [`docs/DEPLOY_SRC.md`](docs/DEPLOY_SRC.md) | Source build deployment |
| [`docs/MYSQL.md`](docs/MYSQL.md) | MySQL field inference, index/FK collection |
| [`docs/POSTGRESQL.md`](docs/POSTGRESQL.md) | PostgreSQL pg_catalog, SSL, multi-schema |
| [`docs/CLICKHOUSE.md`](docs/CLICKHOUSE.md) | ClickHouse HTTP, sort/partition keys |
| [`docs/REDIS.md`](docs/REDIS.md) | Redis keyspace analysis, risk diagnostics |
| [`docs/MONGO.md`](docs/MONGO.md) | MongoDB auth troubleshooting, read-only metadata |
| [`docs/ELASTICSEARCH.md`](docs/ELASTICSEARCH.md) | Elasticsearch index mappings, HTTPS |
| [`CHANGELOG.md`](CHANGELOG.md) | Changelog (Chinese) |
| [`CHANGELOG_EN.md`](CHANGELOG_EN.md) | Changelog (English) |
| [`issues.json`](issues.json) | Issue tracking |

---

## License

Apache 2.0 © 2025-2026 WWT

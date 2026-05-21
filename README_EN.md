# dbexplain — Database Context Compiler

> **Database Context Compiler** — Deterministic ground truth for AI agents.

`dbexplain` is a **zero-dependency, statically compiled** CLI tool. Given database connection strings, it auto-extracts table structures, columns, indexes, and foreign keys, outputting deterministic, verifiable relationship information — with zero AI inference or semantic guessing.

The "ground truth layer" for databases in the AI era.

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

### Option 1: Online Install (Recommended)

One command for global tool install + AI Skill deployment:

```bash
bash db-relationship-explainer/scripts/install.sh
```

After install, create the config file:

```bash
cat > ~/.config/dbexplain/.env.dbexplain << 'EOF'
DB1=mysql://root:pass@127.0.0.1:3306/mydb?label=my-mysql
DB2=redis://:pass@127.0.0.1:6379/0?label=my-redis
DB3=postgres://user:pass@127.0.0.1:5432/mydb?label=my-pg
EOF
```

Run directly (no cd required):

```bash
dbexplain -env                  # Terminal formatted report
dbexplain -env -json -o report.json  # JSON output
dbexplain --manual --language en     # Full manual
```

> The installer auto-detects your platform and downloads the matching binary from GitHub Releases.

### Option 2: Offline Install

Pre-download the binary, then install with `--offline`:

```bash
# 1. Download on a machine with internet
wget https://github.com/IamWWT/understand_dbs_skills/releases/download/v0.0.5/dbexplain-linux-amd64

# 2. Copy to offline environment, then:
bash db-relationship-explainer/scripts/install.sh --offline ./dbexplain-linux-amd64
```

Interactive offline mode (prompts for binary path):

```bash
bash db-relationship-explainer/scripts/install.sh --offline
```

Tool only, no Skill:

```bash
bash db-relationship-explainer/scripts/install.sh --offline ./dbexplain-linux-amd64 --no-skill
```

### Option 3: Manual Binary Download

```bash
wget https://github.com/IamWWT/understand_dbs_skills/releases/download/v0.0.5/dbexplain-linux-amd64
chmod +x dbexplain-linux-amd64
sudo mv dbexplain-linux-amd64 /usr/local/bin/dbexplain
dbexplain --version
```

### Build from Source

```bash
cd src && go mod tidy && bash build.sh
```

Binaries are generated in the `release/` directory.

---

## Usage

```bash
# Single database
dbexplain -dsn 'mysql://user:pass@localhost:3306/shop?label=shop-db'

# Multiple heterogeneous databases
./dbexplain \
  -dsn 'mysql://root:pwd@host1:3306/orders' \
  -dsn 'postgres://u:p@host2:5432/users' \
  -dsn 'redis://:pwd@host3:6379/0?label=cache'

# Load from .env, filter with include/exclude
dbexplain -env -include 'mysql,postgres'
dbexplain -env -exclude 'mongodb,qdrant'

# Write to file (Windows CN: auto GBK, others: UTF-8 BOM, Notepad/CMD compatible)
dbexplain -env -o report.md
dbexplain -env -json -o report.json

# Custom timeout (default 20s)
dbexplain -env -timeout 60s
```

### Option Reference

| Option | Description |
|--------|-------------|
| `-dsn <string>` | Database connection string, repeatable |
| `-env` | Load DSNs from config file (search: `DBPROBE_ENV_FILE` → `.env.dbexplain` → XDG/user config → `.env`) |
| `-config <file>` | Read DSN array from JSON file |
| `-include <filter>` | Only include matching DSNs (by kind/label/index, comma-sep) |
| `-exclude <filter>` | Exclude matching DSNs |
| `-json` | Output JSON format |
| `-o <file>` | Write output to file |
| `--log-dir <dir>` | Log output directory (default `./logs`) |
| `-timeout <duration>` | Per-DSN timeout (default 20s) |
| `--version` | Print version |
| `--manual` | Full help manual (`--language en` for English) |
| `--filter <keyword>` | Filter `--manual` output (case-insensitive) |
| `--human` | Human-friendly output with context markers |
| `--context <dir>` | Write AI context files to directory |
| `--cache <file>` | Schema fingerprint cache for delta detection |
| `--language zh|en` | Manual language (default zh) |

---

## Usage Scenarios

### For AI Agents

```bash
# Output JSON for programmatic or AI Agent consumption
dbexplain -env -json -o report.json

# Generate AI context files (for embedding in agent prompts)
dbexplain -env --context ./context
# Outputs: summary.json / topology.json / diagnostics.json / chunks/*.md

# Incremental change detection (with cron)
dbexplain -env --cache schema_cache.json
# First run creates cache; subsequent runs output schema_cache_delta.json
```

### For Humans

```bash
# Direct terminal rendering (default text format with color highlights)
dbexplain -env

# Human-friendly format (with [table=] [pattern=] context markers)
dbexplain -env --human

# Write Markdown file (with UTF-8 BOM, Windows Notepad compatible)
dbexplain -env --human -o report.md

# Search the manual
./dbexplain --manual --filter redis
```

### Per-Database Examples

**MySQL**
```bash
dbexplain -dsn 'mysql://root:pwd@127.0.0.1:3306/shop?label=shop-db'
```

**PostgreSQL**
```bash
dbexplain -dsn 'postgres://user:pwd@127.0.0.1:5432/warehouse?label=my-pg&sslmode=disable'
```

**Redis (Cluster)**
```bash
dbexplain -dsn 'redis://:pwd@10.0.0.1:7000/0?cluster=true&label=my-cluster'
```

**ClickHouse**
```bash
dbexplain -dsn 'clickhouse://default:pwd@127.0.0.1:8123/default?label=my-ch'
```

**SQLite**
```bash
dbexplain -dsn 'sqlite:///home/user/data/app.db?label=local-db'
```

**MongoDB**
```bash
dbexplain -dsn 'mongodb://admin:pwd@127.0.0.1:27017/mydb?authSource=admin&label=my-mongo'
```

**Elasticsearch (HTTPS)**
```bash
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

> More database details: `./dbexplain --manual [--filter <keyword>]`

---

## DSN Format

```
scheme://[user:password@]host[:port][/dbname][?label=alias&params...]
```

**Common Parameters:**

| Parameter | Applies To | Description |
|-----------|------------|-------------|
| `label=<alias>` | All | Instance alias, determines log file `logs/<label>.log` |
| `cluster=true` | Redis | Cluster mode, auto-scan all shards |
| `tls=true` | ES, Redis | Enable TLS |
| `sslmode=<mode>` | PostgreSQL | SSL mode: `disable`/`require`/`verify-ca`/`verify-full` |
| `authSource=<db>` | MongoDB | Authentication database name |

**Config file search order (`-env` mode):**

1. `DBPROBE_ENV_FILE` environment variable
2. `.env.dbexplain` in current directory
3. `~/.config/dbexplain/.env.dbexplain` (Linux/macOS) or `%USERPROFILE%\.dbexplain\.env.dbexplain` (Windows)
4. `.env` in current directory (legacy backward compat)

**.env.dbexplain Template:**

```ini
# MySQL
DB1=mysql://root:password@127.0.0.1:3306/mydb?label=my-mysql

# PostgreSQL
DB2=postgres://postgres:password@127.0.0.1:5432/mydb?label=my-pg&sslmode=disable

# ClickHouse
DB3=clickhouse://default:password@127.0.0.1:8123/default?label=my-ch

# SQLite (absolute path)
DB4=sqlite:///home/user/data/app.db?label=my-sqlite

# Redis standalone
DB5=redis://:password@127.0.0.1:6379/0?label=my-redis

# Redis cluster
DB6=redis://:password@10.0.0.1:7000/0?cluster=true&label=my-redis-cluster

# Elasticsearch
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

## Safety

All operations are **read-only**: MySQL/PostgreSQL only `SELECT`/`SHOW`/`PRAGMA`; Redis only `SCAN`/`TYPE`/`HSCAN` (strict sampling caps); MongoDB only `ListCollectionNames`/`EstimatedDocumentCount`. Never writes, modifies, or deletes data.

- Passwords automatically redacted in output and logs
- Per-DSN independent logging (`logs/<label>.log`)
- Filter skip records written to `logs/filter.log`, not polluting terminal output
- Parameterized queries prevent injection
- Redis sampling caps: 2000 keys, 5 fields, 512 bytes, 10 stream messages

---

## AI Assistant Skill Integration

`install.sh` installs both tool and skill by default. Or run separately:

```bash
# One-click (tool + skill, online)
bash db-relationship-explainer/scripts/install.sh

# One-click (tool + skill, offline)
bash db-relationship-explainer/scripts/install.sh --offline ./dbexplain-linux-amd64

# Tool only, skip skill deployment
bash db-relationship-explainer/scripts/install.sh --no-skill

# Skill only (when tool is already installed)
bash db-relationship-explainer/scripts/install-skill.sh

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
- **Cross-compile**: `bash build.sh` (linux/darwin/windows x amd64/arm64)

---

## Documentation Index

| Document | Content |
|----------|---------|
| [`CONSTITUTION.md`](CONSTITUTION.md) | Project constitution (core principles, dev constraints) |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Architecture vision and roadmap |
| [`docs/MYSQL.md`](docs/MYSQL.md) | MySQL field inference, index/FK collection |
| [`docs/POSTGRESQL.md`](docs/POSTGRESQL.md) | PostgreSQL pg_catalog, SSL, multi-schema |
| [`docs/CLICKHOUSE.md`](docs/CLICKHOUSE.md) | ClickHouse HTTP, sort/partition keys |
| [`docs/REDIS.md`](docs/REDIS.md) | Redis keyspace analysis, risk diagnostics |
| [`docs/MONGO.md`](docs/MONGO.md) | MongoDB auth troubleshooting, read-only metadata |
| [`docs/ELASTICSEARCH.md`](docs/ELASTICSEARCH.md) | Elasticsearch index mappings, HTTPS |
| [`docs/DEPLOY_SKILLS.md`](docs/DEPLOY_SKILLS.md) | Skill deployment guide |
| [`docs/DEPLOY_SRC.md`](docs/DEPLOY_SRC.md) | Source build deployment |
| [`CHANGELOG.md`](CHANGELOG.md) | Changelog (Chinese) |
| [`CHANGELOG_EN.md`](CHANGELOG_EN.md) | Changelog (English) |
| [`issues.json`](issues.json) | Issue tracking |

---

## License

Apache 2.0 © 2025-2026 WWT

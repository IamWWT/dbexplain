name: db-relationship-explainer
description: >
  Zero-dependency database schema explorer supporting MySQL, PostgreSQL, ClickHouse, SQLite,
  Redis, MongoDB, Elasticsearch, Qdrant and more. Auto-generates table cards, column comments,
  cross-DB relationship graphs and health reports. Suitable for explaining table structures,
  analyzing database relationships, DB inspection, and cross-DB dependencies.
user-invocable: true
trigger:
  - "explain table structure"
  - "analyze database relationships"
  - "cross-DB dependencies"
  - "generate ER diagram"
  - "understand database"
  - "database inspection"
  - "database health check"
---
## 1. First Use: Install the Tool

If `dbexplain` is not yet installed (`dbexplain --version` returns command not found), run:

```bash
bash scripts/install.sh
```

This downloads and installs `dbexplain` to `/usr/local/bin` and creates a config template `~/.config/dbexplain/.env.dbexplain`.

## 2. Core Principles

- **Read-Only Safe**: The tool only performs SELECT / SHOW / SCAN read operations. It never writes or modifies data.
- **Privacy Protection**: The Agent must **never** view, log, or ask users for plaintext passwords. Users should pass passwords via config files. The tool automatically redacts passwords.
- **Responsibility Boundary**: The Agent can only invoke the tool. It must **never create, modify, or read config file contents**.
- **Global Command**: `dbexplain` lives in system PATH after installation. Call it from any directory.

## 3. Usage

### Method 1: User Provides DSN Directly

When the user provides connection info (e.g. "analyze MySQL at 192.168.1.1:3306, database testdb"), the Agent constructs a DSN and calls:

```bash
dbexplain -dsn 'mysql://user:password@host:3306/db?label=alias'
```

If a password is needed, prompt: "To protect your password, consider configuring it in `~/.config/dbexplain/.env.dbexplain`, or type it directly in the command (wrap the DSN in single quotes)."

### Method 2: Config File (Recommended for multiple DBs or password protection)

Config file search priority (`-env` auto-discovery):
1. `DBPROBE_ENV_FILE` environment variable (optional override)
2. `./.env.dbexplain` (current directory)
3. `./.env.dbexplain.enc` (current directory, auto-decrypt)
4. `~/.config/dbexplain/.env.dbexplain`
5. `~/.config/dbexplain/.env.dbexplain.enc` (auto-decrypt)

Guide the user to create a config at `~/.config/dbexplain/.env.dbexplain`:

```ini
DB1=mysql://user:password@host:3306/db?label=myapp
DB2=redis://:password@host:6379/0?label=cache
```

Once the user confirms, run:

```bash
dbexplain -env
```

The Agent must never view or edit the config file. If the user reports the config file is missing, respond with the correct path and format, and wait for the user to act.

### Encrypted Config Files (v0.0.6)

Users can encrypt their config file with a machine fingerprint. The encrypted file can only be decrypted on the same machine. **The Agent must NEVER view, ask for, or log the user's password.** The user runs these commands themselves in their terminal:

```bash
# Encrypt config file (machine fingerprint, no password)
dbexplain encrypt ~/.config/dbexplain/.env.dbexplain

# IMPORTANT: Delete the plaintext config after encryption!
rm ~/.config/dbexplain/.env.dbexplain
```

If the user chooses password-enhanced mode:

```bash
# User runs this themselves (Agent cannot see password input)
dbexplain encrypt ~/.config/dbexplain/.env.dbexplain --password

# Delete plaintext, save password to key file (user does this, Agent cannot read)
rm ~/.config/dbexplain/.env.dbexplain
echo "user-chosen-password" > ~/.config/dbexplain/.encryption_key
chmod 600 ~/.config/dbexplain/.encryption_key
```

After encryption, `dbexplain -env` auto-discovers and decrypts the `.enc` file (no env vars needed). The Agent should remind the user:
1. **Always delete the plaintext config file** after encryption (otherwise it takes priority)
2. The key file `~/.config/dbexplain/.encryption_key` should have permissions 600
3. The Agent **will never** read or modify these files

### Method 3: JSON Config File

The user provides a JSON file path. The Agent uses `-config <path>`.

### List Configured Databases (v0.0.7)

Before performing any operation, the Agent should first use the `list` subcommand to view the available database configurations:

```bash
dbexplain list
```

Output is a mapping table with these columns, helping the Agent select the correct `--db N` or `--label <name>`:

| Field | Description |
|-------|-------------|
| INDEX | DB index (maps to `--db N`, e.g. DB1=1, DB2=2) |
| LABEL | DSN label (maps to `--label`) |
| KIND | Database type (mysql/redis/mongodb, etc.) |
| HOST:PORT | Host and port |
| DATABASE | Database name |

**Security note**: The `list` command only displays metadata (label/kind/host/dbname). It **never outputs DSN connection strings, passwords, or any credentials**. Even if the config file is encrypted (`.enc`), its content is never decrypted for display.

### Method 4: Read-Only Query Execution (v0.0.7)

After understanding the schema, the Agent can use the `execute` subcommand to run sandboxed read-only queries to verify hypotheses or check data. **All 9 database types support queries.**

#### SQL Database Queries

```bash
# Basic SELECT query (auto-adds LIMIT 1000)
dbexplain execute -env --label shop-db 'SELECT COUNT(*) FROM orders'

# EXPLAIN query plan
dbexplain execute -env --label my-pg --explain 'SELECT * FROM orders WHERE user_id=42'

# Custom timeout and row limit
dbexplain execute -env --label shop-db --timeout 30 --limit 500 'SELECT * FROM events'
```

#### Non-SQL Database Native Queries

```bash
# Use --db index (pair with dbexplain list to see mapping)
dbexplain execute -env --db 1 'SELECT * FROM users LIMIT 5'

# Use --label name
dbexplain execute -env --label shop-db 'SELECT COUNT(*) FROM orders'

# Elasticsearch (standard SQL via _sql endpoint)
dbexplain execute -env --label es-test 'SHOW TABLES'
dbexplain execute -env --label es-test 'SELECT * FROM index_name WHERE status="active"'

# MongoDB (JSON format)
dbexplain execute -env --label mongo '{"find":"users","filter":{"age":{"$gt":18}},"limit":100}'
dbexplain execute -env --label mongo '{"aggregate":"orders","pipeline":[{"$match":{"status":"done"}}]}'

# Redis (native commands, 30+ command whitelist)
dbexplain execute -env --label redis 'GET user:1001'
dbexplain execute -env --label redis 'HGETALL session:abc'
dbexplain execute -env --label redis 'SCAN 0 MATCH user:* COUNT 100'

# Qdrant (JSON format)
dbexplain execute -env --label qdrant '{"count":"documents"}'
dbexplain execute -env --label qdrant '{"scroll":"documents","limit":20}'
```

#### Security Constraints

The Agent must understand and follow these constraints:

1. **Read-only enforcement**: All queries are protected by sqlguard whitelist (SQL) or per-connector whitelists (non-SQL). Write ops like DROP/INSERT/UPDATE/DELETE are rejected.
2. **Multi-statement ban**: Semicolon-concatenated statements are detected and rejected (prevents SQL injection escape).
3. **Auto LIMIT**: SELECT queries without LIMIT automatically get `LIMIT 1000` appended, preventing full table scans.
4. **Concurrent mutex**: Only one query per label at a time.
5. **Output format separation**: Query result JSON (`columns + rows`) is fully independent from schema collection JSON (`instances + refs`).
6. **Password protection**: Query results contain no connection info or passwords.

#### Typical Agent Use Cases

- After schema collection, a field's meaning is unclear → use `execute` to view sample data for that field
- Suspecting a foreign key relationship → use `execute` to verify referential integrity
- Need to confirm table row count / data distribution → use `execute` with `SELECT COUNT(*)` or grouping queries
- Risk indicators found during inspection → use `execute` to confirm risk impact scope

## 4. Common Parameters

| Parameter | Description |
|-----------|-------------|
| `-dsn <str>` | Direct DSN, can be repeated for multiple instances |
| `-env` | Load DSNs from config file (auto-searches multi-level paths) |
| `-config <file>` | Read DSN array from JSON file |
| `-json` | Output JSON format |
| `-o <file>` | Write report to file |
| `--log-dir <dir>` | Log output directory (default `/var/log/dbexplain`) |
| `--context <dir>` | AI context output (summary.json / topology.json / diagnostics.json / chunks/) |
| `-cache <file>` | Schema fingerprint cache for incremental change detection |
| `-timeout <dur>` | Per-DSN collect timeout (default 20s) |
| `-include <f>` | Only collect matching DSNs (by kind/label/key, comma-separated) |
| `-exclude <f>` | Exclude matching DSNs |
| `--human` | Human-friendly output with `[table=]`/`[pattern=]` context markers |
| `--version` | Print version |

### execute Subcommand Parameters

| Parameter | Description |
|-----------|-------------|
| `execute <query>` | Read-only query (SQL / JSON / native command), format separated from schema collection |
| `--label <name>` | Match DSN by label |
| `--db <N>` | Match DSN by DB index (DB1=1, DB2=2) |
| `--limit <N>` | Max rows returned (default 1000) |
| `--timeout <N>` | Query timeout in seconds (default 30) |
| `--explain` | Wrap with EXPLAIN for query plan (SQL databases only) |

## 5. DSN Advanced Parameters

- **Redis Cluster**: `redis://:password@host:7000/0?cluster=true&label=cluster`
- **Elasticsearch TLS**: Use `elasticsearchs://` scheme prefix or `?tls=true`
- **PostgreSQL SSL**: `?sslmode=disable|require|verify-ca|verify-full`

## 6. Agent Workflow

1. **Ensure tool is available**: If `dbexplain --version` fails, run `bash scripts/install.sh`.
2. **List available databases**: If the user uses a config file (typical scenario), first run `dbexplain list` to see the INDEX/LABEL/KIND/HOST:PORT/DATABASE mapping for all configured databases, confirming the target database and its index.
3. **Identify intent**:
   - User wants to **understand database structure** → collect schema:
     - User provides connection info → construct DSN and call with `-dsn`.
     - User has not provided connection info → ask if they've configured `~/.config/dbexplain/.env.dbexplain`.
       - Configured → `dbexplain -env`
       - Not configured → guide to create config file, wait, then execute.
   - User wants to **verify hypotheses / check data / confirm details** → use `execute` subcommand:
     - Based on the `list` output, select the target database (`--db N` or `--label <name>`).
     - Based on already-collected schema, construct safe queries for verification.
     - Agent should construct queries autonomously, not rely on users to provide raw SQL.
     - Query results are used to verify field semantics, FK relationships, data distributions, etc.
4. **Troubleshooting**:
   - `dbexplain` not found → `bash scripts/install.sh`
   - Config file not found → check `~/.config/dbexplain/.env.dbexplain` or encrypted `~/.config/dbexplain/.env.dbexplain.enc`
   - `READ_ONLY_VIOLATION` → query contains disallowed write operation; fix SQL and retry
   - `CONCURRENT_LIMIT` → another query is running on the same label; retry after it completes
   - `QUERY_ERROR` → connection failure or SQL syntax error; check DSN or fix query
5. **Present results**: Show the tool output to the user, and optionally provide suggestions based on the report.

## 7. Notes

- If `dbexplain` is not in PATH, run `bash scripts/install.sh` first.
- For passwords with special characters like `!`, wrap the entire DSN in **single quotes** on the command line; no escaping needed in `.env.dbexplain`.
- The tool prints progress on stderr ("Collecting… Done"), which does not affect the final report.
- MongoDB DSNs must include the database name and `authSource` parameter.
- Full documentation: `dbexplain all` (replaces the old `dbexplain --manual`)
- **List databases**: `dbexplain list` shows INDEX/LABEL/KIND/HOST:PORT/DATABASE mapping for all configured databases. Like `-env`, passwords are always redacted as `{dbuser}`/`{dbpassword}` placeholders; full DSNs or credentials are never leaked.
- **Credential safety**: Both `list` and `-env` output use `{dbuser}`/`{dbpassword}` placeholders in place of real credentials, ensuring sensitive information never appears in logs or terminal output.
- Uninstall tool: `bash scripts/uninstall.sh`; Uninstall Skill: `bash scripts/uninstall-skill.sh`

### execute Subcommand Notes

- **Schema collection vs query execution separation**: Schema collection outputs `instances/refs/groups/issues`; query execution outputs `columns/rows/row_count/execution_time`. The two JSON formats are completely different — the Agent must not mix them.
- **Collect first, query later**: The Agent should first collect schema via `-env` to understand database structure, then decide whether to run queries. Do not skip schema collection and jump straight to querying.
- **Agent constructs queries autonomously**: The Agent should write SQL/native queries based on analysis goals; do not pass raw user natural language directly to `execute`.
- **Non-SQL query formats**: Elasticsearch uses standard SQL; MongoDB/Qdrant use JSON; Redis uses native commands. The Agent must choose the correct format for each database type.
- **Query results are limited**: Default max 1000 rows; oversized results are auto-truncated (`truncated: true`). The Agent should narrow queries if truncation occurs.
- **Concurrent limiter**: Only one query at a time per label. If `CONCURRENT_LIMIT` occurs, wait for the previous query to complete, then retry.
- **Security documentation**: See `docs/EXECUTE.md`

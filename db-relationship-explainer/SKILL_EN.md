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

## 5. DSN Advanced Parameters

- **Redis Cluster**: `redis://:password@host:7000/0?cluster=true&label=cluster`
- **Elasticsearch TLS**: Use `elasticsearchs://` scheme prefix or `?tls=true`
- **PostgreSQL SSL**: `?sslmode=disable|require|verify-ca|verify-full`

## 6. Agent Workflow

1. **Ensure tool is available**: If `dbexplain --version` fails, run `bash scripts/install.sh`.
2. **Identify intent**:
   - User provides connection info → construct DSN and call with `-dsn`.
   - User has not provided connection info → ask if they've configured `~/.config/dbexplain/.env.dbexplain`.
     - Configured → `dbexplain -env`
     - Not configured → guide to create config file, wait, then execute.
3. **Troubleshooting**:
   - `dbexplain` not found → `bash scripts/install.sh`
   - Config file not found → check `~/.config/dbexplain/.env.dbexplain` or encrypted `~/.config/dbexplain/.env.dbexplain.enc`
4. **Present results**: Show the tool output to the user, and optionally provide suggestions based on the report.

## 7. Notes

- If `dbexplain` is not in PATH, run `bash scripts/install.sh` first.
- For passwords with special characters like `!`, wrap the entire DSN in **single quotes** on the command line; no escaping needed in `.env.dbexplain`.
- The tool prints progress on stderr ("Collecting… Done"), which does not affect the final report.
- MongoDB DSNs must include the database name and `authSource` parameter.
- Full documentation: `dbexplain --manual`
- Uninstall tool: `bash scripts/uninstall.sh`; Uninstall Skill: `bash scripts/uninstall-skill.sh`

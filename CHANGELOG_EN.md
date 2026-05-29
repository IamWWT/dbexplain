# Changelog

## v0.1.0 (2026-05-29) — Deep Security Hardening & Architecture Alignment & File Query Engine

### Security Fixes (P0)
- **WITH CTE write bypass fix**: `WITH ... INSERT/UPDATE/DELETE ...` only checked first token (WITH), CTE body writes fully bypassed validation. Added `containsCTEWrite()` for deep CTE body scan, rejecting WITH queries containing write operations
- **SELECT INTO bypass fix**: `SELECT * INTO new_table` starts with SELECT, bypassing read-only check. Added `isSelectInto()` to detect INTO TABLE clauses (excluding MySQL INTO @var), rejecting PostgreSQL DDL writes

### Security Hardening (P1)
- **ANALYZE/REINDEX removed from readOps**: `ANALYZE` writes to statistics tables, `REINDEX` locks tables rebuilding indexes. Moved from whitelist to blacklist
- **SET SESSION connection pool race fix**: MySQL SET max_execution_time / PG SET statement_timeout executed on different connections than the subsequent query, rendering timeouts ineffective. `ExecQuery` now forces single-connection mode (`SetMaxOpenConns(1)`)
- **matchStarSelect anchor fix**: Regex `\ASELECT` only matched start position; `WITH cte AS (SELECT * FROM t)` SELECT * was missed. Changed to `\bSELECT` for global matching
- **Policy config leak fix**: `loadEnvFile()` used `os.Setenv` to pass policy config, leaking to `/proc/[pid]/environ`
- **APP_ENCRYPTION_KEY cleanup**: `os.Unsetenv("APP_ENCRYPTION_KEY")` immediately after decryption, minimizing password exposure window

### Correctness Fixes (P1-P2)
- **PostgreSQL FK schema JOIN**: FK query was missing `pg_namespace` JOIN, causing FK results to mix between tables with the same name in different schemas
- **PostgreSQL index parsing**: `strings.LastIndex(def, ")")` broke on function indexes (`lower(email)`) and INCLUDE columns. Added `extractIndexColumns()` with bracket depth tracking
- **Cache atomic write**: `os.WriteFile` is non-atomic; process crash corrupts cache. Switched to temp file + `os.Rename()` atomic operation

### PostgreSQL Multi-Schema Support
- **Schema discovery**: `collectPGDB()` now queries `pg_namespace` for all non-system schemas, no longer hardcoded to `public` only
- **Row count from pg_class**: Added `n_live_tup` collection via `pg_class.reltuples` for per-table row estimates

### Architecture Alignment (Constitution Article 10)
- **CapSQL/CapFile capabilities**: New `CapSQL` and `CapFile` constants in `capabilities.go`
- **Unified connector declarations**: All 5 SQL connectors (MySQL/PostgreSQL/SQLite/ClickHouse/ES) declare `CapSQL`; CSV/XLSX declare `CapFile`
- **isSQLKind() deleted**: Hardcoded kind switch in `execute.go` replaced by `capabilities.FromProvider(c).Has(capabilities.CapSQL)`, eliminating the constitutional anti-pattern of type-based branching
- **New databases no longer need execute.go changes**: Just implement Connector + declare CapSQL, execute auto-routes correctly

### JSON Output Format Change
- **`instances` wrapper**: Schema collection JSON now wrapped in `{"instances": [...]}` envelope with `groups`, `issues`, `refs` top-level keys. The `dsn` field is no longer output per-instance
- **Backward compat note**: Consumers reading `kind`/`label`/`databases` directly from the top level must update to read from `instances[0]`

### Documentation Alignment (Phase D1-D5)
- **24+ .md files aligned** with v0.1.0 code: version numbers, PostgreSQL schema scope, Qdrant TLS/execute, Redis readOps whitelist, data source counts, deprecated `--manual` references
- **`docs/ALGORITHMS.md`**: Added `vector` and `file` capabilities; updated version status
- **`docs/ARCHITECTURE.md`**: Replaced `--manual` with `all`/`<dbtype>`; updated directory structure
- **`docs/POLICY.md`**: Added troubleshooting reference table (4 common issues)
- **`README.md` / `README_EN.md`**: Simplified by ~62% (541→207 / 540→194 lines), moved detail to docs/
- **`issues.json`**: Merged ISSUE-062.md content as ISSUE-064; resolved numbering collision

### Test Framework Expansion
- **`docs/test/12-capability-routing.md`**: New test suite covering CapSQL routing, PostgreSQL multi-schema, matchStarSelect with CTE, file data source policy, JSON instances wrapper format
- **`docs/test/02-schema-collection.md`**: JSON validation updated for `instances` wrapper format
- **`docs/test/11-end-to-end.md`**: JSON structure expectations aligned with v0.1.0 output format
- All 15 DSN schema collection verified; all 8 unit test packages pass

### File Query Engine (Pure Go In-Memory SQL Engine)
- **`src/connector/filequery/` — 7 new files**: Pure Go dependency-free SQL engine for CSV/XLSX business analysis
- **AST + Lexer + Recursive Descent Parser**: `ast.go` / `lexer.go` / `parser.go` — supports SELECT, WHERE, GROUP BY, ORDER BY, JOIN, LIMIT/OFFSET, aggregate functions, CAST/ABS/LIKE/IN/BETWEEN
- **Hash JOIN engine**: Cross-file JOIN via hash index; column name disambiguation (qualified `t.col` vs unqualified); JOIN sources auto-loaded via `resolveJoinSources()` in execute.go
- **Expression evaluator**: `evaluator.go` — comparison/arithmetic/LIKE/IN/AND/OR operators, column arithmetic, CAST type conversion
- **Hash aggregation**: `aggregate.go` — SUM/AVG/COUNT/MAX/MIN aggregate functions
- **44 unit tests**: Covering full grammar paths and edge cases
- **Architecture consistent**: Connector interface unchanged, Queryable interface unchanged, CapFile tag unchanged, policy engine agnostic

### QA Scenario Expansion (Q09-Q15)
- **7 new business analysis scenarios**: GROUP BY + AVG, ORDER BY + LIMIT, CAST + column arithmetic, GROUP BY date, AND multi-condition, cross-file JOIN, nested arithmetic + ABS
- **`testdata/qa/questions/Q09-Q15.md`**: New question files with business context + verification SQL + expected output
- **`testdata/qa/.env.qa-touch-join`**: New cross-file JOIN test configuration
- **`docs/test/13-file-query-engine.md`**: New L7 test document, 10/10 verification items passed

### Bug Fixes
- **CSV UTF-8 BOM auto-strip**: `readCSVData()` detects EF BB BF prefix, fixes first column `csmgr_refno` showing empty
- **JOIN source DSN filtering fix**: `execute.go` was filtering by label before JOIN source resolution; changed to collect all entries then use `filterEntries()`
- **JOIN alias overwrite fix**: `executor.go` added existence check on sources map, preventing nil overwrite when alias is missing
- **Error visibility fix**: csv.go now passes through underlying parse errors instead of masking with ErrNotSupported
- **`resolveDSNEntries()` removed**: Replaced by inline loading + `filterEntries()`

## v0.0.9 (2026-05-28)

### CSV/TSV/XLSX File Processing
- **CSV/TSV Schema Collection**: New `csv://` / `tsv://` DSN schemes supporting 3 path modes — single file, directory scan, and glob patterns (`*`/`?`/`[`). First row as column names, sampled type inference (INTEGER > FLOAT > DATE > TEXT)
- **XLSX Schema Collection**: New `xlsx://` DSN scheme iterating all sheets as tables. Built into main module, standard build includes it (`github.com/xuri/excelize/v2` is a permanent dependency)
- **Encoding Support**: UTF-8 default, `?encoding=gbk` parameter for GBK/GB2312/GB18030 encoded files
- **Custom Delimiter**: CSV defaults to comma, TSV defaults to tab, overridable via `?delimiter=tab|pipe|semicolon`
- **Shared Type Inference**: New `connector/infer.go` with priority-ordered column type detection (INTEGER → FLOAT → DATE → TEXT)
- **Read-only Query Execution**: `execute` subcommand supports CSV/TSV/XLSX — `SELECT * [LIMIT N [OFFSET M]]` only. File queries bypass sqlguard sandbox and policy engine (files are inherently read-only)
- **CLI Help Subcommands**: `dbexplain csv` / `dbexplain xlsx` print bilingual reference manuals

### Documentation
- `docs/FILE_PROCESSING.md`: Dedicated CSV/TSV/XLSX file processing documentation (new)
- `docs/test/`: Layered test documentation directory (new, 12 files covering all features)
- `README.md` / `README_EN.md`: Added CSV/XLSX entries to supported data sources; updated download URL versions
- All install/uninstall scripts version strings updated

### Output Log Optimization
- **Progress messages routed to log files**: `[采集中]` / `[完成]` moved from stderr to per-label log files (`/var/log/dbexplain/<label>.log`), no longer polluting `--json` / `--human` output
- **Third-party library warnings redirected**: `log.SetOutput()` redirects Qdrant client etc. stderr warnings to `/var/log/dbexplain/dbexplain.log`
- **Collection summary log**: New `collect.log` recording total collection duration or failure summary

### CLI & UX
- **`--human` after query**: Go flag stops at first non-flag arg; `execute "SELECT 1" --human` now works by scanning `fs.Args()` after parse
- **`--label` global flag**: Schema collection mode now supports `--label` as alias for `-include`, consistent with execute subcommand

### Policy Engine Fixes (ISSUE-062)
- **`DENY_TABLES=schema` prefix matching**: `extractTableNames()` previously only extracted `TABLES` (dropping `information_schema.`), making `DENY_TABLES=information_schema` ineffective. Fixed to capture fully-qualified names `information_schema.TABLES` and split into schema + table parts
- **`DENY_COLUMNS=table.col` wildcard query bypass**: SQL `SELECT * FROM table` had no explicit column refs, bypassing column-level checks. Added `matchStarSelect()` to detect `SELECT *` and match table prefixes
- **MongoDB/Qdrant native query column bypass**: `CheckNative()` previously skipped column-level checks. `{"find":"collection"}` returning all fields now checks `DENY_COLUMNS=collection.field`, unless the projection excludes that field

### Single Binary Architecture (Merge)
- Merged `build_excel.sh` + `src_excel` sub-module into main module, `github.com/xuri/excelize/v2` as permanent compile dependency
- Single binary ~41MB, zero runtime dependencies, xlsx adds ~2.1MB (~5%)

### Version Tracking
- Version: v0.0.9
- New data source types: CSV/TSV/XLSX files

## v0.0.8 (2026-05-27)

### Security Policy Engine (ISSUE-061)
- **Fine-grained access control**: New `src/policy/` package with 3-level deny — statement-level (substring match), table-level (table/collection/key name extraction), column-level (`table.column` reference matching), providing a second layer of access control after `sqlguard` validation and before query execution
- **All 9 database types covered**: SQL types (MySQL/PostgreSQL/GaussDB/SQLite/ClickHouse/Elasticsearch) support all 3 levels; MongoDB/Qdrant support statement+collection level; Redis supports statement+key level (with wildcard matching)
- **Global + per-DSN config**: `DENY_TABLES`/`DENY_COLUMNS`/`DENY_STATEMENTS` support global config and `DB<n>_` prefix for per-DSN appending
- **Column value masking**: `MASK_COLUMNS` replaces sensitive column values post-execution (e.g. `password_hash=***`), as an alternative to hard blocking. Supports glob matching, works across all database types
- **Dedicated documentation**: New `docs/POLICY.md` with per-database deny rules and configuration examples
- **Unit tests**: 39 test cases (Load/CheckSQL/CheckNative/Extract full coverage) + 10+ regression tests for security bypass vectors

### Credential Protection
- **DSN error sanitization**: New `sanitizeErr()` function redacts passwords from DSN parse errors before stderr output, preventing credential leakage from malformed DSN strings
- **OS env isolation**: `loadEnvFile()` refactored to return `[]dsnEntry` directly, eliminating DSN password residue in process environment from the `os.Setenv`→`os.Getenv` round-trip. Non-DSN config items unaffected
- **ClickHouse header-based auth**: `chHTTP.query()` auth changed from URL query params to `X-ClickHouse-User`/`X-ClickHouse-Key` request headers, preventing password leakage in HTTP logs or Referer headers (closes ISSUE-043)

### Policy Bypass Prevention
- **Quoted identifier normalization**: `extractTableNames()`/`extractColumnRefs()` pre-process via `normalizeIdentifiers()`, stripping backtick/double-quote/bracket quotes before extraction. Prevents ``SELECT * FROM `sensitive` `` from bypassing table-level deny
- **Whitespace normalization**: `CheckSQL()`/`CheckNative()` pre-process via `normalizeWhitespace()`, collapsing all whitespace sequences to single spaces. Prevents `DROP  TABLE` from bypassing statement-level patterns
- **Redis glob rewrite**: `filepath.Match` treats `/` as path separator, so `CONVERSATION:*` didn't match `CONVERSATION:abc/123`. Replaced with custom `globMatch()` that treats all characters equally
- **Subquery LIMIT hardening**: `AutoLimit()` uses `hasOuterLimit()` which strips parenthesized subquery content before LIMIT detection. Prevents `SELECT * FROM (SELECT ... LIMIT 99999) AS t` from bypassing auto-injection

### Output Safety
- **Terminal injection defense**: `--human` output sanitized via `sanitizeCell()`, stripping ANSI escape sequences (ESC+`[...`+letter) and control characters (0x00-0x1F, 0x7F), preserving tab/newline/CR. JSON output uses Go `json.Encoder` native escaping, no extra handling needed. Applies to all 9 database types
- **Column width cap**: `formatHuman()` caps column width at `maxColWidth=256`, truncating oversized cells with `…` indicator. `--human` only, prevents oversized cells from overwhelming terminal/memory

### Connectivity & Concurrency
- **ES TLS verification parameterized**: New DSN parameter `?tls-skip-verify=true` replaces hardcoded `InsecureSkipVerify` (closes ISSUE-042). ES help docs updated
- **Schema collection concurrency limit**: New `--conn N` flag (default 10), using channel semaphore to limit concurrent schema collection goroutines

### CLI & Diagnostics
- **`list` index alignment**: `dbexplain list` INDEX column changed from `envKey` (DB1/DB2) to sequential numbers (1/2/3), aligning with `execute --db N` 1-based positional index
- **Malformed glob warnings**: Added `log.Printf` warnings for `globMatch()` and `filepath.Match()` error-drops in `policy.go`, making misconfigured patterns discoverable

### Documentation Updates
- `docs/POLICY.md`: Security policy engine documentation (new)
- `docs/EXECUTE.md`: Security architecture section expanded with bypass prevention and output safety
- `docs/SECURITY_CHECKLIST.md`: 10+ new check items (credential protection/input validation/runtime safety/transport security)
- `docs/CLICKHOUSE.md`: Auth method updated (URL params → request headers)
- `docs/ELASTICSEARCH.md`: TLS description updated (hardcoded skip → `?tls-skip-verify=true` parameter)
- `src/execute_test.go`: 13 new test cases (sanitizeCell/formatHuman full coverage)
- Deleted `docs/TEST_v0.0.7.md`, created `docs/TEST_v0.0.8.md`

### Tracking Issues
- **ISSUE-061**: Fine-grained security policy engine (implemented in v0.0.8)
- **ISSUE-034**: GaussDB/TDSQL compatibility documentation (implemented in v0.0.8)
- **ISSUE-042**: ES InsecureSkipVerify hardcoded (v0.0.8 closed)
- **ISSUE-043**: ClickHouse URL password leak (v0.0.8 closed)

## v0.0.7 (2026-05-26)

### Go Module Publishing (REQ-1)
- **Module path**: `module dbexplain` → `module github.com/IamWWT/dbexplain`, Go-standards compliant
- **18 files, 44 import lines** updated to full module path
- **Public API**: New `src/core/` package exporting `Collect()` / `CollectToGraph()` / `CollectToJSON()`, directly importable by Go projects like VeinMap
- **IR Graph builder**: `src/core/graph.go` — `BuildGraph()` converts schema.Instance to IR Graph (nodes + columns + edges)

### Schema Enhancements (REQ-2, REQ-3, REQ-6, REQ-7)
- **ForeignKey completion**: Added `OnDelete` / `OnUpdate` fields (CASCADE, SET NULL, RESTRICT, NO ACTION)
- **SQLite FK collection**: Existing on_update/on_delete from `PRAGMA foreign_key_list` now correctly stored in ForeignKey struct
- **MySQL FK enrichment**: Added `information_schema.REFERENTIAL_CONSTRAINTS` query for DELETE_RULE / UPDATE_RULE
- **PostgreSQL FK enrichment**: FK query now includes `confupdtype` / `confdeltype`, with `pgFKAction()` mapping single-char codes to readable strings
- **JSON refs enhancement**: `jsonRef` now includes 8 structured fields (from_instance/from_db/from_table/from_col/to_instance/to_db/to_table/to_col), while preserving from/to for backward compatibility
- **IR Graph edge metadata**: `BuildGraph()` outputs constraint_name / on_delete / on_update in Edge Metadata

### Bug Fixes (REQ-5)
- **SQLite INTEGER PRIMARY KEY nullable fix**: Changed `c.Nullable = notnull == 0` to `c.Nullable = notnull == 0 && pk == 0`, preventing SQLite auto-increment PKs from being incorrectly marked nullable

### Runtime Resiliency (REQ-4)
- **Log directory fallback**: When `/var/log/dbexplain` is not writable, auto-fallback to `$XDG_STATE_HOME` → `$HOME/.local/state` → `os.TempDir()`, fixing log write failures in containers/non-root environments
- **`resolveLogDir()`**: New multi-level fallback helper function

### Security Audit (REQ-8)
- **Full-chain password audit**: Reviewed all 8 connectors + render.go + main.go output paths
- Confirmed zero password leakage across JSON output (Redacted DSN), label fields, log files (Redacted), and -context output (name-only)

### Read-Only Query Execution (REQ-10)
- **`dbexplain execute`**: New execute subcommand for sandboxed read-only queries, returning structured data tables (output format fully separated from schema collection JSON)
- **sqlguard read-only validation**: New `src/sqlguard/` package with triple-layer protection — verb whitelist (SELECT/EXPLAIN/WITH/SHOW/DESCRIBE/DESC/PRAGMA), multi-statement detection (rejects semicolon concatenation), auto LIMIT injection (appends `LIMIT 1000` when missing)
- **query execution engine**: New `src/query/` package defining `Queryable` interface (separate from `Connector`), `QueryResult`/`ExecuteOpts` unified types, `QueryLock` per-label concurrent mutex
- **All 9 database types covered**: 5 SQL databases (MySQL/PostgreSQL/GaussDB/SQLite/ClickHouse) use sqlguard validation + `database/sql` execution; Elasticsearch supports standard SQL via `_sql` REST endpoint
- **Non-SQL native query support**:
  - Elasticsearch: `_sql` REST endpoint, returns `{"columns": [...], "rows": [...]}`
  - MongoDB: JSON format `{"find":"collection","filter":{...},"limit":100}` / `{"aggregate":"collection","pipeline":[...]}`
  - Redis: Space-separated native commands, 30+ command whitelist (GET/HGETALL/SCAN/PING etc.), rejects SET/DEL write ops
  - Qdrant: JSON format `{"scroll":"collection_name","limit":100}` / `{"count":"collection_name"}`
- **Query routing**: `isSQLKind()` routes by DSN type — SQL types through sqlguard, non-SQL types through per-connector internal whitelists
- **Dual timeout protection**: Application context timeout + database-level statement timeout (MySQL `max_execution_time` / PG `statement_timeout` / CH `max_execution_time`)
- **Security documentation**: New `docs/EXECUTE.md` covering security architecture, output format, usage examples, and CONSTITUTION compliance
- **`--human` table output**: New `--human` flag for execute — renders query results as ASCII table (MySQL/pg CLI style) instead of default JSON. NULL values clearly displayed, auto-width columns. Works across all 9 database types
- **CLI example library**: New `docs/CLI_EXAMPLES.md` covering 7 active data sources with 13 executable queries, all verified against the live environment

### Security Enhancements
- **Redacted() credential sanitization fix**: URL-encoded passwords (e.g. `%23`) no longer leak; both username and password sanitized to `{dbuser}:{dbpassword}` placeholders, replacing the old `user:***` format
- **`dbexplain list` subcommand**: Lists INDEX/LABEL/KIND/HOST:PORT/DATABASE mapping for all configured databases, zero credential exposure, encrypted `.env` auto-decrypted
- **`-env` DSN mapping summary**: Before collection, prints `DB1 → label (kind://{dbuser}:{dbpassword}@host/db)` mapping for confirming `--db N` / `--label` correspondence

### Test Coverage (v0.0.7 Reinforcement)
- **sqlguard unit tests**: 28 cases — Validate() full verb whitelist/blacklist, multi-statement edges/empty queries/leading whitespace/parenthesized CTEs; AutoLimit() appends/skips/trailing semicolons/case-insensitive detection
- **query unit tests**: 15 cases — QueryLock lock/unlock/concurrent mutex/multi-label independent/re-entry verification/scale testing
- **MongoDB/Redis live verification**: openim-redis:6389 + video-redis:6379 + mongo-test:27017 end-to-end execute testing completed
- **Bug fix**: Redis ExecQuery Do() argument omission (command name not passed to go-redis, fixed)
- **Total test cases**: 231+ → 120 unit (dsn:33 + schema:44 + sqlguard:28 + query:15) + 111 integration/CLI

### Tracking Issues
- **ISSUE-054 ~ ISSUE-060**: 7 new requirement tracking issues for v0.0.7

## v0.0.6 (2026-05-21)

### Config Encryption
- **`dbexplain encrypt`**: New encrypt subcommand for encrypting `.env` config files with machine fingerprint
- **Machine-only mode (default)**: Key derived from hardware characteristics (machine-id/motherboard UUID/CPU model/hostname), no password needed, file only decryptable on the same machine
- **Password-enhanced mode**: `encrypt --password` provides PBKDF2-HMAC-SHA256(100k) dual protection (password + machine fingerprint)
- **Runtime auto-decrypt**: `-env` mode auto-detects encrypted files (header byte 0x00/0x01), no extra flags required
- **`APP_ENCRYPTION_KEY`**: Password-mode decryption via this environment variable (optional override; default reads from `~/.config/dbexplain/.encryption_key` file)
- **Cross-platform pure Go**: Linux (`/etc/machine-id`, DMI, `/proc/cpuinfo`), macOS (`sysctl hw.*`), Windows (Registry MachineGuid), CGO_ENABLED=0
- **Algorithm**: XChaCha20-Poly1305 (AEAD) + SHA-256 / PBKDF2-HMAC-SHA256 key derivation
- **Security**: Output file permissions `0600`, password input without echo, generic decryption error messages

### Config Search Enhancement
- **findConfigFile()**: Added `.env.dbexplain.enc` and `.env.enc` to search order, unified priority for encrypted and plaintext files

### Documentation
- `README.md` / `README_EN.md`: New "Encrypt Config Files" section with full usage examples
- `--manual` updated with complete encrypt subcommand documentation (bilingual)
- `-h` updated with "Encryption" option group
- `docs/SECURITY_CHECKLIST.md`: New "Config Encryption Checks" section
- `docs/ARCHITECTURE.md`: New "Config Encryption Architecture" section
- `.gitignore`: Added `*.enc` exclusion rules

### CLI Subcommand Hierarchy Restructure
- **`dbexplain <dbtype>`**: 9 database type subcommands (mysql/postgres/gaussdb/clickhouse/sqlite/redis/mongodb/elasticsearch/qdrant), each outputs a database-specific reference manual
- **Alias support**: `postgres`=`pg`/`postgresql`, `clickhouse`=`ch`, `sqlite`=`sqlite3`, `elasticsearch`=`es`
- **`dbexplain all`**: Replaces `--manual`, full reference manual. Supports `--filter` keyword search and `--language zh|en`
- **`dbexplain -h`**: Redesigned as a compact structured overview (Usage / Database types / Flags / Examples / See), upgraded from flat 8-group flag dump
- **Backward compat**: `--manual` still works, prints deprecation note on stderr guiding users to `dbexplain all`

### Install Script Enhancements
- **Removed `DBPROBE_ENV_FILE` interactive prompt**: `findConfigFile()` auto-discovery eliminates manual config, install scripts no longer ask to set this env var
- **Encryption guidance**: `install.sh` / `install.ps1` / `install-skill.sh` success messages now include encryption steps
- **`dbexplain all`**: Install script help and success messages now reference `dbexplain all` instead of `dbexplain --manual`

### Tracking Issue
- **ISSUE-053**: Consider removing plaintext `.env` support in a future major version, keeping only encrypted files (`open`, `security/breaking-change/future`)

## v0.0.5 (2026-05-21)

### One-Click Install & Deployment
- **`scripts/install.sh`**: Linux/macOS one-click installer with online (GitHub Releases) and offline modes
- **`scripts/install.ps1`**: Windows PowerShell one-click installer with automatic user PATH configuration
- **`scripts/uninstall.sh` / `scripts/uninstall.ps1`**: Companion uninstall scripts with silent mode (`--all`)
- **`scripts/install-skill.sh`**: Multi-platform skill deployment script (interactive target selection)
- **`scripts/uninstall-skill.sh`**: Skill uninstaller script
- **Global install**: Binary installed to system PATH (Linux/macOS: `/usr/local/bin/dbexplain`, Windows: `%LOCALAPPDATA%\dbexplain\`)
- **User-level config**: `.env.dbexplain` per XDG spec (`~/.config/dbexplain/`), with optional `DBPROBE_ENV_FILE` for custom paths

### Config Search
- **Multi-level fallback**: `DBPROBE_ENV_FILE` → `.env.dbexplain` (CWD) → `~/.config/dbexplain/.env.dbexplain` → `.env` (CWD, legacy compat)
- No more `cd <skill-dir>` required — tool runs from any directory in `-env` mode

### New Options
- **`--log-dir <dir>`**: User-specifiable log output directory (default `/var/log/dbexplain`), affects `filter.log` and per-instance logs

### Skill Adaptation
- **SKILL_ZH.md / SKILL_EN.md**: Skill split into Chinese and English versions; `SKILL.md` kept as Chinese copy for platform auto-discovery
- **SKILL.md**: Removed `cd <skill-dir>` requirement, updated to global `dbexplain` invocation, added multi-level config search path docs
- **Skill installer**: Prioritizes system `dbexplain` in PATH; skill directory binary is now `dbexplain` symlink (platform-agnostic name)
- **`--lang zh|en`**: `install.sh` and `install-skill.sh` new language option for installing Chinese or English Skill
- **Version**: install/uninstall skill scripts bumped to v0.0.5

### Documentation
- `--manual` updated: config search priority section, `--log-dir` option, all `./dbexplain` → `dbexplain`
- **New `docs/SECURITY_CHECKLIST.md`**: Pre-release security audit checklist covering credential protection, file encoding, input validation, and more

### Bug Fixes (13 items)

| Issue | Severity | Description |
|-------|----------|-------------|
| ISSUE-040 | CRITICAL | `.env` real credentials removed from Git tracking; `.gitignore` added `src/.env` |
| ISSUE-041 | HIGH | `src/logs/` production log directory added to `.gitignore` to prevent DB name leaks |
| ISSUE-044 | LOW | Deleted `analyze/infer.go` dead code, eliminating `strings.Contains(name, "ip")` false-match bug |
| ISSUE-045 | MEDIUM | Added `RowCount > 0` guard for PostgreSQL sample row fetch, aligning with MySQL/ClickHouse |
| ISSUE-046 | LOW | `longestCommonPrefix` preserves full prefix when no `_`/`-` separator, preventing empty cluster names |
| ISSUE-047 | MEDIUM | GaussDB instance Kind fixed from hardcoded `"postgres"` to DSN-specified `"gaussdb"` |
| ISSUE-048 | MEDIUM | JSON output now includes `op_stats` field (seq_scan/idx_scan/query_count etc.) |
| ISSUE-049 | LOW | MySQL dual `SHOW INDEX` queries merged into one, halving network round-trips |
| ISSUE-051 | HIGH | `-json -o` output no longer prepends UTF-8 BOM, ensuring standard JSON parser compatibility |
| ISSUE-052 | HIGH | UTF-8 BOM in `.env.dbexplain` (Windows Notepad) caused parse failure; godotenv error messages leaked passwords |

### Known Security Limitations (2 items, open)

| Issue | Description |
|-------|-------------|
| ISSUE-042 | ES TLS `InsecureSkipVerify=true`, acceptable as diagnostic tool, long-term needs cert config |
| ISSUE-043 | ClickHouse password transmitted via URL query param; consider HTTP Basic Auth Header instead |

## v0.0.4 (2026-05-20)

### Core Architecture
- **IR v1**: Universal graph primitives (Node, Column, Edge) independent of database type
- **Capability Architecture**: Connectors declare capabilities; extractors work by capability, not DB type
- **Unified Diagnostics**: Deterministic issue detection runners (MissingPK, LargeTableWithoutIndex, NoTTL, StaleStream, etc.)

### New Features
- **Importance Ranking**: Multi-factor weighted scoring (graph_degree, fk_centrality, row_count, index_density, write_intensity, query_frequency) with graceful degradation when operational stats are unavailable
- **Context Compression**: Layered AI Agent output — `summary.json`, `topology.json`, `diagnostics.json`, `retrieval_chunks/`
- **Schema Fingerprinting**: SHA-256 hashing of columns, indexes, FKs for delta detection (`--cache` flag)
- **Operational Stats (Phase 3)**: Per-table query frequency and write intensity from built-in system catalogs (zero-config, graceful degradation)
- **`--manual` flag**: Comprehensive help documentation with per-DB sections and `--language zh|en` support

#### Feature → Output Mapping

| Feature | Trigger | Output Location | Effect |
|---------|---------|-----------------|--------|
| Importance Ranking | Always on | Terminal: table ordering; `--context`: `summary.json` `importance_score` field | Important tables listed first; AI agents prioritize them |
| Context Compression | `--context <dir>` | `summary.json` / `topology.json` / `diagnostics.json` / `chunks/*.md` | Layered structured output, ready for AI agent prompt injection |
| Schema Fingerprinting | `--cache <file>` | `<file>` snapshot + `<file>_delta.json` diff | Incremental change detection, cron-friendly monitoring |
| Operational Stats | Always on (graceful degradation) | `summary.json` `query_frequency` / `write_intensity` | Feeds into importance ranking; falls back when unavailable |
| Human-Friendly Output | `--human` | Terminal: `[table=]`/`[pattern=]` context markers | Explicitly labels data source types |
| Filter Logging | `-include` / `-exclude` | `logs/filter.log` | Skip messages kept out of terminal output |
| Full Manual | `--manual [--filter x] [--language en]` | Terminal stdout | 600+ line detailed docs organized by database type |
| File Output BOM | `-o <file>` | Auto-prepended UTF-8 BOM in output files | Correct Chinese rendering in Windows Notepad/CMD |

### Windows Compatibility
- **UTF-8 BOM**: Auto-prepended to `-o` file output for Windows Notepad/CMD encoding recognition
- **System code page detection**: On Windows, runtime ACP detection. Chinese systems (936) auto-convert to GBK for correct `type` command and Notepad display. Other locales keep UTF-8 BOM.
- **ANSI escape code fix**: `noColor` changed from init-time var to runtime func, preventing escape codes from leaking into captured file output
- **ASCII-safe rendering**: Replaced Unicode box-drawing (`─` U+2500), bullet (`•` U+2022), and ellipsis (`…` U+2026) with ASCII equivalents

### Bug Fixes
- Fixed TOCTOU race window in `GetConnector`
- Fixed password leak in DSN filter skip messages (`parsed.Redacted()` instead of `e.raw`)
- Fixed terminal color output loss (only pipe-capture on `-o`; direct stdout for terminal rendering)
- Fixed `go vet` non-constant format string warnings (`fmt.Fprintf` → `fmt.Fprint`)

### UX Improvements
- **`--filter` flag**: `--manual --filter <keyword>` filters manual output by keyword (case-insensitive), enabling quick lookups within the ~600-line manual
- **Reorganized `-h`**: Upgraded from alphabetical flag dump to 7-group categorized output (Input Sources / Filtering / Output Control / Display Format / AI Context / Performance / Help), bilingual via `--language`
- **`-h` bilingual**: `-h --language en` outputs English help (Chinese by default); pre-scans `--language` before flag parsing
- **`--human` flag**: Human-friendly output with `[table=]`/`[pattern=]`/`[database=]`/`[instance=]` context markers
- **Context markers**: Type-specific labels per database kind (SQL=table, Redis=pattern, MongoDB/Qdrant=collection, ES=index)
- **Filter log redirection**: `-include`/`-exclude` skip/exclude messages now written to `logs/filter.log` instead of polluting terminal output, keeping reports clean for both humans and AI

### Documentation
- `docs/ARCHITECTURE.md`: Project vision as Database Context Compiler, added Security section (password leak prevention as top priority)
- `docs/ALGORITHMS.md`: Full algorithm reference with compatibility matrices and fallback mechanisms
- `docs/TEST_METHODOLOGY_v0.0.4.md`, `docs/TEST_REPORT_v0.0.4.md`: Layered test methodology and report (83+ cases, with actual shell execution output)
- README new "Usage Scenarios" chapter (AI Agent / Human / 9 per-DB examples)
- `MEMORY.md` new version performance comparison section (mandatory for each release)
- Constitution updated with IR-first, Deterministic-only, and Graph-first principles

---

## v0.0.3

- Multi-schema collection (PostgreSQL/GaussDB)
- SSL mode configuration
- DSN filtering (`--include`/`--exclude`)
- Unit tests and CI/CD pipeline
- Skill install/uninstall scripts

## v0.0.2

- Concurrent collection with goroutines
- Panic isolation per connector
- Redis streaming key analysis with pattern inference
- Column comment inference from sample rows
- Connector self-registration pattern
- Progress logging for large tables

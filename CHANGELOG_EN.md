# Changelog

## v0.1.3 (2026-06-03) — DuckDB Optional Connector + Dual-Build Release

### ✨ New Features

- **DuckDB Embedded SQL Engine**
  Supports in-memory mode (`duckdb:///:memory:`) and file database mode (`duckdb:///path/to/file.db`). Full metadata collection via system functions `duckdb_tables/columns/constraints` + row counting + sampling, plus `ExecQuery`, DSL `@label.duckdb` binding, and `EXPLAIN` format adaptation. Forces `access_mode=READ_ONLY` for read-only safety.

- **DuckDB File Analysis Engine**
  Analyze Parquet/CSV/JSON files directly via `read_parquet()`/`read_csv_auto()`/`read_json()`. The `allowed_path` parameter controls readable directories, preventing path traversal.

- **Dual-Release Pipeline**
  - **Standard edition (`-std`)**: Pure Go, no CGO dependencies, cross-compiled for 5 platforms (Linux/Windows/macOS amd64/arm64)
  - **DuckDB full edition (`-duckdb`)**: Current platform + DuckDB support, CGO requires gcc/clang
  `release.sh` automates the two-phase build process.

- **CLI Help Hierarchy Restructuring**
  Commands grouped under `Schema` / `Query` / `Utility` / `Help`; data sources categorized as `SQL` / `NoSQL` / `File`. Standard builds show DuckDB build hint, DuckDB builds show normal entry.

- **REPL DSL Syntax & Federated Query**
  REPL now auto-detects `@label.table` DSL syntax for single-source (SQL/file) and federated cross-source JOIN. Queries containing `@` route through the DSL path; queries without it follow the existing single-source path. Added 4 REPL-safe DSL dispatch functions.

### 🐛 Fixes

- **DuckDB DSN Parsing**
  `duckdb://:memory:` misparsed by Go's standard library as a port number → changed to `duckdb:///:memory:` (triple slash), with a dedicated handling branch in the connection string builder.

- **Missing Subcommand Registration**
  `main.go` was missing the `duckdb` case, causing silent exit → now added.

- **Goroutine Panic Recovery**
  Three locations missing `defer/recover()` (Schema collection goroutine outer layer, output capture `io.Copy` goroutines) — all patched to prevent single-point crashes from bringing down the process.

- **Path Prefix Matching Security Hole**
  `allowed_path` used `strings.HasPrefix` without a separator guard (e.g., `/data` incorrectly matched `/data_backup`) → added trailing separator check.

- **Error Log Loss**
  - XLSX query original error swallowed by `ErrNotSupported` → added `log.Printf` to preserve context
  - ClickHouse/ES `io.ReadAll` errors silently discarded via `_` → changed to log recording
  - Delta/Diff JSON `WriteFile`/`MarshalIndent` errors silently ignored → added error logging

- **CJK Table Alignment**
  `fmt.Sprintf("%-30s")` padded by byte count, causing Chinese character misalignment in chart view → replaced with visual-width-aware `pad(inst.Label, 30)`.

### 🏗️ Build & Release

- **CGO Introduction & Constitutional Exception**
  Project's first CGO dependency (DuckDB Go driver embeds C++ engine). `CONSTITUTION.md` Article 4 amended with an exception clause.

- **UPX Maximum Compression**
  - DuckDB full edition: 100 MB → 23 MB (-77%)
  - Standard edition: 42 MB → 9.1 MB (-78%)
  Full verification chain: `file`/`ldd`/`nm -D`/`upx -t` all passed.

- **Build Tag Isolation**
  DuckDB requires explicit `-tags duckdb` + `CGO_ENABLED=1`, excluded from the `full` tag set. Stub files provide friendly build hints.

- **Full Verification**
  `go build ./...` / `go vet ./...` / `go test ./...` + `bash build.sh prod/minimal` + `bash release.sh` all passing.

### 📚 Documentation

- **Added** `DUCKDB.md` (usage guide) and `DUCKDB_IMPL.md` (implementation boundaries & security model)
- **README bilingual sync**: Source count 11 → 12, capability matrix gains DuckDB row, dev guide updated with `release.sh` and naming conventions
- **Test docs**: New `16-duckdb.md` (20 E2E tests), `RESULTS.md` updated (128/128), test overview updated (16 data sources)
- **SECURITY_CHECKLIST.md** adds DuckDB file path security check items
- **Install scripts**: `install.sh`/`install.ps1` version v0.1.2 → v0.1.3, download URL uses `-std` suffix
- **CHANGELOG bilingual sync**: This version fully recorded in both languages.
- **README architecture diagram**: Added `DBEXPLAIN-ARCH.png` system architecture diagram between capability matrix and core capabilities
- **README REPL DSL note fix**: Updated stale "no DSL mode" label to "supports DSL single-source & federated cross-source JOIN"
- **Test docs**: RESULTS.md updated to 130/130 test items, removed "REPL does not support DSL/federated queries" from known limitations

## v0.1.2 (2026-06-03) — CLI Enhancement + DSL Federated Query + Build System Optimization

### New Features
- **DSL Federated Query** (ISSUE-069): Cross-source JOIN/UNION support. Removed `len(kinds)>1` blocker, data materialization + filequery federated merge layer. SQL ↔ file ↔ mixed source JOIN fully supported
- **`dbexplain collect` subcommand** (ISSUE-072): Schema collection migrated from default behavior to explicit subcommand. `dbexplain collect -env --human` new path, backward-compatible fallback retained. `dbexplain` with no args shows help
- **CLI REPL mode** (ISSUE-070): `dbexplain repl` interactive query loop. `.conn` switch data source, `.help`/`.exit`/Ctrl+D, automatic timing and row count
- **`--explain` output formatting** (ISSUE-071): Database-specific EXPLAIN syntax (MySQL FORMAT=JSON, PostgreSQL ANALYZE BUFFERS, SQLite QUERY PLAN, ClickHouse PLAN). MySQL FORMAT=JSON --human readable rendering

### Fixes
- **DSL file JOIN fix** (ISSUE-069): `dslExecFile()` passed `nil allEntries`, causing file-source DSL JOIN resolution to fail. Changed to pass global entries
- **ClickHouse REPL trailing semicolon fix**: ClickHouse connector appends `SETTINGS max_execution_time=N FORMAT JSON` after query; trailing `;` caused multi-statement error. `repl.go` adds `TrimRight(";")` auto-strip
- **Elasticsearch REPL JSON query clear error**: Native JSON queries in REPL previously returned confusing `READ_ONLY_VIOLATION`. Added JSON pre-check with clear workaround message (`execute -env --label` SQL mode or `collect` schema collection)
- **Windows atomic write fix**: `cache.go` `os.Rename` fails when target exists. Added `os.Remove` before rename on `runtime.GOOS == "windows"`
- **CJK display fix**: `render/table.go` column width used `len()` (bytes) breaking Chinese/Korean/fullwidth character alignment. Added `visualWidth()` for display-width calculation with padding compensation
- **`render.go` `pad()`/`truncate()` UTF-8 fix**: Byte slicing could produce invalid UTF-8. Changed to `[]rune`-based visual width truncation
- **`dsn.go` `SQLitePath()` Windows fix**: Missing Windows `/C:` prefix stripping, causing absolute path DSNs to fail on Windows
- **`dsn.go` `FilePath()` security hardening**: Added `filepath.Clean()` to prevent path traversal

### Build & Release
- **Build Tags**: 10 connector files added `//go:build mysql || full` conditional compilation, enabling selective driver selection
- **`build.sh` 4 modes**: prod (5 platforms + full + UPX), dev (current + full), test (+race), minimal (custom tags)
- **UPX compression**: `--best --lzma` reduces full driver binary 42 MB → 9.5 MB (-78%), zero runtime dependency. `upx -t` integrity verified
  - Linux/Windows fully compressed; darwin (Mach-O) cross-compiled binaries skipped due to UPX 5.0.0 compatibility limitation — native macOS builds work fine
- **`build.sh --help`**: New `--help`/`-h` flag with full usage, Tag→Kind→DSN scheme panorama
- **`--no-upx`/`--upx` dynamic control**: Pass from anywhere in args, force skip or enable UPX
- **Binary security verification**: `file`(statically linked) / `ldd`(no dynamic refs) / `nm -D`(no dyn symbols) / `upx -t`(integrity) / isolated run — all passed
- Full `go build ./...` + `go vet ./...` + `go test ./...` + `bash build.sh` verification passing

### Documentation
- **Pre-release checklist supplement** (ISSUE-073): SECURITY_CHECKLIST.md §6 added version consistency, CHANGELOG completeness, issues.json validation, binary smoke test, artifact integrity, stale doc reference check
- **`docs/test/01-environment.md` expanded**: New §1.9 build mode factor analysis (compile time/size/functionality/UPX/security/scenario guide/verification/conclusions) with measured data
- **`docs/test/RESULTS.md` build optimization**: 5 tag combinations with measured sizes, panorama Tag→Kind→DSN scheme mapping, security verification table
- **`docs/DEPLOY.md` build table**: Mode descriptions updated with explicit GOOS/GOARCH lists
- **`docs/SKILL_AUTHORING.md` overhaul**: Karpathy context engineering principles — added context economy/verifiability principles, metadata entry emphasis, description formula, input definition section, failure handling rules, progressive disclosure guide, eval-first iteration flow, complete example template
- **SKILL_ZH.md / SKILL_EN.md refactored**: 330→197 lines (within 200-line cap). Removed SQL syntax/error tables (referenced via references/), added input definition/failure handling, streamlined DSL/delta detection/params redundancy
- **README Capability Matrix**: Replaced simple "Supported Data Sources" table with a 5-column capability matrix (Collect/SQL Query/REPL/DSL Federated/File Engine) × 11 data sources — one-glance panorama
- **`docs/test/RESULTS.md` consolidation**: Merged 3 redundant sections (v0.1.0/v0.1.1/duplicate v0.1.2) into a single v0.1.2 report, added "Closed-Loop Verification Fixes" section
- **`docs/REPL.md` updated**: Removed ClickHouse semicolon limitation (fixed); ES limitation expanded with detailed workarounds (SQL via `_sql`/collect)
- **`docs/test/09-cli-help.md` expanded**: Added ClickHouse semicolon and ES JSON test cases
- **REPL `.list`/`.databases` command** (ISSUE-074): New REPL dot commands to list all configured data sources with index, label, kind, and current connection marker
- **CONSTITUTION.md review and update**: Core deliverables corrected (removed unimplemented IR Product concept); Principle 3 split into Collect/Query phases with updated MongoDB descriptions; Build & Release section streamlined to DEPLOY.md reference
- **SECURITY_CHECKLIST.md enhanced**: New §5a pre-commit quick check (5 items, ~30s); §6 added 5 new pre-release checks (script version consistency, test doc version expectations, Markdown link validity, cross-platform version consistency, dev binary `-tags full` verification); §6 new "Historical Pitfalls" table
- **Full stale reference cleanup**: Fixed `../docs/CONFIG_SEARCH.md` → `../CONFIG_SEARCH.md` in 10 test docs; fixed broken links in DEPLOY.md and file_index.md; 20+ stale references corrected project-wide
- **Script version sync**: install.sh/uninstall.sh/install.ps1/uninstall.ps1 headers and `$VERSION` uniformly updated to v0.1.2 (previously lingering v0.1.0)

## v0.1.1 (2026-06-02) — Internal Restructuring + Unified DSL Query Entry

### Project Structure
- **Go standard layout**: Monolithic files split into `cmd/` + `internal/`: `main.go`(2482→910 lines) extracted config/encrypt/list/manual/output; `execute.go`(585→264 lines) extracted render/queryutil/dsnfilter/executor
- **Full internal/ migration**: 14 top-level packages moved to `src/internal/` by dependency order, old dirs cleaned; `src/` now only holds `cmd/` + `internal/` + build files
- **Shared SQL AST**: filequery lexer/parser/AST extracted as `internal/sqlast/`, reused by sqlguard/policy/dsl with Go type aliases, 60+ tests passing

### New Features
- **DSL query mode** (`--dsl`): Reference data sources via `@label.table` (e.g. `SELECT * FROM @my-mysql.users`). Pipeline: preprocess → sqlast parse → symbol binding → backend routing, full security chain synced
- **Schema Diff** (`dbexplain diff`): Field-level change detection (column/index/FK), 4 comparison modes (cache-baseline / since / two-file / list-versions), supports `--human` and JSON output, 23 unit tests
- **Window functions**: ROW_NUMBER / RANK / DENSE_RANK / NTILE / LAG / LEAD / FIRST_VALUE / LAST_VALUE / aggregate OVER + ROWS/RANGE frame specs, 36+ test cases

### Security Enhancements
- **AST-level validation**: sqlguard/policy prioritize `sqlast.Parse()` AST analysis, fall back to string matching; AutoLimit checks AST Limit field to avoid duplicate injection
- **AutoLimit duplicate injection fix**: `parseTableRef()` didn't handle `schema.table` qualified names, causing schema-prefixed queries to get double LIMIT. Added `parseQualifiedName()` for multi-part identifiers
- **Policy DENY_TABLES matching fix**: `extractTablesFromAST()` didn't expand `schema.table` for matching, so `DENY_TABLES=users` had no effect on `SELECT * FROM public.users`. Added table name splitting logic

### Documentation
- **README bilingual split**: README.md → symlink to README_ZH.md, new `README_EN.md` for independent English maintenance. Added doc navigation table, streamlined content
- **New `docs/USAGE_GUIDE.md`**: Comprehensive coverage of all 11 data sources from install to query, with Linux/macOS/Windows instructions
- **Stale content fixes**: CLI_EXAMPLES.md CSV section corrected (file engine supports full SQL); sql-syntax.md/troubleshooting.md window function labels updated to ✅ supported
- **docs/test/ cleanup**: Removed outdated PNG screenshots

### Testing
- **Unified standards**: All test docs using `cd src + BIN=../release/dbexplain` portable paths, no absolute path dependencies
- **New test docs**: 14-schema-diff.md(24 items) + 15-window-functions.md(36 items)
- **Full E2E verification**: 15 data sources 91/91 passed, covering DSL mode, Schema Diff, window functions

### Build & Release
- `build.sh` version bumped to v0.1.1
- `issues.json`: Fixed JSON syntax errors + ~70 stale path updates + 3 new issues, total 68

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

### ORDER BY Computed Alias Sorting Fix
- **Problem**: `SELECT ..., CAST(total AS FLOAT) / cnt * 100 AS ir ORDER BY ir DESC` — `ir` is a computed column alias (not a raw CSV column), `colMap` lookup fails causing no sorting to be applied
- **Fix**: `executor.go` ORDER BY comparator added alias fallback: when `colMap` lookup fails, search SELECT columns for matching alias and evaluate via `Eval()` before comparing

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

### File Query Engine Enhancements
- **NULLS FIRST/LAST**: ORDER BY supports `col DESC NULLS FIRST` / `col ASC NULLS LAST`; DESC defaults to NULLS FIRST, ASC defaults to NULLS LAST when no direction specified
- **UNION / UNION ALL**: `parseSingleSelect()` extracted; new `UnionStmt` AST node; `executeUnion()` + `mergeResults()` — UNION ALL concatenates, UNION deduplicates via row-value hash
- **DISTINCT ON**: After ORDER BY sort, keeps first row per distinct ON column group; PostgreSQL-compatible semantics
- **Subquery IN / NOT IN**: `SubqueryExpr` AST node + `subqueryCache` pre-computation cache; `parseComparison()` handles both prefix NOT (`NOT col IN (...)`) and postfix NOT (`col NOT IN (...)`), also NOT LIKE / NOT BETWEEN
- **66 unit tests** (44 original + 22 new): NULLS lex/parse/exec, UNION ALL parse/exec, UNION dedup, DISTINCT ON parse/exec, subquery IN/NOT IN full pipeline

### File Query Engine Enhancements v2 — SQL Compatibility Extension
- **Double-quoted string literals**: New `readDoubleQuotedString()` function, both `"value"` and `'value'` are accepted. MySQL-compatible, no longer throws parse error on double-quoted SQL
- **IS NULL / IS NOT NULL**: New `IS` keyword, null-value predicate (CSV empty strings treated as NULL). MySQL/PostgreSQL compatible
- **HAVING clause**: Post-GROUP BY filter supporting SELECT column aliases in aggregate conditions. MySQL/PostgreSQL compatible
- **LEFT JOIN / RIGHT JOIN**: `JoinClause` now has `JoinType` field; hash JOIN engine extended for left/right outer join semantics. MySQL/PostgreSQL compatible
- **XLSX multi-sheet support**: `ExecQuery` matches sheet by SQL FROM table name; `resolveJoinSources` loads all sheets as independent NamedData. Each sheet queryable as a separate SQL table
- **ROUND single-argument form**: `ROUND(col)` defaults to 0 decimal places, no longer requires explicit `n`
- **Multi-DSN error improvement**: When multiple DSNs match, all available labels and file paths are listed, helping agents select the correct `--label`
- **File-not-found hint**: CSV/XLSX `os.Open` failure now gives explicit `file not found: <path> (use absolute path)`
- **`references/sql-syntax.md`**: Standalone SQL syntax reference file; SKILL.md simplified and references it
- **SKILL.md tone improvement**: Changed from "not supported" to "full SQL reference at sql-syntax.md", more agent-friendly

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

### File Query Engine Correctness Fixes
- **CAST conversion returning "0" fix**: `CAST(x AS INTEGER/FLOAT)` on failure returned `Value("0")` instead of the original value; changed to return `val`
- **SUM returning "0" for all-non-numeric groups fix**: When all values in a group were non-numeric, SUM returned `"0"`; changed to return `""` (SQL NULL semantics, consistent with MAX/MIN)
- **AVG count==0 returning "0" fix**: When all group values were non-numeric, AVG returned `"0"`; changed to return `""`
- **Eval error silent swallow fix**: `buildResult()` column projection and `executeAggregation()` expression evaluation silently returned `""` on Eval errors; changed to propagate errors
- **JOIN column map out-of-range fix**: After hash JOIN, `colMap` rebuild only used primary table alias; JOIN table qualified column indices used `len(primaryHeader)` offset causing out-of-range access. Changed to build colMap per-source with correct offsets

### Third-Party Distribution Package
- **`testdata/account-manager-skill/`**: Self-contained package for third-party customization; QwenPaw agent reads `SKILL.md` directly from directory
- **Layout**: `SKILL.md` + `assets/`(5 pre-compiled platform binaries) + `scripts/`(install/uninstall) + `references/`(table specs) + `.env.example`
- **Offline install**: `bash scripts/install.sh --offline ./assets/dbexplain-linux-amd64`; auto-detects in assets/ when no path specified
- **SQL transparency**: SKILL.md switched from "unsupported list" to "complete supported syntax table" — AI agent never guesses

### macOS Gatekeeper Compatibility
- **Quarantine auto-removal in installers**: Both `install.sh` scripts (dbexplain-skill + account-manager-skill) added `remove_quarantine()` — automatically runs `xattr -d com.apple.quarantine` after binary install on macOS
- **SETUP.md**: Added macOS Gatekeeper notes and manual workaround instructions

### dbexplain-skill Best Practice Generalization
- **SKILL_ZH.md / SKILL_EN.md**: New sections — "Traceable Analysis" (no fabricated data, per-conclusion SQL citations), "File Query Best Practices" (preview → clarify → analyze), "Error Handling (9+3 classification)"
- **`references/sql-syntax.md`**: New, generalized from account-manager-skill with neutral column names (`sales_data`/`department`/`employee_id`), scoped to file datasources
- **`references/troubleshooting.md`**: New, generalized from account-manager-skill and expanded with DB connection troubleshooting (DNS/auth/timeout/SSL), organized in 9+3 categories
- **install-skill.sh**: Now deploys `references/` directory on install/update; `--verify` checks its integrity

### Version Tracking
- Version: v0.1.0
- All 38 doc-code discrepancies resolved

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

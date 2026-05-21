# Changelog

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

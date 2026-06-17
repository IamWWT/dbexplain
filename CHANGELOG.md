# 变更日志

## v0.1.8 (2026-06-17) — `-env` 参数彻底移除（Breaking Change）

### 💥 Breaking: `-env` 参数彻底移除

- **`-env` 参数从所有子命令中移除**：`-env` 标志已无实际用途——核心逻辑 `shouldLoadEnv := *useEnv || !hasExplicitSource` 使得没有 `-dsn`/`-config` 时已自动加载 `.env.dbexplain`。现在自动加载是唯一行为路径。
- **不再支持 `--env=false`**：`dbexplain check --env=false`（跳过 .env 加载的唯一方式）不再可用。需要仅使用指定 DSN 时，请用 `-dsn` 替代。
- **受影响命令**: `dbexplain`、`collect`、`execute`、`repl`、`list`、`check`、`encrypt`、`diff`
- **影响范围**: 源码 7 文件 + 38 文档文件（~200 处 `-env` 引用全部清除）
- **迁移**: 移除所有命令调用中的 `-env` 参数。工具已默认自动加载 `.env.dbexplain`，无需任何额外参数。
- **ISSUE-098**: 重构任务追踪。

## v0.1.7 (2026-06-16) — Prometheus Meta 表行输出 + CTE 写检测加固 + GaussDB Oracle 兼容模式适配

### ✨ Prometheus meta 表行输出

- **`_labels` / `_metrics` 表 JSON 输出增加 `rows`**: `dbexplain --json` 输出中 Prometheus 的连接器采集的 `_labels`（~206 条 label 名）和 `_metrics`（~644 条 metric 元数据含 type/help/unit）现包含 `rows` 字段。**优势**：VeinMap 等 LLM Agent 不再只能看到表名和列结构，可直接消费语义化样本数据做 NL→PromQL 语义匹配，是 Agent 智能问数的 P0 阻塞解除。
- **`SampleRows` 通用 Schema 字段**: `schema.Table` 新增 `SampleRows []map[string]any`，JSON 渲染层自动透出为 `rows`。**优势**：通用扩展点，未来任意连接器都可以填充样本数据行供 Agent 消费。

### 🐛 CTE 写操作检测加固

- **WITH + 主查询写操作拦截**: `WITH x AS (SELECT 1) INSERT INTO y VALUES (1)` 绕过只读校验的漏洞修复。三层加固：(1) WITH 检测提前到 AST 解析之前；(2) `containsCTEWrite()` 增加主查询体写动词检查；(3) 修复 `break` 在 `switch` + `for` 嵌套中只跳出 `switch` 的 Go 语义陷阱（改用 labeled break）。**优势**：消除 CTE 写操作绕过 sqlguard 的路径，增强只读安全防护的完备性。

### 🔧 GaussDB Oracle 兼容模式全面适配

- **独立 GaussDB 连接器** (`connector/gaussdb.go`): 从 `postgresConnector` 分离为独立的 `gaussdbConnector`，复用 `collectPGDB()`、`buildPGDSN()` 等包级函数实现零代码重复。注册 kind `"gaussdb"`，DSN 方案 `gaussdb://` 不变。
- **EXPLAIN 格式适配**: GaussDB Oracle 兼容模式不支持 `BUFFERS` 选项，`wrapExplain()` 改为 `EXPLAIN (ANALYZE, FORMAT TEXT)`（PostgreSQL 保留 `BUFFERS`）。
- **CLI 帮助更新**: `manual.go` 和 `db_sections.go` 中 PostgreSQL 和 GaussDB 分开说明，标注 GaussDB Oracle 兼容模式的已知限制。
- **新增文档**: `docs/databases/gaussdb.md` — GaussDB 独立兼容性指南，包含 DSN 配置、Oracle 模式说明、已验证兼容项、已知差异、分布式注意事项。
- **文档清理**: `COMPATIBILITY_GAUSSDB_TDSQL.md` 拆分为纯 TDSQL 内容，GaussDB 部分移至新文档。
- **`datistemplate` 回退**: 当 `pg_database.datistemplate` 列缺失时（Oracle 兼容模式），自动回退到无该列的备用查询。
- **`oracleCompatible=true` DSN 参数**: 新增可选参数，设置后跳过 PG 模式专属的 `datistemplate` 查询，直接使用 Oracle 兼容模式的备选查询。避免日志中出现无效查询失败记录。示例：`gaussdb://user:pwd@host:5432/mydb?oracleCompatible=true&label=my-gauss-oracle`。

### 🐛 DSN 密码特殊字符兼容

- **`#` 自动转义**: DSN 密码中的 `#` 字符（如 `redis://:pwd#123@host`）被 Go `url.Parse` 解释为 URL fragment 导致解析失败。新增 `escapeUserinfoHash()` 预处理，自动将 userinfo 段内 `#` 转义为 `%23`。修复涉及全 connector。（ISSUE-091）

### 🛠 `dbexplain check` 配置检查子命令

- **新增 `dbexplain check`**: 验证 .env 配置文件格式、DSN 语法正确性、数据库连通性。支持 `--dsn`（重复）、`--config`（JSON）、`--timeout`、`--env` 参数。自动发现 .env.dbexplain，带密码脱敏的错误信息显示。退出码：全通过为 0，有失败项为 1。
- **测试文档**: `docs/test/21-check-command.md` — 11 项端到端测试覆盖全部场景。

### 📋 SQL 日志截断可配置化

- **`--sql-log-max-len` 全局参数**: SQL 日志截断长度从硬编码 2000 改为默认 5000 字符，用户可通过全局 `--sql-log-max-len N` 指定。标注是字符（characters）而非行数（rows）。
- **`connector.TruncateSQL()` 统一入口**: 提取重复截断逻辑到单一定义，所有 ~11 处截断点统一调用。新增 `connector.MaxSQLLogLen` 包级变量。
- **单元测试**: 默认值、截断行为、自定义长度三项测试覆盖。

### 📄 文档

- **`docs/test/21-check-command.md`**: 新增 check 子命令 11 项测试用例。
- **`docs/CODE_MAP.md`**: 新增 `dbexplain check` → `internal/check/handler.go` 映射。
- **`docs/file_index.md`**: 新增 `test/21-check-command.md` 条目。
- **`docs/test/RESULTS.md`**: 更新 v0.1.7 测试结果。
- **`.env.dbexplain.example`**: 补充 GaussDB DSN 配置示例。

### ⚡ Schema 采集性能优化

- **PG/MySQL 批量查询替代 N+1**: PostgreSQL 和 MySQL schema 采集改为 per-schema 批量查询，替代原来的 per-table N+1 模式。100 表的场景从 400-600 次数据库往返降至 ~7 次。重构 `fillPGTable`/`fillMySQLTable`，批量查询 columns/indexes/FK 后 Go 侧 map 分组。（P1）
- **`--sample` 可选样本行（默认关闭）**: 新增全局 flag，启用 `SELECT * ... LIMIT 1` 样本行采集用于注释推断。默认**不采集**，需显式 `--sample` 开启。465 表场景节省 ~14 秒采集时间。（P2）
- **`--skip-opstats` 跳过 MySQL op_stats**: 新增 flag，跳过 MySQL `performance_schema.table_io_waits_summary_by_table` 采集。100 表省 100 次查询。（P3）
- **CSV/XLSX 流式读取**: CSV `SELECT * LIMIT N` 改用 `csvReader.Read()` 逐行读取（`readCSVDataStreaming`），XLSX 改用 `excelize.Rows` 迭代器。大文件 LIMIT 查询从 O(N) 内存降到 O(limit)。（P5）

### ✨ `--table` / `--tables` Schema 采集参数

- **`--table NAME`（connector 级 SQL 过滤）**: collect 子命令新增参数，在 **connector 层**做 table 级 schema 采集。MySQL/PostgreSQL/SQLite/ClickHouse/DuckDB/Oracle 在 table list 查询注入 `AND table_name IN (...)` 参数化过滤，columns/indexes/FKs 查询同步过滤。非 SQL 数据源（Redis/Mongo/ES/Qdrant/Prometheus/CSV/XLSX/Hive）返回 `--table not supported` 并跳过。（P1）
- **`--tables`（精简列表模式）**: render 层输出 compact 表格列表（name/engine/rows/size/comment），不输出 columns/indexes/FKs/relationships/clusters/issues。全部采集后渲染精简输出。（P3）
- **`--table` AI Context chunks 过滤**: `output.WriteContext()` + `GenerateChunks()` 新增 `tableName` 参数，指定 `--table NAME` 时 chunks/ 目录只输出该表的 .md。summary/topology/diagnostics 保持全局概览。（P4）
- **`--table` / `--tables` 互斥校验**: 同时指定时 `log.Fatal` 退出。（P5）
- **实现机制**: 新增 `connector.WithTableFilter(ctx, names)` / `GetTableFilter(ctx)` context 传参模式（复用 `WithSample` pattern），`Connector` 接口不变。各 SQL 连接器分别处理 dialect 占位符（`?`/`$N`/`:N`/单引号转义）。
- **变更文件**: `connector.go`（context 方法）、7 个 SQL connector（注入过滤）、8 个非 SQL connector（跳过+提示）、`main.go`（flag 定义+注入）、`render.go`（tablesOnly 模式）、`output.go`+`context/compress.go`（chunks 过滤）。（15 文件）

### ⚡ Schema 采集降级优化（PG/GaussDB/MySQL）

- **PG/GaussDB 三级降级链**: 当前 4 表 JOIN（pg_tables + pg_class + pg_namespace + pg_stat_user_tables LEFT JOIN）作为 Level 1（最快最全）。pg_class/pg_namespace 权限不足时自动降级到 Level 2（pg_stat_user_tables 单表，无 size/comment），仍不可用时降级到 Level 3（pg_tables 仅表名）。`isPermissionErr()` 检测 `permission denied`，安全不影响正常路径。（P1）
- **MySQL 二级降级链**: information_schema.TABLES（Level 1，最快）权限不足时自动降级到 SHOW TABLE STATUS（Level 2），Go 侧应用 table filter 兜底。（P1）
- **SQL 日志改进**: PG/GaussDB 表采集循环：SQL template 在循环前统一打印一次（`[Level 1] table query: ...`），循环内只打 `schema=xxx → N tables`，不再 per-schema 重复打印查询全文。schema 名以 `schema=` 前缀显式标记，替代裸 `$1`。（P3）
- **`isPermissionErr()` 共享**: 提取到 `connector.go` 包级函数，PG 和 MySQL 共用，避免重复定义。（P3）
- **安全**: Level 2/3 查询均使用参数化 `$1` + 可选 `$2, $3, ...`，防 SQL 注入。脱敏错误信息不暴露密码/路径。

### 🐛 `--timeout` 超时不生效修复

- **`statement_timeout` 加 cap（30s）**: `collectPGDB()` 中 `statement_timeout` 原来设为剩余时间（如 18s），但 PostgreSQL 的 `statement_timeout` 是 per-statement 的，10 个查询各得 18s 可累计 180s，远超 `--timeout`。修复：上限 30s 防止单个 statement 占满整个 timeout 预算。新增 `setPGStatementTimeout()` 共享 helper。（P0）
- **外层 DB 列表查询补齐 `statement_timeout`**: `postgresConnector.Collect()` 和 `gaussdbConnector.Collect()` 中的 `pg_database` 列表查询原来裸奔，没有服务端超时保护。现也在外层调用 `setPGStatementTimeout()`。（P1）
- **MySQL 采集路径补齐 `max_execution_time`**: `collectMySQLDB()` 入口新增 `SET SESSION max_execution_time` 作为服务端超时，上限 30000ms（30s）。原来 MySQL 采集路径完全没有服务端超时，全靠 Go context 取消（`lib/pq` 类驱动取消不可靠）。（P1）

### 📋 日志统一合并

- **单文件 `dbexplain.log`**: 移除 collect.log 和 per-label `<label>.log` 文件，所有 goroutine 日志统一写入 `dbexplain.log`。每行 `[label=X] [kind=Y] ` 前缀区分实例。（P2）
- **实现**: goroutine 内 logger 使用 `log.Writer()`（指向 `dbexplain.log`）+ `fmt.Sprintf()` 构建带前缀的 `log.New()`，`Logf()` 调用无需改动。（P2）
- **collect 摘要日志**: `collectLogger` 改为 `log.Printf("[collect-summary] ...")`，同样写入 `dbexplain.log`。（P4）

- **`inferRefs()` name index**: 外键推断增加 `map[name][]tableEntry` 倒排索引，复杂度从 O(N_cols × N_tables) 降至 O(N_cols)（map lookup O(1)）。1000 表从千万级比较降至万级。（P4）

### 🛠 `dbexplain check` 默认加载 .env

- **`--env` 默认值改为 `true`**: `dbexplain check` 不带参数时自动加载 `.env.dbexplain`，无需显式传 `--env`。（6）

### 🐛 GaussDB 超时 & 密码修复

- **`statement_timeout` 服务端超时兜底**: `collectPGDB()` 启动时从 context deadline 提取剩余时间，执行 `SET statement_timeout = 'Ns'`。解决 GaussDB Oracle 模式下 `lib/pq` 的 Go context 取消不传播的问题，确保服务端主动取消长时间运行查询。
- **`SetMaxOpenConns(1)`**: GaussDB/PostgreSQL 的 Collect 函数设置单连接，确保 `statement_timeout` 对本 session 全部查询生效。
- **DSN 密码 `#@!` 兼容**: `escapeUserinfoHash()` 处理 `#` 后 DSN 中 `@!` 等字符的正确传递。

### 🐛 `config.SanitizeErr()` 死循环 — check/collect 进程卡死 (ISSUE-095)

- **根因**: `d.Redacted()` 占位符 `gaussdb://{dbuser}:{dbpassword}@host:port/db` 仍匹配 URL 模式 `://user:pass@host`。SanitizeErr 第一遍替换 `{dbpassword}→***`，第二遍对 `***→***` 无操作但字符串未变 → 无限循环。28P01 认证失败等错误经过此路径导致进程永久卡死。
- **修复**: `newMsg == msg → break`，检测替换前后字符串无变化即退出循环。
- **防御加固**:
  - GaussDB/PostgreSQL `db.Close()` 改为 goroutine 关闭 (`defer func() { go db.Close() }()`)，防止 lib/pq 内部清理阻塞 Collect 返回
  - `check/handler.go` for 循环内 `defer cancel()` 改为 select 后显式 `cancel()`，避免 cancel 函数堆积

### 🔧 `dbexplain check --label` 支持

- **`--label` 过滤**: `dbexplain check` 新增 `--label` 参数，支持按 label 过滤待检查的 DSN。

### 🔧 Hint SQL `/*+ */` 透传测试验证

- **`sqlguard` hint SQL 测试**: Validate/AutoLimit/firstWord 新增 GaussDB/MySQL/Oracle/PG 风格 `/*+ */` hint 测试用例，验证 hint 注释不干扰只读校验和 LIMIT 注入。（6 项）
- **`executor` wrapExplain hint SQL 测试**: 新建 `executor_test.go`，验证 7 种 SQL 数据库（gaussdb/mysql/postgres/oracle/hive/sqlite/clickhouse）的 EXPLAIN 包装与 hint 透传正确性。（7 项）
- **回退路径确认**: `/*+ */` 注释导致 AST 解析失败时，Validate/AutoLimit 自动回退到字符串首词检测，hint 作为 SQL 正文透传到数据库。所有原生支持 hint 的数据库（GaussDB/MySQL/Oracle/Hive + PostgreSQL via pg_hint_plan）均可正常使用。

## v0.1.6 (2026-06-12) — Bug Bash + Prometheus DSL 内核升级

### ✨ Prometheus DSL 内核升级

- **promql() 原始表达式透传**: `FROM @label.promql(任意 PromQL 表达式)` — 支持多指标二元运算（`rate(a[5m]) / rate(b[5m])`）、PromQL 函数（topk / histogram_quantile 等）、联邦 JOIN。WHERE/GROUP BY 需在表达式内联。**优势**：DSL 编译器无法覆盖全部 PromQL 语法（PromQL 持续演进），透传模式让用户永远不受 DSL 编译能力限制，享受 PromQL 全部表达能力。
- **ORDER BY / LIMIT / OFFSET**: Go 层后处理，数值列/标签列/NULL 分层排序。**优势**：首次支持 Prometheus 查询结果排序分页，之前结果顺序不固定且无法截断；可精确获取 Top-N 或分页浏览。
- **SELECT 列投影 + GROUP BY 聚合**: SELECT 列名/别名、COUNT/AVG/SUM 等编译为 PromQL by()。**优势**：之前仅 `SELECT *`，大量标签列噪音大；列投影精简输出，聚合函数让用户用 SQL 语法做 PromQL `by()` 分组，降低学习成本。
- **Collect 移除 job 表** (ISSUE-088): `@my-prom.xxx` 引用的是 metric 名，非 job 名。**优势**：Collect 输出与 DSL 查询模型对齐，避免用户混淆。

> **统一查询体验**：团队只需掌握一套 SQL 语法即可操作关系库（MySQL/PG/Oracle）和时序库（Prometheus），`promql()` 透传 + 跨源联邦 JOIN 实现异构数据统一分析。

### 🐛 Bug Bash（21 项修复）

- P0/P1/P2 防御性增强 — nil 解引用、错误吞没、goroutine 泄漏、DSN 密码泄露共 17 项
- DSL 联邦 JOIN 修复 (ISSUE-087): 物化别名使用占位符导致 JOIN 解析失败
- **闭环验证**: go build + go vet + go test + selective compile 全部通过

### ⚡ 体验优化

- execute 参数位置无关，EXPLAIN 双重包裹防护，空 SQL 错误提示优化

## v0.1.5 (2026-06-11) — Oracle + Hive 连接器 + Policy 列剥离

### ✨ 新功能

- **Oracle 数据库连接器** (第 14 种数据源): 新增 `oracle://` / `oracles://` DSN 方案。支持 Schema 采集（all_tables/all_tab_columns/all_constraints/all_ind_columns）、外键 4 表 JOIN 位置对齐、EXPLAIN 两步法（db.Conn() session 固定）、AutoLimit 适配（LIMIT → FETCH FIRST）。使用 go-ora 纯 Go 驱动，无 CGO 依赖。能力标签：CapSQL/CapForeignKey/CapRowCount/CapSampling/CapIndex。

- **Hive 数据库连接器** (第 15 种数据源): 新增 `hive://` / `hives://` DSN 方案。使用 HiveServer2 SQL（端口 10000），DESCRIBE FORMATTED 采集列信息，行数固定 -1（避免触发 MR/Tez），采样用 LIMIT 1。支持 NOSASL/NONE/LDAP/KERBEROS 四种认证方式，Kerberos 使用纯 Go 实现（beltran/gosasl，无 CGO）。能力标签：CapSQL/CapRowCount/CapSampling。

- **DENY_COLUMNS + SELECT * 自动列剥离**: 当查询使用 `SELECT *` 且配置了 `DENY_COLUMNS` 时，不再拦截报错。改为查询正常执行，在结果输出前自动移除被禁止的列（`StripDeniedColumns`）。所有 15 种数据源统一生效（SQL/CSV/XLSX/NoSQL/时序库等）。

### 📚 文档

- **CHANGELOG.md / CHANGELOG_EN.md**: 更新至 v0.1.5

### 🔧 内部重构

- **`policy.go`**: 移除 `CheckSQL` 中 `SELECT *` + `DENY_COLUMNS` 的拦截逻辑（原第 107-121 行）；新增 `StripDeniedColumns(result)` 后置列剥离函数，与 `ApplyMask` 同级
- **`executor.go`**: `ExecQuery` 在 `ApplyMask` 之后调用 `StripDeniedColumns`
- **`file_exec.go`**: `HandleFileExecute` 在 `ApplyMask` 之后调用 `StripDeniedColumns`

### 📚 文档

- **新增** `docs/databases/relational/ORACLE.md`（Oracle 数据源使用手册）
- **新增** `docs/databases/analytical/HIVE.md`（Hive 数据源使用手册）
- **README 中英同步**：数据源计数 13 → 15，连接器层新增 Oracle(关系型)/Hive(分析型)
- **`docs/CODE_MAP.md`**: 新增 Oracle/Hive 模块映射 [+2 行]
- **`docs/file_index.md`**: 新增 ORACLE.md / HIVE.md

## v0.1.4 (2026-06-04) — Prometheus 时序数据库连接器 + 采集指标收集

### ✨ 新功能

- **Prometheus 时序数据库连接器** (ISSUE-079): 新增 `prometheus://` DSN 方案，第 13 种数据源类型。通过 Prometheus HTTP API v2.x 采集 targets（scrapePool 分组为表）、标签名（`_labels` 表）与指标元数据（`_metrics` 表含 name/type/help/unit）。支持 PromQL 即时查询，四种结果类型（vector/matrix/scalar/string）映射为标准 `QueryResult`。默认 10s 超时，支持 `?timeout=N`、`?tls=true` DSN 参数。`full` 构建标签自动包含。

- **CapPromQL 能力**: 新增 `CapPromQL` 能力常量，声明连接器支持 PromQL 查询能力。在非 SQL 执行路径下（`opts.IsSQL=false`）绕过 sqlguard/AutoLimit/EXPLAIN，直接执行 PromQL。

- **采集指标收集与 Prometheus 文本输出** (ISSUE-076): 采集管道自动收集每个 DSN 的执行耗时、成功/失败状态、表数量等结构化指标。JSON 输出嵌入 `"metrics"` 顶层字段；`--metrics` 标志将 Prometheus 文本格式输出到 stderr，不污染 stdout。

- **CLI 交互增强**: `-env` 配置自动发现——用户执行命令不指定 `-dsn`/`-config` 时自动从配置文件加载 DSN，无需显式 `-env` 标志。`dbexplain list` 输出顶部显示当前配置来源路径；空配置文件/模板检测，输出文件路径和编辑引导，提示用户修改文件并增加自己的连接配置。

### 🐛 修复

- **REPL Prometheus DSL 路由修复**: `replExecDSL()` 原来按 `SourceKind`（SourceSQL/SourceFile/SourceNative）分发查询，Prometheus 的 `SourceNative` 类型走入默认错误分支。改为按 `primary.Vendor`（VendorSQL/VendorFile/VendorPromQL）分发，与 `execute.go` 的 `handleDSLExecute()` 路由逻辑一致。新增 `replExecPromQL()` 函数处理 PromQL 编译和执行。确保 REPL 与 execute 子命令使用统一的 DSL 路由架构。

### 📚 文档

- **新增** `docs/databases/prometheus.md`（Prometheus 数据源使用手册）
- **新增** `docs/test/18-prometheus.md`（13 项 E2E 测试：DSN 解析/采集/查询/REPL/安全/向后兼容）
- **README 中英同步**：数据源计数 12 → 13，能力矩阵新增时序型 Prometheus 行(Collect/REPL/PromQL)
- **README 架构重构**: 双语言新增架构层次描述（5 层 ASCII 图 + 层级说明表格）。二维表化能力全景映射矩阵、核心能力、输出格式、安全防护、二进制变体、CLI 常用参数、文档导航。更新 ES REPL 标记为✅，Prometheus DSL 脚注改为支持联邦。
- **docs/file_index.md**：新增 prometheus.md 到§三时序型类别
- **docs/CODE_MAP.md**：模块映射新增 Prometheus 连接器 + CapPromQL 能力矩阵列 + 文档交叉引用
- **CLI_EXAMPLES.md**: 新增第 11 章 Prometheus 执行示例（execute/DSL/REPL/安全检查），更新第 13 章 REPL 输出和数据源表格
- **docs/test/README.md**：测试总览更新（17 数据源/153 测试项/L8 Prometheus）
- **docs/test/RESULTS.md**：v0.1.4 测试结果，153/153 测试项通过（100%）
- **CHANGELOG 中英同步**：本版本完整记录双语言版本。

### 🏗️ 构建与发布

- **构建标签**: prometheus 连接器使用 `//go:build prometheus || full` 标准模式，自动包含在 `full` 构建和所有标准版二进制中。无 stub 需要。
- **DuckDB linux/arm64 交叉编译** (ISSUE-081): release.sh 自动检测 `aarch64-linux-gnu-gcc` 工具链，可用时额外产出 `dbexplain-linux-arm64-duckdb` 交叉编译二进制。ARM64 平台使用 `-static-libgcc -static-libstdc++` 替代 `-static`（Ubuntu 22.04 交叉工具链 glibc 存在 `R_AARCH64_LD64_GOTPAGE_LO15` GOT overflow 限制）。
- **Go 1.26+ linker 引号兼容** (ISSUE-080): Go 1.26+ 的 `cmd/internal/quoted.Split` 仅将字段首字符的引号视为引号字符，`-extldflags='-static-libgcc ...'` 被按空格错误切分。修复：改用 `'-extldflags=-static-libgcc -static-libstdc++'` 整字段引号包裹。
- **macOS UPX 跳过检测**: darwin/arm64（UPX 无 arm64 Mach-O 支持）和 darwin/amd64 交叉编译（UPX 5.x CantUnpackException）显式跳过 UPX 压缩，打印具体原因。修复 `| tail -1` 管道静默吞 UPX 错误码的问题。
- **tar 分类打包** (release.sh Phase 3): 发布脚本按平台粒度产出独立 tarball，命名 `dbexplain-${VERSION}-${plat}-${edition}-{upx,noupx}.tar.gz`，每包仅含单一平台二进制，文件名标示 UPX 压缩状态。共 12 个（std × 5 × 2 变体 + duckdb × 2 × 2 变体，darwin 仅 noupx）。
- **TSV Kind 修正**: `csv.go Collect()` 检测 `d.Raw` 前缀 `tsv://` 后设 `Kind="tsv"`，不再硬编码 `"csv"`。向下兼容：TSV DSN 路由仍走 csv 连接器，仅 schema 实例的 `kind` 字段正确。
- **REPL 无配置启动 + .connect 命令**: 无 DSN 条目时进入 `(disconnected)` 状态，显示友好提示。新增 `.connect <dsn-url>` 命令动态连接数据源，添加到 `allEntries` 列表并支持后续 `.conn` 切换。
- **REPL ES JSON 原生查询**: ES JSON 查询（`{"query":{"match_all":{}}}`）路由到 `/_search` REST 端点。ES connector 的 `ExecQuery()` 检测 JSON 入参后执行 `IsSQL=false` 路径，绕过 sqlguard。响应从 `hits.hits[]._source` 提取动态列名和行数据。
- **文件查询引擎哈希索引**: `filequery/executor.go` 对 `WHERE col = literal` 简单等值条件构建临时 `map[string][]int` 哈希索引，将 O(n) 全表扫描降为 O(1) 哈希查找。`OR`/`LIKE`/`BETWEEN` 等复杂条件自动回退到全表扫描。
- **Prometheus DSL 联邦查询**: `dslExecFederated()` 和 `replExecFederated()` 的 `SourceNative` case 按 `VendorPromQL` 二次分发——获取 Prometheus 连接器，通过 `ExecQuery(IsSQL:false)` 执行 PromQL，转换结果到 `filequery.NamedData` 参与联邦合并。
- **ACID 评估 (ISSUE-082)**: DSL 联邦查询无分布式事务评估——只读联邦查询不需要写事务，文件数据源不支持事务，SQL 数据库的 SNAPSHOT ISOLATION 在联邦上下文中无意义。当前阶段不需要 ACID 保证。
- **install.sh tarball 目录匹配修复**: `extract_from_tarball()` 中 `grep -E` 正则匹配目录名（尾部带 `/`）后 `cp` 因缺少 `-r` 失败。添加 `grep -v '/$'` 排除目录条目，确保仅匹配文件。
- **4 二进制变体闭环验证**: std-noupx/std-upx/duckdb-noupx/duckdb-upx 各 65-67 项测试全部通过。新增 `docs/test/test-runner.sh` 自动测试脚本（分层 L1-L8 + 新增特性 + E2E 外部数据库）。UPX 压缩体积 42MB→9.2MB（78%），启动延迟约 430ms。
- **全量验证**: `go build ./...` / `go vet ./...` / `go test ./...` 全部通过，4 二进制变体全部验证通过。

## v0.1.3 (2026-06-03) — DuckDB 可选连接器 + 构建系统双版本发布

### ✨ 新功能

- **DuckDB 嵌入式 SQL 引擎**
  支持内存模式（`duckdb:///:memory:`）与文件数据库模式（`duckdb:///path/to/file.db`）。覆盖完整元数据采集（系统函数 `duckdb_tables/columns/constraints` + 行数统计 + 采样）、`ExecQuery` 查询执行、DSL `@label.duckdb` 绑定及 `EXPLAIN` 格式适配。强制 `access_mode=READ_ONLY` 保障只读安全。

- **DuckDB 文件分析引擎**
  通过 `read_parquet()`/`read_csv_auto()`/`read_json()` 直接分析 Parquet/CSV/JSON 文件。`allowed_path` 参数控制可读目录，防止路径遍历越界。

- **双版本发布体系**
  - **标准版（`-std`）**：纯 Go，无 CGO 依赖，跨 5 平台（Linux/Windows/macOS amd64/arm64）
  - **DuckDB 全量版（`-duckdb`）**：当前平台 + DuckDB 支持，CGO 依赖 gcc/clang
  发布脚本 `release.sh` 自动完成两阶段构建。

- **CLI 帮助层次化重构**
  命令按 `Schema` / `Query` / `Utility` / `Help` 分组展示，数据源按 `SQL` / `NoSQL` / `File` 分类。标准版提示 DuckDB 构建方式，DuckDB 版正常显示入口。

- **REPL DSL 语法与联邦查询**
  REPL 模式现自动检测 `@label.table` DSL 语法，支持单源（SQL/文件）和联邦跨源 JOIN。查询含 `@` 时路由至 DSL 执行路径，不含时走原有单源查询路径。添加 4 个 REPL-safe DSL 分发函数。

### 🐛 修复

- **DuckDB DSN 解析**
  `duckdb://:memory:` 被 Go 标准库误解析为端口号 → 改为 `duckdb:///:memory:`（三斜杠），连接串构建函数增加专门处理分支。

- **子命令注册遗漏**
  `main.go` 中 `duckdb` case 缺失导致命令静默退出 → 已补全。

- **Goroutine panic 恢复**
  三处缺失 `defer/recover()` 的位置（Schema 采集 goroutine 外层、输出捕获 `io.Copy` goroutine）全部补上，避免单点崩溃影响整体进程。

- **路径前缀匹配安全漏洞**
  `allowed_path` 使用 `strings.HasPrefix` 缺少分隔符守卫（如 `/data` 误匹配 `/data_backup`）→ 添加末尾分隔符检查。

- **错误日志丢失**
  - XLSX 查询原始错误被 `ErrNotSupported` 吞没 → 增加 `log.Printf` 保留上下文
  - ClickHouse/ES 的 `io.ReadAll` 错误被 `_` 忽略 → 改为日志记录
  - Delta/Diff JSON 输出的 `WriteFile`/`MarshalIndent` 错误被 `_` 忽略 → 增加错误日志

- **CJK 字符表格对齐**
  `fmt.Sprintf("%-30s")` 按字节填充导致图表模式中文错位 → 改用基于视觉宽度的 `pad(Inst.Label, 30)`。

### 🏗️ 构建与发布

- **CGO 引入及宪法例外**
  项目首次引入 CGO 依赖（DuckDB Go 驱动内嵌 C++ 引擎）。`CONSTITUTION.md` 增加第 4 条例外说明。

- **UPX 极致压缩**
  - DuckDB 全量版：100 MB → 23 MB（-77%）
  - 标准版：42 MB → 9.1 MB（-78%）
  全链路验证：`file`/`ldd`/`nm -D`/`upx -t` 全部通过。

- **构建标签隔离**
  DuckDB 需要显式 `-tags duckdb` + `CGO_ENABLED=1`，且不在 `full` 标签中。提供 stub 文件给出友好构建提示。

- **全量验证**
  `go build ./...` / `go vet ./...` / `go test ./...` + `bash build.sh prod/minimal` + `bash release.sh` 全部通过。

- **DuckDB 完全静态链接**
  DuckDB 构建新增平台特定 `-extldflags`：Linux 使用 `-static` 实现零 ldd 依赖（`ldd` 显示 "not a dynamic executable"）；macOS 使用 `-static-libgcc -static-libstdc++`，仅保留系统必有的 `/usr/lib/libSystem.B.dylib`。

- **macOS UPX 跳过逻辑修复**
  修复了两个 macOS 平台的 UPX 问题：(1) darwin/arm64 — UPX 任何版本均不支持 arm64 Mach-O，现显式跳过并打印原因；(2) darwin/amd64 交叉编译 — UPX 5.x 对 Go 交叉编译的 Mach-O 抛出 `CantUnpackException`，前因 `| tail -1` 管道静默吞掉错误码，现修复为正确捕获退出码并输出诊断。原生 macOS 构建的 darwin/amd64 二进制（UPX ≤ 4.x）不受影响。

### 📚 文档

- **新增** `DUCKDB.md`（使用指南）、`DUCKDB_IMPL.md`（实现边界与安全模型）
- **README 中英同步**：数据源计数 11 → 12，能力矩阵新增 DuckDB 行，开发指南补充 `release.sh` 与命名规范
- **测试文档**：新增 `16-duckdb.md`（20 项 E2E），更新 `RESULTS.md`（128/128）、测试总览（16 数据源）
- **SECURITY_CHECKLIST.md** 增加 DuckDB 文件路径安全检查项
- **安装脚本**：`install.sh`/`install.ps1` 版本号 v0.1.2 → v0.1.3，下载 URL 使用 `-std` 后缀
- **CHANGELOG 中英同步**：本版本完整记录双语言版本。
- **README 架构图**: 能力矩阵与核心能力之间新增 `DBEXPLAIN-ARCH.png` 系统架构图
- **README REPL DSL 说明修复**: 更新失效的"不支持 DSL 模式"标注为"支持 DSL 单源/联邦跨源 JOIN"
- **测试文档**: RESULTS.md 更新为 130/130 测试项，移除"REPL 不支持 DSL/联邦查询"已知局限

## v0.1.2 (2026-06-03) — CLI 交互增强 + DSL 联邦查询 + 构建系统优化

### 新功能
- **DSL 联邦查询** (ISSUE-069): 跨数据源 JOIN/UNION 支持。移除 `len(kinds)>1` 阻断，数据物化 + filequery 联邦合并层。SQL ↔ 文件 ↔ 混合源 JOIN 全支持
- **`dbexplain collect` 子命令** (ISSUE-072): Schema 采集从默认行为迁移为显式子命令。`dbexplain collect -env --human` 新方式，向后兼容保留 fallthrough 采集。无参 `dbexplain` 显示帮助
- **CLI REPL 模式** (ISSUE-070): `dbexplain repl` 交互式查询循环。`.conn` 切换数据源、`.help`/`.exit`/Ctrl+D、自动计时与行数统计
- **`--explain` 输出格式化** (ISSUE-071): 按数据库类型使用特定 EXPLAIN 语法（MySQL FORMAT=JSON、PostgreSQL ANALYZE BUFFERS、SQLite QUERY PLAN、ClickHouse PLAN）。MySQL FORMAT=JSON --human 可读渲染

### 修复
- **DSL 文件 JOIN 修复** (ISSUE-069): `dslExecFile()` 传入 `nil allEntries` 导致文件源 DSL 模式下 JOIN 无法解析。改为传入全局 entries
- **ClickHouse REPL 尾部分号冲突修复**: ClickHouse 驱动在查询后追加 `SETTINGS max_execution_time=N FORMAT JSON`，尾部 `;` 导致多语句错误。`repl.go` 新增 `TrimRight(";")` 自动清除尾部分号
- **Elasticsearch REPL JSON 查询友好提示**: JSON 原生查询在 REPL 中原返回 `READ_ONLY_VIOLATION` 混淆提示。新增 JSON 前置检测，输出清晰绕过方案（`execute -env --label` SQL 模式或 `collect` 采集 Schema）
- **Windows 原子写入修复**: `cache.go` `os.Rename` 在目标文件存在时失败。新增 `runtime.GOOS == "windows"` 时先 `os.Remove`
- **CJK 显示修复**: `render/table.go` 列宽使用 `len()`（字节）导致中文/韩文/全角字符表格错位。新增 `visualWidth()` 按显示宽度计算并补偿 padding
- **`render.go` `pad()`/`truncate()` UTF-8 修复**: 字节切片可能产生无效 UTF-8 序列。改为基于 `[]rune` 视觉宽度截断
- **`dsn.go` `SQLitePath()` Windows 修复**: 缺少 Windows `/C:` 前缀剥离逻辑，导致绝对路径 DSN 在 Windows 上可能失败
- **`dsn.go` `FilePath()` 安全加固**: 新增 `filepath.Clean()` 防止路径遍历

### 构建与发布
- **构建标签（Build Tags）**: 10 个 connector 文件添加 `//go:build mysql || full` 等条件编译，支持按需选择数据库驱动
- **`build.sh` 4 种编译模式**: prod（5 平台全驱动+UPX）、dev（当前平台+全驱动）、test（+race）、minimal（按需驱动）
- **UPX 极致压缩**: `--best --lzma` 压缩，全驱动 42 MB → 9.5 MB（-78%），零运行时依赖。`upx -t` 完整性验证通过
  - Linux/Windows 全量压缩；darwin (Mach-O) 交叉编译产物因 UPX 5.0.0 兼容性限制跳过压缩，macOS 原生编译时 UPX 正常工作
- **`build.sh --help`**: 新增 `--help`/`-h` 参数，输出完整使用说明、Tag→Kind→DSN scheme 全景映射
- **`--no-upx`/`--upx` 动态控制**: 从命令行任意位置传入，强制跳过或启用 UPX 压缩
- **产物安全验证**: `file`(statically linked) / `ldd`(无动态引用) / `nm -D`(无动态符号) / `upx -t`(完整性) / 隔离运行全链路通过
- 全量 `go build ./...` + `go vet ./...` + `go test ./...` + `bash build.sh` 验证通过

### 文档
- **发布前检查标准补充** (ISSUE-073): SECURITY_CHECKLIST.md §6 追加版本一致性、CHANGELOG 完整性、issues.json 有效性、二进制冒烟测试、产物完整性、文档陈旧引用检查
- **`docs/test/01-environment.md` 大幅扩展**: 新增 §1.9 构建模式因素影响分析（编译时间/体积/功能/UPX/安全/场景推荐/安全验证/结论），实际实测数据
- **`docs/test/RESULTS.md` 构建优化段**: 更新 5 种标签组合实测体积、全景 Tag→Kind→DSN scheme 映射、安全验证表
- **`docs/DEPLOY.md` build.sh 表格**: 模式说明更新为明确的 GOOS/GOARCH 列表
- **`docs/SKILL_AUTHORING.md` 全面优化**: 融入 Karpathy 上下文工程理念 — 新增上下文经济/可验证性原则、元数据入口强调块、description 写作公式、输入定义章节、失败处理规则、渐进披露指南、eval-first 迭代流程、完整示例模板
- **SKILL_ZH.md / SKILL_EN.md 重构**: 330→197 行（符合 200 行上限）。移除 SQL 语法表/错误处理表（引用 references/），新增输入定义/失败处理规则，精简 DSL/增量检测/参数表等冗余内容
- **README 能力全景映射矩阵**: "支持的数据源"表替换为 5 列能力模块（Schema采集/SQL查询/REPL/DSL联邦/文件引擎）× 11 数据源的能力矩阵，一图览全局
- **`docs/test/RESULTS.md` 整理**: 合并 v0.1.0/v0.1.1/重复 v0.1.2 三个冗余章节为单一 v0.1.2 报告，新增"本次闭环验证修复"清单
- **`docs/REPL.md` 更新**: 移除 ClickHouse 分号限制（已修复），ES 限制补充详细绕过方案 (SQL via `_sql`/collect)
- **`docs/test/09-cli-help.md` 扩展**: 新增 ClickHouse 分号和 ES JSON 测试用例
- **REPL .list/.databases 命令** (ISSUE-074): 新增 REPL 内 `list`/`.databases` 命令，显示所有已配置数据源的序号、label、kind 及当前连接标记
- **CONSTITUTION.md 审查更新**: 核心交付物修正（去除未实现的 IR Product 概念）；Principle 3 区分 Collect/Query 阶段并更新 MongoDB 描述；构建与发布章节精简为 DEPLOY.md 引用
- **SECURITY_CHECKLIST.md 增强**: 新增 §5a 提交前快速检查（5 项，~30 秒）；§6 新增 5 项发布前检查（脚本版本一致性、测试文档版本预期、Markdown 链接有效性、全平台版本一致性、dev 二进制 -tags full 验证）；§6 新增"历史上的坑"表格
- **全量文档陈旧引用修复**: 10 个测试文档 CONFIG_SEARCH.md 路径修复（`../docs/`→`../`）、DEPLOY.md/file_index.md 断裂链接修复、脚本版本号一致性等 20+ 处修复
- **脚本版本号同步**: install.sh/uninstall.sh/install.ps1/uninstall.ps1 注释和 `$VERSION` 统一更新为 v0.1.2（此前残留 v0.1.0）

## v0.1.1 (结构整理 + 统一 DSL 查询入口)

### 项目结构
- **Go 标准布局**: 单文件拆分为 `cmd/` + `internal/` 结构：`main.go`(2482→910行) 拆出 config/encrypt/list/manual/output；`execute.go`(585→264行) 拆出 render/queryutil/dsnfilter/executor
- **全量 internal 迁移**: 14 个顶层包按依赖顺序迁入 `src/internal/`，旧目录全部清理，`src/` 仅保留 `cmd/` + `internal/` + 构建文件
- **共享 SQL AST**: filequery 的词法/语法/AST 类型提取为 `internal/sqlast/`，sqlguard/policy/dsl 统一复用，60+ 测试通过

### 新功能
- **DSL 查询模式** (`--dsl`): 通过 `@label.table` 统一引用数据源，支持 `SELECT * FROM @my-mysql.users`。预处理 → sqlast 解析 → 符号绑定 → 后端路由，安全管道全同步
- **Schema Diff** (`dbexplain diff`): 字段级变更检测（列/索引/外键三级），4 种对比模式（cache-baseline / since / two-file / list-versions），支持 `--human` 和 JSON 输出，23 单元测试
- **窗口函数**: ROW_NUMBER / RANK / DENSE_RANK / NTILE / LAG / LEAD / FIRST_VALUE / LAST_VALUE / 聚合 OVER + ROWS/RANGE 窗口帧，36+ 测试用例

### 安全增强
- **AST 级校验**: sqlguard/policy 优先使用 `sqlast.Parse()` AST 分析，失败时回退字符串检测；AutoLimit 通过 AST Limit 字段判断避免重复注入
- **AutoLimit 重复注入修复**: `parseTableRef()` 未处理 `schema.table` 限定名，带 Schema 前缀的查询被追加第二个 LIMIT。新增 `parseQualifiedName()` 处理多节标识符
- **策略引擎表匹配修复**: `extractTablesFromAST()` 未将 `schema.table` 展开匹配，`DENY_TABLES=users` 对 `SELECT * FROM public.users` 不生效。新增表名拆解逻辑

### 文档
- **README 中英文分拆**: README.md 改为指向 README_ZH.md 的符号链接，新增 `README_EN.md`，中英文独立维护。新增文档导航表格，精简内容聚焦核心能力
- **新增 `docs/USAGE_GUIDE.md`**: 全场景傻瓜用法手册，覆盖全部 11 种数据源从安装到查询的完整流程，三平台操作说明
- **过期内容修正**: CLI_EXAMPLES.md CSV 章节修正（文件引擎已支持完整 SQL）；sql-syntax.md/troubleshooting.md 窗口函数标记更新为 ✅ 已支持
- **docs/test/ 清理**: 移除过期 PNG 截图文件

### 测试
- **统一验证标准**: 所有测试文档使用 `cd src + BIN=../release/dbexplain` 可移植路径，移除绝对路径依赖
- **新增测试文档**: 14-schema-diff.md(24项) + 15-window-functions.md(36项)
- **全量 E2E 验证**: 15 数据源 91/91 项通过，DSL 模式、Schema Diff、窗口函数全覆盖

### 构建与发布
- `build.sh` 版本号更新为 v0.1.1
- `issues.json` 修复 JSON 语法错误 + 约 70 处旧路径更新 + 新增 3 个 issue，总计 68 个

## v0.1.0 (深度安全加固 & 架构对齐 & 文件查询引擎)

### 安全修复 (P0)
- **WITH CTE 写操作绕过 sqlguard 修复**: `WITH ... INSERT/UPDATE/DELETE ...` 原先只检查第一个 token (WITH)，CTE 体中的写操作完全绕过校验。新增 `containsCTEWrite()` 深度扫描 CTE 体，拒绝含写操作的 WITH 查询
- **SELECT INTO 绕道 sqlguard 修复**: `SELECT * INTO new_table` 用 SELECT 开头绕过只读校验。新增 `isSelectInto()` 检测 INTO 表子句（排除 MySQL INTO @var），拒绝 PostgreSQL DDL 写操作

### 安全加固 (P1)
- **ANALYZE/REINDEX 从 readOps 移除**: `ANALYZE` 写入统计表，`REINDEX` 锁表重建索引。从白名单移至黑名单
- **SET SESSION 连接池竞态修复**: MySQL SET max_execution_time / PG SET statement_timeout 与后续查询在不同连接上执行导致超时失效。`ExecQuery` 现在强制单连接模式 (`SetMaxOpenConns(1)`)
- **matchStarSelect 正则锚点修复**: 正则 `\ASELECT` 只匹配起始位置，`WITH cte AS (SELECT * FROM t) SELECT ...` 中的 `SELECT *` 被遗漏。改为 `\bSELECT` 全局匹配
- **policy 配置泄漏修复**: `loadEnvFile()` 中用 `os.Setenv` 传递策略配置泄漏到 `/proc/[pid]/environ`
- **APP_ENCRYPTION_KEY 清除**: 解密完成后立即 `os.Unsetenv("APP_ENCRYPTION_KEY")`，缩短密码在进程环境的暴露窗口

### 正确性修复 (P1-P2)
- **PostgreSQL FK schema JOIN**: FK 查询缺少 `pg_namespace` JOIN，不同 schema 的同名表 FK 结果混杂
- **PostgreSQL 索引解析**: `strings.LastIndex(def, ")")` 对函数索引 (`lower(email)`) 和 INCLUDE 列解析错误。新增 `extractIndexColumns()` 带括号深度追踪
- **cache 原子写入**: `os.WriteFile` 非原子写入、进程崩溃导致 cache 损坏。改为 temp file + `os.Rename()` 原子操作

### ORDER BY 计算列别名排序修复
- **问题**: `SELECT ..., CAST(total AS FLOAT) / cnt \* 100 AS ir ORDER BY ir DESC` — `ir` 是计算列别名（非原始 CSV 列），`colMap` 查找不到导致排序失效，结果实际未排序
- **修复**: `executor.go` 中 ORDER BY 比较函数增加别名回退路径：`colMap` 查找失败时遍历 SELECT 列搜索匹配别名，用 `Eval()` 计算表达式值后再比较

### PostgreSQL 多 Schema 支持
- **Schema 发现**: `collectPGDB()` 改为查询 `pg_namespace` 获取所有非系统 schema，不再硬编码 `public`
- **行数获取**: 新增 `pg_class.reltuples` → `n_live_tup` 采集，提供每表行数估计

### 架构对齐 (宪法第 10 条落地)
- **CapSQL 能力声明**: `capabilities.go` 新增 `CapSQL` 和 `CapFile` 常量
- **Connector 统一声明**: 所有 5 个 SQL connector (MySQL/PostgreSQL/SQLite/ClickHouse/ES) 声明 `CapSQL`；CSV/XLSX 声明 `CapFile`
- **isSQLKind() 删除**: `execute.go` 中硬编码的 kind switch 替换为 `capabilities.FromProvider(c).Has(capabilities.CapSQL)`，宪法禁止的类型分支模式已消除
- **新增数据库不再需改 execute.go**: 只需实现 Connector + 声明 CapSQL，execute 自动正确路由

### JSON 输出格式变更
- **`instances` 包装**: Schema 采集 JSON 输出改为 `{"instances": [...]}` 顶级包装，新增 `groups`/`issues`/`refs` 顶级键。实例不再输出 `dsn` 字段
- **向后兼容说明**: 直接读取顶层 `kind`/`label`/`databases` 的消费者需改为从 `instances[0]` 读取

### 文档对齐 (Phase D1-D5)
- **24+ .md 文件与 v0.1.0 代码对齐**: 版本号、PostgreSQL schema 范围、Qdrant TLS/execute、Redis readOps 白名单、数据源计数、`--manual` 弃用引用
- **`docs/ALGORITHMS.md`**: 新增 `vector`/`file` 能力；更新版本状态表
- **`docs/ARCHITECTURE.md`**: `--manual` 替换为 `all`/`<dbtype>`；更新目录结构
- **`docs/POLICY.md`**: 新增排障参考表（4 种常见问题）
- **`README.md` / `README_EN.md`**: 精简约 62%（541→207 / 540→194 行），详细内容引导至 docs/
- **`issues.json`**: 合并 ISSUE-062.md 内容为 ISSUE-064；解决编号冲突

### 测试框架扩展
- **`docs/test/12-capability-routing.md`**: 新增测试套件覆盖 CapSQL 路由、PostgreSQL 多 Schema、matchStarSelect + CTE、文件数据源策略、JSON instances 包装格式
- **`docs/test/02-schema-collection.md`**: JSON 验证适配 `instances` 包装格式
- **`docs/test/11-end-to-end.md`**: JSON 结构预期与 v0.1.0 实际输出对齐
- 全部 15 个 DSN Schema 采集经实机验证；全部 8 个单元测试包通过

### 文件查询引擎 (纯 Go 内存 SQL 引擎)
- **`src/connector/filequery/` 新增 7 文件**: 纯 Go 无外部依赖的 SQL 查询引擎，支持 CSV/XLSX 业务分析
- **AST + 词法 + 递归下降解析**: `ast.go` / `lexer.go` / `parser.go` — 支持 SELECT、WHERE、GROUP BY、ORDER BY、JOIN、LIMIT/OFFSET、聚合函数、CAST/ABS/LIKE/IN/BETWEEN
- **哈希 JOIN 引擎**: 跨文件 JOIN 通过哈希索引实现；列名消歧义（限定名 `t.col` vs 非限定）；JOIN 源通过 execute.go 的 `resolveJoinSources()` 自动加载
- **表达式求值**: `evaluator.go` — 比较/算术/LIKE/IN/AND/OR 运算符、列间算术、CAST 类型转换
- **哈希聚合**: `aggregate.go` — SUM/AVG/COUNT/MAX/MIN 聚合函数
- **44 个单元测试**: 覆盖全部语法路径和边界情况
- **架构一致**: Connector 接口不变、Queryable 接口不变、CapFile 标签不变、策略引擎不感知

### 文件查询引擎增强
- **NULLS FIRST/LAST 排序**: ORDER BY 子句支持 `col DESC NULLS FIRST` / `col ASC NULLS LAST`；DESC 无方向默认为 NULLS FIRST、ASC 无方向默认为 NULLS LAST
- **UNION / UNION ALL**: `parseSingleSelect()` 拆分后新增 `UnionStmt` AST 节点；`executeUnion()` + `mergeResults()` 实现：UNION ALL 直接拼接、UNION 行值哈希去重
- **DISTINCT ON**: ORDER BY 排序后按指定列组保留首行；与 PostgreSQL 语义一致
- **子查询 IN / NOT IN**: `SubqueryExpr` AST 节点 + `subqueryCache` 预计算缓存；`parseComparison()` 同时支持前缀 NOT (`NOT col IN (...)`) 和后缀 NOT (`col NOT IN (...)`)，以及 NOT LIKE / NOT BETWEEN
- **66 个单元测试**（原 44 + 新增 22）：NULLS 词法/解析/执行、UNION ALL 解析/执行、UNION 去重、DISTINCT ON 解析/执行、子查询 IN/NOT IN 全链路

### 文件查询引擎增强 v2 — SQL 兼容性扩展
- **双引号字符串字面量**: 新增 `readDoubleQuotedString()`，`"value"` 和 `'value'` 均支持，双引号 SQL 不再报错。与 MySQL 兼容
- **IS NULL / IS NOT NULL**: 新增 `IS` 关键字解析，空值判断（CSV 空字符串视为 NULL）。兼容 MySQL/PostgreSQL
- **HAVING 子句**: GROUP BY 后支持 HAVING 过滤，引用 SELECT 列别名做聚合后条件筛选。兼容 MySQL/PostgreSQL
- **LEFT JOIN / RIGHT JOIN**: `JoinClause` 新增 `JoinType` 字段，哈希 JOIN 引擎扩展为支持左连接/右连接语义。兼容 MySQL/PostgreSQL
- **XLSX 多 Sheet 支持**: `ExecQuery` 按 SQL FROM 表名匹配 Sheet；`resolveJoinSources` 加载全部 Sheet 为独立 NamedData。每 Sheet 可单独作为 SQL 表查询
- **ROUND 单参数**: `ROUND(col)` 默认小数位为 0，无需显式传 `n`
- **多 DSN 错误优化**: 匹配到多个 DSN 时列出所有可用 label 和文件路径，方便 agent 选择正确的 `--label`
- **文件不存在提示**: CSV/XLSX 文件 `os.Open` 失败时明确提示 `file not found: <path> (use absolute path)`
- **`references/sql-syntax.md`**: 完整 SQL 语法参考独立文件，SKILL.md 精简语法表并引用之
- **SKILL.md 语气优化**: "不在列表内的**不支持**" 改为 "完整语法参考和示例见 sql-syntax.md"，agent 提示更友好

### QA 场景扩展 (Q09-Q15)
- **7 个新业务分析场景**: 覆盖 GROUP BY + AVG、ORDER BY + LIMIT、CAST + 列间算术、GROUP BY date、AND 多条件、跨文件 JOIN、嵌套算术 + ABS
- **`testdata/qa/questions/Q09-Q15.md`**: 新建问题文件，每个含业务背景 + 验证 SQL + 预期输出
- **`testdata/qa/.env.qa-touch-join`**: 新增跨文件 JOIN 测试配置
- **`docs/test/13-file-query-engine.md`**: 新建 L7 测试文档，10/10 验证项通过

### 文件修复
- **CSV UTF-8 BOM 自动剥离**: `readCSVData()` 检测 EF BB BF 前缀，修复首列 `csmgr_refno` 空值问题
- **JOIN 源 DSN 过滤修复**: `execute.go` 原按 label 过滤后丢失 JOIN 源；改为收集所有 entries 再用 `filterEntries()` 筛选
- **JOIN alias 覆盖 bug 修复**: `executor.go` 增加 sources 存在性检查，防止 alias 不存在时覆盖为 nil
- **错误信息可见性修复**: csv.go 改为透传底层解析错误，不再用 ErrNotSupported 掩盖
- **`resolveDSNEntries()` 删除**: 被内联加载 + `filterEntries()` 替代

### 文件查询引擎正确性修复
- **CAST 转换失败返回 "0" 修复**: `CAST(x AS INTEGER/FLOAT)` 转换失败时返回 `Value("0")` 而非原始值，改为返回 `val`
- **SUM 全非数值组返回 "0" 修复**: 分组内所有值均非数值时 SUM 返回 `"0"`，改为返回 `""`（SQL NULL 语义，与 MAX/MIN 一致）
- **AVG count==0 返回 "0" 修复**: 分组内无非数值可转换时 AVG 返回 `"0"`，改为返回 `""`
- **Eval 错误静默吞没修复**: `buildResult()` 列投影和 `executeAggregation()` 表达式求值中 Eval 错误曾静默返回 `""`，改为传播错误
- **JOIN 后列映射越界修复**: 哈希 JOIN 后 `colMap` 重建仅使用主表别名，JOIN 表限定名列索引偏移为 `len(primaryHeader)` 导致越界。改为按主表+JOIN 表逐源构建正确偏移

### 第三方分发包定型
- **`testdata/account-manager-skill/`**: 独立于主项目的第三方分发包，QwenPaw agent 直接读取目录 SKILL.md 识别技能
- **目录结构**: `SKILL.md` + `assets/`(5 平台预编译二进制) + `scripts/`(install/uninstall) + `references/`(表字段定义) + `.env.example`
- **离线安装**: `bash scripts/install.sh --offline ./assets/dbexplain-linux-amd64`，不指定路径时自动检测 assets/
- **SQL 能力透明化**: SKILL.md 从"不支持清单"改为"支持语法完整列表"，AI agent 无需猜测

### macOS Gatekeeper 兼容
- **install.sh macOS quarantine 自动移除**: 2 个 `install.sh`（dbexplain-skill + account-manager-skill）新增 `remove_quarantine()` 函数，安装二进制后自动执行 `xattr -d com.apple.quarantine`，用户无需手动操作
- **SETUP.md**: 增加 macOS Gatekeeper 说明和手动解决方法

### dbexplain-skill 最佳实践泛化
- **SKILL_ZH.md / SKILL_EN.md**: 新增"可追溯分析"（严禁编造数据、逐条标注来源SQL）、"文件查询最佳实践"（数据预览→澄清需求→业务分析）、"错误处理（9+3 分类）"章节
- **`references/sql-syntax.md`**: 新建，从 account-manager-skill 泛化，中性列名（`sales_data`/`department`/`employee_id`），明确仅覆盖文件数据源
- **`references/troubleshooting.md`**: 新建，从 account-manager-skill 泛化并扩展数据库连接排障（DNS/认证/超时/SSL 等），按 9+3 分类组织
- **install-skill.sh**: 安装/更新时自动部署 `references/` 目录，`--verify` 验证其完整性

### 版本跟踪
- 版本号: v0.1.0
- 文档与代码间 38 处差异全部已修复

## v0.0.9 (2026-05-28)

### CSV/TSV/XLSX 文件处理
- **CSV/TSV 文件 Schema 采集**: 新增 `csv://` / `tsv://` DSN 方案，支持单文件、目录扫描、Glob 通配符（`*`/`?`/`[`）三种路径模式。首行作列名，采样推断列类型（INTEGER > FLOAT > DATE > TEXT）
- **XLSX 文件 Schema 采集**: 新增 `xlsx://` DSN 方案，遍历所有 Sheet 作为表。内建于主模块，标准构建即包含（`github.com/xuri/excelize/v2` 为永久依赖）
- **文件编码支持**: UTF-8 默认，`?encoding=gbk` 参数支持 GBK/GB2312/GB18030 编码
- **自定义分隔符**: CSV 默认逗号，TSV 默认制表符，支持 `?delimiter=tab|pipe|semicolon` 覆盖
- **类型推断共享**: 新建 `connector/infer.go`，按 INTEGER → FLOAT → DATE → TEXT 优先级判定列类型
- **只读查询执行**: `execute` 子命令支持 CSV/TSV/XLSX——仅 `SELECT * [LIMIT N [OFFSET M]]`，不支持 WHERE/JOIN/ORDER BY。文件查询绕过 sqlguard（文件天生只读）但仍受策略引擎约束（DENY_TABLES, MASK_COLUMNS）
- **CLI 帮助子命令**: `dbexplain csv` / `dbexplain xlsx` 输出中英双语专项参考手册

### 文档更新
- `docs/FILE_PROCESSING.md`: CSV/TSV/XLSX 文件处理专项文档（新建）
- `docs/test/`: 分层测试文档目录（新建，12 个文件覆盖所有功能）
- `README.md` / `README_EN.md`: 支持数据源新增 CSV/XLSX 条目；更新下载 URL 版本号
- 所有安装/卸载脚本版本号同步更新

### 输出日志优化
- **采集进度消息路由**: `[采集中]` / `[完成]` 从 stderr 移至 per-label 日志文件（`/var/log/dbexplain/<label>.log`），不再污染 `--json` / `--human` 输出
- **第三方库警告重定向**: `log.SetOutput()` 将 Qdrant 等第三方库的 stderr 警告重定向到 `/var/log/dbexplain/dbexplain.log`
- **采集汇总日志**: 新增 `collect.log`，记录全部 DSN 采集完成时长或失败汇总

### CLI 与 UX
- **`--human` 可放查询语句后**: Go flag 遇到 SQL 位置参数后停止解析，`execute "SELECT 1" --human` 在扫描 `fs.Args()` 后正确生效
- **`--label` 全局标志**: schema 采集模式新增 `--label` 作为 `-include` 别名，与 execute 子命令行为一致

### 策略引擎修复 (ISSUE-062)
- **`DENY_TABLES=schema` Schema 前缀匹配修复**: `extractTableNames()` 原先只提取 `TABLES`（丢弃 `information_schema.`），导致 `DENY_TABLES=information_schema` 不生效。修复为提取全限定名 `information_schema.TABLES` 并拆分为 schema + table 两部分分别匹配
- **`DENY_COLUMNS=table.col` 全字段查询绕过修复**: SQL `SELECT * FROM table` 无显式列引用绕过列级检查。新增 `matchStarSelect()` 检测 `SELECT *` 并匹配表前缀
- **MongoDB/Qdrant 原生查询列级绕过修复**: `CheckNative()` 原先跳过列级检查。`{"find":"collection"}` 全字段返回时检查 `DENY_COLUMNS=collection.field` 是否匹配，除非投影已排除该字段

### 单二进制架构（合并）
- 合并 `build_excel.sh` + `src_excel` 子模块进入主模块，`github.com/xuri/excelize/v2` 为永久编译依赖
- 单二进制 41MB，零运行时依赖，xlsx 贡献约 2.1MB（~5%）

### 版本跟踪
- 版本号: v0.0.9
- 新增数据源类型: CSV/TSV/XLSX 文件

## v0.0.8 (2026-05-27)

### 安全策略引擎 (ISSUE-061)
- **细粒度访问控制**: 新增 `src/policy/` 包，三层拒绝策略——语句级（子串匹配）、表级（表名/集合名/Key名提取）、列级（`table.column` 引用匹配），在 `sqlguard` 校验之后、查询执行之前提供第二层访问控制
- **9 种数据库全覆盖**: SQL 类（MySQL/PostgreSQL/GaussDB/SQLite/ClickHouse/Elasticsearch）支持全部三层策略；MongoDB/Qdrant 支持语句+集合级；Redis 支持语句+Key 级（含通配符匹配）
- **全局+按 DSN 配置**: `DENY_TABLES`/`DENY_COLUMNS`/`DENY_STATEMENTS` 支持全局配置和 `DB<n>_` 前缀按 DSN 追加
- **列值屏蔽**: `MASK_COLUMNS` 执行后替换敏感列值（如 `password_hash=***`），替代硬阻断。支持通配符匹配，所有数据库通用
- **专用文档**: 新建 `docs/POLICY.md`，按 9 种数据库逐一说明禁用规则和配置方式
- **单元测试**: 39 测试用例（Load/CheckSQL/CheckNative/Extract 全覆盖）+ 10+ 安全绕过回归用例

### 凭据保护
- **DSN 错误消息脱敏**: 新增 `sanitizeErr()` 函数，DSN 解析错误中的密码在输出到 stderr 前统一脱敏，防止畸形 DSN 泄露凭据
- **OS 环境变量隔离**: `loadEnvFile()` 重构为直接返回 `[]dsnEntry`，消除通过 `os.Setenv`→`os.Getenv` 传递 DSN 密码的中间窗口。非 DSN 配置项不受影响
- **ClickHouse 请求头鉴权**: `chHTTP.query()` 鉴权方式从 URL 查询参数改为 `X-ClickHouse-User`/`X-ClickHouse-Key` 请求头，防止密码在 HTTP 日志或 Referer 头中泄露（关闭 ISSUE-043）

### 策略绕过防护
- **引用标识符归一化**: `extractTableNames()`/`extractColumnRefs()` 新增 `normalizeIdentifiers()` 预处理，剥离反引号/双引号/方括号后再提取，防止 ``SELECT * FROM `sensitive` `` 绕过表级拒绝
- **空白字符归一化**: `CheckSQL()`/`CheckNative()` 新增 `normalizeWhitespace()`，将所有空白序列折叠为单空格后匹配，防止 `DROP  TABLE` 绕过语句级策略
- **Redis 通配符重写**: `filepath.Match` 将 `/` 视为路径分隔符导致 `CONVERSATION:*` 不匹配 `CONVERSATION:abc/123`。自实现 `globMatch()` 对所有字符等同对待
- **子查询 LIMIT 防绕过**: `AutoLimit()` 新增 `hasOuterLimit()`，剥离括号内子查询内容后再检测 LIMIT，防止 `SELECT * FROM (SELECT ... LIMIT 99999) AS t` 绕过自动注入

### 输出安全
- **终端注入防御**: `--human` 输出新增 `sanitizeCell()`，剥离 ANSI 转义序列（ESC+`[...`+字母）及控制字符（0x00-0x1F、0x7F），保留 tab/换行/回车。JSON 输出由 Go `json.Encoder` 原生处理，无需额外防护。覆盖全部 9 种数据库
- **列宽上限**: `formatHuman()` 新增 `maxColWidth=256`，超长 cell 截断并追加 `…` 标识。仅 `--human` 生效，防止巨量 cell 撑爆终端/内存

### 连接与并发
- **ES 证书验证参数化**: 新增 DSN 参数 `?tls-skip-verify=true`，替代全局硬编码 `InsecureSkipVerify`（关闭 ISSUE-042）。ES 帮助文档同步更新
- **Schema 采集并发限流**: 新增 `--conn N` 参数（默认 10），使用 channel 信号量限制 schema 采集的并发 goroutine 数

### CLI 与诊断
- **`list` 索引对齐**: `dbexplain list` 的 INDEX 列从 `envKey`（DB1/DB2）改为纯序号（1/2/3），与 `execute --db N` 的 1-based 位置索引一致
- **Malformed glob 告警**: `policy.go` 中 `globMatch()` 和 `filepath.Match()` 错误忽略处增加 `log.Printf` 警告输出，便于发现配置错误

### 文档更新
- `docs/POLICY.md`: 安全策略引擎完整文档（新建）
- `docs/EXECUTE.md`: 安全架构章节补充策略防绕过、输出安全说明
- `docs/SECURITY_CHECKLIST.md`: 新增 10+ 安全检查项（凭据保护/输入验证/运行时安全/传输安全）
- `docs/CLICKHOUSE.md`: 认证方式更新（URL 参数 → 请求头）
- `docs/ELASTICSEARCH.md`: TLS 描述更新（硬编码跳过 → `?tls-skip-verify=true` 参数化）
- `src/execute_test.go`: 新增 13 个测试用例（sanitizeCell/formatHuman 全覆盖）
- 删除 `docs/TEST_v0.0.7.md`，新建 `docs/TEST_v0.0.8.md`

### 跟踪问题
- **ISSUE-061**: 细粒度安全策略引擎（v0.0.8 已实现）
- **ISSUE-034**: GaussDB/TDSQL 兼容性文档（v0.0.8 已实现）
- **ISSUE-042**: ES InsecureSkipVerify 硬编码（v0.0.8 已关闭）
- **ISSUE-043**: ClickHouse URL 密码泄露（v0.0.8 已关闭）

## v0.0.7 (2026-05-26)

### Go 模块化发布 (REQ-1)
- **模块路径**: `module dbexplain` → `module github.com/IamWWT/dbexplain`，符合 Go 模块规范
- **18 个文件 44 行 import 路径**全部替换为完整模块路径
- **公共 API**: 新建 `src/core/` 包，导出 `Collect()` / `CollectToGraph()` / `CollectToJSON()` 三个函数，VeinMap 等 Go 项目可直接 import 调用
- **IR Graph 构建器**: `src/core/graph.go` — `BuildGraph()` 将 schema.Instance 转为 IR Graph（节点+列+边）

### Schema 增强 (REQ-2, REQ-3, REQ-6, REQ-7)
- **ForeignKey 补全**: 新增 `OnDelete` / `OnUpdate` 字段（CASCADE、SET NULL、RESTRICT、NO ACTION）
- **SQLite FK 采集**: `PRAGMA foreign_key_list` 中原有的 on_update/on_delete 数据现已正确存入 ForeignKey 结构
- **MySQL FK 补全**: 新增 `information_schema.REFERENTIAL_CONSTRAINTS` 查询，获取 DELETE_RULE / UPDATE_RULE
- **PostgreSQL FK 补全**: FK 查询追加 `confupdtype` / `confdeltype` 列，`pgFKAction()` 将单字符码映射为可读字符串
- **JSON refs 增强**: `jsonRef` 新增 8 个结构化字段（from_instance/from_db/from_table/from_col/to_instance/to_db/to_table/to_col），同时保留 from/to 向后兼容
- **IR Graph 边元数据增强**: `BuildGraph()` 在 Edge Metadata 中输出 constraint_name / on_delete / on_update

### Bug 修复 (REQ-5)
- **SQLite INTEGER PRIMARY KEY nullable 修复**: 将 `c.Nullable = notnull == 0` 修正为 `c.Nullable = notnull == 0 && pk == 0`，SQLite 自增主键不再误标为 nullable

### 运行环境增强 (REQ-4)
- **日志目录回退**: `/var/log/dbexplain` 不可写时，自动回退到 `$XDG_STATE_HOME` → `$HOME/.local/state` → `os.TempDir()`，解决容器/非特权用户环境日志写入失败问题
- **`resolveLogDir()`**: 新增多级回退辅助函数

### 安全审计 (REQ-8)
- **全链路密码审计**: 审查 8 个 connector + render.go + main.go 所有输出路径
- 确认 JSON 输出（Redacted DSN）、label 字段（无密码）、日志文件（Redacted）、-context 输出（name-only）全链路无密码泄露

### 只读查询执行 (REQ-10)
- **`dbexplain execute`**: 新增 execute 子命令，在沙箱保护下执行只读查询，返回结构化数据表（与 schema 采集 JSON 格式完全分离）
- **sqlguard 只读校验**: 新建 `src/sqlguard/` 包，三层防护——动词白名单（SELECT/EXPLAIN/WITH/SHOW/DESCRIBE/DESC/PRAGMA）、多语句检测（拒绝分号拼接）、自动 LIMIT 注入（无 LIMIT 时追加 `LIMIT 1000`）
- **query 查询引擎**: 新建 `src/query/` 包，定义 `Queryable` 接口（独立于 `Connector`）、`QueryResult`/`ExecuteOpts` 统一类型、`QueryLock` per-label 并发互斥
- **9 种数据库全覆盖**: 5 种 SQL 数据库（MySQL/PostgreSQL/GaussDB/SQLite/ClickHouse）走 sqlguard 校验 + `database/sql` 执行，Elasticsearch 通过 `_sql` REST 端点支持标准 SQL
- **非 SQL 数据库原生查询支持**:
  - Elasticsearch: `_sql` REST 端点，响应 `{"columns": [...], "rows": [...]}`
  - MongoDB: JSON 格式 `{"find":"collection","filter":{...},"limit":100}` / `{"aggregate":"collection","pipeline":[...]}`
  - Redis: 空间分隔原生命令，30+ 命令白名单（GET/HGETALL/SCAN/PING 等），拒绝 SET/DEL 等写操作
  - Qdrant: JSON 格式 `{"scroll":"collection_name","limit":100}` / `{"count":"collection_name"}`
- **查询路由机制**: `isSQLKind()` 根据 DSN 类型决定校验路径，SQL 类走 sqlguard，非 SQL 类各连接器内部白名单验证
- **双超时保护**: 应用层 context 超时 + 数据库层语句超时（MySQL `max_execution_time` / PG `statement_timeout` / CH `max_execution_time`）
- **安全文档**: 新建 `docs/EXECUTE.md`，全面记录安全架构、输出格式、使用示例和 CONSTITUTION 合规情况
- **`--human` 表格输出**: execute 新增 `--human` 参数，查询结果以 ASCII 表格渲染（类 mysql/pg CLI 风格），替代默认 JSON。NULL 值清晰标注，自动列宽对齐。9 种数据库通用
- **CLI 案例库**: 新建 `docs/CLI_EXAMPLES.md`，覆盖 7 个有数据的数据源共 13 条可执行查询，全部经本环境实测验证

### 安全增强
- **Redacted() 凭证脱敏修复**: URL 编码密码（如 `%23`）不再泄露；用户名和密码同时脱敏为 `{dbuser}:{dbpassword}` 占位符，替代原来的 `user:***` 格式
- **`dbexplain list` 子命令**: 列出所有已配置数据库的 INDEX/LABEL/KIND/HOST:PORT/DATABASE 映射表，零凭证暴露，加密 `.env` 自动解密
- **`-env` DSN 映射摘要**: 采集开始前打印 `DB1 → label (kind://{dbuser}:{dbpassword}@host/db)` 映射，方便确认 `--db N` / `--label` 对应关系

### 测试覆盖 (v0.0.7 补强)
- **sqlguard 单元测试**: 28 用例 — Validate() 全部动词白/黑名单、多语句边界/空查询/空白前导/括号 CTE；AutoLimit() 追加/跳过/尾部分号/大小写检测
- **query 单元测试**: 15 用例 — QueryLock 加锁/解锁/并发互斥/多标签独立/重入验证/规模测试
- **MongoDB/Redis 实机验证**: openim-redis:6389 + video-redis:6379 + mongo-test:27017 完成端到端 execute 测试
- **Bug 修复**: Redis ExecQuery Do() 参数遗漏（命令名未传入 go-redis，已修复）
- **总测试用例**: 231+ → 120 单元 (dsn:33 + schema:44 + sqlguard:28 + query:15) + 111 集成/CLI

### 跟踪问题
- **ISSUE-054 ~ ISSUE-060**: v0.0.7 新增 7 个需求跟踪 issue

## v0.0.6 (2026-05-21)

### 配置加密
- **`dbexplain encrypt`**: 新增 encrypt 子命令，使用机器指纹加密 `.env` 配置文件
- **机器指纹模式（默认）**: 基于硬件特征（machine-id/主板 UUID/CPU 型号/hostname）生成加密密钥，无需密码，加密后文件仅能在同一台机器上解密
- **密码增强模式**: `encrypt --password` 提供 PBKDF2-HMAC-SHA256(100k) 双重保护（密码 + 机器指纹）
- **运行时自动解密**: `-env` 模式自动检测加密文件（首字节 0x00/0x01），无需额外参数
- **`APP_ENCRYPTION_KEY`**: 密码模式通过此环境变量提供解密密码（可选覆盖，默认从 `~/.config/dbexplain/.encryption_key` 文件读取）
- **跨平台纯 Go**: Linux (`/etc/machine-id`, DMI, `/proc/cpuinfo`)、macOS (`sysctl hw.*`)、Windows (Registry MachineGuid)，CGO_ENABLED=0
- **加密算法**: XChaCha20-Poly1305 (AEAD) + SHA-256 / PBKDF2-HMAC-SHA256 密钥派生
- **安全文件权限**: 加密输出文件 `0600`，密码输入不回显，解密失败不暴露内部原因

### 配置搜索增强
- **findConfigFile()**: 新增 `.env.dbexplain.enc` 和 `.env.enc` 搜索支持，加密文件与明文文件统一搜索优先级

### 文档
- `README.md` / `README_EN.md`: 新增"加密配置文件"章节，包含完整使用示例
- `--manual` 手册新增加密子命令完整文档（中英双语）
- `-h` 帮助新增"加密"参数组
- `docs/SECURITY_CHECKLIST.md`: 新增"配置加密检查"章节
- `docs/ARCHITECTURE.md`: 新增"配置加密架构"章节
- `.gitignore`: 新增 `*.enc` 排除规则

### CLI 子命令层级重构
- **`dbexplain <dbtype>`**: 9 个数据库类型子命令（mysql/postgres/gaussdb/clickhouse/sqlite/redis/mongodb/elasticsearch/qdrant），每个输出对应数据库的专用参考手册
- **别名支持**: `postgres`=`pg`/`postgresql`, `clickhouse`=`ch`, `sqlite`=`sqlite3`, `elasticsearch`=`es`
- **`dbexplain all`**: 替代 `--manual`，完整参考手册。支持 `--filter` 关键词过滤和 `--language zh|en`
- **`dbexplain -h`**: 重新设计为简洁结构化概览（Usage / Database types / Flags / Examples / See），从 8 组参数分栏升级为子命令层级
- **向后兼容**: `--manual` 仍可用，stderr 输出废弃提示引导用户使用 `dbexplain all`

### 安装脚本增强
- **移除 `DBPROBE_ENV_FILE` 交互提示**: `findConfigFile()` 自动搜索机制消除手动配置需求，安装脚本不再询问设置环境变量
- **加密引导**: `install.sh` / `install.ps1` / `install-skill.sh` 成功消息新增"加密配置"引导步骤
- **`dbexplain all`**: 安装脚本帮助和成功消息中 `dbexplain --manual` 替换为 `dbexplain all`

### 跟踪问题
- **ISSUE-053**: 未来大版本移除明文 `.env` 支持，仅保留加密文件（`open`, `security/breaking-change/future`）

## v0.0.5 (2026-05-21)

### 一键安装与部署
- **`scripts/install.sh`**: Linux/macOS 一键安装脚本，支持在线（GitHub Releases）和离线模式
- **`scripts/install.ps1`**: Windows PowerShell 一键安装脚本，自动配置用户 PATH
- **`scripts/uninstall.sh` / `scripts/uninstall.ps1`**: 配套卸载脚本，支持静默模式（`--all`）
- **`scripts/install-skill.sh`**: Skill 多平台部署脚本（交互选择目标平台）
- **`scripts/uninstall-skill.sh`**: Skill 卸载脚本
- **全局安装**: 二进制安装到系统 PATH（Linux/macOS: `/usr/local/bin/dbexplain`，Windows: `%LOCALAPPDATA%\dbexplain\`）
- **用户级配置**: 配置文件 `.env.dbexplain` 按 XDG 规范存放（`~/.config/dbexplain/`），可选设置 `DBPROBE_ENV_FILE` 指向自定义路径

### 配置搜索
- **多级回退**: `DBPROBE_ENV_FILE` → `.env.dbexplain` (CWD) → `~/.config/dbexplain/.env.dbexplain` → `.env` (CWD, 旧版兼容)
- 不再需要 `cd <skill-dir>`，工具在任意目录均可运行 `-env` 模式

### 新参数
- **`--log-dir <dir>`**: 用户可指定日志输出目录（默认 `/var/log/dbexplain`），影响 `filter.log` 和各实例独立日志

### Skill 适配
- **SKILL_ZH.md / SKILL_EN.md**: Skill 中英文分拆，`SKILL.md` 保留为中文副本供平台自动发现
- **SKILL.md**: 移除 `cd <skill-dir>` 要求，更新为全局 `dbexplain` 调用方式，添加多级配置搜索路径说明
- **Skill 安装脚本**: 优先检测系统 PATH 中的 `dbexplain`，Skill 目录中的二进制改为 `dbexplain` symlink（平台无关名）
- **`--lang zh|en`**: `install.sh` 和 `install-skill.sh` 新增语言参数，支持安装中文或英文版 Skill
- **版本号**: install/uninstall skill 脚本升级到 v0.0.5

### 文档
- `--manual` 手册更新：添加配置搜索优先级章节、`--log-dir` 参数、所有 `./dbexplain` 改为 `dbexplain`
- **新增 `docs/SECURITY_CHECKLIST.md`**：发布前安全检查手册，涵盖凭证保护、文件编码、输入验证等 7 大类检查项

### Bug 修复 (13 项)

| Issue | 严重度 | 描述 |
|-------|--------|------|
| ISSUE-040 | CRITICAL | `.env` 真实凭证已从 Git 追踪中移除，`.gitignore` 新增 `src/.env` |
| ISSUE-041 | HIGH | `src/logs/` 生产日志目录加入 `.gitignore`，防止泄露数据库名 |
| ISSUE-044 | LOW | 删除 `analyze/infer.go` 死代码，消除 `strings.Contains(name, "ip")` 误匹配 bug |
| ISSUE-045 | MEDIUM | PostgreSQL 采样行为空表添加 `RowCount > 0` 守卫，对齐 MySQL/ClickHouse |
| ISSUE-046 | LOW | `longestCommonPrefix` 无 `_`/`-` 分隔符时保留完整前缀，聚类名不再变空串 |
| ISSUE-047 | MEDIUM | GaussDB 实例 Kind 从硬编码 `"postgres"` 修复为 DSN 指定值 `"gaussdb"` |
| ISSUE-048 | MEDIUM | JSON 输出补充 `op_stats` 字段（seq_scan/idx_scan/query_count 等操作统计） |
| ISSUE-049 | LOW | MySQL 两次 `SHOW INDEX` 查询合并为一次，网络往返减半 |
| ISSUE-051 | HIGH | `-json -o` 输出不再添加 UTF-8 BOM，确保标准 JSON 解析器兼容 |
| ISSUE-052 | HIGH | Windows 记事本保存的 `.env.dbexplain` 含 UTF-8 BOM 导致解析失败；godotenv 错误消息泄露密码 |

### 安全已知限制 (2 项，保持开放)

| Issue | 描述 |
|-------|------|
| ISSUE-042 | ES TLS `InsecureSkipVerify=true`，诊断工具场景可接受，长期需支持证书配置 |
| ISSUE-043 | ClickHouse 密码通过 URL 查询参数传输，建议改用 HTTP Basic Auth Header |

## v0.0.4 (2026-05-20)

### 核心架构
- **IR v1**: 通用图原语（Node、Column、Edge），独立于数据库类型
- **能力架构**: 连接器声明能力，提取器按能力工作而非按数据库类型
- **统一诊断**: 确定性问题检测器（MissingPK、LargeTableWithoutIndex、NoTTL、StaleStream 等）

### 新功能
- **重要性排序**: 多因子加权评分（图度、外键中心性、行数、索引密度、写入强度、查询频率），操作统计数据不可用时自动降级
- **上下文压缩**: 分层 AI Agent 输出 — `summary.json`、`topology.json`、`diagnostics.json`、`retrieval_chunks/`
- **Schema 指纹**: 对列、索引、外键进行 SHA-256 哈希，支持增量变更检测（`--cache` 参数）
- **操作统计 (Phase 3)**: 从内建系统表采集每表查询频率和写入强度（零配置，自动降级）
- **`--manual` 参数**: 完整帮助手册，支持按数据库分类展示和 `--language zh|en` 语言切换

#### 功能与输出位置对照

| 功能 | 触发参数 | 输出位置 | 效果 |
|------|----------|----------|------|
| 重要性排序 | 默认启用 | 终端：表排列顺序；`--context`：`summary.json` 中 `importance_score` 字段 | 重要表排前面，Agent 优先关注 |
| 上下文压缩 | `--context <dir>` | `summary.json` / `topology.json` / `diagnostics.json` / `chunks/*.md` | 分层结构化输出，直接喂给 AI Agent |
| Schema 指纹 | `--cache <file>` | `<file>` 快照 + `<file>_delta.json` 增量差异 | 增量变更检测，配合 cron 做监控 |
| 操作统计 | 默认启用（自动降级） | `summary.json` 中 `query_frequency` / `write_intensity` | 影响重要性排序权重；不可用时自动回退 |
| 人类友好输出 | `--human` | 终端：`[table=]`/`[pattern=]` 等上下文标记 | 明确标注数据来源类型 |
| 过滤日志 | `-include` / `-exclude` | `logs/filter.log` | 跳过消息不污染终端输出 |
| 完整手册 | `--manual [--filter x] [--language en]` | 终端标准输出 | 600+ 行按数据库分类的详细文档 |
| 文件输出 BOM | `-o <file>` | 输出文件头部自动添加 UTF-8 BOM | Windows 记事本/CMD 正确显示中文 |

### Windows 兼容性
- **UTF-8 BOM**: `-o` 文件输出自动添加 BOM，Windows 记事本/CMD 正确识别编码
- **系统代码页自适应**: Windows 下运行时检测 ANSI 代码页（ACP），中文系统（936）自动转 GBK，`type` 命令和记事本均正确显示中文；其他 locale 保持 UTF-8 BOM
- **ANSI 转义码修复**: `noColor` 从包初始化变量改为运行时函数，防止转义码泄漏到捕获的文件输出中
- **ASCII 安全渲染**: 将 Unicode 制表符（`─` U+2500）、项目符号（`•` U+2022）和省略号（`…` U+2026）替换为 ASCII 等效字符

### Bug 修复
- 修复 `GetConnector` 中的 TOCTOU 竞态窗口
- 修复 DSN 过滤跳过消息中的密码泄漏（`parsed.Redacted()` 替代 `e.raw`）
- 修复终端颜色输出丢失（仅 `-o` 时走 capture pipe，终端直接输出到 stdout）
- 修复 `go vet` 非恒定格式串警告（`fmt.Fprintf` → `fmt.Fprint`）

### 交互增强
- **`--filter` 参数**: `--manual --filter <关键字>` 按行过滤手册输出，方便快速查找（忽略大小写）
- **`-h` 重组**: 从字母序 dump 升级为 7 组分栏输出（数据源/过滤/输出控制/显示格式/AI 上下文/性能/帮助），中英双语随 `--language` 切换
- **`-h` 双语**: `-h --language en` 输出英文帮助，默认中文；预扫描 `--language` 实现
- **`--human` 参数**: 人类友好输出，带 `[table=]`/`[pattern=]`/`[database=]`/`[instance=]` 上下文标记
- **上下文标记**: 不同数据库类型使用不同标签（SQL=table, Redis=pattern, MongoDB/Qdrant=collection, ES=index）
- **过滤日志重定向**: `-include`/`-exclude` 的跳过/排除消息写入 `logs/filter.log`，不再污染终端输出，保持报告干净可读（人和 AI 均适用）

### 文档
- `docs/ARCHITECTURE.md`: Database Context Compiler 架构愿景，新增安全性章节（密码防泄漏为第一要义）
- `docs/ALGORITHMS.md`: 完整算法参考，含兼容性矩阵和兜底机制
- `docs/TEST_METHODOLOGY_v0.0.4.md`, `docs/TEST_REPORT_v0.0.4.md`: 分层测试方法论与实测报告（83+ 用例，含真实 shell 执行输出）
- README 新增"使用场景"章节（AI Agent 用法 / 人类用法 / 9 种数据库示例）
- `MEMORY.md` 新增版本性能对比章节（每次发版必做）
- 宪法更新：新增 IR 优先、纯确定性、Graph First 原则

---

## v0.0.3

- 多 Schema 采集（PostgreSQL/GaussDB）
- SSL 模式配置
- DSN 过滤（`--include`/`--exclude`）
- 单元测试和 CI/CD 流水线
- Skill 安装/卸载脚本

## v0.0.2

- 并发采集（goroutine）
- 每连接器 panic 隔离
- Redis 流式 key 分析与模式推断
- 基于采样行的列注释推断
- 连接器自注册模式
- 大表采集进度日志

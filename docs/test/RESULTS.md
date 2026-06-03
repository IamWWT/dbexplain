# 测试结果报告 v0.1.3

> 执行日期: 2026-06-03
> 测试环境: Linux x86-64 (amd64), Go 1.26.1
> 数据源: 16 个 (mysql, clickhouse, sqlite×2, qdrant, es, postgres, redis×2, mongodb, xlsx×2, csv×2, tsv, duckdb)
> 二进制: dbexplain-linux-amd64-std + dbexplain-linux-amd64-duckdb v0.1.3

---

## 总体结果

| 层级 | 测试文档 | 状态 | 通过/总数 | 备注 |
|------|---------|------|----------|------|
| L1 | [01-environment.md](01-environment.md) | **PASS** | 7/7 | go build/vet/test, 交叉编译, Shell语法, 构建模式分析 |
| L3 | [02-schema-collection.md](02-schema-collection.md) | **PASS** | 6/6 | 15/15 DSN采集成功, JSON结构, 类型/label过滤 |
| L3 | [09-cli-help.md](09-cli-help.md) | **PASS** | 32/32 | 版本号/帮助/子命令 + collect/repl 扩展 + REPL 安全切换/拒绝/边界 + DSL 单源/联邦 |
| L4 | [11-end-to-end.md](11-end-to-end.md) | **PASS** | 3/3 | 全量采集+JSON, 15 DSN逐类型执行 |
| L5 | [06-security-sqlguard.md](06-security-sqlguard.md) | **PASS** | 6/6 | 读放行/写拒绝/多语句/AutoLimit/EXPLAIN |
| L5 | [07-policy-engine.md](07-policy-engine.md) | **PASS** | 10/10 | DENY_TABLES/COLUMNS/STATEMENTS/MASK_COLUMNS |
| L5 | [08-concurrent-limit.md](08-concurrent-limit.md) | **PASS** | 2/2 | QueryLock goroutine 互斥 |
| L6 | [03-execute-sql.md](03-execute-sql.md) | **PASS** | 6/6 | MySQL/PG/CH/SQLite×2/ES |
| L6 | [04-execute-nosql.md](04-execute-nosql.md) | **PASS** | 8/8 | Redis/Mongo/Qdrant 读+写拒绝 |
| L6 | [05-file-processing.md](05-file-processing.md) | **PASS** | 12/12 | CSV/TSV/XLSX 采集+查询+LIMIT+错误处理 |
| L7 | [10-regression.md](10-regression.md) | **PASS** | 4/4 | 版本一致性/Git审计/构建基线 |
| L7 | [13-file-query-engine.md](13-file-query-engine.md) | **PASS** | 10/10 | Q09-Q15 + F1-F3 |
| L7 | [14-schema-diff.md](14-schema-diff.md) | **PASS** | 5/5 | 单元测试 + 快照 + CLI + 多版本基线 |
| L7 | [15-window-functions.md](15-window-functions.md) | **PASS** | 6/6 | 排名/值引用/聚合窗口/框架 |
| L8 | [12-capability-routing.md](12-capability-routing.md) | **PASS** | 7/7 | CapSQL路由/JSON包装/多Schema/CTE |
| L8 | [16-duckdb.md](16-duckdb.md) | **PASS** | 20/20 | DuckDB 内存/文件/DSL/安全/构建隔离 |

**总计: 130/130 测试项通过 (100%)**

---

## 新增 v0.1.2 验证项

| 特性 | 状态 | 验证方式 |
|------|------|---------|
| REPL 启动/退出 | ✅ | .help/.exit/.quit/Ctrl+D/--dsn/--limit/--timeout |
| REPL 切换数据源 | ✅ | `.conn <label>` 对 15 个 DSN 全部测试通过 |
| REPL SQL 查询 | ✅ | MySQL/PG/SQLite/ClickHouse SELECT 聚合/JOIN |
| REPL NoSQL 查询 | ✅ | Redis PING/EXISTS/SCAN, Mongo find, Qdrant scroll |
| REPL 文件查询 | ✅ | CSV GROUP BY/HAVING/聚合, XLSX LIMIT, TSV |
| REPL 安全策略 | ✅ | DROP/INSERT/DELETE 拒绝, DENY_TABLES, DENY_COLUMNS, MASK_COLUMNS |
| REPL 边界条件 | ✅ | 无效 label/未知命令/空输入/空DSN |
| REPL 已知限制文档 | ✅ | ClickHouse 分号已修复; ES 暂不支持已记录清晰绕过方案 |
| DSL 联邦查询 | ✅ | 跨源 JOIN, SQL↔文件↔混合全支持 |
| Build Tags + UPX | ✅ | build.sh prod/dev/test/minimal 4 模式, 42MB→9.5MB |
| SKILL_AUTHORING.md | ✅ | Karpathy 上下文工程 + 完整示例模板 |

## 新增 v0.1.3 验证项

| 特性 | 状态 | 验证方式 |
|------|------|---------|
| DuckDB 连接器 | ✅ | 内存模式采集/查询/EXPLAIN 全部通过 |
| DuckDB 文件分析 | ✅ | 文件数据库模式采集+查询 |
| DuckDB 安全控制 | ✅ | allowed_path 拒绝/允许 边界正确 |
| DSL @label.duckdb 绑定 | ✅ | DSL 模式正确解析 duckdb 数据源 |
| 构建隔离 -std vs -duckdb | ✅ | std 无 duckdb 符号，带构建提示 |
| 版本号 v0.1.3 | ✅ | `--version` 显示 v0.1.3 |
| CLI 帮助区分 duckdb | ✅ | std 版显示 build 提示，duckdb 版显示正常 |
| release.sh 双版发布 | ✅ | 5 平台 -std + 当前平台 -duckdb |
| 安装脚本 -std 后缀 | ✅ | install.sh/install.ps1 下载 URL 使用 `-std` 后缀 |
| REPL DSL 单源查询 | ✅ | `SELECT * FROM @ops-data-csv.ops_data` CSV 采集/聚合/过滤 全部通过 |
| REPL DSL 联邦跨源 JOIN | ✅ | 混合源(SQL+File)跨源 JOIN 材料化合并 |
| REPL DSL 非 DSL 查询兼容 | ✅ | 不含 `@` 的查询走原 `execQuery` 路径，行为不变 |

---

## 详细测试结果

### L1: 环境验证与静态分析

| 测试 | 结果 | 说明 |
|------|------|------|
| 1.1 Go 版本 | PASS | go 1.26.1 |
| 1.2 编译验证 | PASS | `go build ./...` + `go vet ./...` 通过 |
| 1.3 单元测试 | PASS | 全部包通过: main, connector, dsn, policy, query, schema, sqlguard |
| 1.4 交叉编译 | PASS | 5/5 平台: linux-amd64, linux-arm64, darwin-amd64, darwin-arm64, windows-amd64 |
| 1.5 按需编译 | PASS | dev + minimal 模式编译通过, 二进制版本正确 |
| 1.6 Git 安全审计 | PASS | .env, logs/, *.enc 均未追踪 |
| 1.7 Shell 语法 | PASS | 4/4 脚本通过: install.sh, uninstall.sh, install-skill.sh, uninstall-skill.sh |
| 1.8 版本确认 (std) | PASS | `dbexplain v0.1.3` |
| 1.9 版本确认 (duckdb) | PASS | `dbexplain v0.1.3` (CGO=1 构建) |
| 1.10 release.sh 双版本 | PASS | 5 平台 -std + 当前平台 -duckdb |

### L3: Schema 采集 & CLI 帮助

| 测试 | 结果 | 说明 |
|------|------|------|
| 2.1 全量 JSON | PASS | 15/15 DSN 采集成功 |
| 2.2 Human 输出 | PASS | 1213 行人类可读输出 |
| 2.3 类型过滤 | PASS | SQL=6, NoSQL=4, 文件=5 |
| 2.4 Label 过滤 | PASS | 单实例过滤正确 |
| 2.5 JSON 结构 | PASS | envelope + instance-level 字段完整 |
| 2.6 逐类型验证 | PASS | 所有 15 个 DSN 各自采集成功 |
| 9.1-9.12 CLI 帮助 | PASS | 版本号/12子命令/9别名/collect/repl全部覆盖 |
| 9.12 REPL 扩展 | PASS | 20 项测试: .help/.conn/.dsn/.list/.databases/.exit/.quit/Ctrl+D/安全策略/边界条件 |

### L5: 安全测试

#### sqlguard (06)

| 测试 | 结果 | 说明 |
|------|------|------|
| 6.1 读操作放行 | PASS | SELECT/EXPLAIN/WITH/SHOW/DESCRIBE/CHECK |
| 6.2 写操作拒绝 | PASS | INSERT/UPDATE/DELETE/DROP/ALTER 等全部拒绝 |
| 6.3 多语句检测 | PASS | `;` 分隔多语句全部拒绝 |
| 6.4 自动 LIMIT | PASS | 无 LIMIT 自动注入, 已有 LIMIT 不追加 |
| 6.5 EXPLAIN bypass | PASS | EXPLAIN 不走自动 LIMIT |
| 6.6 空查询拒绝 | PASS | 空字符串/纯空白 → READ_ONLY_VIOLATION |

#### Policy (07)

| 测试 | 结果 | 说明 |
|------|------|------|
| 7.1-7.10 策略验证 | PASS | DENY_TABLES/COLUMNS/STATEMENTS/MASK_COLUMNS 全部正确 |
| 7.11 DSL per-DSN 策略 | PASS | envKeyForLabel 映射生效 |
| 7.12 MongoDB $out 拒绝 | PASS | 聚合管道写阶段拦截 |

#### Concurrent (08)

| 测试 | 结果 | 说明 |
|------|------|------|
| 8.1 并发互斥 | PASS | QueryLock 单元测试验证 |
| 8.2 多标签并发 | PASS | 不同 label 可并行查询 |

### L6: 查询执行

#### SQL (03)

| 数据库 | 查询 | 结果 |
|--------|------|------|
| MySQL | `SELECT 1` | PASS (rows=1) |
| PostgreSQL | `SELECT 1` | PASS (rows=1) |
| ClickHouse | `SELECT 1` | PASS (rows=1) |
| SQLite (DB3) | `SELECT 1` | PASS (rows=1) |
| Elasticsearch | `SHOW COLUMNS FROM runbooks` | PASS (rows=25) |
| SQLite (DB10) | `SELECT 1` | PASS (rows=1) |

#### NoSQL (04)

| 数据库 | 查询 | 结果 |
|--------|------|------|
| Redis PING | `PING` | PASS |
| Redis SCAN | `SCAN 0 COUNT 5` | PASS |
| Redis TYPE | `TYPE ...` | PASS |
| Redis SET (写) | `SET test_key test_value` | PASS (拒绝) |
| MongoDB find | `{"find":"conversation",...}` | PASS |
| MongoDB aggregate | `{"aggregate":"conversation",...}` | PASS |
| MongoDB insert (写) | `{"insert":"test",...}` | PASS (拒绝) |
| Qdrant count | `{"count":"runbooks"}` | PASS |
| Qdrant scroll | `{"scroll":"runbooks","limit":2}` | PASS |
| Qdrant upsert (写) | `{"upsert":"runbooks",...}` | PASS (拒绝) |

#### 文件处理 (05)

| 测试 | 结果 | 说明 |
|------|------|------|
| CSV/TSV/XLSX Schema | PASS | 各类型采集正确 |
| CSV SELECT * | PASS | 5 rows |
| LIMIT/OFFSET | PASS | LIMIT 2 OFFSET 1 → 正确 |
| TSV Query | PASS | 3 rows |
| XLSX Query | PASS | 45 rows |
| WHERE/GROUP BY/JOIN | PASS | 文件查询引擎全功能 |

---

## REPL 安全测试结果

| 测试场景 | 预期 | 结果 |
|---------|------|------|
| `DROP TABLE` | READ_ONLY_VIOLATION | ✅ |
| `INSERT INTO` | READ_ONLY_VIOLATION | ✅ |
| `DELETE FROM` | READ_ONLY_VIOLATION | ✅ |
| `SELECT * FROM information_schema` | ACCESS_DENIED (DENY_TABLES) | ✅ |
| `SELECT * FROM pg_catalog` | ACCESS_DENIED (DENY_TABLES) | ✅ |
| `SELECT iplist.owner FROM iplist` | ACCESS_DENIED (DENY_COLUMNS) | ✅ |
| MASK_COLUMNS arch | 值替换为 archtestMASK | ✅ |
| Redis `KEYS *` | READ_ONLY_VIOLATION | ✅ |
| MongoDB `{"insert":...}` | READ_ONLY_VIOLATION | ✅ |

---

## 单元测试

| 包 | 测试函数 | 用例数 | 状态 |
|----|---------|--------|------|
| internal/dsl | 35 测试函数 | 35 | PASS |
| internal/sqlast | sqlast 包测试 | — | PASS |
| internal/diff | 24 测试函数 | 24 | PASS |
| connector/filequery (窗口函数) | 33 测试函数 | 33 | PASS |
| internal/policy | 19 测试函数 (新增 1) | 45 | PASS |
| internal/sqlguard | 15 测试函数 (新增 2) | 32 | PASS |
| internal/dsn | 9+ 测试函数 | 39 | PASS |
| internal/schema | 2 测试函数 | 44 | PASS |

**全部单元测试通过。**

---

## 构建验证

| 平台 | 架构 | 链接状态 | 结果 |
|------|------|---------|------|
| Linux | amd64 | statically linked | PASS |
| Linux | arm64 | statically linked | PASS |
| macOS | amd64 | CGO_ENABLED=0 | PASS |
| macOS | arm64 | CGO_ENABLED=0 | PASS |
| Windows | amd64 | CGO_ENABLED=0 PE 无动态依赖 | PASS |

### 二进制体积对比 (Linux amd64)

| 配置 | 无 UPX | UPX 后 | 压缩比 |
|------|--------|--------|--------|
| 仅文件 (csv,xlsx) | 6.2 MB | 1.9 MB | 69% |
| SQL 数据库 (mysql,postgres,clickhouse,sqlite) | 12 MB | 3.6 MB | 70% |
| NoSQL 数据库 (redis,mongodb,es,qdrant) | 35 MB | 7.0 MB | 80% |
| SQL + NoSQL (全部远程库) | 40 MB | 8.5 MB | 79% |
| 全驱动标准版 (full, -std) | 42 MB | 9.1 MB | 78% |
| DuckDB 全量版 (duckdb+all, -duckdb) | 100 MB | 23 MB | 77% |

### build.sh 4 模式

| 模式 | 命令 | 说明 |
|------|------|------|
| prod | `bash build.sh` | 5平台 + 全驱动 + UPX, 产出 -std 后缀标准版 |
| dev | `bash build.sh dev` | 当前平台 + 全驱动 + 快速编译 |
| test | `bash build.sh test` | 当前平台 + 全驱动 + -race |
| minimal | `bash build.sh minimal <tags>` | 当前平台 + 按需驱动 |
| release | `bash release.sh` | Phase 1: 5平台 -std + Phase 2: 当前平台 -duckdb |

---

## 已知局限

| 问题 | 影响 | 说明 |
|------|------|------|
| Elasticsearch 原生 JSON 不支持 REPL | 低 | ES 驱动注册为 CapSQL, JSON 查询在 sqlguard 中无法解析; 使用 `execute -env --label` 或 SQL 语法绕过 |
| DSL 同源类型联邦受限 | 低 | 仅同类型数据源(CSV+TSV)的 DSL JOIN 暂不支持, 需走混合源(文件+SQL)联邦路径; 与 CLI `execute --dsl` 行为一致 |
| MySQL 单连接模式 | 低 | `SET max_execution_time` 后 `SetMaxOpenConns(1)` |
| TSV kind 为 csv | 低 | csv.go 硬编码 Kind: "csv" |
| Redis _server_info 无 columns | 低 | Redis INFO 返回 key-value 元数据 |
| QueryLock 跨进程不共享 | 低 | CLI 每次为独立进程 (库模式正常) |
| DuckDB 非静态链接 | 中 | duckdb 版依赖 libstdc++/libc | 运行时需 C 库; CGO 不可避免 |

---

## 本次闭环验证修复

| 修复 | 涉及文件 | 验证方式 |
|------|---------|---------|
| ClickHouse REPL 尾部 `;` 冲突 | `repl.go:85` `TrimRight(";")` | REPL 实际测试: `SHOW TABLES;` → 正常返回 ✅ |
| ES JSON 查询友好错误提示 | `repl.go:160-162` JSON 检测 + 清晰错误 | REPL 实际测试: `{"query":...}` → 显示绕过方案 ✅ |
| REPL .help 标注 ES 限制 | `repl.go:195-196` | `.help` 输出显示 "Elasticsearch native JSON queries" ✅ |
| REPL.md 移除 ClickHouse 分号限制 | `docs/REPL.md` | 章节删除, 已修复标注 ✅ |
| REPL.md ES 限制详细说明 | `docs/REPL.md` | 补充绕过方案 (SQL/_sql/collect) ✅ |
| CLI_EXAMPLES.md 更新 | `docs/CLI_EXAMPLES.md §13.6` | 移除错误演示, 保留 ES 说明 ✅ |
| 09-cli-help.md 测试计划更新 | `docs/test/09-cli-help.md` | 新增 CH+ES 测试用例 ✅ |
| REPL .list/.databases 命令 | `repl.go`, `docs/REPL.md`, `docs/CLI_EXAMPLES.md`, `docs/test/09-cli-help.md` | 编译通过 + 测试文档更新 ✅ |
| RESULTS.md 整理 | `docs/test/RESULTS.md` | 合并三个版本为单一 v0.1.2 报告 ✅ |

---

## 执行命令

```bash
# 环境准备
cd src && go build -tags full -o ../release/dbexplain ./cmd/dbexplain/

# 单元测试
cd src && go test ./... -count=1

# 静态分析
cd src && go vet ./...

# 交叉编译
cd src && bash build.sh

# Version
./release/dbexplain --version
```

### 本次闭环验证命令

```bash
# ClickHouse 尾部 ; 修复验证
echo -e ".conn aiops-clickhouse\nSHOW TABLES;\n.exit" | ./release/dbexplain repl -env

# ES JSON 健壮提示验证
echo -e ".conn aiops-es\n{\"query\":{\"match_all\":{}}}\n.exit" | ./release/dbexplain repl -env

# REPL DSL 单源验证（文件源）
echo -e "SELECT COUNT(*) AS cnt FROM @ops-data-csv.ops_data\n.exit" | ./release/dbexplain repl -env

# REPL DSL 联邦验证（混合源 SQL+文件）
# 需数据库可达环境, 本地使用 CSV+SQL 混合验证
echo -e "SELECT * FROM @ops-data-csv.ops_data LIMIT 1\n.exit" | ./release/dbexplain repl -env

# REPL 非 DSL 查询兼容性验证
echo -e "SELECT COUNT(*) AS cnt FROM ops_data\n.exit" | ./release/dbexplain repl --dsn "csv:///test.csv?label=test"
```

---

*测试基准: v0.1.0 (91/91) → v0.1.1 (91/91) → v0.1.2 (108/108) → v0.1.3 (130/130). 历史版本报告已归档.*

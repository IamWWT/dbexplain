# 测试结果报告 v0.1.0

> 执行日期: 2026-05-29
> 测试环境: Linux x86-64 (amd64), Go 1.26.1
> 数据源: 15 个 (mysql, clickhouse, sqlite×2, qdrant, es, postgres, redis×2, mongodb, xlsx×2, csv×2, tsv)
> 二进制: dbexplain-linux-amd64 v0.1.0

---

## 总体结果

| 层级 | 测试文档 | 状态 | 通过/总数 | 备注 |
|------|---------|------|----------|------|
| L1 | [01-environment.md](01-environment.md) | **PASS** | 7/7 | go build/vet/test, 交叉编译5平台, Shell语法, 安全审计 |
| L3 | [02-schema-collection.md](02-schema-collection.md) | **PASS** | 6/6 | 15/15 DSN采集成功, JSON结构验证, 类型/label过滤 |
| L3 | [09-cli-help.md](09-cli-help.md) | **PASS** | 10/10 | 版本号, 帮助, 12子命令, 9别名, 参数说明 |
| L4 | [11-end-to-end.md](11-end-to-end.md) | **PASS** | 3/3 | 全量采集+JSON验证, 15 DSN逐类型执行, 汇总报告 |
| L5 | [06-security-sqlguard.md](06-security-sqlguard.md) | **PASS** | 6/6 | 8读动词放行, 11写动词拒绝, 多语句, 自动LIMIT, EXPLAIN, 空查询 |
| L5 | [07-policy-engine.md](07-policy-engine.md) | **PASS** | 10/10 | DENY_TABLES/COLUMNS/STATEMENTS, 非SQL数据库, MASK_COLUMNS, 防绕过 |
| L5 | [08-concurrent-limit.md](08-concurrent-limit.md) | **PASS** | 2/2 | QueryLock goroutine级互斥, 多标签并发 (CLI跨进程为设计局限) |
| L6 | [03-execute-sql.md](03-execute-sql.md) | **PASS** | 6/6 | MySQL/PG/CH/SQLite×2/ES 查询执行 |
| L6 | [04-execute-nosql.md](04-execute-nosql.md) | **PASS** | 8/8 | Redis/MongoDB/Qdrant 读+写拒绝 |
| L6 | [05-file-processing.md](05-file-processing.md) | **PASS** | 12/12 | CSV/TSV/XLSX 采集+查询+LIMIT/OFFSET+错误处理 |
| L7 | [10-regression.md](10-regression.md) | **PASS** | 4/4 | 版本一致性, Git安全审计, 构建基线 |
| L7 | [13-file-query-engine.md](13-file-query-engine.md) | **PASS** | 10/10 | Q09-Q15 业务分析查询 + F1-F3 安全策略验证 |
| L8 | [12-capability-routing.md](12-capability-routing.md) | **PASS** | 7/7 | CapSQL路由, JSON包装, PG多Schema, matchStarSelect, CTE策略, 文件策略, 能力一致性 |

**总计: 91/91 测试项通过 (100%)**

---

## 详细结果

### L1: 环境验证与静态分析

| 测试 | 结果 | 说明 |
|------|------|------|
| 1.1 Go 版本 | PASS | go 1.26.1 |
| 1.2 编译验证 | PASS | `go build ./...` + `go vet ./...` 通过 |
| 1.3 单元测试 | PASS | 全部包通过: main, connector, dsn, policy, query, schema, sqlguard |
| 1.4 交叉编译 | PASS | 5/5 平台: linux-amd64, linux-arm64, darwin-amd64, darwin-arm64, windows-amd64 |
| 1.5 Git 安全审计 | PASS | .env, logs/, *.enc 均未追踪 |
| 1.6 Shell 语法 | PASS | 4/4 脚本通过: install.sh, uninstall.sh, install-skill.sh, uninstall-skill.sh |
| 1.7 版本确认 | PASS | `dbexplain v0.1.0` |

### L3: Schema 采集

| 测试 | 结果 | 说明 |
|------|------|------|
| 2.1 全量 JSON | PASS | 15/15 DSN 采集成功, JSON envelope `{instances, refs, groups, issues}` |
| 2.2 Human 输出 | PASS | 1213 行人类可读输出, 含 15 实例详情 |
| 2.3 类型过滤 | PASS | SQL=6, NoSQL=4, 文件=5 (tsv报告为csv, 见已知问题) |
| 2.4 Label 过滤 | PASS | 单实例过滤正确 |
| 2.5 JSON 结构 | PASS | envelope + instance-level 字段完整 |
| 2.6 逐类型验证 | PASS | 所有 15 个 DSN 各自采集成功 |

**JSON 结构**: v0.1.0 使用顶层信封格式:
```json
{
  "instances": [...],
  "refs": [...],
  "groups": [...],
  "issues": [...]
}
```

### L5: 安全测试

#### sqlguard (06)

| 测试 | 结果 | 说明 |
|------|------|------|
| 6.1 读操作放行 | PASS | SELECT, EXPLAIN, WITH CTE, SHOW, DESCRIBE, CHECK — 全部正常返回 |
| 6.2 写操作拒绝 | PASS | INSERT/UPDATE/DELETE/DROP/ALTER/CREATE/TRUNCATE/RENAME/REPLACE/GRANT/REVOKE — 全部 `READ_ONLY_VIOLATION` |
| 6.3 多语句检测 | PASS | `SELECT 1; SELECT 2` 和 `SELECT 1; DROP TABLE users` 均拒绝 |
| 6.4 自动 LIMIT | PASS | `SELECT *` → LIMIT 1000 自动注入, 有 LIMIT 时不追加 |
| 6.5 EXPLAIN bypass | PASS | EXPLAIN 不走自动 LIMIT, 返回执行计划列 |
| 6.6 空查询拒绝 | PASS | 空字符串和纯空白均返回 `READ_ONLY_VIOLATION` |

#### Policy (07)

| 测试 | 结果 | 说明 |
|------|------|------|
| 7.1 语句级拒绝 | PASS | `DENY_STATEMENTS=FLUSHALL + FLUSHALL` → ACCESS_DENIED |
| 7.2 表级拒绝 | PASS | `DENY_TABLES=iplist + SELECT * FROM ...iplist` → ACCESS_DENIED |
| 7.3 列级拒绝 | PASS | `DENY_COLUMNS=...hostip` → ACCESS_DENIED |
| 7.4 MongoDB 集合拒绝 | PASS | `DENY_TABLES=user` → ACCESS_DENIED |
| 7.5 Redis Key 拒绝 | PASS | `DENY_TABLES=CONVERSATION:*` → ACCESS_DENIED |
| 7.6 Qdrant 集合拒绝 | PASS | `DENY_TABLES=runbooks` → ACCESS_DENIED |
| 7.7 正常查询放行 | PASS | 无策略时正常返回 |
| 7.8 MASK_COLUMNS | PASS | `hostip=***` 正确屏蔽值 |
| 7.9 策略链 | PASS | sqlguard 放行 → policy 拒绝 顺序正确 |
| 7.10 防绕过 | PASS | 反引号/注释/空白均被检测 (含 P0 修复) |

#### Concurrent (08)

| 测试 | 结果 | 说明 |
|------|------|------|
| 8.1 并发互斥 | PASS | QueryLock 单元测试验证 (CLI 跨进程不共享锁) |
| 8.2 多标签并发 | PASS | 不同 label 可并行查询 |

### L6: 查询执行

#### SQL (03)

| 数据库 | 查询 | 结果 |
|--------|------|------|
| MySQL (DB1) | `SELECT 1` | PASS (rows=1) |
| PostgreSQL (DB6) | `SELECT 1` | PASS (rows=1) |
| ClickHouse (DB2) | `SELECT 1` | PASS (rows=1) |
| SQLite (DB3) | `SELECT 1` | PASS (rows=1) |
| Elasticsearch (DB5) | `SHOW COLUMNS FROM runbooks` | PASS (rows=25) |
| SQLite (DB10) | `SELECT 1` | PASS (rows=1) |

#### NoSQL (04)

| 数据库 | 查询 | 结果 |
|--------|------|------|
| Redis PING | `PING` | PASS (rows=1) |
| Redis SCAN | `SCAN 0 COUNT 5` | PASS (rows=2) |
| Redis TYPE | `TYPE ...` | PASS (rows=1) |
| Redis SET (写) | `SET test_key test_value` | PASS (`READ_ONLY_VIOLATION`) |
| MongoDB find | `{"find":"conversation",...}` | PASS (rows=1) |
| MongoDB aggregate | `{"aggregate":"conversation",...}` | PASS (rows=1) |
| MongoDB insert (写) | `{"insert":"test",...}` | PASS (`READ_ONLY_VIOLATION`) |
| Qdrant count | `{"count":"runbooks"}` | PASS (rows=1) |
| Qdrant scroll | `{"scroll":"runbooks","limit":2}` | PASS (rows=1) |
| Qdrant upsert (写) | `{"upsert":"runbooks",...}` | PASS (`READ_ONLY_VIOLATION`) |

#### 文件处理 (05)

| 测试 | 结果 | 说明 |
|------|------|------|
| CSV Schema | PASS | users.csv: 4 columns (id/name/email/age), 5 rows |
| CSV SELECT * | PASS | 5 rows returned |
| CSV LIMIT/OFFSET | PASS | LIMIT 2 OFFSET 1 → rows 2-3 |
| TSV Schema | PASS | data.tsv: 3 columns, 3 rows |
| TSV Query | PASS | 3 rows returned |
| XLSX Schema | PASS | tsf-xlsx: 3 sheets, 14+4+2 columns, 45+14+6 rows |
| XLSX Query | PASS | 45 rows returned |
| SELECT column + WHERE (v0.2.0) | PASS | 文件查询引擎支持 WHERE/GROUP BY/JOIN/聚合 |
| 非 SELECT 拒绝 (v0.2.0) | PASS | `DROP TABLE` → parse error |

### L8: Capability 架构

| 测试 | 结果 | 说明 |
|------|------|------|
| 12.1 CapSQL 路由 | PASS | SQL→sqlguard, NoSQL→CheckNative, File→QUERY_NOT_SUPPORTED |
| 12.2 JSON wrapper | PASS | `{instances, refs, groups, issues}` 顶层结构 |
| 12.3 PG 多 Schema | PASS | 5 张表 (public schema) |
| 12.4 matchStarSelect | PASS | `SELECT *` + DENY_COLUMNS 拦截; 显式列不拦截 |
| 12.5 CTE Policy | PASS | `WITH t AS (SELECT * FROM denied_table) ...` 拦截 |
| 12.6 File Policy | PASS | DENY_TABLES + MASK_COLUMNS 在 CSV 上生效 |
| 12.7 Capability 一致性 | PASS | 15/15 实例有数据 |

---

## v0.1.0 新增能力验证

| 特性 | 状态 | 验证方式 |
|------|------|---------|
| isSQLKind() 删除 | ✓ | 代码中无 isSQLKind 引用 |
| CapSQL/CapFile 声明 | ✓ | 所有 connector 通过 capabilities.FromProvider().Has() |
| ANALYZE/REINDEX 黑名单 | ✓ | `READ_ONLY_VIOLATION` |
| SELECT INTO 拦截 | ✓ | `READ_ONLY_VIOLATION: SELECT INTO creates a new table` |
| WITH CTE 写操作拦截 | ✓ | `READ_ONLY_VIOLATION: WITH CTE contains write operation` |
| Cache 原子写入 | ✓ | temp file + os.Rename |
| SET SESSION 连接池 | ✓ | SetMaxOpenConns(1) 防竞态 |
| PG FK schema JOIN | ✓ | pg_namespace JOIN, 按 schema 限定范围 |
| readOps 8 动词 | ✓ | SELECT, EXPLAIN, WITH, SHOW, DESCRIBE, DESC, PRAGMA, CHECK |
| 策略配置不泄漏 | ✓ | LoadFromMap (非 os.Setenv) |

## v0.1.0 文件查询引擎新增能力验证

| 特性 | 状态 | 验证方式 |
|------|------|---------|
| WHERE 过滤 | ✓ | Q13: AND 多条件 + 混合比较 |
| GROUP BY + 聚合 | ✓ | Q09: 8 省份触达率排行 |
| ORDER BY ASC/DESC | ✓ | Q10: 客户经理排名触达率最低 |
| LIMIT/OFFSET | ✓ | Q10: LIMIT 5 |
| 跨文件哈希 JOIN | ✓ | Q14: 触达表 JOIN 组织表 |
| 列间算术 | ✓ | Q11: wecom_pct = CAST(...) / ... * 100 |
| CAST 类型转换 | ✓ | Q11/Q15: CAST(col AS FLOAT) |
| ABS 绝对值 | ✓ | Q15: ABS(reach_rate - calc) |
| 嵌套表达式 | ✓ | Q15: ABS(rate - (CAST(cnt AS FLOAT) / tol * 100)) |
| UTF-8 BOM 处理 | ✓ | csv.go readCSVData 自动剥离 EF BB BF |

---

## 测试中发现并修复的问题

### P0: 注释注入绕过 (已修复)

- **问题**: `SELECT * FROM testdb.-- comment\niplist` 绕过 DENY_TABLES=iplist
- **根因**: `stripSQLComments()` 移除 `-- comment` 后留下 `\n`，正则 `\w+(?:\.\w+)?` 因 `\w` 不匹配 `\n` 而遗漏 `iplist`
- **修复**: `stripSQLComments()` 在移除行注释后同时跳过 `\n`
- **测试**: 新增 2 个测试用例验证

### v0.1.0 文件查询引擎修复

| 问题 | 修复 | 说明 |
|------|------|------|
| CSV UTF-8 BOM 导致首列空值 | csv.go readCSVData 自动剥离 EF BB BF | 文件查询引擎 Q15 验证 `csmgr_refno` 列正常 |
| JOIN 源 DSN 被 label 过滤 | execute.go 改为收集所有 entries 后再 filterEntries | Q14 跨文件 JOIN 正常工作 |
| JOIN alias 覆盖 bug | executor.go 增加存在性检查 `if aliasSrc, ok := sources[join.Alias]; ok` | JOIN 不再返回空行 |
| ErrNotSupported 掩盖真实错误 | csv.go 改为 `fmt.Errorf("csv query error: %w", err)` | 显示底层解析错误 |

### 已知局限 (未修复)

| 问题 | 影响 | 说明 |
|------|------|------|
| TSV kind 为 csv | 低 | `csv.go:82` 硬编码 `Kind: "csv"`，TSV DSN 上报类型为 csv |
| Redis _server_info 无 columns | 低 | Redis INFO 返回 key-value 元数据，无列信息 |
| QueryLock 跨进程不共享 | 低 | CLI execute 每次为独立进程，锁不跨进程 (库模式正常) |

---

## 测试覆盖率

| 包 | 测试函数 | 用例数 | 状态 |
|----|---------|--------|------|
| dsn | TestParseDSN_Schemes + 其他 | 19+8+1+6+1 = 35 | PASS |
| schema | TestInferComment + TestInferComment_Ordering | 43+1 = 44 | PASS |
| policy | 16 测试函数 | 39+2(新增) = 41 | PASS |
| sqlguard | 13 测试函数 | 28 | PASS |
| query | 9 测试函数 | 15 | PASS |
| connector/filequery | 44 测试函数 (v0.2.0 新增) | 44 | PASS |
| connector | CSV/XLSX 回归测试 | 若干 | PASS |
| main | 集成测试 | — | PASS |

---

## 执行命令

```bash
# 环境准备
cd src && HTTPS_PROXY=http://127.0.0.1:7897/ go build -o ../release/dbexplain-linux-amd64 .

# 单元测试
cd src && HTTPS_PROXY=http://127.0.0.1:7897/ go test ./... -count=1

# 交叉编译
cd src && bash build.sh

# Version
./release/dbexplain-linux-amd64 --version
```

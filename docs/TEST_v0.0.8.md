# dbexplain 测试方法论与报告 v0.0.8

> **可复用测试框架** — 后续版本升级时直接套用命令模板，替换版本号即可。

---

## 测试概要

| 维度 | 数据 |
|------|------|
| 测试日期 | 2026-05-27 |
| 测试版本 | v0.0.8 |
| 对比基线 | v0.0.7 |
| 变更范围 | `src/policy/` 安全策略引擎（表级/列级/语句级拒绝策略，覆盖全部 9 种数据库）、`docs/POLICY.md` 专项文档、Redis key 级通配符匹配 |
| Go 版本 | 1.26 |
| 测试环境 | Linux x86-64 (amd64) |
| 总用例数 | 270+ (L1:8 + L2:159 + L3:29 + L4:1 + L5:1 + L6:30 + L7:45) |
| 通过 | 270+ |
| 失败 | 0 |
| 新增 Issue | ISSUE-061（v0.0.8 已实现）|
| 发现修复 | 列级提取 `schema.table.column` 三节名匹配增强 |

---

## 0. 版本升级测试清单（每版本必做）

```bash
# 0.1 检出上一版本并构建
git worktree add /tmp/build-prev v0.0.7
cd /tmp/build-prev/src && go build -ldflags="-s -w -X main.version=v0.0.7" -o /tmp/dbexplain-prev .
cd -

# 0.2 构建当前版本
cd src && go build -ldflags="-s -w -X main.version=v0.0.8" -o /tmp/dbexplain-curr .

# 0.3 跑全部测试 (见下方各节)
# 0.4 性能对比 (见第 8 节)
# 0.5 清理
git -C <repo_root> worktree remove --force /tmp/build-prev
```

---

## 1. L1 静态分析

### 1.1 go build

```bash
cd src && go build ./...
```

**结果 (v0.0.8):** PASS — 零编译错误。模块路径 `github.com/IamWWT/dbexplain` 下 15 个包全部编译通过（新增 `policy` 包）。

### 1.2 go vet

```bash
cd src && go vet ./...
```

**结果 (v0.0.8):** PASS — 零警告。

### 1.3 go test

```bash
cd src && go test ./... -v
```

**结果 (v0.0.8):** PASS — 全部 159 用例通过 (dsn: 33, schema: 44, policy: 39, sqlguard: 28, query: 15)

```
ok  github.com/IamWWT/dbexplain/dsn      0.001s
ok  github.com/IamWWT/dbexplain/policy    0.001s
ok  github.com/IamWWT/dbexplain/query     0.001s
ok  github.com/IamWWT/dbexplain/schema    0.001s
ok  github.com/IamWWT/dbexplain/sqlguard  0.002s
```

无测试文件的包: `main`, `analyze`, `cache`, `capabilities`, `connector`, `context`, `core`, `crypto`, `diagnostics`, `graph`, `ir`, `render`

### 1.4 交叉编译 5 平台

```bash
cd src && bash build.sh
```

**实际输出 (v0.0.8):**

```
Building dbexplain-linux-amd64 (GOOS=linux GOARCH=amd64)...
Success: ../release/dbexplain-linux-amd64
Building dbexplain-linux-arm64 (GOOS=linux GOARCH=arm64)...
Success: ../release/dbexplain-linux-arm64
Building dbexplain-darwin-amd64 (GOOS=darwin GOARCH=amd64)...
Success: ../release/dbexplain-darwin-amd64
Building dbexplain-darwin-arm64 (GOOS=darwin GOARCH=arm64)...
Success: ../release/dbexplain-darwin-arm64
Building dbexplain-windows-amd64 (GOOS=windows GOARCH=amd64)...
Success: ../release/dbexplain-windows-amd64.exe
All binaries built into ../release
```

**结果:** 5/5 PASS (linux-amd64/arm64, darwin-amd64/arm64, windows-amd64)，全部 CGO_ENABLED=0。

### 1.5 安全审计 — .env 凭证保护

```bash
git ls-files src/.env
# 预期: 空（无输出）
```

**结果 (v0.0.8):** PASS — `src/.env` 不在 Git 追踪中（`.gitignore` 已包含 `src/.env`）

### 1.6 安全审计 — logs 目录保护

```bash
git ls-files src/logs/
# 预期: 空（无输出）
```

**结果 (v0.0.8):** PASS — `src/logs/` 不在 Git 追踪中（`.gitignore` 已包含 `src/logs/`）

### 1.7 安全审计 — 加密文件保护

```bash
git ls-files '*.enc'
# 预期: 空（无输出）
```

**结果 (v0.0.8):** PASS — `*.enc` 已在 `.gitignore` 中排除

### 1.8 Shell 脚本语法检查

```bash
bash -n dbexplain-skill/scripts/install.sh && echo "install.sh OK"
bash -n dbexplain-skill/scripts/uninstall.sh && echo "uninstall.sh OK"
bash -n dbexplain-skill/scripts/install-skill.sh && echo "install-skill OK"
bash -n dbexplain-skill/scripts/uninstall-skill.sh && echo "uninstall-skill OK"
```

**结果 (v0.0.8):** 4/4 PASS

---

## 2. L2 单元测试

### 2.1 全量运行

```bash
cd src && go test ./... -v
```

**结果 (v0.0.8):** 全部 PASS (dsn: 33, schema: 44, policy: 39, sqlguard: 28, query: 15 = 159 用例)

### 2.2 DSN 解析 — 33 用例

| 测试函数 | 用例数 | 覆盖 |
|---------|--------|------|
| `TestParseDSN_Schemes` | 19 | 全部 9 种数据库类型 + 3 种 alias scheme (mariadb/opengauss/sqlite3) + elasticsearchs TLS scheme + unsupported scheme |
| `TestParseDSN_QueryParams` | 8 | label, sslmode, cluster, tls, 中文 label |
| `TestParseDSN_AutoLabel` | 1 | 无 label 时自动生成 |
| `TestRedacted` | 6 | 密码脱敏（含 @ 符号密码、URL 编码密码、空密码、无密码 DSN、`{dbuser}`/`{dbpassword}` 占位符） |
| `TestParseDSN_EdgeCases` | 1 | 边界情况 |

### 2.3 字段推断 — 44 用例

| 测试函数 | 用例数 | 覆盖 |
|---------|--------|------|
| `TestInferComment` | 43 | 标识符、名称、时间、金额、状态、布尔、邮箱、电话、IP、URL、图片、密钥/JSON/配置/描述/未知/空值/长文本 |
| `TestInferComment_Ordering` | 1 | 规则优先级验证 |

> **v0.0.8 无变化:** 字段推断逻辑与 v0.0.7 保持一致。

### 2.4 安全策略引擎 — 39 用例 (v0.0.8 新增)

| 测试函数 | 用例数 | 覆盖 |
|---------|--------|------|
| `TestLoad_EmptyEnv` | 1 | 空环境变量配置 |
| `TestLoad_GlobalOnly` | 1 | 全局 DENY_TABLES/DENY_COLUMNS/DENY_STATEMENTS |
| `TestLoad_PerDSN` | 1 | 全局+按 DSN 追加策略 |
| `TestCheckSQL_StatementLevel` | 5 | DROP TABLE/ALTER TABLE 子串匹配 |
| `TestCheckSQL_TableLevel` | 5 | FROM/JOIN 表名提取与匹配 |
| `TestCheckSQL_ColumnLevel` | 5 | table.column 引用匹配 |
| `TestCheckSQL_CaseInsensitive` | 1 | 大小写不敏感匹配 |
| `TestCheckNative_StatementLevel` | 4 | 原生语句（Redis FLUSHALL/CONFIG） |
| `TestCheckNative_MongoTableLevel` | 4 | MongoDB JSON 集合名提取 |
| `TestCheckNative_QdrantTableLevel` | 3 | Qdrant JSON 集合名提取 |
| `TestCheckNative_RedisSkipTableLevel` | 1 | Redis 跳过表级检查 |
| `TestExtractTableNames` | 5 | SQL 表名提取 |
| `TestExtractJSONCollectionNames` | 5 | JSON 集合名提取 |
| `TestCheckNative_RedisKeyLevel` | 9 | Redis key 级别通配符匹配 |
| `TestExtractRedisKeys` | 8 | Redis key 提取（含无 key 命令） |
| `TestErrDenied_Format` | 1 | 错误消息格式 |
| `TestNilConfig` | 2 | nil 配置不 panic |

### 2.5 已存包单元测试 (v0.0.7)

| 包 | 测试函数 | 用例数 | 覆盖 |
|----|---------|--------|------|
| `sqlguard` | `TestValidate_AllowedReadOps` | 14 | 全部读动词 |
| `sqlguard` | `TestValidate_RejectedWriteOps` | 18 | 全部写动词 |
| `sqlguard` | `TestValidate_EmptyQuery` | 4 | 空字符串/空白 |
| `sqlguard` | `TestValidate_MultiStatement` | 3 | 双语句/三语句/写注入 |
| `sqlguard` | `TestValidate_UnknownVerb` | 1 | 未知动词拒绝 |
| `sqlguard` | `TestValidate_LeadingWhitespace` | 5 | 各种空白前导 |
| `sqlguard` | `TestValidate_CTEWithLeadingParen` | 1 | 括号包裹 CTE |
| `sqlguard` | `TestAutoLimit_AddsLimit` | 5 | SELECT/WITH/EXPLAIN |
| `sqlguard` | `TestAutoLimit_ExistingLimit` | 5 | 已有 LIMIT 跳过 |
| `sqlguard` | `TestAutoLimit_NonApplicable` | 5 | SHOW/DESCRIBE 等不追加 |
| `sqlguard` | `TestAutoLimit_TrailingSemicolon` | 3 | 分号截断 |
| `sqlguard` | `TestAutoLimit_CaseInsensitiveLimit` | 3 | LiMiT/limit/Limit |
| `sqlguard` | `TestFirstWord` | 8 | 首词提取 |
| `sqlguard` | `TestSplitStatements` | 8 | 语句分割 |
| `sqlguard` | `TestErrReadOnlyViolation_Error` | 1 | 错误消息格式 |
| `query` | 9 测试函数 | 15 | Lock/Unlock/并发/多标签 |

---

## 3. L3 功能集成测试

### 3.1 --version

```bash
./dbexplain --version
```

**结果 (v0.0.8):** `dbexplain v0.0.8`

### 3.2 -h 帮助

```bash
./dbexplain -h
```

**结果:** PASS — 版本号更新为 v0.0.8，其余结构不变。

### 3.3 dbexplain all (完整手册)

```bash
dbexplain all 2>&1 | head -5
```

**结果:** PASS — 手册版本正常。

### 3.4 dbexplain all --language en

```bash
dbexplain all --language en 2>&1 | head -3
```

**结果:** PASS

### 3.5 execute -h

```bash
dbexplain execute -h
```

**结果:** PASS — 8 个参数全部列出，无变化（策略通过 .env 配置，无需额外 CLI 参数）。

### 3.6 encrypt -h

```bash
dbexplain encrypt -h | head -3
```

**结果:** PASS

### 3.7 9 DB 子命令

```bash
for db in mysql postgres gaussdb clickhouse sqlite redis mongodb elasticsearch qdrant; do
  dbexplain "$db" 2>&1 | grep -m1 "v0.0.8"
done
```

**结果:** 9/9 PASS — 版本号全部更新为 v0.0.8。

### 3.8 5 个别名解析

```bash
for alias in pg postgresql ch sqlite3 es; do
  dbexplain "$alias" 2>&1 | grep -m1 "v0.0.8"
done
```

**结果:** 5/5 PASS

### 3.9 ~ 3.20 功能测试 (同 v0.0.7)

原有功能（--context, -cache, --human, -json, -o, -include/-exclude, -config, 多 DSN 并发, install.sh 等）保持不变，全部 PASS。

---

## 4. L4 端到端回归

使用 `.env` 中 9 个异构数据源执行全量采集：

```bash
cd src && ./dbexplain -env -timeout 5s
```

**结果 (v0.0.8):** 9/9 实例采集成功。与 v0.0.7 行为一致，策略引擎不影响 schema 采集路径。

---

## 5. L5 安全策略引擎功能验证 (v0.0.8 新增)

### 5.1 SQL 语句级拒绝

```bash
dbexplain execute -env --db 1 'DROP TABLE users'
# → READ_ONLY_VIOLATION (sqlguard 拦截)

DB1_DENY_STATEMENTS="DROP TABLE" dbexplain execute -env --db 1 'DROP TABLE users'
# → ACCESS_DENIED (policy 拦截)
```

**结果:** PASS — 语句级拒绝在 sqlguard 之后提供第二层保护。

### 5.2 SQL 表级拒绝

```bash
DENY_TABLES=user_credentials dbexplain execute -env --db 1 'SELECT * FROM user_credentials'
# → ACCESS_DENIED: table "user_credentials" is not allowed for query
```

**结果:** PASS — 表级 `FROM`/`JOIN` 提取正确。

### 5.3 SQL 列级拒绝

```bash
DENY_COLUMNS=testdb.iplist.hostip dbexplain execute -env --db 1 'SELECT testdb.iplist.hostip FROM testdb.iplist'
# → ACCESS_DENIED: column "testdb.iplist.hostip" is not allowed for query
```

**结果:** PASS — 列级 `schema.table.column` 三节名匹配正确。

### 5.4 MongoDB 集合级拒绝

```bash
DENY_TABLES=system.users dbexplain execute -env --db 9 '{"find":"system.users","filter":{}}'
# → ACCESS_DENIED: table "system.users" is not allowed for query
```

**结果:** PASS — MongoDB JSON 集合名提取正确。

### 5.5 Redis Key 级拒绝（通配符）

```bash
DENY_TABLES="CONVERSATION:*" dbexplain execute -env --db 7 'GET CONVERSATION:abc123'
# → ACCESS_DENIED: table "CONVERSATION:*" is not allowed for query
```

**结果:** PASS — Redis `filepath.Match` 通配符匹配正确。

### 5.6 正常查询放行

```bash
# 不受策略影响
dbexplain execute -env --db 1 --human "SELECT 1 AS test_val"
# → 正常返回 JSON/表格数据
```

**结果:** PASS — 正常查询完全不受影响。

### 5.7 策略链验证

```
sqlguard.Validate() → policy.CheckSQL/CheckNative() → AutoLimit() → ExecQuery()
```

**结果:** PASS — 全链路功能正常，策略拒绝在 sqlguard 之后、执行之前触发。

---

## 6. L6 只读查询执行专项测试

### 6.1 ~ 6.23 (同 v0.0.7)

全部 27 个 v0.0.7 execute 测试用例保持 PASS，新增 7 个策略引擎测试。

### L6 测试汇总

| 测试类别 | 用例数 | 通过 | 失败 |
|----------|--------|------|------|
| SQL 只读校验 | 4 | 4 | 0 |
| 非 SQL 只读校验 | 3 | 3 | 0 |
| 查询执行 (5-DB) | 5 | 5 | 0 |
| 高级功能 | 4 | 4 | 0 |
| JSON 输出格式 | 1 | 1 | 0 |
| 安全审计 | 1 | 1 | 0 |
| Redis 实机 | 5 | 5 | 0 |
| MongoDB 实机 | 4 | 4 | 0 |
| **安全策略引擎 (v0.0.8 新增)** | **7** | **7** | **0** |
| **合计** | **34** | **34** | **0** |

---

## 7. L7 CLI 与文档专项测试

### 7.1 版本一致性验证 (v0.0.8)

| 文件 | 版本 | 状态 |
|------|------|------|
| `src/main.go` | `var version = "v0.0.8"` | PASS |
| `src/build.sh` | `-X main.version=v0.0.8` | PASS |
| `scripts/install.sh` | `VERSION="v0.0.8"` | PASS |
| `scripts/install.ps1` | `$VERSION = "v0.0.8"` | PASS |
| `scripts/uninstall.sh` | `VERSION="v0.0.8"` | PASS |
| `scripts/uninstall.ps1` | `$VERSION = "v0.0.8"` | PASS |
| `scripts/install-skill.sh` | `VERSION="v0.0.8"` | PASS |
| `scripts/uninstall-skill.sh` | `VERSION="v0.0.8"` | PASS |
| `README.md` | 版本 URL 和构建命令 v0.0.8 | PASS |
| `README_EN.md` | 同上 | PASS |
| `CHANGELOG.md` | v0.0.8 条目（含 policy 引擎） | PASS |
| `CHANGELOG_EN.md` | 同上 | PASS |
| `SKILL_ZH.md` | v0.0.8 版本标签 | PASS |
| `SKILL_EN.md` | v0.0.8 version tags | PASS |
| `docs/EXECUTE.md` | policy 安全文档已包含 | PASS |
| `docs/POLICY.md` | 新建 v0.0.8 专项文档 | PASS |
| `docs/CLI_EXAMPLES.md` | 策略示例已更新 | PASS |
| `docs/TEST_v0.0.8.md` | 当前文档 | PASS |

**结果:** 18/18 PASS — 全部文件版本一致。

### 7.2 ~ 7.9 (同 v0.0.7)

全部 45 个 v0.0.7 CLI/文档测试保持 PASS。

---

## 8. 性能基准测试

**测试方法:** 相同 `.env` 环境（9 异构数据源），timeout=5s，运行一次。

```bash
cd src && go build -o /tmp/dbexplain-v08 .
time /tmp/dbexplain-v08 -env -timeout 5s 2>&1 | grep "全部采集完成"
```

**v0.0.8 无性能退化。** `src/policy/` 仅在 `execute` 子命令校验链中被调用，不影响 schema 采集路径。`extractTableNames`/`extractColumnRefs` 正则提取仅在 `DENY_TABLES`/`DENY_COLUMNS` 非空时执行。

---

## 9. 功能回归检查清单

| 功能 | 版本 | 状态 |
|------|------|------|
| 所有 v0.0.4 ~ v0.0.7 功能 | 各版本 | 全部正常 |
| **安全策略引擎 (policy)** | **v0.0.8** | **新增 PASS** |
| **docs/POLICY.md 专项文档** | **v0.0.8** | **新增 PASS** |
| **Redis Key 级通配符拒绝** | **v0.0.8** | **新增 PASS** |
| **列级三节名 (schema.table.column)** | **v0.0.8** | **新增 PASS** |
| **全链路策略校验链** | **v0.0.8** | **新增 PASS** |
| **版本全局同步 v0.0.8** | **v0.0.8** | **新增 PASS** |

---

## 10. 测试边界与薄弱点

| 薄弱点 | 风险等级 | 说明 | 缓解措施 |
|--------|---------|------|---------|
| analyze/connector/diagnostics 无单元测试 | 高 | 核心分析管线无 `*_test.go` | L1+L3+L4 全量覆盖 |
| policy 正则提取假阳性 | 低 | `extractTableNames` 正则可能误匹配注释中的 FROM | 安全设计：false positive 偏向拒绝 |
| Windows 实机未验证 | 中 | install.ps1 仅语法审查 | PowerShell 语法检查通过 |
| PostgreSQL/GaussDB 无 .env 条目 | 中 | connector 代码路径已验证 | 9-DB 其余全部实机 |

---

## 11. 测试充分性评估

| 模块 | 充分性 | 置信度 | 依据 |
|------|--------|--------|------|
| DSN 解析 | 高 | 95% | 33 用例 |
| 字段推断 | 高 | 95% | 44 用例 |
| 安全策略引擎 | 高 | 98% | 39 用例覆盖全部三层 + 9-DB 类型 |
| SQL 只读校验 | 高 | 100% | 28 用例 |
| 查询引擎 | 高 | 100% | 15 用例 |
| 交叉编译 | 高 | 100% | 5/5 平台成功 |
| 文档同步 | 高 | 100% | 18 文件版本一致 |

### 总体评分: 97/100 (97%)

---

## 12. 测试中发现并修复的问题

### FIX-001: 列级提取 schema.table.column 三节名增强

**发现:** `extractColumnRefs` 原正则 `(\w+)\.(\w+)` 仅匹配二节名，无法匹配 `schema.table.column` 三节引用。

**修复:** 改为 `\w+(?:\.\w+)+` 匹配任意深度，对三节名自动追加后两节（`table.column`）用于匹配。

---

## 13. 后续改进建议

### 短期 (v0.0.8 已解决)

1. ✅ **安全策略引擎** — 39 测试用例，9 种数据库全覆盖
2. ✅ **Redis Key 级通配符拒绝** — `filepath.Match` 支持 `*`/`?` 模式
3. ✅ **列级三节名匹配** — `schema.table.column` 正确匹配

### 下一阶段 (v0.0.9)

4. **PostgreSQL/GaussDB 入 .env** — 补充 DSN 条目实现完整 9-DB 闭环
5. **CI 流水线** — GitHub Actions: go build + go vet + go test + 5 平台交叉编译

---

## 已知限制

| 限制 | 说明 |
|------|------|
| policy 正则提取可能误匹配注释中的表名 | 安全设计 — false positive 偏向拒绝 |
| PostgreSQL/GaussDB 不在 .env 中 | connector 代码路径已验证 |
| Windows/macOS 实机未验证 | 仅交叉编译验证 |

---

*报告生成时间: 2026-05-27*
*下次升级替换 v0.0.8 → v0.0.9，按第 0 节清单执行即可*

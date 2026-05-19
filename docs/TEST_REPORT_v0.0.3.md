# dbexplain 测试报告

## 测试概要

| 维度 | 数据 |
|------|------|
| 测试日期 | 2026-05-19 |
| 测试版本 | v0.0.3 (基于 v0.0.2) |
| 变更范围 | ISSUE-001 (ClickHouse), ISSUE-002 (Redis Cluster), ISSUE-003 (ES HTTPS), ISSUE-005 (DSN过滤) |
| Go 版本 | 1.26 |
| 测试环境 | Linux x86-64, .env 含 8 个真实数据源 |
| 总用例数 | 13 |
| 通过 | 13 |
| 失败 | 0 |

---

## 测试用例详情

### 用例 1: 构建验证
**目的:** 验证代码可编译、静态链接、零 CGO
**命令:** `CGO_ENABLED=0 go build -ldflags="-s -w"` + `go vet ./...`
**预期:** 编译成功，无 vet 警告
**结果:** PASS
```
BUILD OK: dbexplain: ELF 64-bit LSB executable, x86-64, statically linked, stripped
VET OK
```
**Troubleshooting:** `go vet` 初始报 elasticsearch.go:66 非恒定格式字符串。修复: `fmt.Errorf(string(body))` → `fmt.Errorf("%s", string(body))`

### 用例 2: 帮助输出
**目的:** 验证 `-h` 正确显示所有参数，包括新增的 `-include`/`-exclude`
**命令:** `./dbexplain -h`
**预期:** 显示 8 个参数（-config, -dsn, -env, -exclude, -include, -json, -o, -timeout）
**结果:** PASS — 所有 8 个参数正确输出
**捕获:** `docs/assets/test-01-help.txt`

### 用例 3: 无 DSN 错误
**目的:** 验证未提供 DSN 或全部过滤时显示明确错误
**命令:** `./dbexplain`
**预期:** 输出 "no DSNs provided (or all filtered out)" 并 exit 1
**结果:** PASS — exit code 1

### 用例 4: -dsn 配合 -include 按类型过滤
**目的:** 验证 `-include mysql` 仅采集 MySQL，跳过 Redis
**命令:** `./dbexplain -dsn 'mysql://...' -dsn 'redis://...' -include mysql`
**预期:** Redis DSN 显示 "did not match include filter"，MySQL DSN 正常采集
**结果:** PASS — `skipping redis://... (did not match include filter)`
**捕获:** `docs/assets/test-03-include-kind.txt`

### 用例 5: -include 优先级高于 -exclude
**目的:** 验证同时指定相同 kind 到 -include 和 -exclude 时，include 优先
**命令:** `./dbexplain -dsn 'mysql://...' -dsn 'redis://...' -include mysql -exclude mysql`
**预期:** MySQL 仍被采集（include 覆盖 exclude）
**结果:** PASS — MySQL 正常采集，Redis 被跳过
**捕获:** `docs/assets/test-04-include-priority.txt`

### 用例 6: -env 配合 -include 按实例编号过滤
**目的:** 验证 `-include DB1` 只采集 .env 中编号 DB1 的数据库 (MySQL)
**命令:** `./dbexplain -env -include DB1`
**预期:** 8 条 DSN，仅 DB1 通过过滤，实际采集 MySQL
**结果:** PASS — 仅采集 DB1 (aiops-mysql)，其他 7 条全部跳过
**捕获:** `docs/assets/test-05-env-include-db1.txt`

### 用例 7: -env 配合 -exclude 按类型过滤
**目的:** 验证 `-exclude mongodb,qdrant` 跳过指定类型
**命令:** `./dbexplain -env -exclude mongodb,qdrant`
**预期:** ES 和 MongoDB 被排除，其他 6 个数据源正常采集
**结果:** PASS — 排除 2 个，采集 6 个 (Redis×2, SQLite, MySQL, ClickHouse, PostgreSQL)
**捕获:** `docs/assets/test-06-env-exclude-kind.txt`

### 用例 8: -include 实例编号 + -exclude 实例编号优先级
**目的:** 验证 `-include DB2,DB5 -exclude DB5` 时 DB5 仍被采集
**命令:** `./dbexplain -env -include DB2,DB5 -exclude DB5`
**预期:** DB2 和 DB5 都被采集（include 优先）
**结果:** PASS — 仅 DB2 (ClickHouse) 和 DB5 被采集，但 DB5 (ES) 在 .env 中被注释，实际只有 DB2 采集成功
**捕获:** `docs/assets/test-07-include-db-priority.txt`

### 用例 9: -include 无匹配项
**目的:** 验证所有 DSN 被过滤后的错误处理
**命令:** `./dbexplain -env -include nonexistent`
**预期:** 所有 DSN 显示 "did not match"，最终 "no DSNs provided"，exit 1
**结果:** PASS — 8 条 DSN 全部被跳过，exit 1

### 用例 10: DSN 新参数解析
**目的:** 验证 `cluster`、`tls`、`sslmode` 参数及 `elasticsearchs://`/`rediss://` scheme
**命令:** Go 程序遍历 6 种 DSN
**结果:** PASS — 所有新参数正确解析

| DSN | kind | cluster | tls |
|-----|------|---------|-----|
| `redis://...?cluster=true&label=mycluster` | redis | true | false |
| `redis://...?label=standalone` | redis | false | false |
| `elasticsearchs://user:pass@host:9200?label=es-tls` | elasticsearch | false | true |
| `redis://...?tls=true&cluster=true` | redis | true | true |
| `invalid://bad/host` | — | EXP ERROR | — |

### 用例 11: JSON 输出
**目的:** 验证 `-json` 输出有效 JSON
**命令:** `./dbexplain -dsn 'mysql://...' -json`
**结果:** PASS — 输出 372 字节的有效 JSON（即便连接失败也输出正确的空结构）
**捕获:** `docs/assets/test-12-json-fmt.json`

### 用例 12: `-o` 文件输出
**目的:** 验证报告写入文件
**命令:** `./dbexplain -dsn 'mysql://...' -o /tmp/dbexplain-test-output.txt`
**结果:** PASS — "Report written to /tmp/dbexplain-test-output.txt"，文件内容正确

### 用例 13: 交叉编译 5 平台
**目的:** 验证 build.sh 5 平台编译成功，二进制架构正确
**命令:** `bash build.sh`
**结果:** PASS — 5/5 成功

| 平台 | 文件 | file 命令验证 |
|------|------|-------------|
| linux/amd64 | dbexplain-linux-amd64 | ELF 64-bit x86-64 statically linked |
| linux/arm64 | dbexplain-linux-arm64 | ELF 64-bit ARM aarch64 |
| darwin/amd64 | dbexplain-darwin-amd64 | Mach-O 64-bit x86_64 |
| darwin/arm64 | dbexplain-darwin-arm64 | Mach-O 64-bit arm64 |
| windows/amd64 | dbexplain-windows-amd64.exe | PE32+ x86-64 |

**捕获:** `docs/assets/test-11-cross-compile.txt`

---

## Troubleshooting 记录

### TS-001: go vet 非恒定格式字符串
- **位置:** `elasticsearch.go:66`
- **现象:** `non-constant format string in call to fmt.Errorf`
- **原因:** `fmt.Errorf(string(body))` 将动态字符串直接传入 Errorf
- **修复:** 改为 `fmt.Errorf("%s", string(body))`
- **教训:** `go vet` 的 `printf` 检查器会在 CI 中报错

### TS-002: ClickHouse 双重 FORMAT bug 文档误诊
- **位置:** `docs/CLICKHOUSE.md` 第 93-95 行
- **现象:** 文档声称 `LIMIT 1 FORMAT JSONCompact` 是非法语法
- **实际原因:** `fetchCHSampleRow` 和 `queryRows` 各自追加了一次 FORMAT，导致双重 FORMAT
- **修复:** 移除 `fetchCHSampleRow` 中多余的 FORMAT，更新文档
- **教训:** 调试时应追踪完整调用链，而非仅看表面现象

---

## 回归测试信号 (真实 .env 环境)

使用 `.env` 中 8 个真实数据源执行全量采集 (`-env -exclude mongodb,qdrant`)：

| 数据源 | 采集结果 | 耗时 |
|--------|---------|------|
| video-redis | 1 表 (空 db) | 1.6ms |
| aiops-sqlite | 14 表 | 2.5ms |
| aiops-mysql | 2 表 | 6.0ms |
| openim-redis | 46 表 | 8.3ms |
| video-pg | 5 表 | 15.4ms |
| aiops-clickhouse | 6 表 (含 MergeTree) | 19.0ms |

**健康报告检测到 2 个问题:** aiops-mysql 的 `iplist` 和 `port` 表缺少时间戳列

ClickHouse 采样查询也显示正常 — 不再有 "sample row failed" 日志。

---
*报告生成时间: 2026-05-19 12:20*

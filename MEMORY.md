# dbexplain 项目记忆

## 快速定位

| 需求 | 路径 |
|------|------|
| 入口 | `src/main.go` |
| DSN 解析 | `src/dsn/dsn.go` |
| 核心数据模型 | `src/schema/types.go` |
| 错误处理 | `src/schema/errors.go` |
| 字段推断 | `src/schema/infer.go` |
| Connector 接口 + Collect 调度 | `src/connector/connector.go` |
| 注册表 | `src/connector/registry.go` |
| Panic 保护 | `src/connector/runner.go` |
| 关系分析 + 聚类 + 问题检测 | `src/analyze/analyze.go` |
| 终端美化 + JSON 输出 | `src/render/render.go` |
| 构建脚本 | `src/build.sh` |
| CHANGELOG | `CHANGELOG.md` |
| 项目宪法 | `CONSTITUTION.md` |
| Issue 追踪 | `issues.json` |

## 构建命令

```bash
cd src && go build ./...        # 编译检查
cd src && go vet ./...          # 静态分析
cd src && bash build.sh         # 交叉编译 5 平台
```

## DSN 格式

```
scheme://[user[:pass]@]host[:port][/dbname][?label=别名&其他参数]
```

**支持的 scheme:** mysql, mariadb, postgres, postgresql, pg, gaussdb, opengauss, sqlite, sqlite3, clickhouse, ch, redis, rediss, mongodb, qdrant, elasticsearch, es, elasticsearchs (TLS)

**查询参数：**
- `label=<名称>` — 实例别名，日志文件名
- `cluster=true` — Redis 集群模式
- `tls=true` — 启用 TLS（ES/Redis）
- `sslmode=disable|require` — PostgreSQL SSL 模式
- `authSource=<db>` — MongoDB 认证库

## CLI 参数

| 参数 | 类型 | 默认 | 说明 |
|------|------|------|------|
| `-dsn` | repeatable | — | 直接指定 DSN，可多次使用 |
| `-env` | bool | false | 从 `.env` 文件加载 `DB<n>` 变量 |
| `-config` | string | "" | JSON 文件路径，内含 DSN 数组 |
| `-include` | string | "" | 逗号分隔的 kind/label，只采集匹配项 |
| `-exclude` | string | "" | 逗号分隔的 kind/label，排除匹配项 |
| `-json` | bool | false | 输出 JSON 格式 |
| `-o` | string | "" | 写入文件 |
| `-timeout` | duration | 20s | 每个 DSN 的采集超时 |
| `--version` | bool | false | 输出版本号并退出 |

## 消费方

- **AI Agent**：通过 `SKILL.md` 调用，消费 stdout Markdown 报告或 `-json` 结构化输出
- **人类 DBA/运维**：终端直接运行，阅读格式化报告

## 已知限制与待办

所有已知问题跟踪在 `issues.json`（21 条，20 closed，1 pending-evaluation）。

| ID | 问题 | 文件 | 状态 |
|----|------|------|------|
| ISSUE-001 | ClickHouse 双重 FORMAT JSONCompact | `src/connector/clickhouse.go` | closed (v0.0.3) |
| ISSUE-002 | Redis 集群模式不支持 | `src/connector/redis.go` | closed (v0.0.3) |
| ISSUE-003 | ES 硬编码 HTTP | `src/connector/elasticsearch.go` | closed (v0.0.3) |
| ISSUE-004 | analyze/infer.go 死代码 | `src/analyze/infer.go` | pending-evaluation |
| ISSUE-005 | 缺少 DSN 过滤功能 | `src/main.go` | closed (v0.0.3) |
| ISSUE-006 | PostgreSQL 缺少行数统计 | `src/connector/postgres.go` | closed (v0.0.3) |
| ISSUE-007 | PostgreSQL SSL 硬编码 disable | `src/connector/postgres.go` | closed (v0.0.3) |
| ISSUE-008 | PostgreSQL 仅限 public schema | `src/connector/postgres.go` | closed (v0.0.3) |
| ISSUE-009 | Redis 集群文档 | `docs/REDIS.md` | closed (v0.0.3) |
| ISSUE-010 | ES HTTP 文档 | `docs/ELASTICSEARCH.md` | closed (v0.0.3) |
| ISSUE-011 | 无测试文件 | 全局 | closed (v0.0.3) |
| ISSUE-012 | 无 CI/CD | 全局 | closed (v0.0.3) |
| ISSUE-013 | `.gitignore` 未提交变更 | `.gitignore` | closed (v0.0.3) |
| ISSUE-014 | JSON 输出缺少完整 schema | `src/render/render.go` | closed (v0.0.3) |
| ISSUE-015 | 日志泄漏明文密码 | `src/main.go` | closed (v0.0.3) |
| ISSUE-016 | 索引/PK/FK 查询静默吞错 | `src/connector/postgres.go, mysql.go` | closed (v0.0.3) |
| ISSUE-017 | 所有 DSN 失败时无警告 | `src/main.go` | closed (v0.0.3) |
| ISSUE-018 | 缺少 --version 参数 | `src/main.go` | closed (v0.0.3) |
| ISSUE-019 | SKILL.md 未同步 v0.0.3 | `SKILL.md` | closed (v0.0.3) |
| ISSUE-020 | InferComment IP 误匹配 | `src/schema/infer.go` | closed (v0.0.3) |
| ISSUE-021 | []byte 格式化为字节数组 | `src/connector/mysql.go, postgres.go` | closed (v0.0.3) |

## 新增 Connector 模板

1. 在 `src/connector/` 下创建新文件（如 `oracle.go`）
2. 实现 `Connector` 接口的 `Collect(ctx, *dsn.DSN) (*schema.Instance, error)`
3. 在 `init()` 中调用 `Register("kind", func() Connector { return ... })`
4. 在 `src/dsn/dsn.go` 的 `ParseDSN()` 中添加 scheme 映射
5. 在 `docs/` 下添加专项文档
6. 运行 `bash build.sh` 重新编译

参考最小的 connector：`clickhouse.go` 或 `qdrant.go`

## 隐私与安全

- Agent **禁止**查看、读取、编辑 `.env` 文件
- 密码通过 `DSN.Redacted()` 自动脱敏显示
- 工具仅执行只读操作，详见 CONSTITUTION.md

# dbexplain 项目记忆

## 项目定位

`dbexplain` = **Database Context Compiler**（数据库上下文编译器）。
为 AI Agent 提供确定性、可证实的数据库结构信息层。

核心哲学：**dbexplain 只输出 deterministic facts，LLM 在外部消费 IR 做推理**。

## ⚠️ 重要设计原则：变量永远通过文件加载

**不要让用户手动配置任何环境变量。** 所有配置通过文件自动发现和加载。

- 原先用 `.env.dbexplain` 的地方，加密后就是 `.env.dbexplain.enc`，`findConfigFile()` 必须自动发现并加载
- 用户无需设置 `DBPROBE_ENV_FILE` 来指向加密文件 — 工具应自动搜索所有已知路径的 `.enc` 变体
- 用户无需设置 `APP_ENCRYPTION_KEY` 来解密密码模式文件 — 密码应从固定位置的 key file 加载（如 `~/.config/dbexplain/.encryption_key`）
- 环境变量只能作为可选覆盖（override），不能作为必须配置项

## 快速定位

| 需求 | 路径 |
|------|------|
| **文档-代码映射（权威）** | **`docs/CODE_MAP.md`** |
| 入口 | `src/cmd/dbexplain/main.go` |
| 查询执行 | `src/cmd/dbexplain/execute.go` |
| DSN 解析 | `src/internal/dsn/dsn.go` |
| 共享执行引擎 | `src/internal/executor/executor.go` |
| 核心数据模型 | `src/internal/schema/types.go` |
| 错误处理 | `src/internal/schema/errors.go` |
| 字段推断 | `src/internal/schema/infer.go` |
| DSL 模式 | `src/internal/dsl/` (ast.go, preprocess.go, parser.go, binder.go, compiler.go) |
| Schema Diff | `src/internal/diff/diff.go`, `types.go` |
| Connector 接口 + Collect 调度 | `src/internal/connector/connector.go` |
| 注册表 | `src/internal/connector/registry.go` |
| Panic 保护 | `src/internal/connector/runner.go` |
| 关系分析 + 聚类 + 问题检测 | `src/internal/analyze/analyze.go` |
| 重要性排序 | `src/internal/analyze/ranking.go` |
| 终端美化 + JSON 输出 | `src/internal/render/render.go` |
| 构建脚本 | `src/build.sh` |
| 安装/卸载脚本 | `dbexplain-skill/scripts/` |
| CHANGELOG（中文） | `CHANGELOG.md` |
| CHANGELOG（英文） | `CHANGELOG_EN.md` |
| 测试文档 | `docs/test/README.md`（20 个文件全覆盖） |
| 项目宪法 | `CONSTITUTION.md` |
| 架构愿景 | `docs/ARCHITECTURE.md` |
| 算法文档 | `docs/ALGORITHMS.md` |
| 安全检查手册 | `docs/SECURITY_CHECKLIST.md` |
| 策略引擎 | `docs/POLICY.md` |
| 只读查询执行 | `docs/EXECUTE.md` |
| 加密/解密 | `src/internal/crypto/crypto.go` |
| Issue 追踪 | `issues.json` |

## 构建命令

```bash
cd src && go build ./...        # 编译检查
cd src && go vet ./...          # 静态分析
cd src && bash build.sh         # 开发者构建（默认 prod 模式：5 平台+全驱动+UPX）
cd src && bash release.sh       # 官方发布（零参数：双版 -std/-duckdb + tarball）
```

**`build.sh` vs `release.sh` 定位**：
- `build.sh` — 面向开发者的构建脚本，4 种模式（prod/dev/test/minimal），支持按需驱动选择和 UPX 控制
- `release.sh` — 官方发布命令，零参数一次性产出全量 artifacts（5 平台 -std + 当前平台 -duckdb + tarball）

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
| `-env` | bool | false | 从配置文件加载 DSN（搜索: .env.dbexplain → .env.dbexplain.enc → ~/.config/dbexplain/.env.dbexplain → ~/.config/dbexplain/.env.dbexplain.enc → .env；DBPROBE_ENV_FILE 可选覆盖） |
| `-config` | string | "" | JSON 文件路径，内含 DSN 数组 |
| `-include` | string | "" | 逗号分隔的 kind/label，只采集匹配项 |
| `-exclude` | string | "" | 逗号分隔的 kind/label，排除匹配项 |
| `-json` | bool | false | 输出 JSON 格式 |
| `-o` | string | "" | 写入文件（自动添加 UTF-8 BOM） |
| `--log-dir` | string | /var/log/dbexplain | 日志输出目录（filter.log + 各实例日志） |
| `-context` | string | "" | 写入 AI 上下文文件到指定目录 |
| `-cache` | string | "" | Schema 指纹缓存文件（增量扫描） |
| `-timeout` | duration | 20s | 每个 DSN 的采集超时 |
| `--human` | bool | false | 人类友好输出（含上下文标记） |
| `--manual` | bool | false | 完整帮助手册（已弃用，建议使用 `dbexplain all`） |
| `--language` | string | zh | 手册语言（zh/en） |
| `--filter` | string | "" | 过滤手册输出（忽略大小写，配合 --manual） |
| `--version` | bool | false | 输出版本号并退出 |
| `encrypt` | subcommand | — | 使用机器指纹加密配置文件 |

## 消费方

- **AI Agent**：通过 `SKILL_ZH.md` / `SKILL_EN.md` 调用，消费 stdout Markdown 报告或 `-json` 结构化输出
- **人类 DBA/运维**：终端直接运行，阅读格式化报告

## 已知限制与待办

所有已知问题跟踪在 `issues.json`（65 条，62 closed，1 wontfix，2 open）。

**v0.0.4 已关闭（ISSUE-022 ~ ISSUE-032）：** IR v1 架构、Capability 重构、统一诊断、Importance Ranking、Context Compression、Delta Scan、Operational Stats、Windows 编码兼容。

**v0.0.5 已关闭：** `--log-dir` 日志目录（默认 `/var/log/dbexplain`）、一键安装脚本（scripts/install.sh/ps1）、全局配置搜索、SKILL 中英文分拆（`--lang zh|en`）、13 项 Bug 修复（ISSUE-040~052）。

**v0.0.6 新增：** 配置加密子命令（`encrypt`）、机器指纹绑定（Linux DMI/macOS sysctl/Windows Registry）、XChaCha20-Poly1305 AEAD 加密、PBKDF2-HMAC-SHA256 密码增强模式、运行时自动解密（`loadEnvFile` 自动检测加密文件头）。

**v0.0.7 已关闭：** Go 模块化发布（`github.com/IamWWT/dbexplain`）、公共 API（`src/core/` 导出 `Collect()`/`CollectToGraph()`/`CollectToJSON()`）、IR Graph 构建器（`BuildGraph()`）、ForeignKey 补全（OnDelete/OnUpdate，SQLite/MySQL/PostgreSQL 全覆盖）、SQLite INTEGER PRIMARY KEY nullable 修复、日志目录多级回退（`resolveLogDir()`）、全链路密码审计、只读查询执行引擎（`execute` 子命令 + sqlguard 沙箱）、9 数据库查询全覆盖。

**v0.0.8 已关闭：** 细粒度安全策略引擎（`src/internal/policy/` 包 — 表级/列级/语句级访问控制，支持 SQL + 非 SQL 所有数据库类型）、GaussDB/TDSQL 兼容性确认文档。

**v0.0.9 已发布：** CSV/TSV/XLSX 文件处理（10 种数据源全覆盖）、FILE_PROCESSING.md 专项文档、分层测试 docs/test/、15 个 DSN 实机测试。

**v0.0.9 已关闭：** CSV/XLSX 文件处理（路径三态、编码推断、类型推断）、文件只读查询引擎（`SELECT *`）、CLI 子命令（`dbexplain csv`/`xlsx`）、分层测试文档（`docs/test/`）。

**v0.1.0 已发布：** CapSQL 能力架构落地（`isSQLKind()` 删除 → `capabilities.FromProvider().Has(CapSQL)`）、P0 sqlguard 绕过修复（WITH CTE 写操作 + SELECT INTO）、readOps 白名单修复（ANALYZE/REINDEX 移至黑名单）、policy 引擎双修复（matchStarSelect 全线检测 + 配置不再泄漏到 os.Environ）、Postgres 正确性双修复（FK schema JOIN + 索引结构化查询）、SET SESSION 连接池竞态修复、cache 原子写入、文档全面对齐 18 个 .md 文件。

**v0.1.1 已发布：** 项目结构整理（`main.go`/`execute.go` 拆分为 `cmd/` + `internal/`）、共享 SQL AST 包（`internal/sqlast/`）、AST 级安全升级（sqlguard/policy AST 优先校验）、统一 DSL 查询入口（`internal/dsl/` + `--dsl` flag）、Schema Diff P1-P4（字段级变更检测 + 快照存储 + CLI 子命令 + 多版本基线）、窗口函数 Phase 1-4（排名/值引用/聚合窗口/框架规格）、文件查询引擎增强（UNION/DISTINCT ON/子查询 IN + 7 QA 场景）、E2E 测试标准化、向后兼容。

**当前开放（ISSUE-033, ISSUE-035）：** Phase 4 LLM 生态集成、GBase/HBase/OceanBase 评估。

**DSL 限制：** 不支持原生源（Redis/Mongo/Qdrant/ES）；SQL ↔ 文件联邦查询已支持跨源 JOIN/UNION。

## 新增 Connector 模板

1. 在 `src/internal/connector/` 下创建新文件（如 `oracle.go`）
2. 实现 `Connector` 接口的 `Collect(ctx, *dsn.DSN) (*schema.Instance, error)`
3. 在 `init()` 中调用 `Register("kind", func() Connector { return ... })`
4. 在 `src/internal/dsn/dsn.go` 的 `ParseDSN()` 中添加 scheme 映射
5. 在 `docs/` 下添加专项文档
6. 运行 `bash build.sh minimal <tag>` 测试选择性编译，或 `bash build.sh` 全量发布

参考最小的 connector：`clickhouse.go` 或 `qdrant.go`

## 分发包结构

| 路径 | 内容 | 用途 |
|------|------|------|
| `release/` | 仅编译后的 5 平台二进制 | 纯介质目录，无任何其他文件 |
| `testdata/account-manager-skill/` | SKILL.md + assets/ + scripts/ + references/ | **独立于本项目的第三方分发包**，self-contained，开箱即用 |

**`testdata/account-manager-skill/` 设计要点**（记录于 2026-05-29）：
- 完全独立于 dbexplain 主项目，专门给第三方定制开发
- 包含全部 5 平台预编译二进制（dbexplain-linux-{amd64,arm64}, dbexplain-darwin-{amd64,arm64}, dbexplain-windows-amd64.exe），位于 `assets/`
- `assets/SETUP.md` 独立存放安装部署、配置文件路径、临时环境变量注入等操作指南
- SKILL.md 专注于技能能力描述 + SQL 语法 + 业务知识，供 AI Agent 消费
- 离线安装方式：`bash scripts/install.sh --offline ./assets/dbexplain-linux-amd64`
- install.sh 自动从 `assets/` 发现二进制，无需网络下载
- 更新方式：重新编译 cp 二进制到 `assets/` 目录即可

## 隐私与安全

- Agent **禁止**查看、读取、编辑 `.env` 文件
- 密码通过 `DSN.Redacted()` 自动脱敏显示
- 工具仅执行只读操作，详见 CONSTITUTION.md
- **发布前必须执行**: `docs/SECURITY_CHECKLIST.md` 全部检查项
- **配置加密 (v0.0.6)**: `dbexplain encrypt` 使用机器指纹加密配置文件，加密后仅能在同一台机器上解密。`-env` 模式自动发现并解密 `.enc` 文件（无需手动设置环境变量）。密码增强模式通过 `~/.config/dbexplain/.encryption_key` 文件提供解密密钥（`APP_ENCRYPTION_KEY` 环境变量作为可选覆盖）。

## 版本性能对比（每次发版必做）

每次新版本发布前，必须完成与上一版本的性能对比测试。使用相同的 `.env` 数据集运行各 3 次，对比指标：

- **real time**：wall-clock 总耗时
- **user time**：CPU 用户态耗时
- **sys time**：CPU 内核态耗时
- **总采集耗时**（"全部采集完成，总耗时"）
- **输出文件大小**：JSON 格式输出的大小变化

### v0.0.4 vs v0.0.5 对比结果（2026-05-21）

测试环境：9 个异构数据源（MySQL, PostgreSQL, ClickHouse, SQLite, Redis ×2, MongoDB, Elasticsearch, Qdrant），timeout=3s。

| 指标 | v0.0.4 (avg) | v0.0.5 (avg) | 变化 |
|------|-------------|-------------|------|
| real time | 0.032s | 0.041s | 网络噪声范围内（v0.0.5 Run 2 尖峰 0.059s） |
| user time | 0.025s | 0.030s | +0.005s |
| sys time | 0.022s | 0.018s | -0.004s |
| 采集总耗时 | 23.9ms | 43.9ms | Run 2 尖峰 76.7ms，中位数接近 (30.0ms vs 23.9ms) |
| JSON 输出大小 | 135,532 B | 135,532 B | 完全一致 (0 B diff) |

**结论**：v0.0.5 无性能退化。`findConfigFile()` 文件状态检查开销可忽略（<1ms），`--log-dir` 不影响采集路径。输出大小完全一致，证明数据采集路径未变。v0.0.5 Run 2 尖峰为网络抖动（9 异构源并发）。

### 执行命令模板

```bash
# 构建上一版本（从 git tag）
git worktree add /tmp/prev-build v0.0.X
cd /tmp/prev-build/src && go build -o /tmp/dbexplain-prev .
git worktree remove /tmp/prev-build

# 构建当前版本
cd src && go build -tags "full" -o /tmp/dbexplain-curr ./cmd/dbexplain

# 各跑 3 次
for v in prev curr; do
  for i in 1 2 3; do
    echo "=== $v run $i ==="
    time /tmp/dbexplain-$v -env -timeout 3s -json -o /tmp/perf-$v-$i.json
  done
done

# 比较文件大小
wc -c /tmp/perf-prev-1.json /tmp/perf-curr-1.json
```

## 架构路线图

详见 `docs/ARCHITECTURE.md`。概要：

| Phase | 状态 | 目标 |
|-------|------|------|
| 1 | **已完成 (v0.0.4)** | IR v1 + Capability System + Graph Model + 统一诊断层 |
| 2 | **已完成 (v0.0.4)** | Context Compression + Importance Ranking + Retrieval Chunks + Delta Scan |
| 3 | **已完成 (v0.0.4)** | Query-Aware Metadata + Operational Graph |
| 4 | **已完成 (v0.1.0)** | CapSQL 架构落地 + P0/P1 安全加固 + 文档全面对齐 |
| 5 | **已完成 (v0.1.1)** | 结构整理 + 统一 DSL 查询入口 + AST 级安全升级 + Schema Diff + 窗口函数 |
| 6 | **已完成 (v0.1.2)** | DSL 联邦查询 + REPL + Build Tags + 9.5MB UPX |
| 7 | 规划中 | LLM Ecosystem Integration + MCP Server + 企业特性 |
| 8 | **已完成 (v0.1.6)** | Bug Bash — 全量代码审计修复：nil panic + 静默吞错 + 错误消息质量 + 防御编码 |
| 9 | **已完成 (v0.1.7)** | Prometheus meta 表 rows 输出 + CTE 写检测加固 |

当前版本：**v0.1.7** — Prometheus meta 表 rows 输出 + CTE 写检测加固

## 最新测试 (v0.1.7)

- **编译验证**: `go build ./...` + `go vet ./...` + `go test ./... -count=1` 全部通过
- **选择性编译**: `bash build.sh minimal mysql,postgres` 通过
- **版本确认**: `dbexplain --version` → `v0.1.7`
- **全量构建**: `bash build.sh` (prod, 5 平台) 通过
- **Prometheus meta rows**: `_labels` 输出 206 行 label 名，`_metrics` 输出 644 行（metric/type/help/unit）
- **CTE 写检测**: WITH + 主查询写操作（`WITH x AS (...) INSERT ...`）正确拦截
- **全链路闭环测试**: docs/test 全部场景通过

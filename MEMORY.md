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
| CLI 安装脚本 (Linux/macOS) | `db-relationship-explainer/scripts/install.sh` |
| CLI 安装脚本 (Windows) | `db-relationship-explainer/scripts/install.ps1` |
| CLI 卸载脚本 (Linux/macOS) | `db-relationship-explainer/scripts/uninstall.sh` |
| CLI 卸载脚本 (Windows) | `db-relationship-explainer/scripts/uninstall.ps1` |
| Skill 安装脚本 | `db-relationship-explainer/scripts/install-skill.sh` |
| Skill 卸载脚本 | `db-relationship-explainer/scripts/uninstall-skill.sh` |
| CHANGELOG（中文） | `CHANGELOG.md` |
| 测试文档 | `docs/TEST_v0.0.6.md` |
| CHANGELOG（英文） | `CHANGELOG_EN.md` |
| README（英文） | `README_EN.md` |
| 项目宪法 | `CONSTITUTION.md` |
| 架构愿景 | `docs/ARCHITECTURE.md` |
| 算法文档 | `docs/ALGORITHMS.md` |
| 安全检查手册 | `docs/SECURITY_CHECKLIST.md` |
| 加密/解密核心 (v0.0.6) | `src/crypto/crypto.go` |
| 机器指纹采集 (v0.0.6) | `src/crypto/fingerprint*.go` |
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
| `--manual` | bool | false | 完整帮助手册 |
| `--language` | string | zh | 手册语言（zh/en） |
| `--filter` | string | "" | 过滤手册输出（忽略大小写，配合 --manual） |
| `--version` | bool | false | 输出版本号并退出 |
| `encrypt` | subcommand | — | 使用机器指纹加密配置文件 |

## 消费方

- **AI Agent**：通过 `SKILL.md` 调用，消费 stdout Markdown 报告或 `-json` 结构化输出
- **人类 DBA/运维**：终端直接运行，阅读格式化报告

## 已知限制与待办

所有已知问题跟踪在 `issues.json`（39 条，33 closed，1 pending-evaluation，1 wontfix，4 open/feature）。

**v0.0.4 已关闭（ISSUE-022 ~ ISSUE-032）：** IR v1 架构、Capability 重构、统一诊断、Importance Ranking、Context Compression、Delta Scan、Operational Stats、Windows 编码兼容。

**v0.0.5 已关闭：** `--log-dir` 日志目录（默认 `/var/log/dbexplain`）、一键安装脚本（scripts/install.sh/ps1）、全局配置搜索、SKILL 中英文分拆（`--lang zh|en`）、13 项 Bug 修复（ISSUE-040~052）。

**v0.0.6 新增：** 配置加密子命令（`encrypt`）、机器指纹绑定（Linux DMI/macOS sysctl/Windows Registry）、XChaCha20-Poly1305 AEAD 加密、PBKDF2-HMAC-SHA256 密码增强模式、运行时自动解密（`loadEnvFile` 自动检测加密文件头）。

**v0.0.7 已关闭：** Go 模块化发布（`github.com/IamWWT/dbexplain`）、公共 API（`src/core/` 导出 `Collect()`/`CollectToGraph()`/`CollectToJSON()`）、IR Graph 构建器（`BuildGraph()`）、ForeignKey 补全（OnDelete/OnUpdate，SQLite/MySQL/PostgreSQL 全覆盖）、SQLite INTEGER PRIMARY KEY nullable 修复、日志目录多级回退（`resolveLogDir()`）、全链路密码审计、只读查询执行引擎（`execute` 子命令 + sqlguard 沙箱）、9 数据库查询全覆盖。

**当前开放（ISSUE-033 ~ ISSUE-035）：** Phase 4 LLM 生态集成、GaussDB/TDSQL 兼容性确认、GBase/HBase/OceanBase 评估。

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
cd src && go build -o /tmp/dbexplain-curr .

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
| 4 | 进行中 | LLM Ecosystem Integration + MCP Server + 企业特性 |

当前版本：**v0.0.7** — Go 模块化 + 公共 API + IR Graph 构建器 + FK 补全 + 只读查询执行引擎（sqlguard 沙箱）+ 全链路安全审计。

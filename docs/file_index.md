# docs/ 文件索引

> 本文档索引 `docs/` 下所有文件，方便快速定位。
> 更新时间：v0.1.5 (2026-06-11)

---

## 一、核心架构与设计

| 文件 | 说明 |
|------|------|
| [`ARCHITECTURE.md`](ARCHITECTURE.md) | 整体架构设计：6 层架构、CapSQL 体系、IR 编译管道、安全架构 |
| [`ALGORITHMS.md`](ALGORITHMS.md) | 核心算法说明：Schema 采集、外键发现、健康评分、增量检测、Schema Diff |
| [`CODE_MAP.md`](CODE_MAP.md) | 文档-代码索引：模块↔文件映射、能力矩阵、文档↔源码交叉引用、CLI→处理器、测试覆盖图、常见问题 |
| [`POLICY.md`](POLICY.md) | 策略引擎：DENY_TABLES/DENY_COLUMNS/MASK_COLUMNS 配置规则 |
| [`CONFIG_SEARCH.md`](CONFIG_SEARCH.md) | 配置文件搜索路径与优先级：`findConfigFile()` 自动发现机制；`dbexplain encrypt` 加密保护凭证 |

## 二、部署与使用

| 文件 | 说明 |
|------|------|
| [`USAGE_GUIDE.md`](USAGE_GUIDE.md) | 全场景傻瓜用法手册：14 种数据源从安装到查询 |
| [`CLI_EXAMPLES.md`](CLI_EXAMPLES.md) | CLI 使用示例集合 |
| [`EXECUTE.md`](EXECUTE.md) | execute 子命令详解（只读查询） |
| [`REPL.md`](REPL.md) | REPL 交互模式：启动方式、内命令、自动行为、已知限制（ES 暂不支持等） |
| [`file-sources/FILE_PROCESSING.md`](file-sources/FILE_PROCESSING.md) | 文件查询引擎：CSV/XLSX 处理说明 |
| [`DEPLOY.md`](DEPLOY.md) | 部署说明：build.sh 编译模式、产物清单、平台支持 |

## 三、数据源专属文档（按类别分目录）

| 类别 | 文件 | 数据源 |
|------|------|--------|
| **关系型** | [`databases/relational/MYSQL.md`](databases/relational/MYSQL.md) | MySQL |
| | [`databases/relational/POSTGRESQL.md`](databases/relational/POSTGRESQL.md) | PostgreSQL |
| | [`databases/relational/SQLITE.md`](databases/relational/SQLITE.md) | SQLite |
| | [`databases/relational/DUCKDB.md`](databases/relational/DUCKDB.md) | DuckDB（可选构建，需 `-tags duckdb`） |
| | [`databases/relational/GAUSSDB.md`](databases/relational/GAUSSDB.md) | GaussDB |
| | [`databases/relational/COMPATIBILITY_GAUSSDB_TDSQL.md`](databases/relational/COMPATIBILITY_GAUSSDB_TDSQL.md) | GaussDB / TDSQL 兼容性说明 |
| | [`databases/relational/ORACLE.md`](databases/relational/ORACLE.md) | Oracle（12c+ 需 FETCH FIRST 支持） |
| **分析型** | [`databases/analytical/CLICKHOUSE.md`](databases/analytical/CLICKHOUSE.md) | ClickHouse |
| | [`databases/analytical/HIVE.md`](databases/analytical/HIVE.md) | Hive（通过 HiveServer2 SQL, 端口 10000） |
| **键值型** | [`databases/nosql/REDIS.md`](databases/nosql/REDIS.md) | Redis |
| **文档型** | [`databases/nosql/MONGO.md`](databases/nosql/MONGO.md) | MongoDB |
| | [`databases/nosql/ELASTICSEARCH.md`](databases/nosql/ELASTICSEARCH.md) | Elasticsearch |
| **向量型** | [`databases/nosql/QDRANT.md`](databases/nosql/QDRANT.md) | Qdrant |
| **时序型** | [`databases/prometheus.md`](databases/prometheus.md) | Prometheus |

## 四、安全与检查

| 文件 | 说明 |
|------|------|
| [`SECURITY_CHECKLIST.md`](SECURITY_CHECKLIST.md) | **安全检查手册**（发布前必读）：8 章覆盖凭证保护、文件编码、输入验证、安全传输、运行时安全、发布前快速检查（§6）、配置加密检查、新增安全问题流程 |
| [`SKILL_AUTHORING.md`](SKILL_AUTHORING.md) | SKILL.md 编写规范：Karpathy 上下文工程理念、YAML frontmatter、内容结构、文件大小约束、eval-first 迭代流程 |

## 五、运维

| 文件 | 说明 |
|------|------|
| [`operations/metrics.md`](operations/metrics.md) | 采集指标收集与 Prometheus 文本格式输出（v0.1.4+） |

## 六、测试文档

| 文件 | 说明 |
|------|------|
| [`test/README.md`](test/README.md) | 测试索引与运行方法 |
| [`test/01-environment.md`](test/01-environment.md) | 测试环境准备与构建模式 |
| [`test/02-schema-collection.md`](test/02-schema-collection.md) | Schema 采集测试 |
| [`test/03-execute-sql.md`](test/03-execute-sql.md) | SQL 执行测试 |
| [`test/04-execute-nosql.md`](test/04-execute-nosql.md) | NoSQL 执行测试 |
| [`test/05-file-processing.md`](test/05-file-processing.md) | 文件处理测试 |
| [`test/06-security-sqlguard.md`](test/06-security-sqlguard.md) | SQL 安全防护测试 |
| [`test/07-policy-engine.md`](test/07-policy-engine.md) | 策略引擎测试 |
| [`test/08-concurrent-limit.md`](test/08-concurrent-limit.md) | 并发限制测试 |
| [`test/09-cli-help.md`](test/09-cli-help.md) | CLI 帮助测试 |
| [`test/10-regression.md`](test/10-regression.md) | 回归测试 |
| [`test/11-end-to-end.md`](test/11-end-to-end.md) | 端到端集成测试 |
| [`test/12-capability-routing.md`](test/12-capability-routing.md) | 能力路由测试 |
| [`test/13-file-query-engine.md`](test/13-file-query-engine.md) | 文件查询引擎测试 |
| [`test/14-schema-diff.md`](test/14-schema-diff.md) | Schema Diff 测试 |
| [`test/15-window-functions.md`](test/15-window-functions.md) | 窗口函数测试 |
| [`test/16-duckdb.md`](test/16-duckdb.md) | DuckDB 连接器测试 |
| [`test/17-metrics.md`](test/17-metrics.md) | 采集指标收集与 Prometheus 输出测试 |
| [`test/RESULTS.md`](test/RESULTS.md) | 全量测试结果汇总 |

## 七、资产文件

| 文件 | 说明 |
|------|------|
| [`assets/architecture.drawio`](assets/architecture.drawio) + `.png` | 架构图源文件与导出 |
| [`assets/deployment.drawio`](assets/deployment.drawio) + `.png` | 部署图源文件与导出 |
| [`assets/skill-interaction.drawio`](assets/skill-interaction.drawio) + `.png` | 技能交互图源文件与导出 |
| [`assets/nl2sql_architecture_decision.svg`](assets/nl2sql_architecture_decision.svg) | NL2SQL 架构决策图 |
| [`assets/install-offline-*.png`](assets/install-offline-1.png) | 离线安装截图 |
| [`assets/usages.png`](assets/usages.png) | 使用概览图 |

## 八、其他

| 文件 | 说明 |
|------|------|
| [`RELEASE_WECHAT_v0.1.2.md`](RELEASE_WECHAT_v0.1.2.md) | v0.1.2 公众号发布文案 |
| [`RELEASE_WECHAT_v0.1.5.md`](RELEASE_WECHAT_v0.1.5.md) | v0.1.5 公众号发布文案（新增 Oracle + Hive 连接器，15 种数据源，六层安全防护） |
| [`dbexplain_wechat_article.html`](dbexplain_wechat_article.html) | 微信公众号文章 HTML |

---

## 九、CHANGELOG 编制规则

### 文件位置

| 文件 | 语言 |
|------|------|
| [`CHANGELOG.md`](../CHANGELOG.md) | 中文版（主版本） |
| [`CHANGELOG_EN.md`](../CHANGELOG_EN.md) | 英文版（同步更新） |

### 版本格式

```
## vX.Y.Z (YYYY-MM-DD) — {简短标题}
```

标题用中文 / English 各写一行，简明扼要概括版本重点（如 "CLI 交互增强 + DSL 联邦查询"）。

### 章节与顺序

每个版本按以下固定章节排序（无内容可省略，不写"无"）：

```
## vX.Y.Z (YYYY-MM-DD) — {标题}

### 新功能 / New Features          # 新增功能、新子命令

### 修复 / Fixes                    # Bug 修复、安全漏洞

### 构建与发布 / Build & Release    # 编译系统、CI/CD、产物变化

### 文档 / Documentation            # 文档新增/修改/删除

### 安全 / Security                 # 仅重大安全问题单独列章（可选）
                                   # 一般安全修复归入"修复"
```

### 条目格式

```
- **{功能/修复名称}** ({ISSUE}): {一句话描述。关键细节、原因可展开，保持简洁}
```

- ISSUE 编号用括号标注，如 `(ISSUE-069)`
- 无 ISSUE 编号时可省略
- **加粗** 标注功能名，后跟冒号和空格

### 同步规则

- `CHANGELOG.md` 和 `CHANGELOG_EN.md` 必须同步更新
- 发布前检查（见 `SECURITY_CHECKLIST.md §6`）：
  - 版本一致性：`version.go` / `build.sh ldflags` / `CHANGELOG.md` 版本号一致
  - CHANGELOG 完整性：当前版本所有已关闭 Issue 在 CHANGELOG 列出

### 历史示例

```markdown
## v0.1.2 (2026-06-03) — CLI 交互增强 + DSL 联邦查询 + 构建系统优化

### 新功能
- **DSL 联邦查询** (ISSUE-069): 跨数据源 JOIN/UNION 支持。移除 `len(kinds)>1` 阻断...

### 修复
- **DSL 文件 JOIN 修复** (ISSUE-069): `dslExecFile()` 传入 `nil allEntries`...

### 构建与发布
- **构建标签（Build Tags）**: 10 个 connector 文件添加 `//go:build mysql || full`...

### 文档
- **发布前检查标准补充** (ISSUE-073): SECURITY_CHECKLIST.md §6 追加...
```

---

> 索引维护：新增/删除文档时同步更新本索引。`CODE_MAP.md` 还维护更细粒度的代码-文档交叉引用。

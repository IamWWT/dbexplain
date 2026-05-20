# 变更日志

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

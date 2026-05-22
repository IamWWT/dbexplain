# 变更日志

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


# dbexplain v0.0.6 双版本发布：配置加密 + 一键安装 + 13项 Bug 修复

> 零依赖、9 种数据库的 AI Agent 上下文生成工具。
> 支持 MySQL / PostgreSQL / GaussDB / ClickHouse / SQLite / Redis / MongoDB / Elasticsearch / Qdrant。

---

v0.0.5 和 v0.0.6 合并发布，核心亮点：**配置文件硬件绑定加密**、**全平台一键安装**、**Skill 中英文分拆**、以及 **13 项 Bug 修复**。

---

## v0.0.6 重磅：配置文件硬件绑定加密

数据库密码以明文存储在 `.env` 文件中存在泄露风险。v0.0.6 新增 `encrypt` 子命令，使用机器指纹作为加密密钥，加密后的配置文件仅能在同一台机器上解密。

### 两种加密模式

**机器指纹模式（默认，无需密码）**：

```bash
# 加密配置文件
dbexplain encrypt ~/.config/dbexplain/.env.dbexplain

# 加密后删除明文配置！
rm ~/.config/dbexplain/.env.dbexplain

# 无需任何环境变量，直接运行（自动发现 .enc 文件）
dbexplain -env
```

**密码增强模式（密码 + 机器指纹双重保护）**：

```bash
# 加密时设置密码
dbexplain encrypt ~/.config/dbexplain/.env.dbexplain --password

# 删除明文配置！
rm ~/.config/dbexplain/.env.dbexplain

# 将密码写入密钥文件
echo "yourpassword" > ~/.config/dbexplain/.encryption_key
chmod 600 ~/.config/dbexplain/.encryption_key

# 同样无需环境变量，直接运行
dbexplain -env
```

### 技术细节

- **加密算法**：XChaCha20-Poly1305 (AEAD)，认证加密，防篡改
- **密钥派生**：机器模式用 SHA-256（硬件特征哈希），密码模式用 PBKDF2-HMAC-SHA256（10 万次迭代）
- **纯 Go 实现**：CGO_ENABLED=0，全平台交叉编译
- **安全设计**：加密文件权限 `0600`，密码输入不回显，解密失败不暴露内部原因（防侧信道攻击）
- **零配置自动解密**：`-env` 模式自动发现 `.enc` 加密文件并解密（首字节 `0x00`/`0x01`），无需任何环境变量。密码从 `~/.config/dbexplain/.encryption_key` 文件自动读取
- **重要**：加密后务必删除明文配置文件，否则 `findConfigFile()` 会优先匹配明文文件

### 跨平台机器指纹采集

| 平台 | 数据源 |
|------|--------|
| Linux | `/etc/machine-id`, `/sys/class/dmi/id/product_uuid`, `/proc/cpuinfo`, hostname |
| macOS | `sysctl hw.uuid`, `hw.model`, `hw.machine`, hostname |
| Windows | Registry `HKLM\SOFTWARE\Microsoft\Cryptography\MachineGuid`, hostname |

各数据源独立采集，失败静默跳过，多重兜底。全部不可用时返回明确错误。

### 加密文件搜索

`-env` 模式自动搜索加密文件（无需任何环境变量），搜索优先级：

1. `DBPROBE_ENV_FILE` 环境变量（可选覆盖）
2. `./.env.dbexplain`（当前目录，明文）
3. `./.env.dbexplain.enc`（当前目录，加密文件自动解密）
4. `~/.config/dbexplain/.env.dbexplain`（用户配置目录，明文）
5. `~/.config/dbexplain/.env.dbexplain.enc`（用户配置目录，加密文件自动解密）

> 加密后务必删除明文配置文件，否则工具会优先匹配明文文件。

### 加密的核心优势

1. **无需记忆密码**：机器指纹模式零密码，加密文件即用即解，对用户完全透明
2. **文件脱离机器即失效**：即使 `.enc` 文件被窃取，攻击者在其他机器上也无法解密（硬件指纹不匹配）
3. **纵深防御**：弥补防火墙/ACL 之后的最后一道防线——配置文件落盘加密。即使服务器被侵入，攻击者拿到 `.enc` 文件也无法在其他机器解密使用
4. **合规友好**：满足等保、GDPR 等法规对"敏感凭证静态加密"的要求
5. **零配置体验**：加密后直接 `dbexplain -env`，工具自动发现并解密，不增加运维负担

### 典型应用场景

| 场景 | 说明 |
|------|------|
| **生产服务器部署** | 加密一次，之后每次 `dbexplain -env` 自动解密，运维无感 |
| **CI/CD 流水线** | 构建机上加密配置绑定机器指纹，防止流水线日志泄露凭证 |
| **多机器差异化配置** | 每台机器独立加密各自的 `.env`，即使同一份配置文件模板也不通用 |
| **Docker 容器** | 绑定宿主机指纹，容器重建后仍可解密（前提是宿主机硬件不变） |
| **开发者本地环境** | 保护本地数据库密码，电脑丢失后加密文件不可用 |
| **AI Agent 安全交互** | Agent 只调用工具，绝不接触明文密码；加密文件由用户自行管理 |

---

## v0.0.5：一键安装，开箱即用

过去需要把二进制和配置文件放在同一目录，体验很差。现在一条命令搞定全局安装 + AI Skill 部署：

```bash
# 克隆项目
git clone https://github.com/IamWWT/understand_dbs_skills.git
cd understand_dbs_skills

# Linux/macOS 一键安装（中文 Skill）
bash db-relationship-explainer/scripts/install.sh

# 安装英文 Skill
bash db-relationship-explainer/scripts/install.sh --lang en

# 仅安装工具，跳过 Skill
bash db-relationship-explainer/scripts/install.sh --no-skill

# Windows PowerShell 一键安装
.\db-relationship-explainer\scripts\install.ps1           # 中文
.\db-relationship-explainer\scripts\install.ps1 -Lang en  # 英文
```

安装后 `dbexplain` 直接进系统 PATH，任意目录可运行。配置文件按 XDG 规范存放在 `~/.config/dbexplain/.env.dbexplain`。

配套卸载同样一键完成：

```bash
bash db-relationship-explainer/scripts/uninstall.sh
```

---

## Skill 国际化：中英文分拆

v0.0.5 将 Skill 拆分为 **SKILL_ZH.md**（中文）和 **SKILL_EN.md**（英文），安装脚本均支持 `--lang zh|en` 参数。

Skill 部署脚本也支持独立运行，交互选择目标平台（Claude Code / DeepSeek / AixCoding / Agents）：

```bash
bash db-relationship-explainer/scripts/install-skill.sh           # 中文 Skill
bash db-relationship-explainer/scripts/install-skill.sh --lang en # 英文 Skill
```

---

## 13 项 Bug 修复

v0.0.5 + v0.0.6 累计修复 13 个 Issue（ISSUE-040 ~ ISSUE-052）：

### CRITICAL 安全修复
- **ISSUE-040**：`.env` 真实凭证已从 Git 历史中移除，`.gitignore` 规则到位

### HIGH 高优先级
- **ISSUE-041**：`src/logs/` 生产日志目录加入 `.gitignore`，防止数据库名泄密
- **ISSUE-051**：`-json -o` 输出的 JSON 文件不再含 UTF-8 BOM，`jq`、`python json.load` 等标准解析器正常工作
- **ISSUE-052**：Windows 记事本保存的 `.env.dbexplain` 含 UTF-8 BOM 导致解析失败 — 已修复；godotenv 库错误消息泄露密码 — 已修复

### MEDIUM 数据准确性
- **ISSUE-045**：PostgreSQL 对空表执行无意义采样查询，现已添加 `RowCount > 0` 守卫
- **ISSUE-047**：GaussDB 实例被错误报告为 "postgres"，现已正确显示 "gaussdb"
- **ISSUE-048**：JSON 输出缺少操作统计字段（seq_scan、idx_scan 等），现已补充 `op_stats`

### LOW 工程优化
- **ISSUE-044**：删除 `analyze/infer.go` 死代码，消除 IP 检测误匹配 bug
- **ISSUE-046**：修复聚类命名逻辑，无分隔符时保留完整前缀
- **ISSUE-049**：MySQL 两次 `SHOW INDEX` 查询合并为一次，减少网络往返

---

## 测试体系升级

### v0.0.5 测试报告：105+ 用例

| 测试层 | 用例数 | 范围 |
|--------|--------|------|
| L1 静态分析 | 6 | build + vet + test + 交叉编译 + 安全审计 |
| L2 单元测试 | 77 | DSN 解析 + 字段推断 |
| L3 功能集成 | 20 | 全部 CLI 参数 + 输出格式组合 |
| L4 端到端 | 1 | 9 异构数据源全量采集 |
| L5 Bug 回归 | 12 | ISSUE-040 ~ 051 逐一验证 |

### v0.0.6 加密功能专项测试：28 用例，全部通过

| 测试类别 | 用例数 | 覆盖内容 |
|----------|--------|----------|
| 加密核心功能 | 10 | 机器指纹确定性、加密/解密、文件头验证、权限、随机 nonce、重复加密警告 |
| 密码模式 | 6 | 密码读取、文件头、缺密码处理、正确/错误密码、模式误用 |
| 标志解析 | 2 | `-h`/`--help`、`-o`/`--output`、`-password`/`--password` |
| BOM 兼容性 | 1 | 带 BOM 的 .env 文件加密 |
| 文档完整性 | 3 | `-h` 帮助、`--manual` 中英文加密章节 |
| 交叉编译 | 1 | 5 平台 CGO_ENABLED=0 |
| 静态分析 | 2 | `go build` + `go vet` |
| 文档一致性 | 1 | 17 个文件版本号一致性 |

**总计：全版本 133+ 用例，零失败。**

---

## 快速上手

```bash
# 安装后，创建全局配置
mkdir -p ~/.config/dbexplain
cat > ~/.config/dbexplain/.env.dbexplain << 'EOF'
DB1=mysql://root:pass@127.0.0.1:3306/mydb?label=my-mysql
DB2=redis://:pass@127.0.0.1:6379/0?label=my-redis
DB3=postgres://user:pass@127.0.0.1:5432/mydb?label=my-pg
EOF

# 加密配置（推荐），加密后删除明文！
dbexplain encrypt ~/.config/dbexplain/.env.dbexplain
rm ~/.config/dbexplain/.env.dbexplain

# 扫描所有数据源（自动发现 .enc 文件，无需环境变量）
dbexplain -env

# 指定日志目录
dbexplain -env --log-dir /var/log/dbexplain

# JSON 输出（供 AI Agent 消费）
dbexplain -env -json

# AI 上下文压缩
dbexplain -env --context ./ai-context/

# Schema 指纹（增量变更检测）
dbexplain -env --cache schema_cache.json

# 过滤特定类型
dbexplain -env -exclude redis,mongodb --human
```

---

## 支持 9 种数据库

| 类型 | DSN 示例 |
|------|---------|
| MySQL | `mysql://root:pass@host:3306/db?label=myapp` |
| PostgreSQL | `postgres://user:pass@host:5432/db?sslmode=require` |
| GaussDB | `gaussdb://user:pass@host:5432/db` |
| ClickHouse | `clickhouse://user:pass@host:9000/db` |
| SQLite | `sqlite:///path/to/file.db` |
| Redis | `redis://:pass@host:6379/0?cluster=true` |
| MongoDB | `mongodb://user:pass@host:27017/db` |
| Elasticsearch | `elasticsearch://host:9200?tls=true` |
| Qdrant | `qdrant://host:6333` |

---

## 资源链接

- **GitHub**: [IamWWT/understand_dbs_skills](https://github.com/IamWWT/understand_dbs_skills)
- **完整变更**: [CHANGELOG.md](https://github.com/IamWWT/understand_dbs_skills/blob/main/CHANGELOG.md)
- **完整测试报告**: [docs/TEST_v0.0.6.md](https://github.com/IamWWT/understand_dbs_skills/blob/main/docs/TEST_v0.0.6.md)（133+ 用例，含 v0.0.5 回归 + v0.0.6 加密专项）

---

*dbexplain — Make Databases AI-Readable.*

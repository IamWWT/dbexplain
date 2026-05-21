# DEPLOY_SRC.md – 源码部署指南

本指南帮助你在本地从源码构建 `dbexplain` 工具，并集成到 AI Skill 系统中。

---

## 环境要求

- **操作系统**：Linux / macOS / Windows (WSL 或 Git Bash)
- **Go 版本**：1.26+（已测试 1.26.3）
- **网络**：可访问 GitHub 及 Go 模块代理

---

## 1. 克隆项目

```bash
git clone https://github.com/IamWWT/understand_dbs_skills.git
cd understand_dbs_skills
```

项目结构如下（仅列出关键目录/文件）：

```
understand_dbs_skills/
├── src/                        # 源码主目录
│   ├── analyze/
│   │   ├── analyze.go          # 关系推断、聚类、问题检测
│   │   └── infer.go            # （可扩展）字段语义推断
│   ├── connector/              # 各数据源连接器
│   │   ├── registry.go         # 自注册机制（开闭原则）
│   │   ├── runner.go           # Panic 隔离采集
│   │   ├── connector.go        # 公共接口、日志上下文
│   │   ├── mysql.go
│   │   ├── postgres.go
│   │   ├── sqlite.go
│   │   ├── clickhouse.go
│   │   ├── redis.go            # 流式键空间分析、风险检测
│   │   ├── mongo.go
│   │   ├── qdrant.go
│   │   └── elasticsearch.go
│   ├── dsn/
│   │   └── dsn.go              # DSN 解析与脱敏
│   ├── render/
│   │   └── render.go           # 终端美化 + JSON 输出
│   ├── schema/
│   │   ├── types.go            # 数据结构定义
│   │   ├── errors.go           # 统一 DBError 类型
│   │   └── infer.go            # 字段语义推断规则
│   ├── main.go                 # 入口（并发控制、输出捕获）
│   ├── go.mod
│   └── build.sh                # 多平台交叉编译脚本
├── db-relationship-explainer/  # AI Skill 模板（自包含）
│   ├── SKILL.md
│   ├── .env.dbexplain.example
│   └── scripts/
│       ├── install.sh          # 工具安装器 (Linux/macOS)
│       ├── uninstall.sh        # 工具卸载器 (Linux/macOS)
│       ├── install-skill.sh    # Skill 多平台部署脚本
│       ├── uninstall-skill.sh  # Skill 卸载脚本
│       ├── install.ps1         # 工具安装器 (Windows)
│       └── uninstall.ps1       # 工具卸载器 (Windows)
├── docs/
│   └── MONGO.md                # MongoDB 排障文档
├── README.md
├── LICENSE
```

---

## 2. 下载依赖

进入源码目录并安装所有 Go 依赖：

```bash
cd src
go mod tidy
```

> 如果遇到网络问题，可以设置国内代理：
> ```bash
> go env -w GOPROXY=https://goproxy.cn,direct
> go mod tidy
> ```

---

## 3. 编译

### 3.1 单平台编译（快速验证）

```bash
CGO_ENABLED=0 go build -ldflags="-s -w" -o dbexplain .
```

- `CGO_ENABLED=0` 强制纯 Go 静态链接（SQLite 驱动 `glebarez/go-sqlite` 同样是纯 Go 实现）。
- `-ldflags="-s -w"` 去除调试符号，减小二进制体积。

编译后会在 `src/` 目录下生成 `dbexplain` 可执行文件。

### 3.2 多平台交叉编译

直接运行提供的脚本：

```bash
bash build.sh
```

该脚本会在 `release/` 目录生成以下平台二进制文件：
- `release/dbexplain-linux-amd64`
- `release/dbexplain-linux-arm64`
- `release/dbexplain-darwin-amd64`
- `release/dbexplain-darwin-arm64`
- `release/dbexplain-windows-amd64.exe`

---

## 4. 测试

编译完成后，用本地或测试库验证工具是否正常工作：

```bash
# 用 SQLite 快速测试
echo "CREATE TABLE t(id int primary key, name text);" | sqlite3 /tmp/test.db
./dbexplain -dsn "sqlite:////tmp/test.db"
```

若能看到终端美化的表结构卡片，说明编译成功。

也可以创建 `.env.dbexplain` 配置文件（推荐放在 `~/.config/dbexplain/.env.dbexplain`，详见第 6 节）并通过 `dbexplain -env` 同时测试多个数据源。

---

## 5. 部署到 Skill 系统

推荐使用一键安装脚本完成工具全局安装和 Skill 部署：

```bash
# 一键安装（工具 + Skill）
bash db-relationship-explainer/scripts/install.sh

# 或仅安装工具，稍后单独部署 Skill
bash db-relationship-explainer/scripts/install.sh --no-skill
```

如需手动部署或单独更新 Skill，参见以下步骤。

### 5.1 手动创建 Symlink

如果你手动部署 Skill 目录到 AI 平台（非通过脚本），需要创建指向系统二进制的 symlink：

```bash
# 假设已通过 scripts/install.sh 或手动将 dbexplain 安装到系统 PATH
# 目标 Skill 目录（例如 Claude Code）:
mkdir -p ~/.claude/skills/db-relationship-explainer/tools
ln -s $(which dbexplain) ~/.claude/skills/db-relationship-explainer/tools/dbexplain

# 或从 src/ 编译后手动放置
# cp src/dbexplain ~/.claude/skills/db-relationship-explainer/tools/dbexplain
# chmod +x ~/.claude/skills/db-relationship-explainer/tools/dbexplain
```

### 5.2 运行 Skill 安装脚本

```bash
bash db-relationship-explainer/scripts/install-skill.sh
```

交互选择安装目标：Claude Code / DeepSeek / AixCoding / Agents / 全部平台等。

### 5.3 集成到你的 AI 助手

按照你所使用的 AI Skill 规范（如 Claude Code、自定义 GPT Action）将该目录打包或直接引用，即可让 AI 调用该工具。详见 [`docs/DEPLOY_SKILLS.md`](DEPLOY_SKILLS.md)。

---

## 6. 配置数据库连接

工具支持三种方式指定 DSN：

1. **命令行参数**（推荐）：`-dsn 'scheme://...'`（注意含 `!` 等特殊字符时使用单引号包裹）
2. **配置文件**（`-env` 模式）：创建 `.env.dbexplain` 文件，使用 `DB1=, DB2=...` 格式。编号无需连续，程序会自动按数字升序加载。配置文件搜索优先级：
   1. `DBPROBE_ENV_FILE` 环境变量
   2. `.env.dbexplain`（当前目录）
   3. `~/.config/dbexplain/.env.dbexplain`（Linux/macOS）/ `%USERPROFILE%\.dbexplain\.env.dbexplain`（Windows）
   4. `.env`（当前目录，向下兼容旧版）
3. **JSON 配置文件**：`-config dbs.json`

推荐将配置文件放在用户目录下，这样从任意目录运行 `dbexplain -env` 都能加载。

常用 DSN 示例：

```bash
# MySQL
dbexplain -dsn 'mysql://root:password@localhost:3306/mydb?label=prod-mysql'

# PostgreSQL / pgvector
dbexplain -dsn 'postgres://user:pass@localhost:5432/mydb?label=pg'

# SQLite
dbexplain -dsn 'sqlite:///absolute/path/to/database.db'

# ClickHouse
dbexplain -dsn 'clickhouse://default:pass@localhost:8123/default'

# Redis（密码含特殊字符时使用单引号）
dbexplain -dsn 'redis://:password@localhost:6379/0?label=cache'

# MongoDB（必须指定库名）
dbexplain -dsn 'mongodb://user:pass@localhost:27017/mydb?authSource=admin&label=mongo'

# Qdrant（API Key 作为密码）
dbexplain -dsn 'qdrant://:api-key@localhost:6334'

# Elasticsearch
dbexplain -dsn 'elasticsearch://elastic:pass@localhost:9200'
```

> 确保所用数据库账号仅具有**只读权限**，以保证安全。

---

## 7. 常见问题

**Q: 编译时报 `cannot find module`**  
A: 执行 `go mod tidy` 前确认网络通畅，Go 版本 ≥1.26。

**Q: 密码中含 `!` 等特殊字符，命令行报错？**
A: 使用单引号包裹整个 DSN，例如 `-dsn 'redis://:pass!word@host:6379/0'`。在 `.env.dbexplain` 文件中无需转义。

**Q: `.env.dbexplain` 中注释掉前面的条目后，后面的会失效吗？**
A: 不会。程序会扫描所有 `DB<n>` 环境变量并按数字排序，编号无需连续。

**Q: Redis 扫描会拖慢服务吗？**  
A: 使用 `SCAN` 非阻塞迭代，且上限 2000 个 key，对每个模式仅进行一次安全采样（`HSCAN` 5 字段、`GETRANGE` 512 字节等），生产环境可安全使用。

**Q: MongoDB 认证失败**  
A: 检查 `authSource` 是否正确（用户创建时所在的库），详见 `docs/MONGO.md`。

**Q: 程序卡住不输出结果**
A: 可查看 `logs/<label>.log`（默认当前目录）或用 `--log-dir` 指定日志目录，定位具体卡在哪个数据库。必要时使用 `-timeout` 调整每个 DSN 的超时时间。若报告正常生成但终端无输出，可能由管道死锁引起（已修复，请更新到最新代码）。

**Q: 如何添加新的数据库支持**  
A: 在 `src/connector/` 下新建文件，实现 `Connector` 接口，并在 `init()` 中调用 `Register("类型", func() Connector { return xxxConnector{} })`，无需修改其他任何文件。

---

## 8. 卸载

直接删除项目目录及生成的二进制文件即可，无需其他清理。

---

现在你已经完成了从源码到可执行文件的部署，可以立即开始使用 `dbexplain` 探索任意数据库了！ 
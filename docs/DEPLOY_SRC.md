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
├── db-relationship-explainer/  # AI Skill 模板
│   ├── SKILL.md
│   └── tools/                  # 预编译二进制放置处
├── docs/
│   └── MONGO.md                # MongoDB 排障文档
├── README.md
├── LICENSE
└── deploy.md                   # 本文件
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

该脚本会生成以下平台二进制文件：
- `dbexplain-linux-amd64`
- `dbexplain-linux-arm64`
- `dbexplain-darwin-amd64`
- `dbexplain-darwin-arm64`
- `dbexplain-windows-amd64.exe`

---

## 4. 测试

编译完成后，用本地或测试库验证工具是否正常工作：

```bash
# 用 SQLite 快速测试
echo "CREATE TABLE t(id int primary key, name text);" | sqlite3 /tmp/test.db
./dbexplain -dsn "sqlite:////tmp/test.db"
```

若能看到终端美化的表结构卡片，说明编译成功。

也可以创建 `.env` 文件（参考下一节）并通过 `./dbexplain -env` 同时测试多个数据源。

---

## 5. 部署到 Skill 系统

`db-relationship-explainer/` 目录已经包含 Skill 定义文件 `SKILL.md`，你只需将对应平台的二进制放入 `tools/` 子目录即可。

### 5.1 复制二进制文件

```bash
# 假设你在 src/ 目录下编译完成，返回项目根目录
cd ..
cp src/dbexplain-linux-amd64 db-relationship-explainer/tools/dbexplain-linux-amd64
# 如需其他平台，同样复制
```

### 5.2 调整 SKILL.md（可选）

`db-relationship-explainer/SKILL.md` 已默认配置好工具的路径 `tools/dbexplain-{platform}`，如果你的 AI 平台要求特殊格式，可微调其中的触发词或说明。

### 5.3 集成到你的 AI 助手

按照你所使用的 AI Skill 规范（如 Claude Code、自定义 GPT Action）将该目录打包或直接引用，即可让 AI 调用该工具。

---

## 6. 配置数据库连接

工具支持三种方式指定 DSN：

1. **命令行参数**（推荐）：`-dsn 'scheme://...'`（注意含 `!` 等特殊字符时使用单引号包裹）
2. **`.env` 文件**：在 `src/` 目录创建 `.env`，使用 `DB1=, DB2=...` 格式。编号无需连续，程序会自动按数字升序加载。
3. **JSON 配置文件**：`-config dbs.json`

常用 DSN 示例：

```bash
# MySQL
./dbexplain -dsn 'mysql://root:password@localhost:3306/mydb?label=prod-mysql'

# PostgreSQL / pgvector
./dbexplain -dsn 'postgres://user:pass@localhost:5432/mydb?label=pg'

# SQLite
./dbexplain -dsn 'sqlite:///absolute/path/to/database.db'

# ClickHouse
./dbexplain -dsn 'clickhouse://default:pass@localhost:8123/default'

# Redis（密码含特殊字符时使用单引号）
./dbexplain -dsn 'redis://:password@localhost:6379/0?label=cache'

# MongoDB（必须指定库名）
./dbexplain -dsn 'mongodb://user:pass@localhost:27017/mydb?authSource=admin&label=mongo'

# Qdrant（API Key 作为密码）
./dbexplain -dsn 'qdrant://:api-key@localhost:6334'

# Elasticsearch
./dbexplain -dsn 'elasticsearch://elastic:pass@localhost:9200'
```

> 确保所用数据库账号仅具有**只读权限**，以保证安全。

---

## 7. 常见问题

**Q: 编译时报 `cannot find module`**  
A: 执行 `go mod tidy` 前确认网络通畅，Go 版本 ≥1.26。

**Q: 密码中含 `!` 等特殊字符，命令行报错？**  
A: 使用单引号包裹整个 DSN，例如 `-dsn 'redis://:pass!word@host:6379/0'`。在 `.env` 文件中无需转义。

**Q: `.env` 中注释掉前面的条目后，后面的会失效吗？**  
A: 不会。程序会扫描所有 `DB<n>` 环境变量并按数字排序，编号无需连续。

**Q: Redis 扫描会拖慢服务吗？**  
A: 使用 `SCAN` 非阻塞迭代，且上限 2000 个 key，对每个模式仅进行一次安全采样（`HSCAN` 5 字段、`GETRANGE` 512 字节等），生产环境可安全使用。

**Q: MongoDB 认证失败**  
A: 检查 `authSource` 是否正确（用户创建时所在的库），详见 `docs/MONGO.md`。

**Q: 程序卡住不输出结果**  
A: 可查看 `src/logs/<label>.log` 定位具体卡在哪个数据库。必要时使用 `-timeout` 调整每个 DSN 的超时时间。若报告正常生成但终端无输出，可能由管道死锁引起（已修复，请更新到最新代码）。

**Q: 如何添加新的数据库支持**  
A: 在 `src/connector/` 下新建文件，实现 `Connector` 接口，并在 `init()` 中调用 `Register("类型", func() Connector { return xxxConnector{} })`，无需修改其他任何文件。

---

## 8. 卸载

直接删除项目目录及生成的二进制文件即可，无需其他清理。

---

现在你已经完成了从源码到可执行文件的部署，可以立即开始使用 `dbexplain` 探索任意数据库了！ 
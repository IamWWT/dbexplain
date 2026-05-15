# deploy.md – 源码部署指南

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

项目结构如下：

```
understand_dbs_skills/
├── src/                        # 源码主目录
│   ├── analyze/
│   │   └── analyze.go
│   ├── connector/
│   │   ├── connector.go
│   │   ├── mysql.go
│   │   ├── postgres.go
│   │   ├── sqlite.go
│   │   ├── clickhouse.go
│   │   ├── redis.go
│   │   ├── mongo.go
│   │   ├── qdrant.go
│   │   └── elasticsearch.go
│   ├── dsn/
│   │   └── dsn.go
│   ├── render/
│   │   └── render.go
│   ├── schema/
│   │   └── types.go
│   ├── main.go
│   ├── go.mod
│   └── build.sh
├── db-relationship-explainer/  # AI Skill 模板
│   ├── SKILL.md
│   └── tools/                  # 编译好的二进制放这里
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

- `CGO_ENABLED=0` 强制纯 Go 静态链接，不依赖任何系统库（包括 SQLite 驱动 `glebarez/go-sqlite` 也是纯 Go 实现）。
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
./dbexplain -dsn "sqlite:///./test.db" -json
```

如果能看到 JSON 格式的数据库结构输出，说明编译成功。

也可以直接使用 `.env` 文件（见下文）配合 `-env` 测试多个数据库。

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

## 6. 配置数据库连接（供参考）

工具支持三种方式指定 DSN：

1. **命令行参数**（推荐）：`-dsn "scheme://..."`
2. **.env 文件**：在 `src/` 目录创建 `.env`，使用 `DB1=, DB2=...`（参考 `.env.example`）
3. **JSON 配置文件**：`-config dbs.json`

常用 DSN 示例：

```bash
# MySQL
./dbexplain -dsn "mysql://root:password@localhost:3306/mydb?label=prod-mysql"

# PostgreSQL / pgvector
./dbexplain -dsn "postgres://user:pass@localhost:5432/mydb?label=pg"

# SQLite
./dbexplain -dsn "sqlite:///absolute/path/to/database.db"

# ClickHouse
./dbexplain -dsn "clickhouse://default:pass@localhost:8123/default"

# Redis
./dbexplain -dsn "redis://:password@localhost:6379/0?label=cache"

# MongoDB（必须指定库名）
./dbexplain -dsn "mongodb://user:pass@localhost:27017/mydb?authSource=admin&label=mongo"

# Qdrant（API Key 作为密码）
./dbexplain -dsn "qdrant://:api-key@localhost:6334"

# Elasticsearch
./dbexplain -dsn "elasticsearch://elastic:pass@localhost:9200"
```

确保所用数据库账号仅具有**只读权限**，以保证安全。

---

## 7. 常见问题

**Q: 编译时报 `cannot find module`**  
A: 执行 `go mod tidy` 前确认网络通畅，Go 版本 ≥1.26。

**Q: Redis 连接失败，密码含特殊字符**  
A: 密码中的 `#` 需编码为 `%23`，`@` 编码为 `%40`。例如 `redis://:p%40ssw%23rd@host:6379`。

**Q: MongoDB 认证失败**  
A: 检查 `authSource` 是否正确（用户创建时所在的库），详见 `docs/MONGO.md`。

**Q: SQLite 编译需要 CGO？**  
A: 项目使用 `github.com/glebarez/go-sqlite` 纯 Go 实现，无需 CGO，设置 `CGO_ENABLED=0` 即可。

**Q: 程序卡住不输出结果**  
A: 可查看 `src/logs/<label>.log` 定位具体卡在哪个数据库。必要时使用 `-timeout` 调整每个 DSN 的超时时间。

**Q: 如何添加新的数据库支持**  
A: 在 `connector/` 下新建文件，实现 `Connector` 接口，并在 `connector.go` 中注册。重新编译即可。

---

## 8. 卸载

直接删除项目目录及生成的二进制文件即可，无需其他清理。

---

现在你已经完成了从源码到可执行文件的部署，可以立即开始使用 `dbexplain` 探索任意数据库了！ 
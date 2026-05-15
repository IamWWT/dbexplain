# dbexplain – 零依赖多数据库结构探查与关系分析工具

`dbexplain` 是一个**静态编译、无外部运行时依赖**的命令行工具，只需提供数据库连接串，即可：

- 🔍 自动导出表结构、列信息、索引、外键
- 🧩 分析跨库、跨实例的表关系（显式外键 + 命名推断）
- 🗺️ 生成聚类关系图与问题诊断
- 📄 支持终端美化输出和 JSON 格式

无需安装任何数据库客户端或驱动——所有逻辑编译进单个二进制文件，可运行在 **Linux/macOS/Windows（x86_64/arm64）** 上，**只读安全**。

---

## 支持的数据库

| 数据库      | 连接方式             | 备注                             |
|-------------|----------------------|----------------------------------|
| MySQL       | `mysql://`           | 通过系统表 + SHOW 命令            |
| PostgreSQL  | `postgres://`        | 通过 pg_catalog                  |
| GaussDB     | `gaussdb://`         | 兼容 PostgreSQL 协议，自动适配    |
| SQLite      | `sqlite://`          | 纯 Go 驱动，无 CGO               |
| ClickHouse  | `clickhouse://`      | 纯 HTTP 接口，无需驱动            |
| Redis       | `redis://`           | 通过 SCAN 推断键模式与结构        |

> 新增数据库只需实现一个 `Connector` 接口，轻松扩展。

---

## 快速开始

### 1. 下载或编译

从 [Releases](https://github.com/yourrepo/dbexplain/releases) 下载对应平台的二进制文件，或自行编译：

```bash
git clone https://github.com/yourrepo/dbexplain.git
cd dbexplain
go mod tidy
bash build.sh   # 生成多平台二进制到当前目录
```

### 2. 运行

```bash
./dbexplain -dsn "mysql://user:pass@localhost:3306/shop?label=shop-db"
```

输出将包含：
- 实例概览
- 每张表的列、类型、约束、索引、外键、行数估算
- 显式外键关系（实线）与推断关系（虚线）
- 表聚类（如 `orders* cluster`）
- 潜在问题（缺主键、未索引外键等）

### 3. 同时分析多个数据库

```bash
./dbexplain \
  -dsn "mysql://root:123@prod-db:3306/orders" \
  -dsn "postgres://admin:pass@crm-db:5432/customers" \
  -dsn "redis://:foobared@cache:6379/0?label=session-cache"
```

### 4. 使用配置文件或 `.env`

**JSON 配置文件** (`dbs.json`)：
```json
[
  "mysql://user:pass@localhost:3306/shop?label=shop",
  "postgres://user:pass@localhost:5432/warehouse"
]
```
运行：`./dbexplain -config dbs.json`

**`.env` 文件**：
```ini
DB1=mysql://user:pass@localhost:3306/shop
DB2=redis://:password@localhost:6379/0?label=cache
```
运行：`./dbexplain -env`

### 5. 输出选项

- `-o report.md`：将结果写入文件
- `-json`：输出 JSON 格式，便于程序消费

---

## DSN 格式

统一采用 URL 格式：

```
scheme://[user[:password]@]host[:port][/dbname][?label=alias]
```

示例：
- MySQL：`mysql://root:123@127.0.0.1:3306/mydb?label=本地库`
- PostgreSQL：`postgres://postgres:pass@localhost:5432/testdb`
- GaussDB：`gaussdb://user:pass@host:25308/db`
- SQLite：`sqlite:///./data.db`
- ClickHouse：`clickhouse://default:@localhost:8123/default`
- Redis：`redis://:foobared@localhost:6379/0`

密码在输出中自动脱敏。

---

## 输出示例（截取）

```
▸ Instances (2)
  shop-db                    mysql    1 db(s), 5 tables
  cache                      redis    1 db(s), 3 tables

▸ shop-db  /  mydb

  orders [InnoDB] ~42,000 rows  核心订单表
────────────────────────────────────────────
  name          type           flags         comment
  id            int(11)        PK NN
  user_id       int(11)        NN
  total         decimal(10,2)  NN
  status        tinyint(4)     NN
  created_at    datetime       NN
  indexes: IDX(user_id)  IDX(status,created_at)

▸ Relationships (3 explicit FK, 2 inferred)
  shop-db/mydb.orders(user_id) ──FK──▶ shop-db/mydb.users(id)
  shop-db/mydb.orders(created_by) ~~?~~~▶ crm-db/public.users(id) (inferred, 85%)

▸ Clusters (2)
  orders* cluster
    • shop-db/mydb/orders
    • shop-db/mydb/order_items
    • shop-db/mydb/users
  ...

▸ Issues (3)
  ⚠ shop-db/mydb/orders  FK column "user_id" has no index — full scan risk
  ℹ shop-db/mydb/order_items  no timestamp column — audit trail gap
  ⚠ shop-db/mydb/logs  no primary key defined
```

---

## 安全性

- **只读操作**：所有 SQL 均为 `SELECT`、`SHOW`、`PRAGMA` 或 Redis 的 `INFO`/`SCAN`/`GET` 等。**绝不会执行写、改、删操作**。
- **SQL 注入防护**：所有查询均使用参数化，标识符严格转义。
- **超时控制**：每个连接有独立的超时（默认 30s），避免挂起。
- **密码脱敏**：输出、日志中的 DSN 均隐藏密码。

---

## 扩展新数据库

1. 在 `connector/` 下新建文件（如 `hbase.go`）
2. 实现 `Connector` 接口：
   ```go
   type Connector interface {
       Collect(d *dsn.DSN) (*schema.Instance, error)
   }
   ```
3. 在 `connector/connector.go` 的 `switch` 中注册新类型
4. 重新编译

支持任何提供只读元数据查询能力的存储系统。

---

## 作为 Skill 集成到 Claude / AI 助手

本项目完全适配 **Claude Code Skill** 体系：

1. 将编译后的二进制放入 `tools/` 目录
2. 编写 `SKILL.md` 定义触发词和指令
3. Claude 即可直接调用该工具理解数据库结构

示例 `SKILL.md`：
```markdown
---
name: db-relationship-explainer
description: 零依赖探查数据库结构，生成关系图与问题报告。
tools:
  - path: tools/dbexplain-{platform}
---
使用 DSN 调用工具分析数据库。
```

---

## 开发

- 语言：Go 1.21+
- 依赖：仅标准库 + 数据库驱动（编译后静态链接）
- 构建：`CGO_ENABLED=0 go build -ldflags="-s -w"`

提交 PR 前请确保：
- 新连接器为只读且安全转义
- 通过 `go vet` 和 `golint`

---

## License

MIT © 2025 dbexplain contributors
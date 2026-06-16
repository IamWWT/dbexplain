# dbexplain 傻瓜用法手册

> 从零开始，5 分钟上手。不用懂任何底层原理。

---

## 这是什么？

`dbexplain` 是一个**命令行工具**，它能帮你做两件事：

1. **看数据库结构** —— 连上数据库，自动列出所有表、字段、索引、外键
2. **查数据** —— 安全地执行查询（只能查，不能改）

一个工具搞定 **14 种数据源**：MySQL / PostgreSQL / GaussDB / ClickHouse / SQLite / DuckDB / Redis / Elasticsearch / MongoDB / Qdrant / Prometheus / CSV / TSV / Excel。

> 本文档覆盖全部数据源的全部使用场景。需要更详细的例子见 [`CLI_EXAMPLES.md`](CLI_EXAMPLES.md)。

---

## 第一步：安装

下载一个文件，放到电脑上就能用。详细安装（含脚本、AI Skill 集成）见 [`DEPLOY.md`](DEPLOY.md)。

### Linux

> v0.1.7+ 以 tarball（`.tar.gz`）形式发布，每个 tarball 对应一个平台。

```bash
# 下载对应平台的 tarball（linux amd64 标准版，UPX 压缩）
wget https://github.com/IamWWT/dbexplain/releases/download/v0.1.7/dbexplain-v0.1.7-linux-amd64-std-upx.tar.gz

# 解压出二进制
tar -xzf dbexplain-v0.1.7-linux-amd64-std-upx.tar.gz
cp dbexplain-v0.1.7-linux-amd64-std-upx/dbexplain-linux-amd64-std ./dbexplain
chmod +x dbexplain
./dbexplain --version
```

> tarball 命名格式：`dbexplain-{版本}-{系统}-{架构}-{版型}.tar.gz`，解压后目录内即对应平台二进制。

### macOS

```bash
# Intel 芯片用 darwin-amd64，M1/M2 芯片用 darwin-arm64
# macOS 交叉编译不支持 UPX，使用 -noupx 包
wget https://github.com/IamWWT/dbexplain/releases/download/v0.1.7/dbexplain-v0.1.7-darwin-arm64-std-noupx.tar.gz

# 解压
tar -xzf dbexplain-v0.1.7-darwin-arm64-std-noupx.tar.gz
cp dbexplain-v0.1.7-darwin-arm64-std-noupx/dbexplain-darwin-arm64-std ./dbexplain
chmod +x dbexplain
./dbexplain --version
```

> 如果系统提示"无法验证开发者"：进"系统设置 → 隐私与安全性"，点"仍要打开"。或运行 `xattr -d com.apple.quarantine dbexplain`。

### Windows

```powershell
# PowerShell
Invoke-WebRequest https://github.com/IamWWT/dbexplain/releases/download/v0.1.7/dbexplain-v0.1.7-windows-amd64-std-upx.tar.gz -OutFile dbexplain-v0.1.7-windows-amd64-std-upx.tar.gz

# 解压（Windows 10+ 内置 tar）
tar -xzf dbexplain-v0.1.7-windows-amd64-std-upx.tar.gz
copy dbexplain-v0.1.7-windows-amd64-std-upx\dbexplain-windows-amd64-std.exe dbexplain.exe
.\dbexplain.exe --version
```

---

## 第二步：配置数据源

创建一个 `.env.dbexplain` 配置文件，把你要连的数据库写进去。

**Linux / macOS：** 文件放在 `~/.config/dbexplain/.env.dbexplain`

**Windows：** 文件放在 `%USERPROFILE%\.config\dbexplain\.env.dbexplain`

### 全部 14 种数据源的配置写法

把下面"你的情况"对应的那行复制到配置文件里，改掉地址/密码即可：

| 你的情况 | 配置文件里写（逐行复制） |
|---------|----------------------|
| **MySQL** | `DB1=mysql://用户:密码@127.0.0.1:3306/库名?label=my-mysql` |
| **PostgreSQL** | `DB2=postgres://用户:密码@127.0.0.1:5432/库名?label=my-pg` |
| **GaussDB** | `DB3=gaussdb://用户:密码@127.0.0.1:5432/库名?label=my-gauss` |
| **ClickHouse** | `DB4=clickhouse://用户:密码@127.0.0.1:8123/库名?label=my-ch` |
| **SQLite** | `DB5=sqlite:////tmp/数据库文件.db?label=my-sqlite` |
| **DuckDB** | `DB12=duckdb:///tmp/分析.db?label=my-duckdb`（可选，需 `-tags duckdb` 构建） |
| **Prometheus** | `DB13=prometheus://用户:密码@127.0.0.1:9090?label=my-prom` |
| **TSV 文件** | `DB14=tsv:///home/你的路径/数据文件.tsv?label=my-tsv` |
| **Excel 文件** | `DB11=xlsx:///home/你的路径/数据文件.xlsx?label=my-xlsx` |

> **密码含特殊字符**（`@` `:` `#` 等）需要编码：`@` → `%40`，`:` → `%3A`，`#` → `%23`

配置好之后，运行下面命令看是否生效：

```bash
dbexplain list -env
# 会列出你配的所有数据库，密码自动隐藏显示
```

---

## 第三步：看数据库结构（Schema 采集）

```bash
# 采集所有数据库的结构
dbexplain -env --human

# 或用显式子命令（v0.1.2+）
dbexplain collect -env --human
```

输出示例：
```
DB1: my-mysql (mysql://...)
  表 (3):
    ┌ users ────────────┐
    │ id        INTEGER PK    │
    │ name      TEXT          │
    │ email     TEXT          │
    └───────────────────┘
```

**各种输出方式：**

```bash
dbexplain -env                     # 终端格式（默认）
dbexplain -env --human             # 带标记的人类可读格式（推荐）
dbexplain -env --json              # JSON 格式（给程序/AI 用）
dbexplain -env --json -o 文件.json # 保存到文件
dbexplain -env --include my-mysql  # 只看某个数据库
dbexplain -env --context ./ctx     # 导出 AI 可直接用的上下文文件
dbexplain -env --tables            # 精简表格列表模式（仅 SQL 数据源，只显示名称/引擎/行数/大小/注释）
dbexplain -env --table users       # 只采集指定表的 schema（仅 SQL 数据源，connector 级 SQL 过滤）
```

---

## 第四步：查数据

> 所有查询都经过安全保护：只能查不能改、没写 LIMIT 的自动加 LIMIT 1000、密码自动隐藏。

### 查 SQL 数据库（MySQL / PostgreSQL / GaussDB / ClickHouse / SQLite / Elasticsearch）

```bash
# 按编号查
dbexplain execute -env --db 1 "SELECT * FROM users LIMIT 10" --human

# 按标签查
dbexplain execute -env --label my-pg "SELECT COUNT(*) FROM orders" --human

# 直接 DSN 查（不需要配置文件）
dbexplain execute -dsn "sqlite:////tmp/test.db" "SELECT * FROM t" --human

# 看查询计划
dbexplain execute -env --db 1 --explain "SELECT * FROM users WHERE id = 1" --human

# 交互式 REPL 模式（v0.1.2+，持续查询不退出）
dbexplain repl --dsn "sqlite:////tmp/test.db"

# 也可以加载配置文件，用 .conn 在多个数据源间切换
dbexplain repl -env
dbexplain repl --dsn "mysql://root:pass@127.0.0.1:3306/mydb"

# 自定义超时（默认 30 秒）和行数（默认 1000）
dbexplain execute -env --db 1 --timeout 60 --limit 500 "SELECT * FROM logs" --human
```

> **REPL 模式说明：**
> - 支持所有 14 种数据源：SQL（MySQL/PG/ClickHouse/SQLite/DuckDB 等）、NoSQL（Redis/ES/Mongo/Qdrant）、时序（Prometheus）、文件（CSV/TSV/Excel）
> - 支持 DSL 模式（`@label.table` 语法），包括单源查询和联邦跨源 JOIN
> - 配合 `-env` 启动后，可用 `.conn <label>` 在已配置的多个数据源之间切换
> - 跨源 JOIN/UNION（联邦查询）直接在 REPL 内使用 DSL 语法
```

### 查 Redis

Redis 直接输原生命令：

```bash
dbexplain execute -env --label my-redis "SCAN 0 MATCH user:* COUNT 10" --human
dbexplain execute -env --label my-redis "GET user:1001" --human
dbexplain execute -env --label my-redis "HGETALL session:abc123" --human
dbexplain execute -env --label my-redis "PING" --human
```

支持 30+ 只读命令：GET, HGET, HGETALL, SCAN, TYPE, LLEN, SMEMBERS, ZRANGE 等。写命令（SET, DEL 等）会被拦截。

### 查 MongoDB

MongoDB 用 JSON 格式查：

```bash
# 查集合里的数据
dbexplain execute -env --label my-mongo '{"find":"users","filter":{"status":"active"},"limit":5}' --human

# 聚合查询
dbexplain execute -env --label my-mongo '{"aggregate":"orders","pipeline":[{"$match":{"status":"done"}}]}' --human
```

支持 `find` 和 `aggregate` 两种操作。

### 查 Qdrant（向量数据库）

```bash
# 遍历集合
dbexplain execute -env --label my-qdrant '{"scroll":"runbooks","limit":20}' --human

# 统计数量
dbexplain execute -env --label my-qdrant '{"count":"runbooks"}' --human
```

### 查 Prometheus（时序数据库）

Prometheus 使用 PromQL（不是 SQL），直接写 PromQL 语句：

```bash
# 所有 up == 1 的目标
dbexplain execute "up == 1" --label my-prom --human

# 聚合查询
dbexplain execute "count by (job) (up)" --label my-prom --human

# 范围查询（过去5分钟平均负载）
dbexplain execute "avg(node_load1[5m])" --label my-prom --human

# 标签过滤
dbexplain execute "up{job='node'}" --label my-prom --human
```

**DSL 模式**（用 SQL 语法查 Prometheus）：

```bash
# DSL 编译为 PromQL
dbexplain execute --dsl "SELECT * FROM @my-prom.up WHERE job='node'" --label my-prom --human
```

**联邦查询**（Prometheus + MySQL 跨源 JOIN）：

```bash
# Prometheus 指标关联 MySQL 表
dbexplain execute -env --dsl "SELECT p.instance, p.hostip, p.job, p.value, i.product, i.subproduct
  FROM @my-prom.up p JOIN @aiops-mysql.iplist i ON p.hostip = i.hostip" --human

# 或 + 文件数据
dbexplain execute -env --dsl "SELECT p.*, c.region
  FROM @my-prom.up p JOIN @my-csv.nodes c ON p.hostip = c.ip" --human
```

> 联邦查询会将 Prometheus 指标全量物化到内存后再执行 JOIN，大指标注意内存。

### 用 DSL 模式查（统一写法）

DSL 模式让你用 `@标签名.表名` 统一引用任意数据源，不用记 `--db` 编号：

```bash
dbexplain execute -env --dsl "SELECT * FROM @my-mysql.users LIMIT 10" --human
dbexplain execute -env --dsl "SELECT * FROM @my-pg.orders WHERE status = 'active'" --human
```

> v0.1.2 起 DSL 模式支持跨源 JOIN/UNION（联邦查询），可用 `@label1.t1 JOIN @label2.t2` 关联不同数据库。支持的联邦数据源：SQL 数据库 + Prometheus（PromQL 物化）+ 文件（CSV/XLSX）。暂不支持 Redis / Mongo / Qdrant / ES 原生数据源，这些用前面的原生命令方式查。

---

## 第五步：查文件数据（CSV / Excel）

不需要数据库软件，dbexplain 自带 SQL 引擎，直接查文件。

```bash
# 查全部数据
dbexplain execute -env --label my-csv "SELECT * FROM 文件名(不含.csv后缀)" --human

# 条件过滤
dbexplain execute -env --label my-csv "SELECT * FROM sales WHERE amount > 1000" --human

# 分组统计
dbexplain execute -env --label my-csv \
  "SELECT region, COUNT(*) AS cnt, AVG(amount) AS avg_amount
   FROM sales GROUP BY region ORDER BY cnt DESC" --human

# 多个文件 JOIN（跨文件关联）
dbexplain execute -env --label my-csv \
  "SELECT s.*, o.dept_name FROM sales s JOIN org o ON s.dept_id = o.dept_id" --human
```

> Excel 用法一样，表名用 Sheet 名（如 `Sheet1`）。如果只有一个 Sheet，也可以用文件名当表名。

### 文件查询支持的全部 SQL

| 你想干嘛 | 怎么写 |
|---------|--------|
| 条件过滤 | `WHERE amount > 1000` |
| 模糊匹配 | `WHERE name LIKE '%张%'` |
| 范围筛选 | `WHERE age BETWEEN 18 AND 60` |
| 值列表匹配 | `WHERE status IN ('active', 'pending')` |
| 空值判断 | `WHERE remark IS NULL` |
| 分组统计 | `GROUP BY region` |
| 聚合函数 | `COUNT`, `SUM`, `AVG`, `MIN`, `MAX` |
| 分组后过滤 | `HAVING avg_amount > 100` |
| 排序 | `ORDER BY amount DESC` |
| 空值排序 | `ORDER BY amount DESC NULLS LAST` |
| 分页 | `LIMIT 10 OFFSET 5` |
| 多表关联 | `JOIN / LEFT JOIN / RIGHT JOIN ... ON ...` |
| 合并结果 | `UNION / UNION ALL` |
| 去重 | `DISTINCT / DISTINCT ON (col1, col2)` |
| 子查询 | `WHERE id IN (SELECT id FROM org ...)` |
| 窗口函数 | `ROW_NUMBER() OVER (PARTITION BY dept ORDER BY score DESC)` |
| 排名 | `RANK()`, `DENSE_RANK()`, `NTILE(4)` |
| 前后行取值 | `LAG(amount)`, `LEAD(amount)` |
| 首行末行 | `FIRST_VALUE(amount)`, `LAST_VALUE(amount)` |
| 聚合窗口 | `SUM(amount) OVER (PARTITION BY dept)` |
| 类型转换 | `CAST(col AS INTEGER)`, `CAST(col AS FLOAT)` |
| 数学运算 | `amount * 0.1`, `ROUND(avg_score, 2)`, `ABS(diff)` |
| 去重计数 | `COUNT(DISTINCT col)` |

---

## 第六步：对比 Schema 变化

想知道数据库结构变了什么（比如加了字段、改了类型）：

```bash
# 1. 先保存一个版本
dbexplain -env --cache /tmp/schema.cache --json -o /tmp/v1.json --version-label v1.0

# 2. 过段时间再采一次
dbexplain -env --cache /tmp/schema.cache --json -o /tmp/v2.json --version-label v2.0

# 3. 对比差异
dbexplain diff --cache /tmp/schema.cache --since v1.0 --human
```

会检测：新增/删除/修改了哪些表、字段（类型/可空/默认值/注释/主键）、索引、外键。

---

## 其他常用操作

| 你想干嘛 | 命令 |
|---------|------|
| 查看已配置的数据库列表 | `dbexplain list -env` |
| 加密配置文件（防密码泄露） | `dbexplain encrypt`（自动搜索 .env.dbexplain，输出 .enc） |
| 查看帮助手册 | `dbexplain all` |
| 只看某个数据库的帮助 | `dbexplain mysql` / `dbexplain redis` / ... |

---

## 常见问题

### 连不上数据库

```
dial tcp: connection refused
```

→ 数据库没启动，或地址端口写错了。先用 `ping` 和 `telnet` 确认能通。

```
access denied for user
```

→ 用户名或密码错了。检查 `.env.dbexplain` 里的密码是否正确。

```
i/o timeout
```

→ 网络不通或防火墙拦截了。检查网络和防火墙规则。

### 找不到配置文件

```
no config file found
```

→ 确认文件在 `~/.config/dbexplain/.env.dbexplain`（Windows 是 `%USERPROFILE%\.config\dbexplain\.env.dbexplain`）。或者把文件放在当前目录下。

### 文件查询报错

```
table "xxx" not found
```

→ SQL 里 FROM 后面要写**文件名（不含 .csv）**，不是 `--label` 的名字。文件叫 `sales_data.csv`，SQL 就写 `FROM sales_data`。

```
parse error
```

→ SQL 语法可能不支持。文件查询引擎不支持 CTE（WITH 语句）、UPDATE/INSERT/DELETE、FROM 子查询。试试 `SELECT * LIMIT 5` 确认工具本身没问题。

### 更多问题

详见 [`troubleshooting.md`](../dbexplain-skill/references/troubleshooting.md) —— 包含 9 类数据库连接问题和 5 类文件查询问题的详细排障。

---

## 下一步

| 如果你想... | 去看 |
|------------|------|
| 看更多真实查询例子（含实际返回数据） | [`docs/CLI_EXAMPLES.md`](CLI_EXAMPLES.md) |
| 部署到服务器或集成 AI Skill | [`docs/DEPLOY.md`](DEPLOY.md) |
| 设置安全策略（禁止查某些表/列） | [`docs/POLICY.md`](POLICY.md) |
| 了解全部命令行参数 | [`README.md`](../README.md) |

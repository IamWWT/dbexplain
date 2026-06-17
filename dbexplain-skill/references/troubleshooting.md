# dbexplain 排障指南

> 适用于 `dbexplain` 采集、`dbexplain execute` 查询、DSL 联邦查询。
> exit code ≠ 0 时按此文档诊断。

---

## 快速诊断

### 数据库连接问题

| 错误信息 | 跳到章节 |
|----------|----------|
| `dial tcp: lookup` / `no such host` | [§1.1 无法解析主机名](#11-无法解析主机名) |
| `connection refused` | [§1.2 端口未开放/服务未启动](#12-端口未开放服务未启动) |
| `i/o timeout` | [§1.3 连接超时](#13-连接超时) |
| `access denied` / `authentication failed` | [§1.4 认证失败](#14-认证失败) |
| `unsupported protocol scheme` | [§1.5 DSN 格式错误](#15-dsn-格式错误) |
| `SSL is required` | [§1.6 SSL/TLS 配置问题](#16-ssltls-配置问题) |
| `database not found` | [§1.7 数据库名或 Schema 不存在](#17-数据库名或-schema-不存在) |
| `no scanners configured` | [§1.8 连接器未初始化](#18-连接器未初始化) |

### 查询问题

| 错误信息 | 跳到章节 |
|----------|----------|
| `parse error` | [§2.1 SQL 语法不支持](#21-sql-语法不支持) |
| `table "xxx" not found` | [§2.2 表名错误](#22-表名错误) |
| `ERROR: multiple DSNs matched` | [§2.3 多 DSN 缺 --label](#23-多-dsn-缺---label) |
| `file not found` / `Instances (0)` | [§2.4 文件路径错误](#24-文件路径错误) |
| 同一命令重复 3 次仍报错 | [§2.5 重试卡死协议](#25-重试卡死协议) |
| `column "xxx" not found` | [§2.2 表名错误](#22-表名错误) |
| `QUERY_ERROR` | [§2.1 SQL 语法不支持](#21-sql-语法不支持) |

### DSL / 联邦查询问题

| 错误信息 | 跳到章节 |
|----------|----------|
| `@xxx not found` | [§3.1 DSL 标签未识别](#31-dsl-标签未识别) |
| `multiple DSNs matched @xxx` | [§3.2 DSL 标签冲突](#32-dsl-标签冲突) |
| `federated JOIN not supported for @xxx` | [§3.3 联邦查询限制](#33-联邦查询限制) |
| `promql() parse error` | [§3.4 promql 语法错误](#34-promql-语法错误) |
| `FILE_ACCESS_DENIED` | [§3.5 DuckDB 文件访问被拒](#35-duckdb-文件访问被拒) |

### NoSQL 查询问题

| 错误信息 | 跳到章节 |
|----------|----------|
| ES `array fields not supported` | [§4.1 ES 数组字段](#41-es-数组字段) |
| Redis `MOVED` / `CLUSTERDOWN` | [§4.2 Redis 集群](#42-redis-集群) |
| Mongo `auth fails` | [§4.3 MongoDB 认证](#43-mongodb-认证) |

---

## 1. 数据库连接问题

适用于 MySQL / PostgreSQL / ClickHouse / SQLite / Redis / MongoDB / Elasticsearch / Qdrant / Oracle / Hive / DuckDB 等数据库类型。

### 1.1 无法解析主机名

**现象**：
```
dial tcp: lookup unknown-host: no such host
```

**根因**：DSN 中的主机名无法通过 DNS 解析。

**修正**：
- 确认主机名拼写正确
- 确认网络可达（ping 测试）
- 如需通过 IP 连接，用 IP 地址替换主机名

### 1.2 端口未开放/服务未启动

**现象**：
```
dial tcp: connect: connection refused
```

**根因**：目标主机端口未监听。常见原因：服务未启动、端口配置错误、防火墙拦截。

**修正**：
- 确认目标服务已启动
- 检查端口号是否正确
- 检查防火墙规则是否放行

### 1.3 连接超时

**现象**：
```
i/o timeout
```

**根因**：网络延迟高、防火墙丢包、或服务负载过高。

**修正**：
- 检查网络连通性（ping/telnet）
- 确认没有防火墙丢弃 SYN 包
- 对大数据集考虑设置 `--timeout` 增大超时时间（默认 20s）
- 可在 DSN 加 `timeout=60` 参数

### 1.4 认证失败

**现象**：
```
access denied for user 'xxx'@'...'
authentication failed
ERROR 1045 (28000): Access denied
```

**根因**：用户名或密码错误；或用户无访问指定数据库的权限。

**修正**：
- 确认用户名和密码正确
- 确认密码不含特殊字符（或在 `.env` 中正确转义）
- 确认用户有远程访问权限
- **Agent 不能查看/记录明文密码** — 让用户自行在 `.env` 检查

### 1.5 DSN 格式错误

**现象**：
```
unsupported protocol scheme ""
parse "://": missing scheme
```

**根因**：DSN 字符串格式不符合规范。

**修正**：
- 确认 DSN 包含完整 scheme 前缀
- 确认 DSN 中无多余空格
- `.env` 文件格式：`DB1=scheme://user:pass@host:port/db?label=xxx`
- 密码含 `@`、`:`、`#` 等特殊字符需 URL 编码

```
正确：DB1=mysql://user:Te%3Fst@localhost:3306/mydb?label=test
错误：DB1=mysql://user:Te?st@localhost:3306/mydb?label=test
```

### 1.6 SSL/TLS 配置问题

**现象**：
```
SSL is required but the server doesn't support it
tls: first record does not look like a TLS handshake
```

**修正**：
- 检查数据库服务端 SSL 配置
- 测试环境可在 DSN 加 `sslmode=disable`（PostgreSQL）
- ES/Redis TLS 用 `rediss://` / `elasticsearchs://` scheme

### 1.7 数据库名或 Schema 不存在

**现象**：
```
database "xxx" does not exist
```

**修正**：
- 确认数据库名正确（大小写敏感）
- 先用 `dbexplain list` 确认可用数据库列表
- MongoDB DSN 必须包含数据库名和 `authSource` 参数

### 1.8 连接器未初始化

**现象**：
```
no scanners configured
```

**根因**：对应的数据库连接器未在编译时启用。

**修正**：
- 运行 `dbexplain all` 查看支持的数据库类型列表
- DuckDB/XLSX 需特定 build tag 版二进制

---

## 2. 查询问题

适用于 SQL / 文件查询引擎（CSV/TSV/XLSX）。

### 2.1 SQL 语法不支持

**现象**：
```
QUERY_ERROR: csv query error: parse error: ...
```

**根因**：SQL 语法超出文件查询引擎支持范围。

**已知不支持语法**：
- CTE（WITH 语句）
- UPDATE/INSERT/DELETE（只读引擎）
- FROM 子查询（如 `SELECT * FROM (SELECT ...) AS t`）

**修正**：
1. 检查 SQL 是否使用了以上不支持语法
2. 改写为等价支持语法（见 `sql-syntax.md`）
3. 确认 SQL 后最简单的 `SELECT * LIMIT 5` 先确认工具可用

### 2.2 表名错误

**现象**：`table "xxx" not found` 或结果意外。

**根因**：`FROM` 用了 `--label` 而不是文件名（文件引擎）。

```
DSN: csv:///...sales_data.csv?label=my_data
正确：SELECT * FROM sales_data    ← 文件名（不含 .csv）
错误：SELECT * FROM my_data       ← 这是 label！
```

### 2.3 多 DSN 缺 --label

**现象**：
```
ERROR: 2 DSNs matched — use --label to select one
```

**修正**：加 `--label` 指定数据源：
```bash
dbexplain execute --label sales 'SELECT COUNT(*) FROM t' --human
```

### 2.4 文件路径错误

**现象**：`file not found` 或 `Instances (0)`。

**根因**：DSN 使用了相对路径。`csv:///file.csv` 解析为 `/file.csv`（从根开始）。

**修正**：用绝对路径：
```bash
DB1=csv:///home/user/data/sales.csv?label=data
```

### 2.5 重试卡死协议

同一命令重复 3 次仍报错 → 停止重试。打开本指南找对应章节，理解根因后重新构造命令。

---

## 3. DSL / 联邦查询问题

### 3.1 DSL 标签未识别

**现象**：`@xxx not found` 或 `source "xxx" not found`。

**根因**：DSL 查询中的 `@label` 与 DSN 配置中的 `label=` 不匹配。

**修正**：
```bash
# 先查看可用 label
dbexplain
# 确认 label 值后用正确的 @label
dbexplain execute --dsl 'SELECT * FROM @正确的label.table' --human
```

### 3.2 DSL 标签冲突

**现象**：`multiple DSNs matched @xxx`。

**根因**：多个 DSN 使用相同 label。

**修正**：检查 `.env.dbexplain`，确保每个 DSN 的 `label=` 唯一。

### 3.3 联邦查询限制

**现象**：`federated JOIN not supported for @xxx`。

**根因**：部分数据源类型不可作为联邦查询的 JOIN 端。

**不支持 JOIN 的数据源**：Redis、MongoDB、Elasticsearch、Qdrant（原生/JSON 查询不可参与联邦 JOIN）。

**修正**：仅 SQL、文件、Prometheus 数据源可参与联邦 JOIN。

### 3.4 promql 语法错误

**现象**：`promql() parse error` 或 Prometheus API 返回错误。

**根因**：promql() 内表达式不符合 PromQL 语法。

**修正**：
```bash
# 确认 promql() 内的表达式语法正确
# 可在 Prometheus UI 的 Graph 页面先验证 PromQL

# 用最简单的 promql() 先确认管道通畅
dbexplain execute --label prom 'SELECT * FROM @prom.promql(up)' --human
```

### 3.5 DuckDB 文件访问被拒

**现象**：`FILE_ACCESS_DENIED: path "xxx" is not within allowed_path`。

**根因**：DuckDB 执行 `read_parquet/read_csv/read_json` 时路径超出 `allowed_path` 白名单。

**修正**：在 DSN 中设置 `allowed_path`：
```
DB1=duckdb:///:memory:?allowed_path=/data/parquet/,/data/csv/&label=duck
```

---

## 4. NoSQL 查询问题

### 4.1 ES 数组字段

**现象**：ES 查询返回数据异常。

**根因**：ES 不支持数组字段，选择了数组类型字段。

**修正**：只选标量字段，避免选择数组类型字段。

### 4.2 Redis 集群

**现象**：`MOVED` 重定向或 `CLUSTERDOWN` 错误。

**根因**：集群模式未在 DSN 中配置。

**修正**：DSN 加 `cluster=true`：
```
DB6=redis://:password@host:7000/0?cluster=true&label=cluster
```

### 4.3 MongoDB 认证

**现象**：MongoDB 认证失败。

**根因**：DSN 缺少 `authSource` 参数或数据库名。

**修正**：
```
mongodb://user:pass@host:27017/db?authSource=admin&label=mongo
```

---

## 5. 配置问题

### 5.1 不知如何配置 .env

**现象**：用户没有 `.env.dbexplain` 文件，不知道格式。

**修正**：项目根目录有模板文件可直接复制：
```bash
# 拷贝模板到默认配置路径
cp dbexplain-skill/.env.dbexplain.example ~/.config/dbexplain/.env.dbexplain
# 或直接在当前目录创建
cp dbexplain-skill/.env.dbexplain.example .env.dbexplain
```

模板内已包含所有 16+ 数据源的 DSN 格式示例，取消注释并按实际值修改即可。Agent **不能**替用户编辑此文件，告知路径和格式后等用户完成。

### 5.2 密码特殊字符导致连接不通

**现象**：
```
access denied for user 'xxx'@'...'
或 DSN parse error
```

**根因**：密码中含 `@`、`:`、`#`、`?`、`%` 等 URL 保留字符，未被正确转义。

DSN 格式本质是 URL，密码部分需 URL 编码：

| 特殊字符 | URL 编码 | 说明 |
|----------|---------|------|
| `@` | `%40` | 会被解析为用户:密码 与 host 的分隔符 |
| `:` | `%3A` | 会被解析为密码结束符 |
| `#` | `%23` | 会被解析为 URL fragment，其后内容被忽略 |
| `?` | `%3F` | 会被解析为 query string 开始 |
| `%` | `%25` | 百分号自身需编码 |
| `空格` | `%20` | URL 不允许裸空格 |
| `/` | `%2F` | 会被解析为路径分隔符 |

```bash
# 错误：密码含 @，解析为 user:pass@host → 密码只取了 Te
DB1=mysql://user:Te@st@localhost:3306/mydb?label=test

# 正确：@ 编码为 %40
DB1=mysql://user:Te%40st@localhost:3306/mydb?label=test
```

> **快速检查**：密码中如有 `@`、`#`、`:`、`?`、`%`，大概率需要 URL 编码。全部特殊字符映射见 `docs/security-policies/PASSWORD_SPECIAL_CHARS.md`。

**命令行提示**：DSN 用**单引号**包裹防止 bash 解释特殊字符：
```bash
dbexplain -dsn 'mysql://user:Te%40st@host:3306/db?label=test'
```

### 5.3 加密配置文件

- Agent **绝不能**读取加密密钥或参与加密过程
- 加密后务必删除明文文件（否则工具优先加载明文）

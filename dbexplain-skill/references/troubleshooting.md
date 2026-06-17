# dbexplain 排障指南

> 适用于 `dbexplain` 采集和 `dbexplain execute` 查询。如遇 exit code ≠ 0，按此文档诊断。

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

### 文件查询问题

| 错误信息 | 跳到章节 |
|----------|----------|
| `parse error` | [§2.1 SQL 语法不支持](#21-sql-语法不支持) |
| `table "xxx" not found` | [§2.2 FROM 表名用了 label](#22-from-表名用了-label) |
| `ERROR: multiple DSNs matched` | [§2.3 多 DSN 缺 --label](#23-多-dsn-缺---label) |
| `file not found` | [§2.4 文件路径错误](#24-文件路径错误) |
| `Instances (0)` / 无查询结果 | [§2.4 文件路径错误](#24-文件路径错误) |
| `column "xxx" not found` | [§2.2 FROM 表名用了 label](#22-from-表名用了-label) |
| `QUERY_ERROR` | [§2.1 SQL 语法不支持](#21-sql-语法不支持) |
| 同一命令重复 3 次仍报错 | [§2.5 重试卡死协议](#25-重试卡死协议) |

---

## 1. 数据库连接问题

适用于 MySQL / PostgreSQL / ClickHouse / SQLite / Redis / MongoDB / Elasticsearch / Qdrant 等数据库类型。

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
- 检查端口号是否正确（MySQL: 3306, PG: 5432, ClickHouse: 8123, Redis: 6379, MongoDB: 27017, ES: 9200）
- 检查防火墙规则是否放行

### 1.3 连接超时

**现象**：
```
i/o timeout
```
或查询长时间无响应后超时。

**根因**：网络延迟高、防火墙丢包、或服务负载过高。

**修正**：
- 检查网络连通性（ping/telnet）
- 确认没有防火墙丢弃 SYN 包
- 对大数据集考虑设置 `--timeout` 增大超时时间（默认 30s）
- 考虑在配置文件中设置 `timeout=60s` 等连接参数

### 1.4 认证失败

**现象**：
```
access denied for user 'xxx'@'...' (using password: YES/NO)
authentication failed
ERROR 1045 (28000): Access denied
```

**根因**：用户名或密码错误；或用户无访问指定数据库的权限。

**修正**：
- 确认用户名和密码正确
- 确认密码不含特殊字符（或在 `.env` 中正确转义）
- 确认用户有远程访问权限（`'user'@'%'` vs `'user'@'localhost'`）
- **Agent 不能查看/记录明文密码** — 让用户自行在 `.env` 检查

### 1.5 DSN 格式错误

**现象**：
```
unsupported protocol scheme ""
parse "://": missing scheme
```

**根因**：DSN 字符串格式不符合规范。

**修正**：
- 确认 DSN 包含完整 scheme 前缀：`mysql://`, `postgres://`, `clickhouse://`, `redis://`, `mongodb://`, `elasticsearch://`, `qdrant://`, `sqlite://`
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
x509: certificate is valid for ...
```

**根因**：服务端要求 SSL 但客户端未配置，或证书不匹配。

**修正**：
- 检查数据库服务端 SSL 配置
- 可在 DSN 参数中配置 SSL 模式（具体参数取决于数据库类型）
- 对测试环境可在连接参数中设置 `sslmode=disable`（如 PostgreSQL）

### 1.7 数据库名或 Schema 不存在

**现象**：
```
database "xxx" does not exist
Schema "xxx" not found
```

**根因**：DSN 中指定的数据库名在目标实例上不存在。

**修正**：
- 确认数据库名正确（大小写敏感）
- 先用 `dbexplain list` 确认可用数据库列表
- MongoDB DSN 必须包含数据库名和 `authSource` 参数

### 1.8 连接器未初始化

**现象**：
```
no scanners configured
```

**根因**：对应的数据库连接器未在编译时启用（如 XLSX 需要 `-tags excel` 构建）。

**修正**：
- 确认 `dbexplain` 版本包含所需连接器
- 运行 `dbexplain all` 查看支持的数据库类型列表
- 对 XLSX 需使用带 `-tags excel` 标志构建的二进制

---

## 2. 文件查询问题

适用于 CSV / TSV / XLSX 文件数据源的查询错误。

### 2.1 SQL 语法不支持

**现象**：
```
QUERY_ERROR: csv query error: parse error: ...
```

**根因**：SQL 语法超出文件查询引擎支持范围。

#### 已知不支持语法

- ~~窗口函数（ROW_NUMBER、RANK 等）~~ ✅ v0.1.1 已支持
- CTE（WITH 语句）
- UPDATE/INSERT/DELETE（只读引擎）
- FROM 子查询（如 `SELECT * FROM (SELECT ...) AS t`）

#### 修正步骤

1. 检查 SQL 是否使用了以上不支持语法
2. 如有，改写为等价的支持语法（替代方案见 [`sql-syntax.md`](sql-syntax.md)）
3. 如 SQL 看起来正确但仍报错，用最简单的 `SELECT * LIMIT 5` 先确认工具本身可用

#### SQL 字符串引号说明

文件查询引擎同时支持单引号和双引号包裹字符串值。注意与 bash 外层引号不冲突：

```bash
# 外层双引号 + SQL 内单引号（推荐含中文/特殊字符时）
dbexplain execute --label my_data \
  "SELECT * FROM t WHERE col = 'value'" --human

# 外层单引号 + SQL 内双引号
dbexplain execute --label my_data \
  'SELECT * FROM t WHERE col = "value"' --human
```

两种均合法，任选一种即可。

---

### 2.2 FROM 表名用了 label

**现象**：
```
table "xxx" not found
```
或查询返回意外结果。

**根因**：SQL 的 `FROM` 子句用了 `--label` 的值，而不是文件名（不含扩展名）。

#### 表名从哪来

```
dbexplain 输出：
  DB1 → my_data  → csv:///...sales_data.csv
         ↑label    ↑文件名 = sales_data

正确 SQL 写法：
  SELECT * FROM sales_data   （用文件名，不含扩展名）

错误 SQL 写法：
  SELECT * FROM my_data       （不是 label！）
```

#### 查看表名的方法

```bash
# Schema 采集后会列出 mapping，如：
# DB1 → my_data  → csv:///...sales_data.csv
#                    ↑最后一段 .csv 前的部分就是表名
dbexplain
```

**规则**：文件名（不含 `.csv` / `.tsv` / `.xlsx`）才是 SQL 中的表名。`--label` 只是用来选择数据源的参数。

---

### 2.3 多 DSN 缺 --label

**现象**：
```
ERROR: 2 DSNs matched — use --label to select one:
  --label sales_data (csv:///...sales_data.csv)
  --label org_info   (csv:///...org_info.csv)
```

**根因**：`.env.dbexplain` 配置了多个数据源，但 `execute` 命令没有 `--label`。

**修复**：输出中已列出可用 label，直接选取所需：
```bash
dbexplain execute --label sales_data 'SELECT *' --limit 5 --human
```

如果仍不确定，先运行 `dbexplain` 查看完整 mapping。

---

### 2.4 文件路径错误

**现象**：
```
file not found: /sales_data.csv (use absolute path)
```
或：
```
Instances (0)
```

**根因**：DSN 中的文件路径使用了相对路径。`csv:///file.csv` 中的路径是绝对路径（从根 `/` 开始），相对路径无法解析。

**修复**：用绝对路径：

```bash
# 错误：相对路径
echo 'DB1=csv:///sales_data.csv?label=data' > .env.dbexplain
# 实际解析为 /sales_data.csv → 找不到文件

# 正确：绝对路径
echo 'DB1=csv:///home/user/data/sales_data.csv?label=data' > .env.dbexplain
```

---

### 2.5 重试卡死协议

#### 如果同一命令重复 3 次仍报错

```bash
# 不要在相同命令上反复重试！
# 每次重试都是完全相同的输入 → 必然得到完全相同的输出

# 正确做法：
1. 停止重试
2. 打开本排障指南，根据错误类型找到对应章节
3. 理解根因后重新构造命令
4. 如仍无法解决，换用简单 SELECT * LIMIT 5 确认工具本身可用
```

#### 常见无效重试模式

| 模式 | 问题 |
|------|------|
| 同一命令跑 10 次 | 每次结果一样，浪费时间 |
| 只改 SQL 内容不改引号结构 | 引号结构不改，parse error 必然复现 |
| 说"改用单引号"但实际命令不变 | agent 的修正意图和执行不一致 |

**核心原则**：一次 error 后，必须**修改命令本身**才能期待不同结果。

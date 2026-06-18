# dbexplain v0.1.9 发布：AI 智能问数的数据库连接层，连接 GaussDB 不再是问题

> 大模型 Agent 再聪明，连不上数据库就是废物。v0.1.9 +  v0.1.8，带来 GaussDB 双驱动原生 SHA256 认证、`check` 并发流式检测、`-env` 彻底移除——每一次发布都在解决"让数据库能被 AI 可靠访问"这件事。

---

![dbexplain 架构全景](../assets/DBEXPLAIN-ARCH.png)

---

## 太长不看版

v0.1.9 合并 v0.1.8 + v0.1.9 两个版本的变化，按对 AI Agent 集成的影响排列：

| 影响 | 变更 | 说明 |
|------|------|------|
| 🔴 **你要知道** | `-env` 参数已移除 | 所有子命令不再接受 `-env`，自动加载 `.env.dbexplain` |
| 🔴 **你要知道** | `dbexplain <flags>` 不再采集 | 必须显式 `dbexplain collect -dsn '...'` |
| 🟢 **AI 集成** | GaussDB 双驱动架构 | gaussdb-go 原生支持华为 SHA256/SM3，28P01 认证失败彻底解决 |
| 🟢 **AI 集成** | `check` 并发流式检测 | 多 DSN 场景从串行改并发，结果按完成顺序逐行打印，不再等全部跑完 |
| 🟢 **AI 集成** | 默认超时 10s → 20s | GaussDB 等慢连接场景更稳妥 |
| 🔧 **内部** | MySQL charset 回归修复 | go-sql-driver v1.10.0 升级导致 charset=utf8mb4 报错已修复 |
| 🔧 **内部** | 日志统一 `dbexplain.log` | execute 所有子路径不再创建零散的 `<label>.log` |

---

## AI 智能问数，第一步永远是"连上数据库"

2026 年的今天，每个做 AI Agent 的团队都想实现"智能问数"——用户说一句话，AI 自动查数据库返回结果。

但现实是：**Agent 再聪明，连不上数据库就是废物**。

过去几个月我们服务了多个 AI Agent 团队，发现他们的 P0 阻塞根本不是 NL2SQL 的准确率，而是：

- GaussDB 连不上（28P01 认证失败）
- 配置了 20 个数据源，check 一遍要等 5 分钟
- 运维团队不知道工具该用哪个参数
- `.env` 文件改了但工具加载的是旧的

v0.1.9 就是冲着这些真实痛点来的。

---

## 1. 🔴 BREAKING: `-env` 参数彻底移除

### 背景

`-env` 参数的历史：在 v0.1.5 之前，`-env` 是必传参数——你不告诉工具配置文件在哪，它就不加载。v0.1.6 开始支持自动发现 `.env.dbexplain`，`-env` 变成可选。到 v0.1.8，自动发现已经覆盖了 100% 的使用场景，`-env` 完全没有存在的必要了。

### 影响范围

7 个源码文件 + 38 个文档文件，~200 处 `-env` 引用全部清除。

### 迁移

```bash
# ❌ 旧写法 (v0.1.7 及之前)
dbexplain check -env /path/to/.env
dbexplain collect -dsn '...' -env /path/to/.env

# ✅ 新写法 (v0.1.9)
dbexplain check                          # 自动发现 .env.dbexplain
dbexplain collect -dsn '...'             # 自动发现 .env.dbexplain
```

工具现在自动按以下优先级查找配置文件：
1. 当前目录 `.env.dbexplain`
2. 当前目录 `.env.dbexplain.enc`（加密）
3. `~/.config/dbexplain/.env.dbexplain`
4. `~/.config/dbexplain/.env.dbexplain.enc`

> **诚实地说**：这是一个破坏性变更。如果你在 CI/CD 脚本里传了 `-env`，需要去掉它。但我们相信这是正确的方向——少一个参数，少一种出错的可能。

---

## 2. 🔴 BREAKING: `dbexplain <flags>` 不再触发采集

### 背景

很久以前（v0.0.x），dbexplain 只有一个命令：`dbexplain -dsn '...'`。后来加了子命令体系（`collect`、`check`、`execute` 等），但 `dbexplain -dsn '...'` 不带子命令时仍然兼容地走采集路径。

v0.1.9 正式移除了这个降级路径：

```bash
# ❌ 不再支持
dbexplain -dsn 'postgres://...'

# ✅ 必须写子命令
dbexplain collect -dsn 'postgres://...'
```

### 好处

- 约 240 行死代码从 `main.go` 移除（文件从 ~940 行缩减至 ~680 行）
- `dbexplain version` 现在是合法子命令
- `--version` 保留向后兼容

> **诚实地说**：这事从 v0.1.2 就该做了，拖到了现在。如果你有脚本在跑裸 `dbexplain -dsn`，需要改成 `dbexplain collect -dsn`。

---

## 3. 🟢 AI 集成 P0: GaussDB 双驱动架构

### 问题：GaussDB 连不上

GaussDB 在国内企业市场占有率越来越高，尤其是金融、运营商、政务行业。但 GaussDB 的认证协议和标准 PostgreSQL 不一样——华为定制了 SHA256 算法（`password_encryption_type=1`），甚至支持国密 SM3（`password_encryption_type=2`）。

标准 PostgreSQL 驱动（`lib/pq` 和 `pgx/v5`）不支持这些算法，导致 GaussDB 用户从第一天就遇到 28P01 认证失败。

### 解决方案：双驱动架构

v0.1.9 引入了**双驱动架构**：

```
PostgreSQL 连接器 → pgx/v5 (标准 PG 驱动，注册为 "postgres")
GaussDB    连接器 → gaussdb-go (华为官方 pgx 分支，注册为 "gaussdb")
```

`gaussdb-go` 是华为云官方维护的 pgx 分支（v1.0.0-rc1），原生支持：
- **password_encryption_type=1**（华为定制 SHA256）
- **password_encryption_type=2**（SM3 国密算法）
- 同时兼容标准 MD5 和 SCRAM-SHA-256

### 技术细节

更关键的是，`gaussdb-go` 不识别 `postgres://` URI scheme。以前 `buildPGDSN()` 输出的 `postgres://user:pass@host:port/db` 传给 gaussdb-go，它直接丢弃 URL 回退到 Unix socket（`/tmp/.s.PGSQL.5432`）和 OS 默认用户（root）：

```
gaussdb-go 收到 postgres://u:p@host:5432/db
  → "我不认识 postgres://"
  → 回退到 /tmp/.s.PGSQL.5432 (Unix socket)
  → 以 root 用户登录
  → 认证失败 (28P01)
```

修复方案：**DSN 构建完全分离**。

| 连接器 | DSN 构建函数 | 输出 Scheme |
|--------|-------------|-------------|
| PostgreSQL | `buildPGDSN()` | `postgres://` |
| GaussDB | `buildGaussDBDSN()` | `gaussdb://` |

`collectPGDB()` / `executeSQLQuery()` 等包级函数继续共享，零代码重复。

### 验证

```bash
# GaussDB 连接（以前报 28P01，现在应成功）
dbexplain collect -dsn 'gaussdb://user:pass@host:5432/mydb?label=my-gauss' --human

# GaussDB Oracle 兼容模式
dbexplain collect -dsn 'gaussdb://user:pass@host:5432/mydb?oracleCompatible=true&label=my-gauss-ora' --human

# PostgreSQL 保持不变
dbexplain collect -dsn 'postgres://user:pass@host:5432/mydb?label=my-pg' --human
```

> **诚实地说**：`gaussdb-go` 目前是 v1.0.0-rc1（预发布版本），不是正式版。我们会在生产环境持续跟进华为的正式发布。但它确实解决了此前完全无法连接的问题。

---

## 4. 🟢 AI 集成 P1: `check` 并发流式检测 + 默认超时 20s

### 以前的问题

假设你管理 20 个数据源（10 个 MySQL、5 个 PG、3 个 Redis、2 个 GaussDB），想检查所有连接是否正常：

```bash
dbexplain check
```

在 v0.1.8 及之前，这是**串行**的——一个接一个检查。GaussDB 慢连接可能要等 10s+，20 个 DSN 全部跑完需要 1-2 分钟。而且结果是一次性批量打印的，你只能干等。

### 现在

v0.1.9 将所有 DSN 检测改为 **goroutine 并发执行**，结果按完成先后顺序**逐行流式打印**：

```
No. EnvKey Label              Kind       Host:Port             Syntax  Connect
── ────── ─────────────────── ────────── ──────────────────── ─────── ───────────────
1   DB1   my-redis            redis      10.0.0.1:6379         ✅ OK   ✅ OK 3ms
2   DB3   my-sqlite           sqlite     /tmp/test.db          ✅ OK   ✅ OK 5ms
3   DB2   my-mysql            mysql      10.0.0.2:3306         ✅ OK   ✅ OK 42ms
4   DB5   my-gauss            gaussdb    10.0.0.5:5432         ✅ OK   ✅ OK 156ms
...
```

快的先出结果，慢的不阻塞快的。20 个 DSN 从 ~90 秒降到 ~20 秒（受最慢的那个限制）。

### 默认超时 10s → 20s

GaussDB 等慢连接的首次握手机制较复杂，10s 经常不够。改为 20s 后，大多数慢连接都能正常完成。`--json` 模式保持完整收集后一次性输出（JSON 需要完整文档结构）。

---

## 5. 🔧 MySQL charset 回归修复

### 这个问题是怎么产生的

v0.1.8 升级了 `go-sql-driver/mysql` 从 v1.9.x 到 v1.10.0。v1.10.0 的 `handleParams()` 函数会遍历 `cfg.Params` 中的所有参数，向 MySQL 发送 `SET key = value`。

我们的 DSN 里有 `charset=utf8mb4`，被当成了系统变量发送：

```
SET charset = 'utf8mb4'
```

MySQL 不认识 `charset` 这个系统变量，报错：

```
Error 1193 (HY000): Unknown system variable 'charset'
```

### 修复

一行改动：把 `charset` 和 `parseTime` 从 `cfg.Params` 移到对应的专有结构体字段。

```go
// 修复前（v1.10.0 开始不再兼容）
cfg.Params["charset"] = "utf8mb4"

// 修复后
// charset 通过 handshake collation 协商，不显式设置
cfg.ParseTime = true  // 用专有字段
```

字符集现在通过驱动默认的 handshake collation（`utf8mb4_general_ci`）自动协商，不需要手动指定。

---

## 6. 📋 日志统一 + SKILL 文档升级

### execute 日志统一

此前 `execute` 的 5 个子路径各自创建独立的 `<label>.log` 文件，`/var/log/dbexplain/` 目录下一堆零散日志。现在全部统一写入 `dbexplain.log`，每行 `[label=X] [kind=Y]` 前缀区分实例。

### SKILL 文档场景化重组

`dbexplain-skill` 是我们提供给 LLM Agent 的"使用说明书"——告诉 AI 怎么调用 dbexplain。v0.1.8 对它做了全面升级：

- 按 P0/P1/P2 优先级重组：数据库巡检/表结构/连通性放最前，Diff/联邦/文件分析靠后
- 连通性检查升级为独立工作流步骤（P0 场景，先于 Schema 采集执行）
- Schema Diff 新增为 P1 场景
- 触发词从 11 个扩展至 16 个

**意义**：AI Agent 调用 dbexplain 时，优先做最重要的事——先检查连通性，再采集 Schema，最后执行查询。

---

## 7. 📊 版本演进

```
v0.0.2: 5 种数据源起步
v0.1.0: 9 种，CapSQL + 文件查询引擎
v0.1.3: + DuckDB，双版本构建
v0.1.4: + Prometheus 时序数据库
v0.1.5: + Oracle + Hive，15 种，六层安全管道
v0.1.6: Prometheus DSL 升级 + Bug Bash 21 项修复
v0.1.7: Schema 采集 100× 加速 + check 子命令 + CTE 写检测 + GaussDB Oracle 兼容
v0.1.8: -env 彻底移除 + SKILL 文档升级 + MySQL charset 修复（未单独发布）
v0.1.9: GaussDB 双驱动架构 + check 并发流式检测 + 默认超时 20s
```

**dbexplain — 16 种异构数据源的确定性上下文编译器。Schema 采集、只读查询、联邦 JOIN、安全审计、连通性检测，All in one 单二进制，零外部依赖。**

---

## 8. 快速试用

```bash
# 1. GaussDB 连接验证（SHA256 认证）
dbexplain check -dsn 'gaussdb://user:pass@host:5432/mydb?label=my-gauss'

# 2. 多数据源并发连通性检查
dbexplain check

# 3. GaussDB Schema 采集
dbexplain collect -dsn 'gaussdb://user:pass@host:5432/mydb?label=my-gauss' --human

# 4. PostgreSQL Schema 采集（不受影响）
dbexplain collect -dsn 'postgres://user:pass@host:5432/mydb?label=my-pg' --human

# 5. 联邦查询：GaussDB + MySQL + CSV
dbexplain execute --dsl "
  SELECT g.instance, g.status, m.product, c.region
  FROM @my-gauss.alert g
  JOIN @my-mysql.iplist m ON g.hostip = m.hostip
  JOIN @my-csv.nodes c ON g.hostip = c.ip
" --human
```

---

## 写在最后

v0.1.9 是 dbexplain 在"让数据库能被 AI 可靠访问"这件事上的又一次推进。

双驱动架构说起来简单（换一个 driver 而已），但实际涉及 DSN 构建、认证协议、EXPLAIN 格式、连接池管理、错误处理的全链路分离。一个 `gaussdb://` 和 `postgres://` 的区别，背后是两套独立驱动的完整适配。

`-env` 移除和 `check` 并发看起来是"删代码"和"改并发"，但它们解决的是同一个问题：**让工具更简单、更快、更不容易出错**。
AI Agent 集成 dbexplain 的时候，少一个参数就少一个出错点，快一秒就少一次超时。

> 做工具就像修路——路修好了，车才能跑得快。dbexplain 不是那辆车（AI Agent），我们是那条路。

---

*项目开源协议：Apache 2.0*
*版本：v0.1.9 (2026-06-18)*
*GitHub: [github.com/IamWWT/dbexplain](https://github.com/IamWWT/dbexplain)*
*发布 Assets: 10 个 tarball 覆盖 5 平台标准版 + linux-amd64 DuckDB 版*


# dbexplain v0.0.7 发布：只读查询执行 + Go 模块化 + 全链路安全审计

> 零依赖、9 种数据库的 AI Agent 上下文生成工具。
> 支持 MySQL / PostgreSQL / GaussDB / ClickHouse / SQLite / Redis / MongoDB / Elasticsearch / Qdrant。

---

v0.0.7 核心亮点：**9-DB 只读查询执行引擎**、**Go 模块化公共 API**、**全链路密码安全审计与修复**、**Schema 外键补全**、**231+ 测试用例覆盖**。

---

## 重磅功能：只读查询执行引擎

v0.0.7 新增 `dbexplain execute` 子命令，在沙箱保护下执行只读查询，输出结构化数据表。与 schema 采集模式完全分离，专为 AI Agent 验证假设、检查数据而设计。

### 三层层层防护

**第一层：sqlguard 动词白名单**
```
允许: SELECT, EXPLAIN, WITH, SHOW, DESCRIBE, DESC, PRAGMA
拒绝: INSERT, UPDATE, DELETE, DROP, ALTER, CREATE, TRUNCATE, GRANT, REVOKE 等 17 种写操作
```

**第二层：多语句检测**
```
拒绝: SELECT 1; DROP TABLE users  → READ_ONLY_VIOLATION
禁止分号拼接，防止 SQL 注入逃逸
```

**第三层：自动 LIMIT 注入**
```
输入: SELECT * FROM huge_table
输出: SELECT * FROM huge_table LIMIT 1000
无 LIMIT 时自动追加，防止全表扫描
```

### 全部 9 种数据库支持

```bash
# SQL 数据库（走 sqlguard 校验）
dbexplain execute -env --label shop-db 'SELECT COUNT(*) FROM orders'
dbexplain execute -env --label my-pg --explain 'SELECT * FROM orders WHERE user_id=42'

# Elasticsearch（通过 _sql REST 端点）
dbexplain execute -env --label es-test 'SHOW TABLES'

# MongoDB（JSON 格式）
dbexplain execute -env --label mongo '{"find":"users","filter":{"age":{"$gt":18}},"limit":100}'

# Redis（原生命令，30+ 命令白名单）
dbexplain execute -env --label redis 'GET user:1001'
dbexplain execute -env --label redis 'HGETALL session:abc'

# Qdrant（JSON 格式）
dbexplain execute -env --label qdrant '{"count":"documents"}'

# 人类可读表格输出（--human）
dbexplain execute -env --db 1 --human 'SELECT * FROM users LIMIT 5'
```

默认输出为 JSON（供 AI Agent 消费），加上 `--human` 后切换为 ASCII 表格：

```
+----+------+-------------------+
| id | name | email             |
+----+------+-------------------+
| 1  | Alice| alice@example.com |
| 2  | Bob  | NULL              |
+----+------+-------------------+
2 row(s) in set (1.2ms)
```

### 安全机制

| 防护层 | 机制 |
|--------|------|
| 动词校验 | sqlguard 白名单（SQL 类）/ 连接器内部白名单（非 SQL 类） |
| 多语句禁止 | 分号分割检测，false-positive 保守拒绝 |
| 自动 LIMIT | SELECT 无 LIMIT 追加 `LIMIT 1000` |
| 并发互斥 | per-label TryLock，同一实例同时只一个查询 |
| 双超时 | 应用 context + 数据库语句超时（MySQL max_execution_time / PG statement_timeout / CH max_execution_time） |
| 凭证保护 | 查询结果 JSON 不包含任何连接信息或密码 |

---

## Go 模块化发布：新go项目可直接 import

v0.0.7 将项目从 `understand_dbs_skills` 正式更名为 `dbexplain`，模块路径从 `module dbexplain` 升级为 `module github.com/IamWWT/dbexplain`，符合 Go 模块规范。新建 `src/core/` 包，导出三个公共 API：

```go
import "github.com/IamWWT/dbexplain/core"

// 采集到 Universe（完整 schema）
universe, err := core.Collect(ctx, dsn)

// 采集到 IR Graph（节点 + 列 + 边）
graph, err := core.CollectToGraph(ctx, dsn)

// 采集到 JSON（一次调用全搞定）
jsonBytes, err := core.CollectToJSON(ctx, dsn)
```

VeinMap 等 Go 项目现在可以直接 import 调用，消除进程启动和 JSON 序列化开销。

---

## Schema 增强：外键补全 + JSON refs 结构化

### ForeignKey OnDelete / OnUpdate

```json
{
  "name": "fk_user_id",
  "on_delete": "CASCADE",
  "on_update": "RESTRICT"
}
```

SQLite 已有数据源（`PRAGMA foreign_key_list`）；MySQL 新增 `information_schema.REFERENTIAL_CONSTRAINTS` 查询；PostgreSQL 新增 `pg_constraint.confupdtype/confdeltype` 解析。IR Graph 构建器（`BuildGraph()`）同步在 Edge Metadata 中输出 `constraint_name` / `on_delete` / `on_update`。

### JSON refs 8 个结构化字段

```json
{
  "from_instance": "shop-db", "from_db": "mydb", "from_table": "orders", "from_col": "user_id",
  "to_instance": "shop-db",   "to_db": "mydb", "to_table": "users",  "to_col": "id",
  "from": "shop-db/mydb.orders(user_id)",   // 向后兼容
  "to": "shop-db/mydb.users(id)"
}
```

---

## 安全增强：凭证脱敏全面修复

### Redacted() 重写

旧版 `strings.Replace` 方式存在严重安全漏洞——URL 编码密码（如 `%23` 替代 `#`）无法正确匹配，导致密码泄露。v0.0.7 完全重写 `Redacted()` 为位置解析法：

```
原始 DSN: redis://default:Pwd1Open2%23IMD@host:6389/0
脱敏后:   redis://{dbuser}:{dbpassword}@host:6389/0
```

同时用户名也被视为敏感信息，统一用 `{dbuser}:{dbpassword}` 占位符替代旧的 `user:***` 格式。

### 新增 `dbexplain list` 子命令

零凭证暴露，列出所有已配置数据库的映射表：

```
INDEX  LABEL         KIND       HOST:PORT        DATABASE
1      shop-db       mysql      192.168.0.1:3306  shop
2      my-redis      redis      192.168.0.2:6379  cache
3      my-pg         postgres   192.168.0.3:5432  warehouse
```

加密的 `.env` 文件自动解密，但绝不会暴露 DSN 连接串或密码。配合 `--db N` 或 `--label <name>` 使用，完美解决加密后"不知道哪个编号对应哪个库"的问题。

### `-env` DSN 映射摘要

采集前打印脱敏映射，用户一眼确认对应关系：
```
DB1 → shop-db (mysql://{dbuser}:{dbpassword}@192.168.0.1:3306/shop)
DB2 → my-redis (redis://{dbpassword}@192.168.0.2:6379)
```

---

## Bug 修复

- **SQLite INTEGER PRIMARY KEY nullable**：`notnull == 0` → `notnull == 0 && pk == 0`，自增主键不再误标为 nullable
- **日志目录回退**：`/var/log/dbexplain` 不可写时自动回退到 `$XDG_STATE_HOME` → `$HOME/.local/state` → `os.TempDir()`，解决容器/非特权用户环境
- **全链路密码审计**：审查 8 个 connector + render.go + main.go 全部输出路径，确认 JSON、label、日志、-context 全链路零密码泄露

---

## 测试体系：231+ 用例零失败

| 测试层 | 用例数 | 覆盖 |
|--------|--------|------|
| L1 静态分析 | 8 | go build + vet + test + 交叉编译 |
| L2 单元测试 | 120 | dsn:33 + schema:44 + sqlguard:28 + query:15 |
| L3 功能集成 | 29 | 全部 CLI 参数 + 输出格式 |
| L4 端到端 | 1 | 9 异构数据源 |
| L5 Bug 回归 | 1 | 全版本 |
| L6 加密专项 | 30 | 加密/解密/密码模式/交叉编译 |
| L7 CLI 子命令 | 45 | execute/list/DSN 映射/手册/非 SQL |

**sqlguard 28 用例**：Validate() 全部动词白名单/黑名单、多语句边界、空查询、空白前导、括号 CTE；AutoLimit() 追加/跳过/尾部分号/大小写检测。

**query 15 用例**：QueryLock 加锁/解锁/并发互斥/多标签独立/重入验证/规模测试。

---

## 其他更新

### [v0.0.6] 配置加密（上一版本回顾）

如果错过了 v0.0.6，这一版的核心能力仍然有效：

```bash
# 机器指纹加密（默认，无需密码）
dbexplain encrypt ~/.config/dbexplain/.env.dbexplain
rm ~/.config/dbexplain/.env.dbexplain

# 加密后无需任何环境变量，直接运行
dbexplain -env
```

加密算法 XChaCha20-Poly1305，纯 Go 实现，加密文件仅能在原机器解密。

---

## 快速上手

```bash
# 安装
git clone https://github.com/IamWWT/dbexplain.git
cd dbexplain
bash db-relationship-explainer/scripts/install.sh

# 创建配置
mkdir -p ~/.config/dbexplain
cat > ~/.config/dbexplain/.env.dbexplain << 'EOF'
DB1=mysql://root:pass@127.0.0.1:3306/mydb?label=my-mysql
DB2=redis://:pass@127.0.0.1:6379/0?label=my-redis
DB3=postgres://user:pass@127.0.0.1:5432/mydb?label=my-pg
EOF

# 加密配置（推荐）
dbexplain encrypt ~/.config/dbexplain/.env.dbexplain
rm ~/.config/dbexplain/.env.dbexplain

# 列出已配置数据库
dbexplain list -env

# 采集 schema
dbexplain -env

# 只读查询
dbexplain execute -env --label my-mysql 'SELECT COUNT(*) FROM orders'

# 人类可读表格（替代 JSON）
dbexplain execute -env --label my-mysql --human 'SELECT * FROM orders LIMIT 5'

# JSON 输出
dbexplain -env -json

# AI 上下文
dbexplain -env --context ./ai-context/

# 查看手册
dbexplain all
dbexplain mysql

# 子命令一览
dbexplain encrypt <file>      # 加密配置
dbexplain list                # 列出数据库
dbexplain execute <query>     # 只读查询
dbexplain <dbtype>            # 数据库手册
dbexplain all                 # 完整手册
```

---

## 资源链接

- **GitHub**: [IamWWT/dbexplain](https://github.com/IamWWT/dbexplain)
- **完整变更**: [CHANGELOG.md](https://github.com/IamWWT/dbexplain/blob/main/CHANGELOG.md)
- **测试报告**: [docs/TEST_v0.0.7.md](https://github.com/IamWWT/dbexplain/blob/main/docs/TEST_v0.0.7.md)（231+ 用例，零失败）
- **查询执行安全文档**: [docs/EXECUTE.md](https://github.com/IamWWT/dbexplain/blob/main/docs/EXECUTE.md)
- **CLI 查询案例库**: [docs/CLI_EXAMPLES.md](https://github.com/IamWWT/dbexplain/blob/main/docs/CLI_EXAMPLES.md)（7 数据源 13 条实测命令）

---

*dbexplain — Make Databases AI-Readable.*

# MongoDB 连接认证与安全采集机制详解（含排障指南）

本文档说明 `dbexplain` 工具中 MongoDB 连接器（`connector/mongo.go`）的核心实现机制，并汇总身份验证失败的常见原因及排障方法。

---

## 一、代码中的重要机制

### 1.1 连接建立与强制认证

```
clientOpts := options.Client().
    ApplyURI(d.Raw).
    SetTimeout(10 * time.Second). // CSOT 统一超时
    SetRetryReads(false).
    SetRetryWrites(false)
```

- **强认证要求**：必须显式指定 `authSource`，否则驱动可能使用默认的 `admin` 库认证，导致 `AuthenticationFailed`。
- **库名必填**：`mongo.go` 中会校验 `d.DBName` 是否为空，若未提供数据库名（例如 DSN 中不包含路径部分）则直接返回错误，要求用户明确指定库名。
- **禁用重试**：所有读取、写入重试均已关闭，避免因网络抖动导致驱动内部无限重试而“卡住”程序。
- **统一超时**：使用 `Client.SetTimeout` 覆盖整个连接的生命周期，确保不会因 MongoDB 无响应导致采集无限挂起。

### 1.2 只读安全策略（零数据风险）

- **只获取集合名称**：调用 `ListCollectionNames`，不涉及任何文档数据。
- **只获取近似文档数**：使用 `EstimatedDocumentCount`（基于元数据），不扫描实际文档。
- **不采样任何文档内容**：表的列仅包含一个虚拟主键 `_id`，类型为 `objectId`，注释为“mongodb document primary key”。没有任何 `find` 或文档扫描操作。
- **连接即断开**：采集完成后立即执行 `client.Disconnect`（带独立超时），释放连接资源。

### 1.3 进度与错误处理

- **日志注入**：通过 `connector/logf` 从上下文获取 logger，每个步骤（连接、Ping、集合列表、单个集合统计）都会输出日志到 `logs/<label>.log`。
- **统一错误类型**：所有错误均包装为 `schema.NewDBError`，包含脱敏 DSN、数据库名、操作名称，方便定位。
- **独立超时派生**：Ping 和 ListCollections 使用从上下文派生的短超时（5s / 10s），不干扰全局超时设置。

### 1.4 集合元数据采集

```
func collectMongoCollectionMeta(...) *schema.Table {
    // 仅获取近似文档数，不采样文档
    coll.EstimatedDocumentCount(estCtx)
    // 构造虚拟表结构
    t.Columns = append(t.Columns, &schema.Column{Name: "_id", Type: "objectId", ...})
}
```

- 每个集合生成一个 `schema.Table`，引擎类型为 `WiredTiger`。
- `RowCount` 来自 `EstimatedDocumentCount` 的近似值，非精确计数，避免慢查询。
- 表注释为空，列注释为固定字符串，清晰表明当前只展示集合元数据。

---

## execute 只读查询

`dbexplain` 提供 `execute` 子命令，支持对 MongoDB 实例执行只读查询，以 JSON 格式描述查询意图，安全地将结果以表格形式输出到终端。

### 查询格式

JSON 格式，支持两种操作：
- **find**：`{"find":"collection","filter":{...},"limit":100}`
- **aggregate**：`{"aggregate":"collection","pipeline":[...]}`

### 校验机制

- **内置操作白名单**：只允许 `find` 和 `aggregate` 两种操作，且每个 JSON 请求必须精确指定其一。
- **拒绝其它操作**：任何包含 `insert`、`update`、`delete`、`drop` 等操作的请求将被拒绝。
- **注意**：MongoDB 校验不经过 `sqlguard` 模块，使用内部独立的白名单校验。

### 自动 LIMIT 追加

- 若 JSON 中未显式指定 `limit` 字段，工具使用 `--limit` 标志值（默认 1000）自动填充。
- 对 `aggregate` 请求，工具在 pipeline 末尾自动追加 `{"$limit": N}` 阶段，确保聚合结果可控。

### 超时控制

通过 MongoDB Go 驱动的上下文超时（Go `context.WithTimeout`）控制整体执行时长。

### 执行方式

使用 `mongo-go-driver` 的 `Find` 或 `Aggregate` 方法，配合 BSON 解码，将文档结果展平输出。

### 输出格式

BSON 文档的键值对被展平为行/列结构，每个字段映射为一列，便于终端表格呈现。

### 最大行数控制

由 `--limit` 命令行标志控制，默认值为 1000。

---

## 二、常见认证错误与排障

### 2.1 典型错误信息

```
AuthenticationFailed: auth error: sasl conversation error: 
unable to authenticate using mechanism "SCRAM-SHA-1"
```

**根本原因**：`authSource` 参数与用户实际创建的数据库不一致。

### 2.2 核心概念

| 参数 | 说明 |
|------|------|
| `authSource` | 用户凭据存储的数据库名称（通常 root 用户在 `admin`，应用用户可能在业务库）。 |
| `label` | 工具别名，仅用于终端显示，不影响连接。 |
| 密码中的特殊字符 | 在 DSN 的查询字符串中需要 URL 编码（如 `#` → `%23`）。命令行中可使用单引号包裹。 |

### 2.3 排障流程

#### ① 确认是否启用认证
```bash
mongosh "mongodb://localhost:27017"
```
无报错则未启用认证，直接使用无密码 DSN；若提示需要认证，继续排查。

#### ② 获取用户信息（Docker 环境）
```bash
docker inspect <容器名> | grep -A 20 MONGO_INITDB
```
查找 `MONGO_INITDB_ROOT_USERNAME`、`MONGO_OPENIM_USERNAME` 等环境变量。

#### ③ 容器内测试登录
```bash
docker exec -it <容器名> mongosh --username <user> --password '<pass>' --authenticationDatabase <候选库>
```
依次尝试 `admin`、`业务库名`、`openim_v3` 等，直到成功。

#### ④ 以 root 身份确认用户归属
```js
use admin
db.system.users.find()
```
或
```js
use 业务库
db.getUsers()
```
输出的 `db` 字段即为正确的 `authSource`。

---

## 三、解决方案

### 3.1 正确构造 DSN

确保连接字符串中包含 `authSource=<用户所在库>` 且库名不能省略。

**错误示例**：
```
mongodb://openIM:Pwd1Open2#IMD@localhost:27017/?authSource=admin
```
**正确示例**：
```
mongodb://openIM:Pwd1Open2%23IMD@localhost:27017/openim_v3?authSource=openim_v3&label=mongo-test
```

### 3.2 命令行参数写法

密码中的 `#` 等字符需 URL 编码，或者使用单引号包裹整个 DSN：
```bash
./dbexplain -dsn 'mongodb://openIM:Pwd1Open2%23IMD@localhost:27017/openim_v3?authSource=openim_v3&label=mongo-test'
```

---

## 四、核心命令速查

| 目的 | 命令 |
|------|------|
| 容器内无认证连接 | `docker exec -it mongo mongosh` |
| 容器内带认证测试 | `docker exec -it mongo mongosh --username <user> --password '<pass>' --authenticationDatabase <db>` |
| 查看初始化环境变量 | `docker inspect mongo \| grep -A 20 MONGO_INITDB` |
| root 登录 | `docker exec -it mongo mongosh --username root --password '<pass>' --authenticationDatabase admin` |
| 列出所有用户 | `db.system.users.find()` |
| 查看指定库的用户 | `use <db>; db.getUsers()` |
| 列出非系统数据库 | `show dbs` |

---

## 五、经验总结

1. **`authSource` 是用户元数据所在的库，不是服务器地址**。root 用户通常在 `admin`，应用用户可能在其他库。
2. 遇到认证失败，先进入容器用 `mongosh` 手动测试，排除网络、密码和认证库错误。
3. Docker 环境务必检查 `docker inspect` 中的 `MONGO_INITDB_*` 变量。
4. 特殊字符（`#`、`@`、`!` 等）在 DSN 的查询字符串中必须百分号编码；在命令行中建议用单引号包裹整个 DSN。
5. 生产环境建议创建只读用户，最小权限原则，避免因工具误用导致数据风险。
6. 使用 `label` 参数为实例设置别名，提高多库环境下的可读性。
7. 由于工具只读取集合元数据，连接账号仅需 `listCollections` 和 `estimatedDocumentCount` 权限，无需读写文档的权限。

通过以上代码机制说明和排障指南，即可安全、高效地使用 `dbexplain` 探查 MongoDB 实例。 
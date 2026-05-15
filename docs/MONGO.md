# MongoDB 连接认证排障指南

本文档总结在 `dbexplain` 工具接入 MongoDB 时遇到的身份验证失败问题，以及通用的 MongoDB 连接排障方法。

---

## 1. 常见错误

```
AuthenticationFailed: auth error: sasl conversation error: 
unable to authenticate using mechanism "SCRAM-SHA-1"
```

**根本原因**：客户端使用的认证数据库（`authSource`）与用户实际创建的位置不匹配。

---

## 2. 核心概念说明

### 2.1 `authSource` —— 认证数据库

- **是什么**：MongoDB 的用户账号存储在某个具体的数据库中，并非全局存在。`authSource` 告诉驱动程序“去哪个库验证这个用户的密码”。
- **为什么需要**：例如 root 用户通常建在 `admin` 库，所以 root 登录时 `authSource=admin`；而应用用户可能建在业务库（如 `openim_v3`），此时必须准确指定 `authSource=openim_v3`，否则驱动默认去 `admin` 验证，导致 `Authentication failed`。
- **在连接串中的写法**：
  ```
  mongodb://user:pass@host:port/?authSource=实际用户所在库
  ```
  *错误示例*：省略 `authSource` 或错误指定为 `admin`。
  *正确示例*：`mongodb://openIM:...@localhost:27017/?authSource=openim_v3`

### 2.2 `label` —— 实例别名（`dbexplain` 工具专用）

- **是什么**：`label` 是本工具 `dbexplain` 提供的一个展示用别名，不影响数据库连接，仅用于终端输出和报告中的显示名称。
- **效果**：不指定 `label` 时，工具显示 `host:port/dbname`；指定 `?label=mongo-test` 后，工具显示 `mongo-test`，便于识别环境。
- **用法示例**：
  ```
  DB5=mongodb://openIM:...@localhost:27017/?authSource=openim_v3&label=mongo-test
  ```
  在报告中会显示：
  ```
  ▸ mongo-test                     mongodb  1 db(s), X tables
  ```

### 2.3 密码中的特殊字符

- 如果密码包含 `#`、`@`、`:` 等 URI 保留字符，必须进行百分号编码。例如 `#` 应写为 `%23`。
- 在命令行测试时，可以用单引号 `'password'` 包裹，避免 shell 解析。

---

## 3. 排障流程

### 3.1 确认 MongoDB 是否启用认证
```bash
# 尝试无认证连接
mongosh "mongodb://localhost:27017"
```
- 如果能连上 → 未启用认证，直接使用无密码连接。
- 如果报错 `Authentication failed` 或 `command requires authentication` → 已启用认证。

### 3.2 获取正确的用户信息（适用于 Docker 环境）
```bash
# 查看容器环境变量，通常包含 root 用户和应用用户信息
docker inspect <容器名> | grep -A 20 MONGO_INITDB
```
关键变量：
- `MONGO_INITDB_ROOT_USERNAME` / `MONGO_INITDB_ROOT_PASSWORD`：root 账户
- `MONGO_OPENIM_USERNAME` / `MONGO_OPENIM_PASSWORD`：应用账户
- `MONGO_INITDB_DATABASE`：应用用户创建的数据库（即 `authSource`）

### 3.3 进入容器内部测试登录
```bash
docker exec -it <容器名> mongosh --username <user> --password '<password>' --authenticationDatabase <db>
```
**示例**：
```bash
# 尝试 admin 库
docker exec -it mongo mongosh --username openIM --password 'Pwd1Open2#IMD' --authenticationDatabase admin

# 尝试 openim 库
docker exec -it mongo mongosh --username openIM --password 'Pwd1Open2#IMD' --authenticationDatabase openim

# 尝试 openim_v3 库（最终成功）
docker exec -it mongo mongosh --username openIM --password 'Pwd1Open2#IMD' --authenticationDatabase openim_v3
```

### 3.4 使用 root 账户确认用户归属
```bash
docker exec -it mongo mongosh --username root --password 'openIM123' --authenticationDatabase admin
```
登录后执行：
```js
use admin
db.system.users.find()
```
或查询特定数据库中的用户：
```js
use openim_v3
db.getUsers()
```
输出会显示用户的 `db` 字段，该字段即为 `authSource`。

---

## 4. 解决方案

### 4.1 正确构造 DSN
确保连接字符串中包含 `authSource=<用户创建数据库>`。

**错误示例**：
```
mongodb://openIM:Pwd1Open2#IMD@localhost:27017/?authSource=admin
```

**正确示例**：
```
mongodb://openIM:Pwd1Open2%23IMD@localhost:27017/?authSource=openim_v3
```

如果希望工具自动探测所有非系统库，可省略数据库路径：
```
mongodb://openIM:Pwd1Open2%23IMD@localhost:27017/?authSource=openim_v3&label=mongo-test
```

---

## 5. 核心命令速查

| 目的 | 命令 |
|------|------|
| 容器内无认证测试 | `docker exec -it mongo mongosh` |
| 容器内带认证测试 | `docker exec -it mongo mongosh --username <user> --password '<pass>' --authenticationDatabase <db>` |
| 查看容器初始化环境变量 | `docker inspect mongo \| grep -A 20 MONGO_INITDB` |
| root 登录 | `docker exec -it mongo mongosh --username root --password '<pass>' --authenticationDatabase admin` |
| 列出所有用户 | `db.system.users.find()` |
| 查看指定数据库的用户 | `use <db>; db.getUsers()` |
| 列出所有非系统数据库 | `show dbs` |

---

## 6. 经验总结

1. **`authSource` 不是服务器地址，而是用户元数据存储的数据库名称**。大部分官方镜像会强制 root 用户在 `admin`，应用用户则可能创建在业务库中。
2. 遇到认证失败，先进入容器用命令行测试，可以快速隔离是网络问题、密码错误还是认证库错误。
3. Docker 环境务必检查 `docker inspect` 输出的环境变量，其中包含了创建用户时的确切参数。
4. 特殊字符（`#`、`@`、`:`）在 URI 中必须百分号编码，但命令行参数中只需用单引号包裹即可。
5. 生产环境建议为用户分配最小权限（如 `readWrite`），并只读连接使用专用账户。
6. 使用 `label` 参数为实例设置别名，可大幅提升多数据库环境下的可读性。

---

通过以上步骤，可快速解决绝大部分 MongoDB 连接认证问题，并将其无缝接入自动化探查工具。
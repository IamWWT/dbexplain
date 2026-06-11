# Hive 数据库连接器

## DSN 格式

```bash
# NOSASL 无认证
hive://host:10000/default?label=my-hive

# LDAP 认证
hive://user:password@host:10000/default?auth=LDAP&label=my-hive

# Kerberos 认证（纯 Go）
hive://host:10000/default?auth=KERBEROS&label=my-hive

# TLS 加密
hive://user:password@host:10000/default?sslcert=/path/cert.pem&sslkey=/path/key.pem&label=my-hive
```

| 项目 | 说明 |
|------|------|
| Scheme | `hive://`（明文）或 `hives://`（TLS） |
| 默认端口 | 10000（HiveServer2） |
| 驱动 | github.com/beltran/gohive/v2（纯 Go，无 CGO） |

## 功能

| 能力 | 支持 | 说明 |
|------|------|------|
| Schema 采集 | ✅ | 通过 DESCRIBE FORMATTED |
| SQL 查询 | ✅ | 通过 gohive database/sql 驱动 |
| EXPLAIN | ✅ | 通过 `EXPLAIN` 前缀 + HiveServer2 |
| 行数统计 | ⚠️ 固定 -1 | 避免触发 MapReduce/Tez 作业 |
| 索引采集 | ❌ | Hive 无传统索引 |
| 外键采集 | ❌ | Hive 无传统约束 |
| 采样 | ✅ | `SELECT * FROM db.table LIMIT 1` |

> **重要**：Hive 行数固定返回 `-1`（unknown）。`SELECT COUNT(*)` 会触发 MapReduce/Tez 作业，不予执行。

## 认证方式

通过 `?auth=` DSN 参数指定认证方式：

| 认证方式 | 说明 | 默认场景 |
|---------|------|---------|
| `NOSASL` | 无认证，原始 Thrift 传输 | 无用户名时默认 |
| `NONE` | SASL PLAIN 用户名密码认证 | 有用户名时默认 |
| `LDAP` | LDAP 认证 | 显式指定 |
| `KERBEROS` | Kerberos 认证（纯 Go） | 显式指定 |

Kerberos 使用 `beltran/gosasl` 纯 Go 实现，**无需安装 libkrb5-dev** 或任何 C 库。

## Schema 采集

使用 HiveServer2 SQL (`DESCRIBE FORMATTED`)，而非 Metastore Thrift API。

### 采集流程

1. `SHOW DATABASES` — 获取所有数据库（跳过 `information_schema`、`sys`、`default`）
2. `SHOW TABLES IN dbname` — 获取每库的表列表
3. `DESCRIBE FORMATTED db.table` — 获取列信息

### DESCRIBE FORMATTED 解析

解析规则：
- 每行 3 列：`col_name`, `data_type`, `comment`
- 跳过以 `#` 开头的行（分区信息、详细信息分隔行）
- 遇到 `# Detailed Table Information` 或 `# Storage Information` 后停止解析列
- 分区列（`# Partition Information` 下面）仍被正确识别为普通列
- 所有列默认可空（`Nullable = true`）

示例输出解析：
```
col_name          data_type          comment
id                int
name              string
dt                string             分区列

# Partition Information
# col_name        data_type          comment
dt                string

# Detailed Table Information
Database:       mydb
...
```

### 采样

使用 `SELECT * FROM db.table LIMIT 1` 获取首行数据，通过规则引擎为无注释字段推断语义注释。

### 列属性

| 属性 | 值 |
|------|-----|
| 可空性 | 全部为 `true`（Hive 默认） |
| 主键 | 无（Hive 无传统主键） |
| 唯一约束 | 无 |
| 索引 | 无 |

## 参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `auth` | 认证方式（NOSASL/NONE/LDAP/KERBEROS） | NOSASL 或 NONE |
| `transport` | 传输模式（binary/http） | binary |
| `http_path` | HTTP 传输模式路径（如 `/hive2`） | 空 |
| `service` | Kerberos 服务名 | hive |
| `sslcert` | SSL 客户端证书路径 | 空 |
| `sslkey` | SSL 客户端密钥路径 | 空 |
| `sslca` | SSL CA 证书路径 | 空 |
| `sslinsecureskipverify` | TLS 时跳过证书验证 | false |

## 已知限制

1. **Metastore 不支持**：使用 HiveServer2 SQL（端口 10000），非 Metastore Thrift API（端口 9083）
2. **行数为 -1**：避免 `SELECT COUNT(*)` 触发 MR/Tez 作业
3. **无约束信息**：Hive 无传统主键/外键/唯一约束
4. **DESCRIBE 依赖**：列信息依赖 HiveServer2 的 `DESCRIBE FORMATTED` 实现
5. **TLS 需文件**：TLS 连接需要提供 `sslcert` + `sslkey` 文件路径
6. **采样触发 MR**：`LIMIT 1` 可能触发 MapReduce 作业（取决于 hive.exec.mode.local.auto 配置）

## 构建要求

Hive 连接器通过 beltran/gohive v2 纯 Go 驱动实现，无 CGO 依赖。包含方式：

```bash
# 包含 Hive
bash build.sh minimal hive

# 联合构建
bash build.sh minimal hive,mysql,postgres

# 全量构建（包含 Hive）
bash build.sh
# 或
bash build.sh prod
```

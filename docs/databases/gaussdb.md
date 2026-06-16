# GaussDB 兼容性指南

> GaussDB（华为高斯数据库）在 `dbexplain` 中的兼容性说明。v0.1.7 实机验证。

---

## GaussDB 概述

GaussDB 兼容 PostgreSQL 协议，通过 `lib/pq` 驱动连接。Schema 采集基于 `pg_catalog` 系统表，与 PostgreSQL 共享采集逻辑。

### DSN 配置

| 项 | 说明 |
|----|------|
| 格式 | `gaussdb://用户:密码@主机:端口/库名?label=别名&sslmode=disable` |
| 默认端口 | 25308 |
| 别名 | `gaussdb`, `opengauss` |
| SSL | 默认 `sslmode=disable`，如需 SSL 配置 `?sslmode=require` |

示例：
```bash
dbexplain -dsn 'gaussdb://user:pwd@192.168.0.1:25308/mydb?label=my-gauss'
```

### Oracle 兼容模式

用户 GaussDB 运行在 Oracle 兼容模式：
- **集中式**: `DBCOMPATIBILITY='A'`
- **分布式**: `DBCOMPATIBILITY='ORA'`

在此模式下，大部分 `pg_catalog` 系统表可用，但存在以下关键差异。

---

## 已验证兼容项（v0.1.7 实机确认）

以下 `pg_catalog` 组件在 GaussDB Oracle 兼容模式下已验证可用：

| 组件 | 状态 | 备注 |
|------|------|------|
| `pg_database` | ✅ 兼容 | `datistemplate` 列可能缺失，自动回退 |
| `pg_namespace` | ✅ 兼容 | 标准 schema 发现 |
| `pg_tables` + `pg_class` + `pg_namespace` | ✅ 兼容 | 使用 `c.oid` 显式 JOIN，跨 schema 同名表安全 |
| `pg_attribute` | ✅ 兼容 | 通过 `pg_class.oid` JOIN 关联 |
| `pg_constraint` | ✅ 兼容 | `contype`, `conkey`, `confkey`, `confupdtype`, `confdeltype` 均可用 |
| `pg_indexes` | ✅ 兼容 | 标准索引视图 |
| `pg_attrdef` | ✅ 兼容 | 默认值表达式查询 |
| `format_type()` | ✅ 兼容 | 类型格式化函数 |
| `col_description()` | ✅ 兼容 | 列注释查询 |
| `obj_description()` | ✅ 兼容 | 表注释查询，需使用 `c.oid` 入参 |
| `pg_get_expr()` | ✅ 兼容 | 默认值表达式反编译（简单常量默认值已验证） |
| `pg_total_relation_size()` | ✅ 兼容 | 表大小查询，需使用 `c.oid` 入参 |
| `pg_stat_user_tables` | ✅ 兼容 | `n_live_tup`, `n_tup_ins/upd/del`, `seq_scan`, `idx_scan` 均可用 |

---

## 已知差异

### `::regclass` 类型转换

| 环境 | 支持情况 |
|------|---------|
| PostgreSQL | `'schema.table'::regclass` 完全支持 |
| GaussDB Oracle 兼容模式 | **不支持**，需使用 `pg_class` + `pg_namespace` 显式 JOIN |
| GaussDB PG 兼容模式 | 部分版本支持，推荐统一使用 JOIN 模式 |

代码中已统一使用 JOIN 替代 `::regclass`，详见 `postgres.go` 列查询。

### pg_stat_user_tables 分布式语义

GaussDB 的 `pg_stat_user_tables` 在**单节点模式**已验证兼容。**分布式模式**下统计语义可能不同：

| 字段 | GaussDB 单节点 | GaussDB 分布式 | 说明 |
|------|---------------|----------------|------|
| `n_tup_ins` | ✅ 已验证 | ⚠️ 待确认 | CN 上可能只返回本地统计 |
| `n_tup_upd` | ✅ 已验证 | ⚠️ 待确认 | 同上 |
| `n_tup_del` | ✅ 已验证 | ⚠️ 待确认 | 同上 |
| `n_live_tup` | ✅ 已验证 | ⚠️ 待确认 | 同上 |

> 注：分布式 GaussDB 在 CN 上查询 `pg_stat_user_tables` 可能只返回该 CN 的本地统计，而非全局聚合值。这是分布式架构的固有限制。

### EXPLAIN 格式

GaussDB Oracle 兼容模式**不支持** `EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)` 中的 `BUFFERS` 选项。自动使用 `EXPLAIN (ANALYZE, FORMAT TEXT)`。

### statement_timeout

`statement_timeout` GUC 在 GaussDB Oracle 兼容模式可能不可用。SET 失败时仅记录日志，查询继续运行（应用层超时兜底）。

### pg_stat_statements 扩展

- GaussDB 可能提供不同的扩展名称或替代方案
- 待确认：`pg_stat_statements` 是否存在
- 替代方案：WDR（Workload Diagnosis Report）通过 `gs_wdr_report()` 和 `dbe_perf` Schema

---

## 连接注意事项

- **驱动**: `github.com/lib/pq`（PostgreSQL 原生驱动）
- **端口**: 默认 25308（PostgreSQL 为 5432）
- **SSL**: 默认 `sslmode=disable`
- **超时**: 连接超时 5 秒，查询超时由应用层控制

---

## 兜底机制

若任一查询在 GaussDB 上失败：
- 工具自动跳过该数据源（不影响其他数据源采集）
- 在日志中记录清晰的跳过原因
- 因子权重自动重新归一化（不影响其他数据源的评分比例）

---

## 状态

| 数据库 | 确认状态 | 最后更新 |
|--------|---------|---------|
| GaussDB 单节点 | ✅ 已验证 | 2026-06-16 |
| GaussDB 分布式 | ⚠️ 部分待确认 | 2026-06-16 |

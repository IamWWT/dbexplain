# GaussDB / TDSQL 兼容性确认清单

> 本文档记录 GaussDB（PostgreSQL 兼容）和 TDSQL（MySQL 兼容）在 `dbexplain` 中的操作语义元数据兼容性注意事项。
>
> **v0.1.7 实机确认**：GaussDB（PG 兼容模式）的 Schema 采集已验证通过，列采集 `::regclass` 问题已修复。

---

## GaussDB（华为，PG 兼容）

### 已验证兼容（v0.1.7 实机确认）

以下 pg_catalog 组件在 GaussDB 上已验证可用：

| 组件 | 状态 | 备注 |
|------|------|------|
| `pg_database` | ✅ 兼容 | 增加 `datistemplate` 列不存在时的回退查询（Oracle 兼容模式） |
| `pg_namespace` | ✅ 兼容 | 标准 schema 发现查询 |
| `pg_tables` + `pg_class` + `pg_namespace` | ✅ 兼容 | 改用 `c.oid` 显式 JOIN 替代 `::regclass`，跨 schema 同名表安全 |
| `pg_attribute` | ✅ 兼容 | 无 `::regclass`，通过 `pg_class.oid` JOIN 关联 |
| `pg_constraint` | ✅ 兼容 | `contype`, `conkey`, `confkey`, `confupdtype`, `confdeltype` 均可用 |
| `pg_indexes` | ✅ 兼容 | 标准索引视图 |
| `pg_attrdef` | ✅ 兼容 | 默认值表达式查询 |
| `format_type()` | ✅ 兼容 | 类型格式化函数 |
| `col_description()` | ✅ 兼容 | 列注释查询 |
| `obj_description()` | ✅ 兼容 | 表注释查询，需使用 `c.oid` 入参而非 `::regclass` |
| `pg_get_expr()` | ✅ 兼容 | 默认值表达式反编译（简单常量默认值已验证） |
| `pg_total_relation_size()` | ✅ 兼容 | 表大小查询，需使用 `c.oid` 入参 |
| `pg_stat_user_tables` | ✅ 兼容 | `n_live_tup`, `n_tup_ins/upd/del`, `seq_scan`, `idx_scan` 均可用 |

### 已知差异

#### `::regclass` 类型转换
- **PostgreSQL**: `'schema.table'::regclass` 语法完全支持
- **GaussDB Oracle 兼容模式**: **不支持**该语法，需使用 `pg_class` + `pg_namespace` 显式 JOIN
- **GaussDB PG 兼容模式**: 部分版本支持，但推荐统一使用 JOIN 模式

#### pg_stat_user_tables 字段兼容性

GaussDB 的 `pg_stat_user_tables` 视图字段名和计数器语义在**单节点模式**下已验证兼容。**分布式模式**下留待确认：

| 字段 | PG 原生行为 | GaussDB 单节点 | GaussDB 分布式 | 影响 |
|------|------------|---------------|----------------|------|
| `n_tup_ins` | INSERT 行数 | ✅ 已验证 | ⚠️ 待确认 | 操作热度统计 |
| `n_tup_upd` | UPDATE 行数 | ✅ 已验证 | ⚠️ 待确认 | 写放大检测 |
| `n_tup_del` | DELETE 行数 | ✅ 已验证 | ⚠️ 待确认 | 墓碑行诊断 |
| `n_live_tup` | 存活行数估计 | ✅ 已验证 | ⚠️ 待确认 | 表大小评估 |

> 注：分布式 GaussDB 在 CN 上查询 `pg_stat_user_tables` 可能只返回该 CN 的本地统计，而非全局聚合值。这是分布式架构的固有限制，非 bug。

### pg_stat_statements 扩展

- GaussDB 可能提供不同的扩展名称或替代方案
- 待确认：`pg_stat_statements` 是否存在，或是否需要启用 `track_activities`
- 替代方案：如果 PG 原生扩展不可用，WDR（Workload Diagnosis Report）可作为替代数据源

### WDR 替代方案

GaussDB 提供 WDR（Workload Diagnosis Report）作为内置诊断功能：
- `gs_wdr_report()` 函数生成诊断报告
- `dbe_perf` Schema 下的系统视图
- 待确认：是否可以通过 SQL 查询方式获取类似 `pg_stat_statements` 的归一化查询统计

### 连接注意事项

- **驱动**: 使用 `lib/pq`（PG 原生驱动），已验证在目标 GaussDB 版本上可用
- **SSL**: 默认 `sslmode=disable`，如 GaussDB 要求 SSL 需配置 `?sslmode=require`
- **超时**: `statement_timeout` GUC 在 GaussDB Oracle 兼容模式可能不可用，无超时保护时查询仍会运行

### 兜底机制

若上述任一查询在 GaussDB 上失败：
- 工具自动跳过该数据源（不影响其他数据源采集）
- 在日志中记录清晰的跳过原因
- 因子权重自动重新归一化（不影响其他数据源的评分比例）

---

## TDSQL（腾讯，MySQL 兼容）

### performance_schema 分布式兼容性

| 表/视图 | MySQL 原生行为 | TDSQL 分布式版确认 | 影响 |
|---------|---------------|-------------------|------|
| `table_io_waits_summary_by_table` | 按表统计 I/O | 是否反映全局统计？ | I/O 热点识别 |
| `events_statements_summary_by_digest` | 归一化语句统计 | 分片查询是否汇总？ | 慢查询分析 |
| `file_summary_by_instance` | 文件级 I/O | 分布式存储是否支持？ | 存储诊断 |

### 关键确认点

1. **分片查询汇聚**：在 TDSQL 分布式版本中，`events_statements_summary_by_digest` 是否对所有分片的数据做全局汇总，还是仅返回当前节点的数据？
2. **全局 vs 本地统计**：`table_io_waits_summary_by_table` 在设置 `GLOBAL` 范围时是否返回所有分片的聚合数据？
3. **Proxy 层影响**：TDSQL 的 Proxy 组件是否对查询结果做了透传或聚合？

### 兜底机制

若上述任一查询在 TDSQL 上失败：
- 工具自动跳过该数据源，不影响其他数据源
- 日志记录具体的查询和失败原因

---

## 确认方式

1. 在有 GaussDB 或 TDSQL 实例的环境中运行：
   ```bash
   dbexplain -dsn 'gaussdb://user:pass@host:25308/db'
   dbexplain -dsn 'mysql://user:pass@host:3306/db?label=tdsql-test'
   ```
2. 观察日志中是否有跳过或警告信息
3. 对比同环境下原生 PG/MySQL 的输出，验证统计字段是否存在差异
4. 将确认结果更新到本文档

---

## 状态

| 数据库 | 确认状态 | 最后更新 |
|--------|---------|---------|
| GaussDB | 待确认（需实际环境） | 2026-05-27 |
| TDSQL | 待确认（需实际环境） | 2026-05-27 |

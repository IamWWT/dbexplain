# GaussDB / TDSQL 兼容性确认清单

> 本文档记录 GaussDB（PostgreSQL 兼容）和 TDSQL（MySQL 兼容）在 `dbexplain` 中的操作语义元数据兼容性注意事项。由于这两个数据库在特定环境下的行为可能与原生 PG/MySQL 不同，以下项目需要在实际环境中确认。

---

## GaussDB（华为，PG 兼容）

### pg_stat_user_tables 字段兼容性

GaussDB 的 `pg_stat_user_tables` 视图字段名和计数器语义需要与原生 PostgreSQL 对比确认：

| 字段 | PG 原生行为 | GaussDB 确认 | 影响 |
|------|------------|-------------|------|
| `n_tup_ins` | INSERT 行数 | 待确认 | 操作热度统计 |
| `n_tup_upd` | UPDATE 行数 | 待确认 | 写放大检测 |
| `n_tup_del` | DELETE 行数 | 待确认 | 墓碑行诊断 |
| `n_tup_hot_upd` | HOT UPDATE 行数 | 待确认 | 存储效率 |
| `n_live_tup` | 存活行数估计 | 待确认 | 表大小评估 |
| `n_dead_tup` | 死行数 | 待确认 | 清理压力 |

### pg_stat_statements 扩展

- GaussDB 可能提供不同的扩展名称或替代方案
- 待确认：`pg_stat_statements` 是否存在，或是否需要启用 `track_activities`
- 替代方案：如果 PG 原生扩展不可用，WDR（Workload Diagnosis Report）可作为替代数据源

### WDR 替代方案

GaussDB 提供 WDR（Workload Diagnosis Report）作为内置诊断功能：
- `gs_wdr_report()` 函数生成诊断报告
- `dbe_perf` Schema 下的系统视图
- 待确认：是否可以通过 SQL 查询方式获取类似 `pg_stat_statements` 的归一化查询统计

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

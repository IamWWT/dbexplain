# TDSQL（腾讯，MySQL 兼容）兼容性确认清单

> 本文档记录 TDSQL（MySQL 兼容）在 `dbexplain` 中的操作语义元数据兼容性注意事项。
>
> GaussDB 兼容性请参阅 [gaussdb.md](../gaussdb.md)。

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

1. 在有 TDSQL 实例的环境中运行：
   ```bash
   dbexplain -dsn 'mysql://user:pass@host:3306/db?label=tdsql-test'
   ```
2. 观察日志中是否有跳过或警告信息
3. 对比同环境下原生 MySQL 的输出，验证统计字段是否存在差异
4. 将确认结果更新到本文档

---

## 状态

| 数据库 | 确认状态 | 最后更新 |
|--------|---------|---------|
| TDSQL | 待确认（需实际环境） | 2026-05-27 |

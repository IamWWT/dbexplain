---
name: db-relationship-explainer
description: 零依赖跨数据库结构探查与关系分析，支持 MySQL/PostgreSQL/GaussDB/SQLite/ClickHouse/Redis，自动生成 ER 图与问题报告。
trigger: "解释表结构" "分析数据库关系" "跨库依赖" "生成 ER 图"
tools:
  - path: tools/dbexplain-{platform}
---

## 使用
1. DSN 格式：`scheme://user:pass@host:port/db[?label=alias]`
2. 运行：`./dbexplain -dsn "mysql://..." -dsn "postgres://..." [-json] [-o report.md]`
3. 输出：实例概览 → 表结构卡片 → 关系列表 → 聚类 → 问题诊断

支持多平台静态二进制（含 ARM）。
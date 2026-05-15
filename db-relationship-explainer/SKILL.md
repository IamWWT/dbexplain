---
name: db-relationship-explainer
description: 零依赖跨数据库结构探查与关系分析，支持 MySQL/PostgreSQL/GaussDB/SQLite/ClickHouse/Redis/Qdrant/Elasticsearch/MongoDB，自动生成表卡片、关系图与问题报告。
trigger:
  - "解释表结构"
  - "分析数据库关系"
  - "跨库依赖"
  - "生成 ER 图"
  - "理解数据库"
  - "数据库巡检"
tools:
  - path: tools/dbexplain-{platform}
    description: 静态编译的数据库探查二进制，接受 DSN 列表，输出 Markdown 格式报告。
---

## 执行流程

1. **确认连接信息**：从用户提供的 DSN 或 .env 文件中提取连接串。  
2. **构造命令**：  
   ```bash
   ./tools/dbexplain-{platform} -dsn "scheme://user:pass@host:port/db?label=别名" [-json] [-o report.md]
   ```  
   支持多次 `-dsn` 同时分析多个库，也可用 `-env` 读取 .env 文件。  
3. **执行并捕获输出**：工具只进行**只读**操作，输出 Markdown 表格和关系图。  
4. **解读结果**：根据输出的表卡片、外键关系、聚类和 Issues 向用户解释。

## 支持的 DSN 格式

- MySQL: `mysql://user:pass@host:port/db`
- PostgreSQL/GaussDB: `postgres://user:pass@host:port/db` (或 `gaussdb://`)
- SQLite: `sqlite:///绝对路径`
- ClickHouse: `clickhouse://user:pass@host:8123/db`
- Redis: `redis://:password@host:6379/0`
- Qdrant: `qdrant://:api-key@host:6334`
- Elasticsearch: `elasticsearch://user:pass@host:9200`

## 输出解读

- **实例概览** → 所有连接的数据库及表数量  
- **表结构卡片** → 列名、类型、约束、索引、注释、行数估算  
- **关系** → 显式外键（实线）和推断外键（虚线，基于命名约定）  
- **聚类** → 通过外键关联的表组  
- **问题** → 缺主键、未索引外键、缺审计时间戳等风险

## 安全保证

- 工具**只执行 SELECT、SHOW、PRAGMA、SCAN、ListCollections 等只读操作**，没有任何写/删/改逻辑。  
- 所有查询参数化，标识符转义，杜绝注入。  
- 密码在输出中自动脱敏。

## 常见用法示例

1. **分析单个 MySQL 库**  
   ```bash
   ./tools/dbexplain-linux-amd64 -dsn "mysql://root:123@localhost:3306/shop"
   ```
2. **跨库联合分析**  
   ```bash
   ./tools/dbexplain -dsn "postgres://..." -dsn "redis://..." -json | python -m json.tool
   ```
3. **使用 .env 批量分析**  
   在 .env 中写入 `DB1=mysql://... DB2=clickhouse://...`，执行：  
   ```bash
   ./tools/dbexplain -env
   ```

## 扩展性

需要支持新数据库时，可在 `connector/` 下实现 `Connector` 接口并注册，编译后替换 `tools/` 中的二进制即可。 
---
name: db-relationship-explainer
description: 零依赖跨数据库结构探查与关系分析，支持 MySQL/PostgreSQL/GaussDB/SQLite/ClickHouse/Redis/Qdrant/Elasticsearch，自动生成表卡片、关系图与问题报告。
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

1. **检查 .env 文件**  
   - 工具**仅**读取本技能目录下的 `.env` 文件（即与 `SKILL.md` 同一目录）。  
   - 若该目录下没有 `.env` 文件，**立刻停止**，并回复用户：  
     ```
     请在技能目录 `db-relationship-explainer/` 下创建 `.env` 文件，按以下格式写入数据库连接串：
     
     DB1=mysql://user:pass@host:3306/db?label=别名
     DB2=postgres://user:pass@host:5432/db?label=别名
     ... 
     
     密码中的特殊字符需 URL 编码（@→%40, #→%23 等）。
     创建后重新运行 `./dbexplain -env` 即可。
     ```
   - **绝对禁止**去其他目录或系统路径搜索 `.env` 文件。

2. **运行工具**  
   ```bash
   ./tools/dbexplain-{platform} -env
   ```  
   或手动指定 DSN（无需 .env）：  
   ```bash
   ./tools/dbexplain-{platform} -dsn "scheme://user:pass@host:port/db?label=别名"
   ```

3. **只读输出**：工具仅执行 SELECT / SHOW / SCAN 等安全操作，输出 Markdown 格式的数据库结构卡片、关系图、问题清单。

## DSN 格式示例

- MySQL: `mysql://user:pass@host:3306/db`
- PostgreSQL: `postgres://user:pass@host:5432/db`
- Redis: `redis://:password@host:6379/0`
- ClickHouse: `clickhouse://default:pass@host:8123/db`
- SQLite: `sqlite:///绝对路径`

密码特殊字符需 URL 编码（`@` → `%40`, `#` → `%23` 等）。

## 输出解读

- **实例概览** → 连接的所有数据库  
- **表结构卡片** → 列、类型、约束、索引、注释  
- **关系** → 显式外键 + 推断外键  
- **聚类** → 通过外键关联的表组  
- **问题** → 缺主键、未索引外键、审计时间戳缺失等

## 安全保证

- 所有操作为只读，无写入/删除/修改逻辑。  
- 查询参数化，标识符转义。  
- 密码在输出中自动脱敏。   
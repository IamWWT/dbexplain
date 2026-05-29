# SQLite 结构采集与排障手册

本文档详细说明 `dbexplain` 工具中 SQLite 连接器（`connector/sqlite.go`）的实现机制，帮助理解其如何通过纯 Go 驱动安全地获取数据库文件中的表结构、列信息、索引和外键，并对无注释字段进行语义推断，同时提供常见问题的排障方法。

---

## 一、代码中的重要机制

### 1.1 连接建立与安全 Ping

```
path := SQLitePath(d)
connStr := path + "?" + params
db, err := sql.Open("sqlite", connStr)
defer db.Close()

pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
if err := db.PingContext(pingCtx); err != nil { ... }
```

- **驱动选择**：使用 `github.com/glebarez/go-sqlite`，纯 Go 实现的 SQLite 驱动，完全零 CGO 依赖。无需系统安装 `libsqlite3.so`，编译产物可独立运行。
- **DSN 格式**：`sqlite:///absolute/path/to/file.db?label=xxx`。注意三个斜杠（`///`）——前两个是 scheme 分隔符，第三个是绝对根路径的 `/`。
- **路径提取**：`SQLitePath()` 函数从 DSN 中剥离 scheme（`sqlite:///`）提取出绝对文件路径。例如 `sqlite:///home/user/data.db?label=mydb` 提取为 `/home/user/data.db`。
- **零 CGO 优势**：跨平台编译无需安装 SQLite 开发库；Linux、macOS、Windows 的编译产物直接运行；不受系统 SQLite 版本限制。
- **超时控制**：Ping 操作使用独立的 5 秒超时。由于 SQLite 是嵌入式数据库（无网络延迟），5 秒通常足够。
- **错误包装**：所有 error 均通过 `schema.NewDBError` 返回，记录脱敏 DSN 和操作上下文。

### 1.2 表信息采集

```
rows, err := db.QueryContext(ctx,
    `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
```

- **系统表过滤**：通过 `name NOT LIKE 'sqlite_%'` 排除 SQLite 内部系统表（如 `sqlite_sequence`、`sqlite_stat1` 等）。`sqlite_master` 本身不会出现在结果中（其 type 不是 `table`）。
- **仅采集基表**：`type='table'` 排除了视图（`type='view'`）和触发器（`type='trigger'`）。
- **行数获取**：SQLite 不维护近似行数统计信息。工具执行 `SELECT COUNT(*) FROM "tablename"` 获取精确行数（触发全表扫描）。由于 SQLite 文件操作极快，即使大表 COUNT 也通常在毫秒级完成，行数直接展示在报告中。
- **无表大小信息**：SQLite 不对外暴露单表大小，报告中跳过大小字段。

### 1.3 列信息采集与语义推断

```
// 表名通过 strings.ReplaceAll(t.Name, "'", "''") 转义防止 SQL 注入
colRows, err := db.QueryContext(ctx,
    fmt.Sprintf("PRAGMA table_info('%s')", strings.ReplaceAll(t.Name, "'", "''")))
```

- **PRAGMA table_info** 返回每列的：`cid`（序号）、`name`（列名）、`type`（类型声明）、`notnull`（是否 NOT NULL，1/0）、`dflt_value`（默认值）、`pk`（是否主键，1/0）。
- **INTEGER PRIMARY KEY 可空修复（v0.0.7）**：SQLite 对 `INTEGER PRIMARY KEY` 列，`PRAGMA table_info` 返回 `notnull=1` 但 `dflt_value=null`。在标准 SQL 语义下自增主键不应为可空。代码通过 `nullable = notnull==0 && pk==0` 双重校验修复：只有 `notnull` 和 `pk` 同时为 0 时才标记为可空，确保自增主键列正确标记为 NOT NULL。
- **标志推断**：从 `pk` 字段判断 `IsPrimary`。SQLite 不直接返回 UNIQUE 约束信息（需通过 `PRAGMA index_list` 获取），因此列级别不单独标记 `IsUnique`。
- **字段注释推断**：SQLite 不原生支持列注释（无 COMMENT ON COLUMN 语法）。工具对所有列均尝试获取一条样本行（`SELECT * FROM tablename LIMIT 1`），通过 `schema.InferComment` 规则引擎基于列名、类型和样本值生成语义注释。
- **安全采样**：采样查询 `LIMIT 1` 仅读取第一行，SQLite 文件操作极快，无性能风险。

### 1.4 外键采集

```
fkRows, err := db.QueryContext(ctx,
    fmt.Sprintf("PRAGMA foreign_key_list('%s')", tableName))
```

- **PRAGMA foreign_key_list** 返回外键信息：`id`（约束序号）、`seq`（列序号）、`table`（引用表名）、`from`（本表列名）、`to`（引用表列名）、`on_update`、`on_delete`、`match`。
- **动作映射**：`on_update` 和 `on_delete` 返回 `CASCADE`、`SET NULL`、`SET DEFAULT`、`RESTRICT`、`NO ACTION` 等标准外键动作，直接写入 `OnUpdate` 和 `OnDelete` 字段。
- **复合外键**：同一 `id` 下多个 `seq` 的列会被合并为一个 `ForeignKey` 对象，列按 `seq` 排序。
- **外键约束开关**：SQLite 默认不强制外键约束（需 `PRAGMA foreign_keys = ON;`）。工具仅读取外键定义，不依赖于约束是否启用。
- **跨文件引用**：SQLite 支持 `ATTACH DATABASE` 引用另一个 `.db` 文件，但 `PRAGMA foreign_key_list` 只返回表名，不返回库名，因此跨数据库引用的 `RefDB` 字段为空。

### 1.5 索引采集

```
idxRows, err := db.QueryContext(ctx,
    fmt.Sprintf("PRAGMA index_list('%s')", tableName))
```

- **PRAGMA index_list** 返回索引列表：`seq`、`name`、`unique`（是否唯一）。
- 对于每个索引，再执行 `PRAGMA index_info('index_name')` 获取索引包含的列名，按 `seqno` 排序。
- 自动创建的索引（如 `sqlite_autoindex_*`）同样会被采集，这些索引对应 UNIQUE 约束和主键约束。
- 索引的 `Unique` 标志直接来源于 `PRAGMA index_list` 的 `unique` 字段。

### 1.6 错误处理与进度日志

- 所有数据库操作均通过 `schema.NewDBError` 包装，记录操作类型（`query tables`、`PRAGMA table_info`、`PRAGMA foreign_key_list` 等）。
- 使用 `logf` 输出进度到 `logs/<label>.log`，包括表采集序号和采样失败警告。
- 单个表查询失败不会中断整个采集流程，工具会继续处理下一个表。
- 文件不存在或权限不足的错误会明确报告文件路径。

---

## 二、常见问题与排障

### 2.1 文件路径错误或文件不存在

**错误示例**：
```
skip sqlite:///...: sqlite ping: unable to open database file: no such file or directory
```

**可能原因与解决方案**：
- **路径错误**：确认 DSN 中使用的是绝对路径（以 `/` 开头）。相对路径在工具运行的不同工作目录下可能解析错误。
- **文件权限**：确保运行 `dbexplain` 的用户对 `.db` 文件及其父目录具有读权限。SQLite 在打开时需要目录的写权限以创建临时日志文件（即使只读操作）。若目录不可写，添加 `?mode=ro` 到 DSN 中以只读模式打开。
- **路径空格**：路径中含有空格时，在 shell 中使用单引号包裹整个 DSN：`'sqlite:///path/with spaces/db.sqlite?label=test'`。

### 2.2 数据库文件被加密

- SQLite 加密扩展（如 SQLCipher、SEE）加密的文件，标准 `go-sqlite` 驱动无法直接打开。需要：
  - 使用支持加密的驱动（如 `github.com/mutecomm/go-sqlcipher`），修改连接器代码适配。
  - 或在采集前将加密数据库解密为明文副本。

### 2.3 外键信息不完整

- 如果创建表时未指定 `FOREIGN KEY` 约束，工具不会生成外键关系。推断关系由分析模块通过列名匹配完成。
- 确认数据库文件中是否启用了外键支持：`PRAGMA foreign_keys;`。即使未启用，`PRAGMA foreign_key_list` 仍能返回外键定义。
- 跨 ATTACH 数据库的外键引用，`PRAGMA foreign_key_list` 的 `table` 字段仅返回表名，无法区分具体数据库文件。

### 2.4 字段注释推断效果不理想

- SQLite 不原生支持列注释，所有注释均依赖 `InferComment` 推断。若推断不准，可针对性修改 `schema.InferComment` 的规则映射。
- 可以考虑使用表级别的 COMMENT 习惯——在创建表时手写一个注释文档（如 Markdown 文件），但工具不会自动读取。

### 2.5 大文件场景下的性能

- SQLite 单个文件可达 TB 级别，采集结构的速度取决于表数量和列数量（而非数据量），因为所有查询使用 `PRAGMA` 和系统表，不扫描数据。
- 采样查询 `LIMIT 1` 仅在极个别极端场景下较慢（如表中第一行恰巧在文件末尾的未压缩区域）。
- 对于 WAL 模式的数据库，工具会在读取时自动处理 WAL 检查点。

---

## 三、核心命令速查（sqlite3 CLI 验证）

| 目的 | sqlite3 CLI 命令 |
|------|------------------|
| 打开数据库 | `sqlite3 /path/to/file.db` |
| 列出所有表 | `.tables` 或 `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%';` |
| 查看表结构 | `.schema tablename` 或 `PRAGMA table_info('tablename');` |
| 查看外键 | `PRAGMA foreign_key_list('tablename');` |
| 查看索引列表 | `PRAGMA index_list('tablename');` |
| 查看索引详情 | `PRAGMA index_info('index_name');` |
| 查看行数（精确） | `SELECT COUNT(*) FROM tablename;` （注意：全表扫描） |
| 检查外键是否启用 | `PRAGMA foreign_keys;` |
| 只读模式打开 | `sqlite3 -readonly /path/to/file.db` |

---

## 四、经验总结

1. **零 CGO 优势**：`github.com/glebarez/go-sqlite` 驱动实现了完全的纯 Go SQLite，编译产物体积小且跨平台无痛，是最适合 Go 生态的嵌入式数据库方案。
2. **权限最小化**：SQLite 作为嵌入式数据库，无需用户名/密码，安全依托于文件系统权限。确保 `.db` 文件的文件权限设置合理（如 `chmod 600`），且运行用户的 `umask` 配置正确。
3. **INTEGER PRIMARY KEY 特殊语义**：SQLite 中 `INTEGER PRIMARY KEY` 列自动成为 `rowid` 的别名，`PRAGMA table_info` 的 `pk` 字段可以正确识别，但 `notnull` 和 `dflt_value` 的组合需要业务逻辑修正（v0.0.7 中已修复）。
4. **无统计信息**：与其他数据库不同，SQLite 不维护表的行数、大小等统计信息。报告中不提供这些字段，AI Agent 不应依赖它们进行容量评估。
5. **采样安全性**：`LIMIT 1` 在 SQLite 中返回任意一行（不是物理第一行），同等轻量，不涉及全表扫描。
6. **并发安全**：SQLite 同一连接不支持并发写入（即使仅读取结构），工具通过 WAL 模式或 `mode=ro` DSN 参数提升并发读取的安全性。
7. **版本兼容**：测试于 SQLite 3.31+ 版本，`PRAGMA` 接口稳定，跨版本兼容性好。
8. **多文件采集**：若项目中有多个 `.db` 文件，需为每个文件配置独立的 DSN，工具不支持通配符或多文件自动发现。

通过以上机制和指南，即可安全、高效地使用 `dbexplain` 对 SQLite 数据库文件进行结构探查与语义丰富。

# 安全检查手册 （Security Checklist）

> 发布前必读。每项必须确认通过，发现新问题及时追加到本手册。

---

## 1. 凭证保护（第一要义）

**任何代码路径都不得将数据库密码写入标准输出、标准错误、日志文件或任何持久化存储。**

### 检查项

- [ ] **DSN 输出** — 所有终端输出、日志、错误消息中的 DSN 必须经过 `Redacted()` 脱敏
  ```go
  // 正确
  log.Printf("skipping %s", parsed.Redacted())
  // 错误
  log.Printf("skipping %s", e.raw)
  ```
- [ ] **Error 消息** — `log.Printf`/`log.Fatalf`/`fmt.Errorf` 的格式化参数不得直接引用原始 DSN 字符串
- [ ] **第三方库错误** — `godotenv` 等第三方库的错误消息可能包含原始文件内容，必须经过 `sanitizeErr()` 处理
- [ ] **JSON 输出** — `-json` 输出永不包含连接字符串
- [ ] **日志文件** — `logs/<label>.log` 中不得出现原始密码
- [ ] **标准输出** — 所有 DSN 回显均经过 `Redacted()`
- [ ] **`.env` 文件** — 确认 `.gitignore` 包含 `src/.env`、`.env`、`.env.dbexplain`

### 历史上的坑

| Issue | 问题 | 修复 |
|-------|------|------|
| ISSUE-040 | `.env` 真实凭证提交到 Git | `.gitignore` 增加规则 |
| ISSUE-041 | `src/logs/` 暴露数据库名 | `.gitignore` 增加规则 |
| ISSUE-052 | godotenv 错误消息泄露完整 DSN | `sanitizeErr()` 脱敏处理 |
| v0.0.4 bug | `filterDSNs` skip 消息用 `e.raw` 泄漏密码 | 改用 `parsed.Redacted()` |

---

## 2. 文件编码与平台兼容

### 检查项

- [ ] **UTF-8 BOM 处理** — 读取配置文件时必须先剥离 BOM (`EF BB BF`)
  - `.env.dbexplain` 通过 `loadEnvFile()` 读取，已内置 BOM 剥离
  - JSON 配置文件：`json.Decoder` 自动处理 BOM
  - 其他文本文件：读取后 `bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})`
- [ ] **Windows GBK** — `-o` 文本输出文件正常（中文系统自动转 GBK，英文系统 UTF-8 BOM）
- [ ] **JSON 输出不加 BOM** — `-json -o` 输出纯 UTF-8，不加 BOM
- [ ] **ASCII 安全渲染** — Unicode 制表符、项目符号等替换为 ASCII 等效字符

### 历史上的坑

| Issue | 问题 | 修复 |
|-------|------|------|
| ISSUE-051 | JSON 输出含 BOM，标准解析器报错 | `-json -o` 不再加 BOM |
| ISSUE-052 | UTF-8 BOM 导致 `.env.dbexplain` 解析失败 | `loadEnvFile()` 剥离 BOM |
| v0.0.4 | Windows 记事本/CMD 中文乱码 | ACP 检测 + GBK 转换 |

---

## 3. 输入验证

### 检查项

- [ ] **DSN 解析** — 恶意构造的 DSN 不会导致 panic 或注入
- [ ] **配置文件** — 畸形 JSON/ENV 文件有友好错误提示，不暴露系统路径
- [ ] **命令行参数** — `--include`/`--exclude` 等过滤参数对特殊字符安全（逗号分隔正常）
- [ ] **数据库名/表名** — 包含特殊字符的标识符正确处理（反引号/双引号转义）
- [ ] **文件路径** — `--log-dir`、`--context`、`--cache`、`-o` 等路径参数防止路径遍历

---

## 4. 安全传输

### 检查项

- [ ] **TLS/SSL** — MySQL/PostgreSQL/ClickHouse 的 TLS 配置正确
  - MySQL: `tls=<config>` 参数生效
  - PostgreSQL: `sslmode` 参数生效
- [ ] **已知限制追踪** — ISSUE-042 (ES InsecureSkipVerify)、ISSUE-043 (ClickHouse URL 密码) 未退化

---

## 5. 运行时安全

### 检查项

- [ ] **只读操作** — 所有 Connector 仅使用 `SELECT`/`SHOW`/`SCAN`/`PRAGMA` 等只读操作
- [ ] **采样上限** — Redis ≤ 2000 key、≤ 5 字段、≤ 512 字节
- [ ] **连接隔离** — 每实例独立连接，单实例 panic 不影响其他实例
- [ ] **超时控制** — 所有数据库连接有合理的连接/读写超时
- [ ] **并发安全** — sync.Map 或 mutex 保护共享状态

---

## 6. 发布前快速检查

每次发布前，逐项执行：

```
[ ] go build 无错误
[ ] go vet  无警告
[ ] go test ./... 全部通过
[ ] 交叉编译 5 平台全部成功
[ ] .gitignore 包含: src/.env, .env, .env.dbexplain, src/logs/
[ ] grep -r "e\.raw" src/        # 确认无原始 DSN 输出
[ ] grep -r "log.*Printf.*%s.*err" src/  # 确认错误消息经过脱敏
[ ] grep -r "\.env" src/ --include="*.go" | grep -v test | grep -v example  # 检查引用
[ ] 以 UTF-8 BOM 编码的 .env.dbexplain 测试 -env 正常加载
[ ] 确认 CHANGELOG.md 列出所有安全问题
[ ] 确认 SECURITY_CHECKLIST.md 追加新发现的问题
```

---

## 7. 新增安全问题流程

发现新安全问题时：

1. **记录 ISSUE** — 创建 GitHub Issue，标记 bug 标签
2. **追加本章** — 在对应检查项的"历史上的坑"表格中追加
3. **更新 CHANGELOG** — 在对应版本的安全/修复章节中追加
4. **团队通报** — Release 描述中重点标注安全问题

---

## 相关文档

- [ARCHITECTURE.md - 第 9 章 安全性](./ARCHITECTURE.md)
- [CHANGELOG.md](../CHANGELOG.md) — 每版本安全问题汇总
- [DEPLOY_SRC.md](./DEPLOY_SRC.md) — 部署安全配置说明

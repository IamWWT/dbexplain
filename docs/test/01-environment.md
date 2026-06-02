# L1: 环境验证与静态分析

---

## 1.1 Go 版本

```bash
go version
# 预期: go 1.26+
```

## 1.2 编译验证

```bash
cd src
go build ./... && echo "build: OK" || echo "build: FAIL"
go vet ./... && echo "vet: OK" || echo "vet: FAIL"
```

## 1.3 单元测试

```bash
cd src && go test ./... -v -count=1 2>&1 | tail -20
```

### DSN 解析 (dsn 包)

| 测试函数 | 用例数 | 覆盖 |
|---------|--------|------|
| `TestParseDSN_Schemes` | 19 | 全部数据库类型 + alias + TLS + unsupported |
| `TestParseDSN_QueryParams` | 8 | label/sslmode/cluster/tls/中文 |
| `TestParseDSN_AutoLabel` | 1 | 无 label 自动生成 |
| `TestRedacted` | 10 | 密码脱敏（含 @/URL编码/空密码/占位符） |
| `TestParseDSN_EdgeCases` | 1 | 边界情况 |

### 字段推断 (schema 包)

| 测试函数 | 用例数 | 覆盖 |
|---------|--------|------|
| `TestInferComment` | 43 | 标识符/名称/时间/金额/状态/布尔/邮箱/电话/IP/URL/密钥等 |
| `TestInferComment_Ordering` | 1 | 规则优先级验证 |

### 安全策略 (policy 包)

| 测试函数 | 用例数 | 覆盖 |
|---------|--------|------|
| 19 测试函数 | 45 | 全局/按DSN/表级/列级/语句级/原生/Mongo/Qdrant/Redis key/DSL per-DSN策略/MongoDB $out拒绝 |

### SQL 只读校验 (sqlguard 包)

| 测试函数 | 用例数 | 覆盖 |
|---------|--------|------|
| 15 测试函数 | 32 | 读动词/写动词(含KILL/SET/FLUSH)/空查询/多语句/自动LIMIT(含LIMIT())/分号/首词提取 |

### 查询并发控制 (query 包)

| 测试函数 | 用例数 | 覆盖 |
|---------|--------|------|
| 9 测试函数 | 15 | Lock/Unlock/并发/多标签 |

## 1.4 交叉编译

```bash
cd src && bash build.sh
# 预期: 5 个平台全部成功
```

## 1.5 安全审计 — 敏感文件不被 Git 追踪

```bash
# .env 文件不在 Git 中
git ls-files src/.env
# 预期: 空（无输出）

# logs/ 目录不在 Git 中
git ls-files src/logs/
# 预期: 空

# 加密文件不在 Git 中
git ls-files '*.enc'
# 预期: 空
```

## 1.6 Shell 脚本语法检查

```bash
bash -n dbexplain-skill/scripts/install.sh && echo "install.sh OK"
bash -n dbexplain-skill/scripts/uninstall.sh && echo "uninstall.sh OK"
bash -n dbexplain-skill/scripts/install-skill.sh && echo "install-skill OK"
bash -n dbexplain-skill/scripts/uninstall-skill.sh && echo "uninstall-skill OK"
```

## 1.7 二进制版本确认

```bash
./dbexplain --version
# 预期: dbexplain v0.1.2
```

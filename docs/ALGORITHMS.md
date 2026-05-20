# dbexplain 算法文档

## 架构总览

所有算法按 **Capability（能力）** 而非数据库类型组织。新增数据库类型只需声明已有 Capability，算法自动适配。

```
Connector 声明 Capability → 算法按 Capability 触发
```

| Capability | 触发算法 | 持有者 |
|---|---|---|
| `foreign_key` | FK 无索引检测、主键缺失检测、关系推断、图聚类 | MySQL, PostgreSQL, GaussDB, SQLite |
| `sampling` | 列注释推断、值类型推断 | MySQL, PostgreSQL, GaussDB, ClickHouse, SQLite, Redis |
| `ttl` | TTL 安全检测 | Redis |
| `partition` | 排序键/分区键识别 | ClickHouse |
| `row_count` | 行数归一化（评分因子） | MySQL, PostgreSQL, GaussDB, ClickHouse, SQLite, MongoDB, Qdrant |
| `index` | 索引密度（评分因子） | MySQL, PostgreSQL, GaussDB, SQLite |

---

## 1. 命名约定关系推断 (Naming Convention Ref Inference)

| 属性 | 说明 |
|---|---|
| **文件** | `src/analyze/analyze.go` → `inferRefs()` |
| **依赖包** | `strings`（标准库） |
| **可替换** | 是。替换 `inferRefs()` 即可，不影响其他模块 |
| **架构层级** | 按 Capability 触发（需 `foreign_key`） |

### 算法原理

当列名符合 `{stem}_id` 模式（如 `user_id`、`order_fk`），自动推断该列引用了名为 `{stem}` 或 `{stem}s` 的表的主键。

```
列名匹配规则:
  *_id  → stem = 去掉 _id 后缀    (如 user_id → user → users 表)
  *id   → stem = 去掉 id 后缀     (如 userid → user → users 表)
  *_fk  → stem = 去掉 _fk 后缀    (如 order_fk → order → orders 表)
```

### 置信度模型

- **85%**: 同实例、同数据库内的引用
- **70%**: 跨数据库的引用
- **55%**: 跨实例的引用

### 去重机制

已存在显式 FK（confidence=100%）的列不会被重复推断。使用 `FromInstance+FromDB+FromTable+FromCol` 作为去重键。

---

## 2. 并查集表聚类 (Union-Find Table Clustering)

| 属性 | 说明 |
|---|---|
| **文件** | `src/analyze/analyze.go` → `clusterGroups()` |
| **依赖包** | `strings`（标准库） |
| **可替换** | 是。可替换为其他图连通分量算法 |
| **架构层级** | 通用算法，不依赖 Capability |

### 算法原理

将所有表作为图节点，FK 关系（显式+推断）作为边，使用 **Union-Find（并查集）** 算法找出所有连通分量。每个连通分量即一个"表聚类"。

```
步骤:
  1. 每个表初始化为独立集合 (parent[table] = table)
  2. 遍历所有关系，对 (from, to) 执行 union
  3. 路径压缩优化 (find 时 parent[x] = find(parent[x]))
  4. 按根节点聚合，同名表优先使用 longestCommonPrefix 命名
```

### 复杂度

- **时间复杂度**: O(N + M·α(N))，N=表数，M=关系数，α=阿克曼反函数（近似常数）
- **空间复杂度**: O(N)

---

## 3. 确定性字段语义推断 (Deterministic Column Comment Inference)

| 属性 | 说明 |
|---|---|
| **文件** | `src/schema/infer.go` → `InferComment()` |
| **依赖包** | `strings`（标准库） |
| **可替换** | 是。替换 `InferComment()` 函数即可 |
| **架构层级** | 按 Capability 触发（需 `sampling`） |

### 算法原理

纯规则引擎，**零 AI 内容**。按优先级匹配列名关键字：

```
匹配优先级（逐项检查，命中即停）:
  1. *id 结尾          → "标识符"
  2. 含 name/title     → "名称"
  3. 含 time/date      → "时间"
  4. 含 amount/price   → "金额/数量"
  5. 含 status/state   → "状态"
  ... (共 15 条规则)
  15. 无匹配但有采样值  → "示例: <截断前20字符>"
  16. 无匹配无采样值    → ""
```

IP 地址检测使用**边界匹配**（`ip_`前缀/`_ip`后缀/`_ip_`中间/`==ip`），避免 `description` 等词被误判为IP。

---

## 4. 多因子重要性评分 (Multi-Factor Importance Ranking)

| 属性 | 说明 |
|---|---|
| **文件** | `src/analyze/ranking.go` → `Ranker.Rank()` |
| **依赖包** | `math`, `sort`（标准库） |
| **可替换** | 是。权重通过 `Ranker.Weights` 可配置；整个 Ranker 可替换为新实现 |
| **架构层级** | 通用算法，消费 `Refs` 和 `Universe` |

### 算法原理

4 个可观测维度加权求和，输出 0-1 分数：

```
importance_score = Σ(weight_i × factor_i)

维度              | 权重  | 计算方法               | 归一化方式
─────────────────┼───────┼────────────────────────┼──────────────
graph_degree     | 0.35  | 表在图中的边数 (入+出)  | / max_degree
fk_centrality    | 0.35  | 被其他表 FK 引用的次数  | / max_fk_refs
row_count        | 0.20  | 表的行数               | log10(v+1)/log10(max+1)
index_density    | 0.10  | 索引数 / 列数          | 自然 0-1, clamp
```

### 权重定制

```go
ranker := analyze.NewRanker()
ranker.Weights = map[string]float64{
    "graph_degree":  0.5,  // 更重视图结构
    "fk_centrality": 0.3,
    "row_count":     0.2,
    "index_density": 0.0,  // 禁用某维度
}
```

### 关键设计

- `logNorm` 使用对数压缩行数量级差异（1万行 vs 1000万行差异被压缩）
- 结果按分数降序排列
- 分数四舍五入保留 3 位小数

---

## 5. 正则键模式聚类 (Regex Key Pattern Clustering)

| 属性 | 说明 |
|---|---|
| **文件** | `src/connector/redis.go` → `normalize()` |
| **依赖包** | `regexp`（标准库） |
| **可替换** | 是。替换 `normalize()` 函数即可 |
| **架构层级** | Redis Connector 专用（`ttl` capability） |

### 算法原理

将 Redis key 中变化的部分替换为类型占位符，使相似 key 聚合为模式：

```
规则（按顺序应用）:
  1. UUID  → uuidRe:  [0-9a-f]{8}-[0-9a-f]{4}-...  → "{uuid}"
  2. 十六进制 → hexRe:  [0-9a-f]{16,}                 → "{hex}"
  3. 数字ID  → numRe:  \d{2,}                         → "{id}"

示例:
  session:a1b2c3d4-... → session:{uuid}
  cache:3f8a9b2c...    → cache:{hex}
  user:12345           → user:{id}
```

聚合后，相同模式分组统计 key 数、TTL 分布等，生成"虚拟表"。

---

## 6. Schema 指纹与增量检测 (Schema Fingerprint & Delta Detection)

| 属性 | 说明 |
|---|---|
| **文件** | `src/cache/cache.go` |
| **依赖包** | `crypto/sha256`, `encoding/hex`, `sort`（标准库） |
| **可替换** | 是。可替换为 MD5/Blake3 等哈希算法 |
| **架构层级** | 通用，独立于 Connector |

### 算法原理

对每个表的三个维度分别计算 hash：

```
Fingerprint = {
    col_hash:   SHA-256(排序列名:类型:可空性)
    index_hash: SHA-256(排序索引名:唯一性:排序列)
    fk_hash:    SHA-256(排序FK列:引用表:引用列:约束名)
}
```

两次扫描中三个 hash 全部相同 → 表未变更 → 可跳过下游处理。

### 增量检测 (Delta)

```
Diff(current, previous) → {
    added:   []string  // 新出现的表
    removed: []string  // 消失的表
    changed: []string  // 指纹变化的表
}
```

### 磁盘存储

指纹持久化到 JSON 文件（`--cache fingerprints.json`），下次加载后与当前扫描结果比较。

---

## 7. 能力驱动诊断规则 (Capability-Driven Diagnostics)

| 属性 | 说明 |
|---|---|
| **文件** | `src/diagnostics/diagnostics.go` |
| **依赖包** | `strings`, `fmt`（标准库） |
| **可替换** | 是。每项规则独立，新增/删除规则不影响其他规则 |
| **架构层级** | 按 Capability 触发 |

### 规则引擎

```
for each table:
    for each rule:
        if rule.Requires != "" && !caps.Has(rule.Requires):
            skip   // 该数据库不支持此能力
        issues += rule.Check(table)

当前规则:
  1. unindexed_fk   → 需 foreign_key: FK列无索引 → warn
  2. wide_table     → 通用: 列数>30 → info
  3. missing_pk     → 需 foreign_key: 无主键 → warn
  4. no_timestamp   → 需 foreign_key 且无 partition:
                      无时间戳列 → info
```

### 新增规则

```go
// 在 standardRules() 中添加:
{
    Name:     "my_new_check",
    Requires: capabilities.CapIndex,
    Check:    func(t *schema.Table, inst, db string, caps *capabilities.Set) []Issue {
        // 自定义诊断逻辑
    },
}
```

---

## 算法替换指南

所有算法均通过**接口或函数替换**实现可插拔：

| 替换目标 | 方式 | 影响范围 |
|---|---|---|
| 关系推断 | 替换 `inferRefs()` | `analyze.go` |
| 表聚类 | 替换 `clusterGroups()` | `analyze.go` |
| 字段推断 | 替换 `InferComment()` | `schema/infer.go` |
| 重要性评分 | 替换 `Ranker` 或调整 `Weights` | `analyze/ranking.go` |
| 键模式聚类 | 替换 `normalize()` | `connector/redis.go` |
| 指纹哈希 | 替换 `sha256Hex()` → MD5/Blake3 | `cache/cache.go` |
| 诊断规则 | 新增/删除 `Rule` 条目 | `diagnostics/diagnostics.go` |

---

## 依赖清单

所有算法仅依赖 **Go 标准库**，无第三方算法包：

| 包 | 用途 |
|---|---|
| `crypto/sha256` | Schema 指纹哈希 |
| `encoding/hex` | Hash 编码为字符串 |
| `math` | 对数归一化、浮点运算 |
| `sort` | 确定性排序（确保 hash 可重复） |
| `strings` | 列名匹配、模式处理 |
| `regexp` | Redis key 正则规范化 |

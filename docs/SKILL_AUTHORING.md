# SKILL.md 编写规范

> 参考：Andrej Karpathy 提出的 SKILL.md 实践 + dbexplain 项目实际经验
> SKILL.md = AI 编程助手的项目级 prompt，教会 AI 如何操作该项目

---

## 核心原则

1. **读者是 AI，不是人** — 应被 AI 直接作为 system prompt 读取，聚焦于教会 AI "如何正确使用这个项目"
2. **越具体越好** — 不写模棱两可的空话。提供的信息越精确，AI 表现越像老手
3. **避免上下文浪费** — 每句话都有用。不包含测试计划、QA 检查清单、变更历史、TODO 列表
4. **可执行优先** — 所有命令应当是可直接复制运行的（无需用户猜测参数）

## 文件位置

```
{project_root}/dbexplain-skill/SKILL.md            # 主技能（中文，默认）
{project_root}/dbexplain-skill/SKILL_ZH.md          # 中文版
{project_root}/dbexplain-skill/SKILL_EN.md          # 英文版
{project_root}/dbexplain-skill/SKILL_{模型}.md      # 特定模型优化的版本
```

## YAML Frontmatter

每个 SKILL.md 必须以 YAML frontmatter 开头，定义元数据：

```yaml
---
name: <技能名>
description: >
  <一句话或短段落说明技能用途。AI 根据此描述决定是否启用技能。
  关键：涵盖触发场景和工具能力边界。>
user-invocable: true  # 是否允许用户显式调用
trigger:              # 用户输入中触发此技能的关键词（可选但推荐）
  - "<触发词1>"
  - "<触发词2>"
---
```

### 命名规范

- 技能名：`{project}-{function}`，全部小写，连字符分隔
- 例如：`dbexplain-skill`、`dbexplain-excel-csv-analysis`
- 中英文版使用相同 name

### 触发词

- 写**用户实际会说的话**，而非技术术语
- 好的：`"分析CSV数据"`、`"触达率排行"`
- 不好的：`"测试CSV文件"`（用户不这么说）、`"QA回归测试"`（AI 视角而非用户视角）

## 内容结构

### 1. 工具概述

简要说明工具是什么、能做什么。聚焦于**此技能关心**的能力，不需要列出所有功能。

**好的写法：**
```
dbexplain v0.2.0+ 文件查询引擎对 CSV/XLSX 支持完整 SELECT 子集：
- WHERE 过滤、GROUP BY + 聚合、ORDER BY 排序
- 跨文件哈希 JOIN、列间算术、CAST/ABS/LIKE/IN/BETWEEN
- LIMIT/OFFSET 分页
```

### 2. 核心规则

AI 必须遵守的行为约束。限于 3-5 条。

```
- 只读安全：仅执行 SELECT。遇到 ACCESS_DENIED 不绕过，如实告知用户
- 列屏蔽：MASK_COLUMNS 替换敏感列值，不影响分析
- BOM 处理：CSV UTF-8 BOM 首列自动剥离，无需关注
```

### 3. 标准工作流

分步骤描述 AI 应如何操作。**每个步骤包含可复制执行的命令**。

标准流程： Schema 采集 → 数据预览 → 业务分析

```
### 1. Schema 采集
DBPROBE_ENV_FILE=<env> dbexplain -env

### 2. 数据预览
DBPROBE_ENV_FILE=<env> dbexplain execute -env --label <label> 'SELECT *' --limit 5 --human

### 3. 业务分析
DBPROBE_ENV_FILE=<env> dbexplain execute -env --label <label> 'SELECT col, AVG(val) FROM t GROUP BY col' --human
```

### 4. 业务知识 / 领域知识（可选但强烈推荐）

对于业务分析技能，此部分至关重要。告诉 AI：
- 数据是什么、粒度如何、覆盖范围
- 关键字段含义
- 常见的坑（如字段拼写错误、特殊值含义、聚合时的注意事项）

### 5. 常见场景（可选）

列出 AI 可能会遇到的主要使用场景，附上对应命令。格式参照标准工作流。

### 6. 快速参考

以表格形式罗列参数、配置文件映射等参考信息。

## 禁止的内容

| 内容 | 理由 | 应该放在哪 |
|------|------|-----------|
| QA 检查清单 | AI 不需要自我检查列表 | `docs/test/` |
| 测试结果 | 已完成的验证结果，AI 无用时 | `docs/test/RESULTS.md` |
| 测试计划/TODO | 未完成的事项，AI 不关心 | issue tracker |
| 变更历史 | AI 不需要知道版本演进 | git log / CHANGELOG.md |
| 数据文件路径细节 | 过于具体的内网路径 | 配置文件中 |
| 环境的安装步骤 | 只在首次配置需要 | scripts/ 或 README |
| 每步命令的预期输出 | 占大量上下文 | `docs/test/` |

## 如何判断内容是否多余

一条内容删掉后，AI 的执行质量是否会显著下降？
- **会** → 保留
- **不会** → 删除

## 文件大小约束

- 目标：**200 行以内**（约 150-200 行最佳）
- 超过 300 行 → 考虑拆分或精简
- 超过 400 行 → 必然注水，必须大幅删减

## 版本管理

- SKILL.md 随代码版本一起更新
- 新能力上线时（如文件查询引擎），同步更新对应的 SKILL.md
- 不同模型版本（如 Qwen 3.5 27B）的特化 SKILL 保持独立文件

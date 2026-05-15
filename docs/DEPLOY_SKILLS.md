# SKILL 部署指南

`db-relationship-explainer` 是一个预配置的 Claude Code / AI Skill，可让 AI 助手直接调用 `dbexplain` 工具分析数据库结构。本指南覆盖 Skill 的安装、配置和验证。

## 📦 Skill 包含哪些文件

在项目根目录的 `db-relationship-explainer/` 下：

```
db-relationship-explainer/
├── SKILL.md                     # Skill 定义文件（触发词、指令、工具路径）
└── tools/                       # 预编译的二进制（需根据平台放置）
    ├── dbexplain-linux-amd64
    ├── dbexplain-linux-arm64
    ├── dbexplain-darwin-amd64
    ├── dbexplain-darwin-arm64
    └── dbexplain-windows-amd64.exe
```

其中：
- `SKILL.md` 包含名称、描述、触发词和工具调用规范。
- `tools/` 目录存放对应平台的静态二进制文件。

## 🏗️ 通用部署结构

大多数 AI 平台（Claude Code、DeepSeek TUI、Continue.dev 等）都遵循类似的 Skill 目录规范：

```
~/.<平台名>/skills/
└── db-relationship-explainer/   # 一个 Skill 一个子目录
    ├── SKILL.md
    └── tools/
        └── dbexplain-{platform}  # 根据当前平台选择一个
```

关键点：
- 目录名通常作为 Skill 的唯一标识。
- `SKILL.md` 必须存在且格式正确。
- `tools/` 下的可执行文件必须有执行权限。

## 🖥️ 各平台部署步骤

### 1. Claude Code (Anthropic 官方)

Claude Code 默认扫描 `~/.claude/skills/` 目录。

```bash
# 1. 创建 Skill 目录
mkdir -p ~/.claude/skills/db-relationship-explainer

# 2. 复制 SKILL.md
cp db-relationship-explainer/SKILL.md ~/.claude/skills/db-relationship-explainer/

# 3. 复制对应平台的二进制（以 Linux amd64 为例）
mkdir -p ~/.claude/skills/db-relationship-explainer/tools
cp db-relationship-explainer/tools/dbexplain-linux-amd64 ~/.claude/skills/db-relationship-explainer/tools/dbexplain-linux-amd64

# 4. 赋予执行权限
chmod +x ~/.claude/skills/db-relationship-explainer/tools/dbexplain-linux-amd64

# 5. 重启 Claude Code 或执行 reload-skills 命令（如果有）
```

**触发词测试**：在 Claude Code 中输入 “解释表结构” 或 “分析数据库关系”，AI 应自动调用该 Skill。

### 2. DeepSeek TUI / DeepSeek Code

DeepSeek TUI 通常使用 `~/.deepseek/skills/` 路径。

```bash
# 创建目录
mkdir -p ~/.deepseek/skills/db-relationship-explainer/tools

# 复制 SKILL.md
cp db-relationship-explainer/SKILL.md ~/.deepseek/skills/db-relationship-explainer/

# 复制二进制（根据你的系统选择）
cp db-relationship-explainer/tools/dbexplain-linux-amd64 ~/.deepseek/skills/db-relationship-explainer/tools/

# 加权限
chmod +x ~/.deepseek/skills/db-relationship-explainer/tools/dbexplain-linux-amd64
```

重启 DeepSeek TUI，然后输入触发词（如“跨库依赖”）即可使用。

### 3. 其他兼容平台 (Continue.dev / Cody / Aider 等)

这些平台大多遵循类似的目录结构，具体路径可能不同，请查阅其文档。

结构同上：`<skill名称>/SKILL.md` + `tools/`。

### 4. 手动调用（无需平台集成）

如果只想在终端直接使用，跳过 SKILL.md，直接运行二进制即可，无需部署到任何平台。

## 🧪 验证 Skill 是否生效

### 验证工具可执行

```bash
# 测试工具本身是否能正常运行（以 SQLite 为例）
echo "CREATE TABLE t(id int);" | sqlite3 /tmp/test.db
~/.claude/skills/db-relationship-explainer/tools/dbexplain-linux-amd64 -dsn "sqlite:////tmp/test.db"
```

应输出类似：
```
▸ Instances (1)
  localhost                    sqlite  1 db(s), 1 tables
...
```

### 验证 AI 是否能调用

在 Claude Code / DeepSeek 中输入：
> 帮我分析一下 testdb 数据库的表结构，MySQL 连接串 mysql://root:123@localhost:3306/testdb

如果 AI 回复中包含了工具输出的表卡片、索引等信息，说明 Skill 已生效。

## ⚙️ 自定义 SKILL.md 中的触发词

如果默认触发词不符合你的习惯，可编辑 `SKILL.md` 中的 `trigger` 字段：

```yaml
trigger:
  - "数据库结构"
  - "表关系图"
  - "数据库巡检"
  # 添加你自己的触发词
```

修改后无需重启即可生效（取决于平台）。

## ❓ 常见问题

**Q: 二进制文件提示 "Permission denied"？**  
A: 执行 `chmod +x` 赋予执行权限。

**Q: AI 没有响应触发词？**  
A: 检查 `SKILL.md` 的格式是否正确（YAML 头 `---` 包裹），确认文件编码为 UTF-8。重启 AI 会话再试。

**Q: 想同时部署多个平台？**  
A: 每个平台目录独立，互不干扰，按需复制即可。

## 📚 相关资源

- [dbexplain 项目主页](https://github.com/IamWWT/understand_dbs_skills)
- [源码部署指南](./DEPLOY_SRC.md)
- [MongoDB 排障文档](./MONGO.md) 
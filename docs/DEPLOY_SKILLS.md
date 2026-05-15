# SKILL 部署指南

`db-relationship-explainer` 是一个预配置的 Claude Code / AI Skill，可让 AI 助手直接调用 `dbexplain` 工具分析数据库结构。本指南覆盖 Skill 的安装、配置和验证，并涵盖多个主流 AI 平台的集成方式。

---

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

---

## 🏗️ 通用部署结构

大多数 AI 平台（Claude Code、Cline、Continue.dev 等）都遵循类似的 Skill 目录规范：

```
~/.<平台名>/skills/
└── db-relationship-explainer/   # 一个 Skill 一个子目录
    ├── SKILL.md
    └── tools/
        └── dbexplain-{platform}  # 根据当前平台选择一个
```

或项目级：

```
<workspace>/.<平台名>/skills/
└── db-relationship-explainer/
    ├── SKILL.md
    └── tools/
```

关键点：
- 目录名通常作为 Skill 的唯一标识。
- `SKILL.md` 必须存在且格式正确（YAML frontmatter + Markdown 指令体）。
- `tools/` 下的可执行文件必须有执行权限。

---

## 🖥️ 各平台部署步骤

### 1. Claude Code（Anthropic 官方）

#### 方式一：用户级（全局可用）

Claude Code 默认扫描 `~/.claude/skills/` 目录（用户级）和 `.claude/skills/` 目录（项目级）。

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

# 5. 验证发现：在 Claude Code 会话中执行
/skills
# 应列出 db-relationship-explainer
```

#### 方式二：项目级（团队共享）

```bash
# 在项目根目录下
mkdir -p .claude/skills/db-relationship-explainer
cp db-relationship-explainer/SKILL.md .claude/skills/db-relationship-explainer/
mkdir -p .claude/skills/db-relationship-explainer/tools
cp db-relationship-explainer/tools/dbexplain-linux-amd64 .claude/skills/db-relationship-explainer/tools/
chmod +x .claude/skills/db-relationship-explainer/tools/dbexplain-linux-amd64
```

#### 方式三：Web/Desktop 端上传 ZIP

适用于团队统一分发。在 Claude 侧边栏中：
1. 打开 Customize → Skills
2. 点击 + → Create skill → Upload a skill
3. 将整个 `db-relationship-explainer/` 目录打包为 ZIP 并上传

#### SKILL.md 规范说明（2026 版）

`SKILL.md` 由两部分组成：YAML frontmatter（元数据）和 Markdown 指令正文。

关键字段：
- `name`：Skill 名称，若省略则使用目录名。
- `description`：描述该 Skill 的用途和触发条件，Claude 据此判断何时自动调用。
- `user-invocable`：是否允许用户通过 `/skill-name` 手动调用。
- `disable-model-invocation`：是否禁止 Claude 自动触发。

触发词测试：在 Claude Code 中输入 "解释表结构" 或 "分析数据库关系"，AI 应自动调用该 Skill。

---

### 2. Cline（VS Code 插件）

Cline 3.48.0 及以上版本支持 Skills 系统。Skills 按需加载（与始终激活的 Rules 不同），不会消耗不相关的上下文。

**启用 Skills**：进入 Settings → Features → 开启 Enable Skills。

**部署步骤**（用户级全局）：

```bash
# 1. 创建目录
mkdir -p ~/.cline/skills/db-relationship-explainer

# 2. 复制 SKILL.md
cp db-relationship-explainer/SKILL.md ~/.cline/skills/db-relationship-explainer/

# 3. 复制二进制
mkdir -p ~/.cline/skills/db-relationship-explainer/tools
cp db-relationship-explainer/tools/dbexplain-linux-amd64 ~/.cline/skills/db-relationship-explainer/tools/
chmod +x ~/.cline/skills/db-relationship-explainer/tools/dbexplain-linux-amd64
```

**部署步骤**（项目级）：

```bash
mkdir -p .cline/skills/db-relationship-explainer
cp db-relationship-explainer/SKILL.md .cline/skills/db-relationship-explainer/
mkdir -p .cline/skills/db-relationship-explainer/tools
cp db-relationship-explainer/tools/dbexplain-linux-amd64 .cline/skills/db-relationship-explainer/tools/
chmod +x .cline/skills/db-relationship-explainer/tools/dbexplain-linux-amd64
```

在 Cline 面板底部点击 Skills 图标，即可查看、切换、创建和管理 Skills。

**关键差异**：Cline Skills 需要 `name` 字段与目录名完全一致。

---

### 3. Continue.dev

Continue.dev 主要通过 `config.yaml` / `config.json` 配置模型、规则和工具，支持自定义 Slash Commands。

配置文件位置：`~/.continue/config.json`

添加自定义 Slash Command 示例：
```json
{
  "slashCommands": [
    {
      "name": "explain-db",
      "description": "使用 dbexplain 分析数据库结构",
      "prompt": "请使用 dbexplain 工具分析以下数据库：$ARGUMENTS"
    }
  ]
}
```

Continue.dev 目前主要通过 MCP 服务器集成外部工具。如需深度集成，建议通过 MCP 协议封装 `dbexplain`。

---

### 4. 其他兼容平台

#### Windsurf / Cursor / Copilot
这些平台可通过项目级 `.rules` 或自定义指令文件实现类似效果。`skills-forge` 工具支持一键安装到 40+ AI 编码代理。

```bash
# 使用 skills-forge 批量部署
pip install skills-forge
skills-forge install output_skills/db-relationship-explainer --target all
```

#### Roo Code
支持 `.roo/rules/` 目录下的规则文件，以及 MCP 工具集成。

#### 手动调用（无需平台集成）
如果只想在终端直接使用，跳过 SKILL.md，直接运行二进制即可，无需部署到任何平台。

---

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

### 验证 Claude Code 是否能发现

在 Claude Code 会话中：
```bash
/skills
```
应列出 `db-relationship-explainer`。

然后输入触发词测试：
> 帮我分析一下 testdb 数据库的表结构

如果 AI 回复中包含了工具输出的表卡片、索引等信息，说明 Skill 已生效。

### 验证 Cline 是否能发现

打开 Cline 面板底部 Skills 菜单，检查 `db-relationship-explainer` 是否已列出并启用。

---

## ⚙️ 自定义 SKILL.md 中的触发词

如果默认触发词不符合你的习惯，可编辑 `SKILL.md` 中的 `trigger` 字段：

```yaml
trigger:
  - "数据库结构"
  - "表关系图"
  - "数据库巡检"
  # 添加你自己的触发词
```

同时确保 `description` 字段清晰描述了 Skill 的用途，因为 Claude Code 和 Cline 都依据 `description` 判断何时自动调用该 Skill。

---

## 📚 工具生态

### skills-forge
`skills-forge` 是 Python 生态中的 Skill 工程化工具链，提供脚手架创建、语法检查、评估测试、打包分发等功能。

```bash
pip install skills-forge
skills-forge create --name db-relationship-explainer
skills-forge lint <skill_dir>
skills-forge pack <skill_dir>
```

### Agent Skills Marketplace
`skillsmp.com` 汇集了大量开源 Skills，可作为设计和编写 SKILL.md 的参考。

---

## ❓ 常见问题

**Q: 二进制文件提示 "Permission denied"？**  
A: 执行 `chmod +x` 赋予执行权限。

**Q: Claude Code 无法发现 Skill？**  
A: 检查 SKILL.md 格式是否正确（YAML frontmatter 必须位于文件开头，以 `---` 包裹）。在会话中执行 `/skills` 确认是否列出。若未列出，重启 Claude Code 会话。

**Q: Cline 中 Skill 未显示？**  
A: 确保已在 Settings → Features 中开启 Enable Skills，且 SKILL.md 中 `name` 字段与目录名完全一致。

**Q: AI 没有响应触发词？**  
A: 检查 `description` 字段是否足够具体（建议 20~200 字），过于模糊的描述可能导致 AI 无法准确判断何时调用。

**Q: 想同时部署多个平台？**  
A: 每个平台目录独立，互不干扰，按需复制即可。也可使用 `skills-forge install --target all` 一键部署。

**Q: SKILL.md 和 CLAUDE.md 有什么区别？**  
A: `CLAUDE.md` 是 Claude Code 的会话级全局指令文件；`SKILL.md` 是模块化、可复用的 Skill 定义文件，支持按需加载，更适合封装特定领域的工具和工作流。

---

## 📚 相关资源

- [dbexplain 项目主页](https://github.com/IamWWT/understand_dbs_skills)
- [源码部署指南](./DEPLOY_SRC.md)
- [MongoDB 排障文档](./MONGO.md)
- [Claude Code Skills 官方文档](https://code.claude.com/docs/en/skills)
- [Cline Skills 文档](https://docs.cline.bot/customization/skills) 
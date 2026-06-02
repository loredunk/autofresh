# Feature Mapping — finding → 产品特性

Use this when writing `recommendations`. **Feature-first**: whenever a finding can be
solved with a native Claude Code / Codex feature, recommend that feature (by name)
before any generic habit advice. Decision: ADR 0006.

Before recommending any configuration/feature, **verify it against current official
docs** (these tools change fast). Only suggest — never auto-change the user's config.

## 映射表

| 观察到的问题 (finding) | 信号 (signals) | 优先建议的产品特性 (feature-first) |
| --- | --- | --- |
| 重复加载 / 低缓存命中 | `cache_hit_rate` 低、many sessions、同项目多 source | 把稳定规则固化到 **CLAUDE.md / AGENTS.md**；保持单会话到阶段边界；新会话用简短交接摘要 |
| 长上下文反复携带 | tokens 高 + `cached_input` 高、少会话、单项目占大头 | **`/compact`** 代替重开；阶段 checkpoint 摘要；用 **`@文件`引用** 代替反复检索；**memory** |
| 工具循环 / 探索漂移 | shell·web ≫ 改文件、`rg/sed/find` 占多、少测试 | **plan mode** 先规划；**subagents** 做受限探索；提前给路径 / 失败命令 / 验收标准 |
| 改完不验证 | 有改文件、无测试命令、却有 CI/构建系统 | **hooks**（SessionStart / PostToolUse 自动跑测试 / typecheck）；测试命令写进 **AGENTS.md / CLAUDE.md** |
| 权限打断多 | `permission_modes` 频繁切换 / 频繁批准 | **`settings.json` 权限 allowlist** 与 **permission 模式**；Codex **`approval_policy` / `sandbox_mode`** |
| reasoning 占比高 / 成本偏高 | `reasoning_ratio` 高、`estimated_cost_usd` 高、贵模型干简单活 | Codex **`model_reasoning_effort` / `model_verbosity`** 调档；简单任务换**小模型**（Haiku / gpt-mini）；按需 **extended thinking** |
| 重复样板提示 | `prompt_signals` 显示同类提示反复、低 `structured_ratio` | 沉淀为 **skills / 自定义 slash 命令**；常用约定写进 **CLAUDE.md** |
| web 搜索偏多 | `web` 类别占比高 | 配 **MCP servers**；把文档放进仓库 / 预置上下文，减少现搜 |
| 提示模糊 / 频繁纠偏 | `correction_rate` 高、低 `constraint_ratio`、低 `file_ref_ratio` | 用 **plan mode** 先对齐；提示里给 **`@文件`引用 + 约束 + 验收标准**；先让 agent 复述再动手 |

> 该表为代表性映射，非穷举；随官方特性演进维护。计分卡的「Prompt 功力」轴正是基于
> `prompt_signals` 的这些数值（见 `scripts/scorecard.py`）。

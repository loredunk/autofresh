# ADR 0006 — 建议优先映射到产品特性（feature-first）

> 状态：已接受（已实现） · 日期：2026-06-02 · 相关：[ADR 0004](0004-skills-based-analysis.md)、
> [ADR 0005](0005-tiered-analysis-and-signals.md)、[`PRD.md`](../PRD.md) §3、[`TODO.md`](../TODO.md) T6

## 背景

本产品的根本目的是**让用户更好地用上 Claude Code / Codex 的特性**。因此分析给出的建议，
凡是能用产品**原生特性**解决的，就应当落到具体特性上，而不是停留在泛泛的「拆分会话 /
写好提示」。现有 [`references/insight-patterns.md`](../../skills/ai-usage-html-report/references/insight-patterns.md)
的 intervention 已零散提到 AGENTS.md、skills、工作拆分，但没有系统化为「问题 → 特性」的映射。
本 ADR 把「特性优先」确立为建议的组织原则。

## 决策

### D1 — 建议排序：先特性，后习惯

每条建议优先给「能用原生特性解决」的具体做法（点名特性与配置），再补「靠习惯 / 提示」的做法。

### D2 — finding → feature 映射（代表性，后续落到 skill reference）

| 观察到的问题（finding） | 优先建议的产品特性（feature） |
| --- | --- |
| 重复加载 / 低缓存命中、长上下文反复携带 | 把稳定规则固化到 **CLAUDE.md / AGENTS.md**；用 **`@文件`引用** 代替反复检索；用 **`/compact`** 代替重开会话；**memory** |
| 工具循环 / 探索漂移（shell·web 远高于改文件） | **plan mode** 先规划；用 **subagents** 做受限探索；提前给路径 / 失败命令 / 验收标准 |
| 改完不验证（有改文件、无测试命令） | **hooks**（SessionStart / PostToolUse 自动跑测试 / typecheck）；把规范测试命令写进 **AGENTS.md / CLAUDE.md** |
| 权限打断多 / 频繁批准 | **`settings.json` 权限 allowlist** 与 **permission 模式**；Codex **`approval_policy` / `sandbox_mode`** |
| reasoning 占比高 / 成本偏高 | Codex **`model_reasoning_effort` / `model_verbosity`** 调档；简单任务换**小模型**（Haiku / gpt-mini）；按需开 **extended thinking** |
| 重复样板提示 / 同类任务反复手敲 | 沉淀为 **skills / 自定义 slash 命令**；常用约定写进 **CLAUDE.md** |
| web 搜索偏多 | 配 **MCP servers**；把文档放进仓库 / 预置上下文，减少现搜 |

> 此表为代表性映射，非穷举；实现时落到 skill 的 reference 文档并随官方特性演进维护。

### D3 — 时效性：建议前核对官方文档

产品特性变化快。给出任何特性 / 配置建议前，按现有 skill「Analysis Guidance」的要求**联网核对最新
官方 Claude Code / Codex 文档**，不依赖既有记忆。本 ADR 强化这条为硬性前置。

### D4 — 不越权：只建议，不自动改配置

skill 只产出建议，由用户确认后自行实施；不自动写 settings / AGENTS.md / 改保活计划
（沿用 [ADR 0002](0002-ai-analyzed-usage-report.md) D3、[ADR 0004](0004-skills-based-analysis.md)）。

## 后果

- 好处：建议可执行、贴合产品能力，真正帮用户「用好特性」，而非泛泛说教。
- 代价：需维护 finding→feature 映射并随特性演进更新；每次给配置建议都要核对文档（D3）。
- 影响：[`references/insight-patterns.md`](../../skills/ai-usage-html-report/references/insight-patterns.md)
  的 intervention 需系统化为特性映射；可能新增一份 feature-mapping 参考（见 TODO T6）。
</content>

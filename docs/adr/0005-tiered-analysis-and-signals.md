# ADR 0005 — 分层分析（会话 / 项目 / 全局）与信号选择

> 状态：已接受（已实现） · 日期：2026-06-02 · 相关：[ADR 0004](0004-skills-based-analysis.md)（细化其分析方式）、
> [`PRD.md`](../PRD.md) §3、[`TODO.md`](../TODO.md) T5

## 背景

[ADR 0004](0004-skills-based-analysis.md) 定下「CLI 出数据、skill 教 agent 给建议」。但分析的
**粒度**与**信号来源**没有展开。用户希望分析能分三个层级——会话级、项目级、全局级——并且
分析应建立在 **user prompt + permission + tool 调用** 上，而**不读取 assistant 回复**，以大幅
降低上下文。本 ADR 把这三层与信号选择固定下来。

按用户决策：三个层级作为现有 `skills/ai-usage-html-report/` skill 的**模式**承载（演进现有
skill，而非新建或拆分）。

## 决策

### D1 — 三个分析层级（同一 skill 的三种 scope）

| scope | 视角 | 数据定位 |
| --- | --- | --- |
| **会话级 session** | 当前这一次会话 | 在会话中插入 skill 即时分析；agent 本就持有当前会话上下文，无需额外读盘 |
| **项目级 project** | 单个项目跨会话 | Claude Code：`~/.claude/projects/<cwd 编码目录>/` 下该项目全部会话 jsonl；Codex：按 repo/cwd 过滤 rollout（复用 `scripts/session_drilldown.py --repo`） |
| **全局级 global** | 跨所有项目/时间窗口 | 复用 `scripts/collect_claude_behavior.py` + `autofresh report`（现有全局聚合路径） |

- **理由**：会话级回答「我这次用得好不好」，项目级回答「这个项目里我的习惯如何」，全局级回答
  「我整体把额度花在哪、怎么省」。粒度不同、行动项也不同。

### D2 — 信号选择：user prompt + permission + tool 调用，不读 assistant 回复

分析只基于三类信号：用户提示（user prompt）、权限（permission 模式/决策）、工具调用（tool calls）。
**绝不读取或导出 assistant 的回复文本。**

- **理由**：
  - assistant 回复体量大，塞进上下文成本高；
  - 对「人如何驱动工具」这件事，回复的诊断价值远低于「人怎么提问、给了什么权限、触发了哪些工具」；
  - 去掉回复后上下文显著变小，会话级「插入即分析」才可行。

### D3 — prompt 读取边界（分层）

- **会话级 / 项目级**：**可读 user prompt**，但限定：仅本机、用户发起、**转述 + 脱敏**、
  绝不逐字成片堆叠、绝不读 assistant 回复。复用 [`references/session-prompt-review.md`](../../skills/ai-usage-html-report/references/session-prompt-review.md)
  的脱敏与改写框架。
- **全局级**：**保持纯聚合**，沿用 [`scripts/collect_claude_behavior.py`](../../skills/ai-usage-html-report/scripts/collect_claude_behavior.py)
  的零 prompt 文本策略（只出计数/类别/扩展名/仓库 basename 等聚合量）。
- **理由**：全局层不需要 prompt 文本就能给统计与趋势；只有要诊断「提示质量」时（会话/项目层）
  才需要 user prompt，且必须脱敏转述。这与现有 skill「Claude Code 纯聚合、Codex 选定会话才按需读
  user prompt」的保守姿态一致，只是把「可读 user prompt」明确扩展到会话/项目层。

### D4 — 数据源

- Claude Code：`~/.claude/projects/**/*.jsonl`（用 cwd 编码目录定位 project；用当前 session 定位会话）。
- Codex：rollout JSONL / `state_*.sqlite`（复用 `parse.go` 与 `session_drilldown.py`）。
- **禁用**：`~/.claude/stats-cache.json`（已被现有 skill 标注为陈旧/错误，沿用禁用）。

## 待定（Open Questions）

- **OQ1**：会话级如何精确定位「当前会话」——优先用 agent 已有上下文，还是定位最新 session jsonl / session id？
- **OQ2**：permission 信号在 Claude Code jsonl 中的具体字段/事件（现有 `collect_claude_behavior.py`
  已取 `permission_modes`，是否够用）。
- **OQ3**：Codex 的「项目维度」过滤口径（按 cwd 还是 git_origin_url）。

## 后果

- 好处：三层覆盖「这次/这个项目/整体」的不同决策需求；去掉 assistant 回复让会话级可即时运行、上下文小。
- 代价：会话/项目层引入 user prompt 读取，需严格脱敏护栏（D3）；定位「当前会话」需实现细节（OQ1）。
- 影响：现有 skill 需新增 scope 维度；PRD §3 增「三层分析与信号模型」一节。
</content>

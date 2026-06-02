# Autofresh 文档

本目录收录 autofresh 的产品与工程文档。约定如下：

- **PRD（产品需求文档）** — 描述「要做什么、为什么做、做到什么程度」。见 [`PRD.md`](PRD.md)。
- **ADR（架构决策记录）** — 记录「关键技术决策及其取舍」，一次决策一篇、只增不改。见 [`adr/`](adr/)。
- **TODO（待办清单）** — 跟踪「接下来要落地的事项」，按优先级与状态滚动维护。见 [`TODO.md`](TODO.md)。

## 文档地图

| 文档 | 作用 | 何时读 / 何时写 |
| --- | --- | --- |
| [`PRD.md`](PRD.md) | 功能的目标、用户故事、范围与验收标准 | 立项 / 改需求时写；动手前读 |
| [`adr/`](adr/) | 不可逆或影响面大的技术选型 | 做关键决策时新增一篇 |
| [`TODO.md`](TODO.md) | 可执行的任务拆解与进度 | 每次推进后更新勾选 |

## 当前主线

近期主线是把现有的 `autofresh report`（只读、脚本友好的本机 Codex 用量报告）升级为
**「脚本采集全局统计 → 大模型语义分析 → 输出建议报告」** 的闭环，覆盖 Claude Code 与 Codex 两侧。

- 需求细节见 [`PRD.md`](PRD.md) 的「AI 用量分析报告」一节。
- 决策背景见 [`adr/0002-ai-analyzed-usage-report.md`](adr/0002-ai-analyzed-usage-report.md)。
- 落地拆解见 [`TODO.md`](TODO.md) 的 **T1** 条目。
</content>
</invoke>

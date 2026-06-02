# ccoach 文档

> 项目原名 **autofresh**（Codex/Claude 保活工具），已剥离保活、聚焦用量分析与建议，更名为
> **ccoach**（本机 AI 用量教练，见 [ADR 0007](adr/0007-drop-keepalive-rebrand-ccoach.md)）。

本目录收录 ccoach 的产品与工程文档。约定如下：

- **PRD（产品需求文档）** — 描述「要做什么、为什么做、做到什么程度」。见 [`PRD.md`](PRD.md)。
- **ADR（架构决策记录）** — 记录「关键技术决策及其取舍」，一次决策一篇、只增不改。见 [`adr/`](adr/)。
- **TODO（待办清单）** — 跟踪「接下来要落地的事项」，按优先级与状态滚动维护。见 [`TODO.md`](TODO.md)。

## 文档地图

| 文档 | 作用 | 何时读 / 何时写 |
| --- | --- | --- |
| [`PRD.md`](PRD.md) | 功能的目标、用户故事、范围与验收标准 | 立项 / 改需求时写；动手前读 |
| [`adr/`](adr/) | 不可逆或影响面大的技术选型 | 做关键决策时新增一篇 |
| [`TODO.md`](TODO.md) | 可执行的任务拆解与进度 | 每次推进后更新勾选 |
| [`scorecard-copy.md`](scorecard-copy.md) | 成绩卡固定文案的中英文案表（i18n） | 设计 / 调整段位文案时维护 |

## 当前主线

ccoach = 本机 AI 用量教练，产品分两块交付，在同一仓库内明确分开：

1. **CLI** — 产出只读、语义化的全局用量数据（默认命令 `ccoach`，即原 `report`）。
2. **skills** — 教 Claude Code / Codex 如何解读 CLI 产物，给出**对人有用的建议**，并渲染
   **可分享成绩卡**。

skill 侧已上线 `skills/ai-usage-html-report/`，后续在其上**演进**：新增**会话 / 项目 / 全局
三层分析**、让建议**优先映射到产品原生特性**、并加一张**可截图的游戏化成绩卡**用于社交传播。
外加一条分发主线：通过 **npm**（`npx ccoach` / `npm i -g ccoach`）让安装更顺手。

- 需求细节见 [`PRD.md`](PRD.md)：§3「AI 用量分析」（含 §3.9 三层分析、§3.10 特性优先建议、§3.11 可分享成绩卡）、§5「分发与安装」。
- 决策背景见下表 ADR。
- 落地拆解见 [`TODO.md`](TODO.md)：**T1**（skills 化分析）、**T5**（三层分析）、**T6**（特性优先建议）、**T7**（可分享成绩卡）、**T4**（npm 分发）。

## ADR 索引

| 编号 | 标题 | 状态 |
| --- | --- | --- |
| [0001](adr/0001-record-architecture-decisions.md) | 采用架构决策记录（ADR） | 已接受 |
| [0002](adr/0002-ai-analyzed-usage-report.md) | AI 用量分析报告：脚本采集 + 本机模型分析 | 部分被 0004 取代 |
| [0003](adr/0003-npm-distribution.md) | 通过 npm 分发，仓库内 CLI 与 skills 分开 | 提议中（包名随 0007 更名） |
| [0004](adr/0004-skills-based-analysis.md) | skills 化分析：CLI 产出数据，skill 教 agent 给建议 | 已接受（已实现） |
| [0005](adr/0005-tiered-analysis-and-signals.md) | 分层分析（会话/项目/全局）与信号选择 | 已接受（已实现） |
| [0006](adr/0006-feature-first-recommendations.md) | 建议优先映射到产品特性（feature-first） | 已接受（已实现） |
| [0007](adr/0007-drop-keepalive-rebrand-ccoach.md) | 剥离保活，聚焦用量分析与建议，更名为 ccoach | 已接受 |
| [0008](adr/0008-gamified-shareable-scorecard.md) | 游戏化等级与可分享成绩卡（病毒传播） | 已接受（已实现） |
| [0009](adr/0009-i18n-scorecard-copy.md) | 成绩卡固定文案的 i18n（人工本地化，zh/en） | 已接受（已实现） |
</content>

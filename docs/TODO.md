# TODO — Autofresh

> 约定：`[ ]` 待办 · `[~]` 进行中 · `[x]` 完成。优先级 P0 > P1 > P2。
> 最近更新：2026-06-02

---

## T1 · AI 用量分析（skills 化）（P0，主线）

> 来源：[`PRD.md`](PRD.md) §3 · 决策：[`adr/0004-skills-based-analysis.md`](adr/0004-skills-based-analysis.md)
> （取代 [ADR 0002](adr/0002-ai-analyzed-usage-report.md) 的「二进制内调模型 / `advise` 子命令」）
>
> **核心目标**：CLI 产出**语义化全局统计**，由 **skill** 教 Claude Code / Codex 解读并给出
> 对人有用的建议。职责：CLI 出「料」，skill 出「解读」。

### T1.1 CLI 数据（出「料」）
- [ ] 复用 `codexreport.Build()` 产出 Codex 侧聚合。
- [ ] **调研** Claude Code 本地用量数据源（路径 / 格式 / 可读性）—— 影响数据结构（ADR 0004 OQ1）。
- [ ] 让 `report --json` 字段**自描述**：每个指标附口径说明（如 `cache_hit_rate` 含义、越高越省）。
- [ ] 评估是否需要更偏「喂模型」的 `report --digest`（ADR 0004 OQ2）。
- [ ] 头部固化口径声明：仅本机数据 / 成本为估算 / 不含配额百分比。
- [ ] 采集逻辑与任何模型解耦，**可在无模型环境下单测**（沿用 Build/Run 分离模式）。

### T1.2 skills 内容（出「解读」）
- [ ] 编写分析 skill：触发场景 + 操作步骤（如 `autofresh report --json --days 7`）+ 解读阈值 + 输出模板。
- [ ] 输出模板固定为「结论 / 依据 / 行动项 / 风险与不确定性」。
- [ ] 口径护栏写进 skill：仅本机数据 / 成本为估算 / **禁止编造配额百分比**。
- [ ] 同时适配 Claude Code 与 Codex 的 skills 机制（ADR 0004 D4 / OQ1）。
- [ ] 空态：无数据时引导 agent 给明确提示而非编造。

### T1.3 文档与验收
- [ ] 在 README / README_EN 增补 skills 安装与使用说明。
- [ ] 跑通 PRD §3.7 的全部验收清单。

---

## T4 · npm 分发（P0）

> 来源：[`PRD.md`](PRD.md) §5 · 决策：[`adr/0003-npm-distribution.md`](adr/0003-npm-distribution.md)
>
> **核心目标**：`npx autofresh` / `npm i -g autofresh` 开箱即用；仓库内 CLI 与 skills 分开发布。

### T4.1 仓库结构
- [ ] 落地 monorepo：`packages/cli/`（`autofresh`）+ `packages/skills/`（`@autofresh/skills`），npm workspaces。
- [ ] 终定包名 / scope（ADR 0003 OQ1）。

### T4.2 CLI 二进制分发
- [ ] CLI 主包 `bin` 薄包装：运行时解析并 exec 对应平台二进制。
- [ ] 平台专属子包 + `optionalDependencies` + `os`/`cpu`（esbuild 式），定平台矩阵（ADR 0003 OQ2）。
- [ ] CI 跨平台构建 Go 二进制并发布各平台子包；npm 与 Release 同源、带 checksum（ADR 0003 D4）。

### T4.3 skills 安装
- [ ] `@autofresh/skills` 可直接 `npm i` 安装。
- [ ] `autofresh skills install`：探测并复制到 Claude Code / Codex 的 skills 目录（ADR 0003 OQ3 / 0004 OQ1）。

---

## T2 · 报告能力增强（P1）
- [ ] 多机/多账号场景下，文档进一步澄清「仅本机」边界，避免用户误解为全局账单。

## T3 · 工程基建（P2）
- [ ] 为 `docs/` 加一条 CI 检查：ADR 文件名编号唯一、状态字段合法。
- [ ] skills 的回归用例：对固定 `report --json` 样本，断言 skill 引导出的建议不越界（不含配额）。

---

## 已完成
- [x] 建立 `docs/`：PRD / ADR / TODO 体系（2026-06-02）。
- [x] 规划 npm 分发与 skills 化分析（ADR 0003 / 0004，2026-06-02）。
</content>

# TODO — Autofresh

> 约定：`[ ]` 待办 · `[~]` 进行中 · `[x]` 完成。优先级 P0 > P1 > P2。
> 最近更新：2026-06-02

---

## T1 · AI 用量分析报告（P0，主线）

> 来源：[`PRD.md`](PRD.md) §3 · 决策：[`adr/0002-ai-analyzed-usage-report.md`](adr/0002-ai-analyzed-usage-report.md)
>
> **核心目标**：用脚本查询 Claude Code / Codex 的**全局用量统计**，把结果以**语义化结构**
> 返回给大模型，让模型分析后给出**建议报告**。即 `collect → digest → advise` 闭环。

### T1.1 采集（collect）
- [ ] 复用 `codexreport.Build()` 产出 Codex 侧聚合，作为 digest 的输入。
- [ ] **调研** Claude Code 本地用量数据源（路径 / 格式 / 可读性）—— 影响 digest 结构（OQ2）。
- [ ] 明确「全局」口径：默认窗口、是否支持 `--days/--since/--date`（沿用 report 的窗口语义）。

### T1.2 语义化（digest）
- [ ] 设计 digest JSON：每个指标自带**释义与口径说明**（如 `cache_hit_rate` 含义、越高越省）。
- [ ] digest 生成逻辑与 LLM 调用解耦，**可在无模型环境下单测**（沿用 Build/Run 分离模式）。
- [ ] 在 digest 头部固化口径声明：仅本机数据 / 成本为估算 / 不含配额百分比。

### T1.3 分析（advise）
- [ ] 经 `internal/provider` 调用本机 LLM（`codex exec` / `claude -p`），不引入第三方网络依赖。
- [ ] 设计分析 Prompt：要求输出「结论 / 依据 / 行动项 / 风险与不确定性」，并**禁止编造配额百分比**。
- [ ] `--target`（分析谁）与 `--provider`（用谁分析）解耦（ADR D4）。

### T1.4 CLI 与呈现
- [ ] 定命令形态：`autofresh advise` 子命令 vs `report --advise`（OQ1，倾向前者）。
- [ ] `--dry-run`：只打印「将发送给模型的内容」，**不调用模型**（隐私预览）。
- [ ] `--json`：输出结构化建议；默认渲染可读文本。
- [ ] 空态：无用量数据时给明确提示而非报错。
- [ ] 缺少对应 CLI 时复用 provider 的可操作失败文案（`LookPath` 预检）。

### T1.5 文档与验收
- [ ] 在 README / README_EN 增补 `advise` 用法与隐私说明。
- [ ] 跑通 PRD §3.7 的全部验收清单。

---

## T2 · 报告能力增强（P1）
- [ ] 评估在 `report` 中也输出 digest（`--digest`），让外部脚本可直接拿语义化数据自行喂模型。
- [ ] 多机/多账号场景下，文档进一步澄清「仅本机」边界，避免用户误解为全局账单。

## T3 · 工程基建（P2）
- [ ] 为 `docs/` 加一条 CI 检查：ADR 文件名编号唯一、状态字段合法。
- [ ] 评估 `advise` 的离线测试夹具（mock provider，断言发送内容与渲染）。

---

## 已完成
- [x] 建立 `docs/`：PRD / ADR / TODO 体系（2026-06-02）。
</content>

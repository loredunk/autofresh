# ADR 0004 — skills 化分析：CLI 产出数据，skill 教 agent 给建议

> 状态：已接受（已实现） · 日期：2026-06-02 · 取代 [ADR 0002](0002-ai-analyzed-usage-report.md) 的 D2/D4 与 `advise` 子命令
> · 相关：[`PRD.md`](../PRD.md) §3、[`TODO.md`](../TODO.md) T1

## 背景

[ADR 0002](0002-ai-analyzed-usage-report.md) 设想由 autofresh 二进制**自己**调用本机 LLM
（`codex exec` / `claude -p`）来生成建议报告（`advise` 子命令）。

但用户的真实使用场景是：本来就在 **Claude Code / Codex** 里工作。与其让 CLI 再去拉起一个
模型，不如把「如何分析」沉淀成 **skill**——agent 自己运行 CLI、按 skill 指引解读输出并给建议。
这正是用户的诉求：「仓库分两部分，一个装 CLI，一个装 skills；skills 本质是教 codex /
claude code 如何分析 CLI 生成的内容，给出对人类有用的建议。」

## 决策

### D1 — 职责切分：CLI 出「料」，skill 出「解读」

- **CLI 职责**：产出**自描述、语义化、稳定**的结构化数据（`report --json`，规划中的
  `--digest`），让任何 agent 无需额外上下文即可读懂每个字段的口径。
- **skill 职责**：以自然语言指令教 agent——何时运行哪条命令、如何解读各指标、
  输出怎样结构的「对人有用的建议」。
- **沿用 0002**：D1（采集/分析解耦）、D3（隐私/可预览）、D5（口径护栏）继续成立，
  只是护栏由 skill 指令承载。

### D2 — 不在二进制内调用 LLM（取代 0002 D2）

autofresh **不**再通过 `internal/provider` 为「分析」目的调用模型；provider 仅服务于
保活 ping。分析完全发生在用户已在用的 agent 侧。

- **理由**：避免「agent 调 CLISP，CLI 再调 agent」的套娃；减少二进制的网络/凭证面；
  分析所用模型天然就是用户当前 agent，无需 0002 D4 的 `--provider` 解耦。
- **后果**：取消 `autofresh advise` 子命令的规划；T1 的「分析」从写 Go 代码变为写 skill 内容。

### D3 — skill 内容结构

每个 skill 至少包含：
- **触发场景**：用户问「我的用量/花销怎么样、怎么省」等。
- **操作步骤**：建议 agent 运行的命令（如 `autofresh report --json --days 7`）。
- **解读指南**：各指标含义与经验阈值（如缓存命中率偏低 → 提示复用上下文）。
- **输出模板**：结论 / 依据 / 行动项 / 风险与不确定性。
- **口径护栏**：强制声明「仅本机数据 / 成本为估算 / 不得编造配额百分比」（沿用 0002 D5）。

### D4 — 双目标：同时面向 Claude Code 与 Codex

skill 以两边都能消费的形式提供（内容主体共享，按各自 skills 机制做最小适配）。
落地路径与格式差异在实现时确认。

## 待定（Open Questions）

- **OQ1**：Claude Code 与 Codex 各自的 skills 机制/目录与文件格式（决定 `@autofresh/skills`
  的产物形态，与 [ADR 0003](0003-npm-distribution.md) D3 联动）。
- **OQ2**：是否仍需要 `report --digest`（比 `--json` 更偏「喂模型」的精简语义包），
  还是 `--json` 已足够给 agent 消费。
- **OQ3**：skill 是否要覆盖「读完建议后帮用户改保活计划」，还是只给建议、由人确认。

## 后果

- 好处：贴合用户真实工作流（就在 agent 里）；二进制更瘦、面更小；分析逻辑用自然语言迭代成本低。
- 代价：依赖各 agent 的 skills 机制；建议质量取决于 skill 写得好不好（需打磨与示例）。
- 影响：PRD §3 的交付物从「`advise` 命令」改为「`@autofresh/skills` + 更好的 `report` 数据」。
</content>

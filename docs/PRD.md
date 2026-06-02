# PRD — ccoach

> 状态：草拟中 · 最近更新：2026-06-02
> · 更名：原 **autofresh**（保活工具）已剥离保活、聚焦用量分析与建议，更名为 **ccoach**（见 [ADR 0007](adr/0007-drop-keepalive-rebrand-ccoach.md)）。

本 PRD 覆盖 ccoach 的整体定位，并重点展开 **AI 用量分析与建议** 能力，以及 **可分享成绩卡**。

---

## 1. 产品定位

ccoach 是一个跨平台（macOS / Linux）的 **本机 AI 用量教练**：只读分析你在 Claude Code / Codex
上的用量，告诉你**花在哪、哪里浪费、怎么用得更好**，并以**可分享的成绩卡**把结果变成社交货币。

1. **用量分析（已上线）**：只读本机记录，输出 Token、成本、工具调用、仓库 / 时段 / 语言 /
   git 习惯 / 配置扫描等。
2. **使用建议（核心）**：由 skill 教 agent 做**分会话 / 项目 / 全局三层**的语义分析，给出
   **特性优先**的建议（凡能用产品原生特性解决的就点名特性去解决）。
3. **可分享成绩卡（传播）**：把用量 / 习惯 / prompt 评成多轴段位，做一张可截图、能炫能自嘲的
   成绩卡（见 §3.11、[ADR 0008](adr/0008-gamified-shareable-scorecard.md)）。

> **不再做保活**：原 autofresh 的 launchd/crontab 保活 ping 已移除（ADR 0007）。

目标用户：重度使用 Claude Code / Codex 的个人开发者，希望「花得值、用在刀刃上」，并乐于分享战绩。

---

## 2. 现状（基线能力）

| 能力 | 命令 | 说明 |
| --- | --- | --- |
| 本机用量报告（默认命令） | `ccoach [--json --days N --since … --date … --by-repo]` | 只读 `~/.codex` rollout，输出 Token / 成本 / 工具 / 仓库 / 时段 / 来源 / 语言 / git 习惯 / 配置扫描；裸命令即出报告，`ccoach report …` 亦可 |
| 双平台 AI 使用报告 skill | `skills/ai-usage-html-report/` | 已上线：用 ccusage + `ccoach report --json` 数据，产出 Claude Code + Codex 双平台 HTML 报告、行为画像，并支持 Codex 高耗会话钻取 |

`ccoach --json` 已经是「脚本友好」的结构化输出（见
[`internal/codexreport/report.go`](../internal/codexreport/report.go) 的 `Report` 结构体，
已含 `Repos / Hours / Sources / Languages / Git / Project / Codex` 等行为维度，由
[`habits.go`](../internal/codexreport/habits.go) / [`language.go`](../internal/codexreport/language.go) /
[`configscan.go`](../internal/codexreport/configscan.go) 产出），是 AI 分析能力的天然数据底座。
本次增强是在**已上线的 `ai-usage-html-report` skill** 之上演进，而非从零新建。

### 现状边界（PRD 需尊重的约束）

- 报告**只反映本机**：同账号多机登录时 rollout 按机器隔离，不跨机器汇总。
- 不输出任何**配额百分比**（CLI 下 `rate_limits` 恒为 null，且配额是账号级、跨机器的）。
- 成本为**估算值**（token × 内置参考价），不等于实际账单。

---

## 3. 新需求：AI 用量分析报告

### 3.1 一句话需求

> 用脚本采集 Claude Code / Codex 的**全局用量统计**，把结果以**语义化结构**喂给大模型，
> 让模型分析并给出一份**可执行的建议报告**。

### 3.2 问题陈述

`report` 当前只「呈现数字」，用户仍需自行解读：缓存命中率是否偏低？reasoning 占比是否过高？
某仓库是否在烧钱？保活窗口是否对齐了真实活跃时段？这一步「从数字到结论」的解读，
正是大模型擅长、且对用户价值最高的部分。

### 3.3 用户故事

- 作为重度用户，我想运行一条命令就得到「**本周我的 Token 都花在哪、哪里浪费、怎么省**」的结论，而不只是表格。
- 作为多工具用户，我想同时看到 **Claude Code 与 Codex** 两侧的统计被放在一起对比与点评。
- 作为注重隐私的用户，我想**自己决定**把哪些数据发给模型，并能先**预览将要发送的内容**。

### 3.4 方案概述（skills 化）

> 方案已调整：不再由 autofresh 二进制自己调模型，而是 **CLI 出数据 + skill 教 agent 解读**。
> 决策见 [`adr/0004-skills-based-analysis.md`](adr/0004-skills-based-analysis.md)
> （取代 [ADR 0002](adr/0002-ai-analyzed-usage-report.md) 的「二进制内调模型 / `advise` 子命令」）。

职责切分如下：

```
CLI（产出料）                         skill（产出解读）
report --json / --digest   ──喂──►   agent(Claude Code / Codex) 按 skill 指引解读 ──► 对人有用的建议
自描述、语义化、稳定的数据             触发场景 + 操作步骤 + 解读阈值 + 输出模板 + 口径护栏
```

1. **CLI**：把全局统计输出成**自描述、语义化、稳定**的结构化数据——每个指标自带口径说明
   （如 `cache_hit_rate=缓存输入/总输入，越高越省钱`），让 agent 无需额外上下文即可读懂。
   Codex 侧复用现有聚合；Claude Code 侧数据源待调研（TODO T1.1）。
2. **skill**：用自然语言教 agent——何时运行哪条命令、如何解读各指标、输出怎样的建议。
   skill 是产品的第二部分交付物，**在已上线的 `ai-usage-html-report` skill 上演进**
   （非新建），独立打包为 `@autofresh/skills`（见 §5、ADR 0003）。
3. **使用**：用户在自己常用的 Claude Code / Codex 里直接问「我的用量怎么样、怎么省」，
   agent 运行 CLI、按 skill 给出结论。分析所用模型天然就是用户当前 agent。

> 本节方案进一步细化为 §3.9「三层分析与信号模型」与 §3.10「特性优先建议」。

### 3.5 skill 内容草案（待实现细化）

每个 skill 至少包含：

- **触发场景**：用户询问用量 / 花销 / 如何省额度 / 保活窗口是否合理。
- **操作步骤**：建议运行的命令，如 `autofresh report --json --days 7`。
- **解读指南**：各指标含义与经验阈值（缓存命中率偏低 → 提示复用上下文；reasoning 占比过高 → 提示精简任务）。
- **输出模板**：结论 / 依据 / 行动项 / 风险与不确定性。
- **口径护栏**：强制声明「仅本机数据 / 成本为估算 / **不得编造配额百分比**」。

### 3.6 范围

**In scope（本期）**
- 增强 CLI 数据：让 `report --json`（及可能的 `--digest`）成为 agent 可直接消费的语义化数据。
- 演进 `ai-usage-html-report` skill：新增**会话 / 项目 / 全局三个 scope**（§3.9、ADR 0005）。
- **特性优先建议**：诊断结果优先映射到 Claude Code / Codex 原生特性（§3.10、ADR 0006）。
- **会话 / 项目层可读 user prompt**（转述 + 脱敏）以诊断提示质量；全局层保持纯聚合。
- 隐私护栏写进 skill 指令（仅本机、估算成本、禁配额幻觉、绝不读 assistant 回复）。

**Out of scope（本期不做）**
- 在二进制内调用 LLM / `autofresh advise` 子命令（已被 ADR 0004 取消）。
- **读取 / 导出 assistant 回复文本**（任一 scope 都不读，见 §3.9 信号选择）。
- 跨机器汇总用量（受 rollout 机器隔离约束）。
- 真实账单 / 配额百分比（口径限制，见 §2 边界）。
- skill 自动「改保活计划」或自动改配置（先只给建议，执行由人确认）。

### 3.7 验收标准

- [ ] `report --json`（及 `--digest`，若引入）字段自描述、口径清晰，agent 无需额外上下文即可解读。
- [ ] skill 能引导 agent 产出「结论 / 依据 / 行动项」的建议。
- [ ] skill 支持**按 scope 切换**（会话 / 项目 / 全局），各 scope 数据定位正确（ADR 0005）。
- [ ] 建议**含具体产品特性项**（如 CLAUDE.md / hooks / permission 设置 / effort 档位），而非仅泛泛说教（ADR 0006）。
- [ ] **任一 scope 都不读取 / 不导出 assistant 回复**；**全局层不出现 user prompt 原文**；
      会话 / 项目层若涉及 user prompt，均为转述 + 脱敏。
- [ ] skill 中固化口径护栏，agent 不会输出配额百分比等越界结论。
- [ ] 无用量数据时，skill 指引 agent 给出明确空态提示而非编造。
- [ ] 数据/口径稳定：字段含义不随版本静默漂移（破坏性变更需版本标注）。

### 3.8 非功能性要求

- **隐私优先**：CLI 默认只读、不外发；分析只基于 user prompt / permission / tool 调用，
  **绝不读 assistant 回复**；user prompt 仅在会话 / 项目层、转述 + 脱敏后使用，全局层纯聚合（§3.9、ADR 0005）。
- **离线友好**：CLI 不为分析目的引入网络依赖；分析发生在用户已在用的 agent 侧。
- **可测试**：CLI 的采集 / digest 逻辑与任何模型解耦，可在无模型环境下单测（沿用 `Build()`/`Run()` 分离）。

### 3.9 三层分析与信号模型（已实现）

> 决策见 [`adr/0005-tiered-analysis-and-signals.md`](adr/0005-tiered-analysis-and-signals.md)。

三个 scope 作为 `ai-usage-html-report` skill 的模式：

| scope | 视角 | 数据定位 |
| --- | --- | --- |
| **会话级 session** | 当前这次会话 | 在会话中插入 skill 即时分析；agent 已持有当前会话上下文 |
| **项目级 project** | 单个项目跨会话 | Claude Code：`~/.claude/projects/<cwd 编码目录>/`；Codex：按 repo/cwd 过滤 rollout |
| **全局级 global** | 跨所有项目 / 时间窗口 | 复用 `collect_claude_behavior.py` + `autofresh report` |

**信号选择**：分析只基于 **user prompt + permission + tool 调用**，**不读取 assistant 回复**
（回复体量大、对「人如何驱动工具」诊断价值低，去掉后上下文显著变小，会话级「插入即分析」才可行）。

**prompt 读取边界**：会话 / 项目层可读 user prompt（仅本机、用户发起、转述 + 脱敏、不逐字成片堆叠），
复用 `references/session-prompt-review.md` 框架；全局层保持纯聚合（零 prompt 文本）。

### 3.10 特性优先建议（已实现）

> 决策见 [`adr/0006-feature-first-recommendations.md`](adr/0006-feature-first-recommendations.md)。

产品目的是帮用户**用好 Claude Code / Codex 的特性**，因此建议先映射到原生特性，再补习惯 / 提示类做法。
代表性映射（非穷举，详见 ADR 0006）：

- 重复加载 / 低缓存、长上下文 → CLAUDE.md / AGENTS.md、`@文件`引用、`/compact`、memory。
- 工具循环 / 探索漂移 → plan mode、subagents、提前给路径 / 验收标准。
- 改完不验证 → hooks 自动跑测试、AGENTS.md 固化测试命令。
- 权限打断多 → `settings.json` 权限 allowlist / permission 模式；Codex `approval_policy` / `sandbox_mode`。
- reasoning 占比高 / 贵 → Codex `model_reasoning_effort` / `model_verbosity`、简单任务换小模型、按需 extended thinking。

给配置建议前须**联网核对最新官方文档**；skill 只建议、不自动改配置。

### 3.11 可分享成绩卡（病毒传播）（已实现）

> 决策见 [`adr/0008-gamified-shareable-scorecard.md`](adr/0008-gamified-shareable-scorecard.md)。

既然用 HTML 呈现，就顺势提供**情绪价值 + 社交传播**：把用量评成**多轴独立段位**，做一张可截图的
成绩卡。对标 ccusage 靠「成本计分卡」病毒传播，ccoach 多了**习惯与 prompt** 两个维度，可以玩得更深。

- **多轴独立评级（不做单一总分）**，让每个人在某条轴上都能拿到「能炫或能笑」的标签：
  - **Prompt 功力**（核心、独有）：大师级 / 老练 / 学徒 / 复读机 / 玄学召唤师。
  - **烧钱姿势**（成本 + 值不值）：性价比刺客 / 理性消费 / 富哥随意 / Opus 锤钉子。
  - **工程素养**（git/session 习惯）：架构师 / 工程师 / 莽夫 / 考古学家。
  - **勤奋度**（频率，纯娱乐）：劳模 / 996 战士 / 养生程序员 / 周末才想起来。
- **封面成绩卡**：竖版、适配手机截图、信息密度适中，置于 HTML 顶部；详细分析在下方。
- **人格总结由模型生成**：把几轴合成一句又准又毒的称号（沿用 skills 化，模型在 agent 侧写）。
- **对比钩子**：「超过了 73% 的用户」相对排名；早期用本地估算、标注、后期校准。
- **分寸**：损但不伤人——只吐槽可改变的行为习惯，不攻击能力 / 人格。
- **隐私即卖点**：prompt 全程本地处理、不上传，并在 README / UI 写明（沿用 §3.8 / ADR 0005）。

---

## 4. 度量

- 采纳率：用户看到建议后是否调整了保活计划 / 使用习惯（可由后续 `plan` 变更间接观察）。
- 可信度：建议中是否出现与口径冲突的表述（如声称配额百分比）——目标为 0。

---

## 5. 分发与安装

> 决策见 [`adr/0003-npm-distribution.md`](adr/0003-npm-distribution.md)。

### 5.1 需求

让用户能用最顺手的方式安装：`npx ccoach` 试用、`npm i -g ccoach` 全局安装；
同时把产品的两块交付物——**CLI** 与 **skills**——在同一仓库内清晰分开、各自独立发布。

> 包名随 [ADR 0007](adr/0007-drop-keepalive-rebrand-ccoach.md) 的更名调整：CLI 包 `ccoach`、
> skills 包 `@ccoach/skills`（ADR 0003 中的 `autofresh` / `@autofresh/skills` 为更名前的旧名）。

### 5.2 方案

- **单仓库、两包**：`ccoach`（CLI，包装 Go 二进制）+ `@ccoach/skills`（skills 内容）。
- **CLI 二进制**走「平台专属 optionalDependencies」分发（esbuild 式），对 `npx` 友好、可复现、
  无 postinstall 联网风险。
- **skills 安装便捷化**：除 `npm i @ccoach/skills` 外，提供 `ccoach skills install`
  把 skills 落到 Claude Code / Codex 的 skills 目录。

### 5.3 验收标准

- [ ] `npx ccoach` 可在不预装的情况下直接跑通（当前平台）。
- [ ] `npm i -g ccoach` 后全局可用 `ccoach`。
- [ ] skills 可经 `npm i @ccoach/skills` 或 `ccoach skills install` 两条路径安装。
- [ ] npm 上的二进制与仓库 CI 同一次构建一致、可校验。
- [ ] README / README_EN 增补 npm 安装方式（保留二进制下载为备选）。
</content>

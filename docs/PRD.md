# PRD — Autofresh

> 状态：草拟中 · 最近更新：2026-06-02

本 PRD 覆盖 autofresh 的整体定位，并重点展开正在规划的 **AI 用量分析报告** 能力。

---

## 1. 产品定位

autofresh 是一个跨平台（macOS / Linux）的 **Codex & Claude 用量保活与洞察** 命令行工具：

1. **保活（已上线）**：在工作时段内按 `5h10m` 间隔自动触发 Codex / Claude 的轻量 ping，
   把 5 小时计费窗口「卡」在真正需要用的时段，减少额度浪费。
2. **洞察（部分上线 → 规划增强）**：只读分析本机用量记录，输出 Token、成本、工具调用、
   按仓库 / 时段的统计；下一步加入 **大模型语义分析与建议报告**。

目标用户：重度使用 Claude Code / Codex 的个人开发者，希望「花得值、用在刀刃上」。

---

## 2. 现状（基线能力）

| 能力 | 命令 | 说明 |
| --- | --- | --- |
| 设定保活计划 | `autofresh set 06:00 --target all` | 写入 launchd / crontab |
| 查看 / 诊断计划 | `autofresh plan` / `doctor` | |
| 手动触发 | `autofresh trigger` | 打印模型回复，确认保活生效 |
| 本机 Codex 用量报告 | `autofresh report [--json]` | 只读 `~/.codex` rollout，输出 Token / 成本 / 工具 / 仓库 / 时段 |

`report --json` 已经是「脚本友好」的结构化输出（见
[`internal/codexreport/report.go`](../internal/codexreport/report.go) 的 `Report` 结构体），
是本次 AI 分析能力的天然数据底座。

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

### 3.4 方案概述

复用已有的 provider 调用层（[`internal/provider/provider.go`](../internal/provider/provider.go)
已封装 `codex exec` 与 `claude -p`），新增一条分析流水线：

```
采集脚本 ──► 语义化 JSON（含字段释义/口径说明）──► 组装分析 Prompt ──► 调用本机 LLM ──► 建议报告
 (collect)        (digest)                          (prompt)          (codex/claude)     (advise)
```

1. **采集（collect）**：脚本汇总全局统计。Codex 侧复用 `report` 的聚合；Claude Code 侧
   新增对其本地用量数据的读取（见 TODO T1.1 的调研项）。
2. **语义化（digest）**：把统计打包成**自带释义**的 JSON——每个指标附口径说明（如
   "cache_hit_rate=缓存输入/总输入，越高越省钱"），降低模型误读概率。
3. **分析（advise）**：把 digest 作为上下文，连同一段固定的分析指令发给本机 LLM，
   要求其输出结构化建议（结论 + 依据 + 行动项 + 风险/不确定性）。
4. **呈现**：默认渲染为可读文本；`--json` 时输出结构化建议，便于二次消费。

### 3.5 交互草案（待 ADR/实现细化）

```bash
autofresh advise                      # 默认：今天，本机 Codex + Claude，调用本机模型给出建议
autofresh advise --days 7             # 最近 7 天
autofresh advise --target codex       # 只分析 codex
autofresh advise --provider claude    # 指定用哪个模型来做分析（与被分析对象解耦）
autofresh advise --dry-run            # 只打印「将要发送给模型的内容」，不实际调用（隐私预览）
autofresh advise --json               # 结构化建议输出
```

> 命令名 `advise` vs 复用 `report --advise` 为待定项，见
> [`adr/0002-ai-analyzed-usage-report.md`](adr/0002-ai-analyzed-usage-report.md)。

### 3.6 范围

**In scope（本期）**
- 复用 Codex 侧已有聚合，产出语义化 digest。
- 调用本机 LLM 生成建议报告（文本 + JSON）。
- `--dry-run` 隐私预览；不联网、不上传到第三方服务（只走用户已登录的本机 CLI）。

**Out of scope（本期不做）**
- 跨机器汇总用量（受 rollout 机器隔离约束）。
- 真实账单 / 配额百分比（口径限制，见 §2 边界）。
- 自动按建议「改计划」（先只给建议，执行由用户确认）。

### 3.7 验收标准

- [ ] `autofresh advise` 在有用量数据时输出包含「结论 / 依据 / 行动项」的报告。
- [ ] 无数据时给出明确的空态提示，而非报错或空白。
- [ ] `--dry-run` 能完整展示「即将发送的内容」，且此时**不**调用任何模型。
- [ ] 本机缺少对应 CLI（codex/claude）时给出可操作的错误提示（复用 provider 的失败文案风格）。
- [ ] `--json` 输出可被脚本解析（稳定字段）。
- [ ] 报告显式标注「成本为估算、仅本机数据、不含配额」等口径，避免误导。

### 3.8 非功能性要求

- **隐私优先**：默认不外发；发送内容用户可预览、可裁剪；文档明确说明数据流向。
- **离线友好**：除调用本机已登录的 codex/claude 外，不引入新的网络依赖。
- **可测试**：采集与 digest 逻辑与 LLM 调用解耦，digest 可在无模型环境下单测（沿用现有
  `Build()` 与 `Run()` 分离的模式）。

---

## 4. 度量

- 采纳率：用户看到建议后是否调整了保活计划 / 使用习惯（可由后续 `plan` 变更间接观察）。
- 可信度：建议中是否出现与口径冲突的表述（如声称配额百分比）——目标为 0。
</content>

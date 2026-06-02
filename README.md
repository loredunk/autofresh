# ccoach

<p align="center"><a href="README_EN.md">English README</a></p>

> 本机 AI 用量教练（macOS / Linux）。只读分析你在 **Codex / Claude Code** 上的用量，
> 告诉你**花在哪、哪里浪费、怎么用得更好**，并把结果做成**可分享的成绩卡**。
>
> 本项目原名 **autofresh**（Codex/Claude 保活工具），已剥离保活、聚焦用量分析与建议，
> 更名为 **ccoach**。设计与决策见 [`docs/`](docs/)（PRD / ADR / TODO）。

## 它能做什么

- **用量报告**：只读本机 `~/.codex` rollout，输出 Token、估算成本、工具调用、按仓库 / 时段 /
  来源 / 语言 / git 习惯 / 配置扫描的统计。纯只读，不改任何东西。
- **使用建议**（skill）：教 Claude Code / Codex 解读这些数据，给出**特性优先**的建议——
  凡能用产品原生特性（CLAUDE.md/AGENTS.md、subagents、hooks、plan mode、permission 设置、
  模型/effort 档位…）解决的，就点名特性去解决。支持**会话 / 项目 / 全局**三层分析。
- **可分享成绩卡**：把用量 / 习惯 / prompt 评成多轴段位（Prompt 功力、烧钱姿势、工程素养、
  勤奋度），在 HTML 报告顶部生成一张能炫能自嘲、可截图的成绩卡（中英双语，由 skill 渲染）。

> **隐私**：所有分析在你本地完成，prompt 内容不离开你的机器。

## 安装

### 从源码编译（当前可用）

标准 Go 模块，入口 [cmd/ccoach/main.go](cmd/ccoach/main.go)，要求 Go 1.22+：

```bash
go build -o ccoach ./cmd/ccoach
```

> npm 分发（`npx ccoach` / `npm i -g ccoach`）与预编译二进制为规划项，见 [docs/TODO.md](docs/TODO.md) T4。

## 用法

报告是默认命令——裸命令即出今天的用量报告：

```bash
./ccoach                      # 今天本机的用量报告
./ccoach --date 2026-05-13    # 指定某一天
./ccoach --since 2026-05-01   # 从某天起到今天
./ccoach --days 7             # 最近 7 天
./ccoach --by-repo            # 按 git 仓库展开（含分支）
./ccoach --json               # 输出 JSON，脚本 / agent 友好
```

> 为兼容习惯，`./ccoach report --json …` 也仍可用（`report` 为可选前缀）。

## 使用建议 skill

更深入的 AI 解读与 HTML 报告，用可复用的 skill
[skills/ai-usage-html-report](skills/ai-usage-html-report/SKILL.md)：它用 `ccoach report --json`
（Codex）+ ccusage（Claude Code）的本机数据，产出双平台 HTML 报告、行为画像，并能从高耗项目
下钻到候选会话；只在你明确授权后才读取所选会话的 user prompt，且绝不读取隐藏的系统提示。

## 说明与边界

- **只反映本机**：同账号多机登录时 rollout 按机器隔离，本工具只读本机文件、不跨机器汇总。
- **不输出配额百分比**：CLI 下 `rate_limits` 恒为 null，且配额是账号级、跨机器的。
- **成本为估算值**（token × 内置参考价），不等于实际账单。
- 时间窗口按本机时区的绝对日期边界划分，报告头部会标明时区。
</content>

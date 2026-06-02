# ADR 0007 — 剥离保活，聚焦用量分析与建议，更名为 ccoach

> 状态：已接受 · 日期：2026-06-02 · 相关：[`PRD.md`](../PRD.md)、[`TODO.md`](../TODO.md)
> · 取代项目此前的「保活（keepalive）」定位

## 背景

项目最初是 **autofresh**：在工作时段按 `5h10m` 间隔触发 Codex / Claude 保活 ping，
把 5 小时计费窗口卡在需要的时段。保活之外，逐步长出了更有价值的一块——**只读用量分析**
（`report` + `internal/codexreport`：token / 成本 / 工具 / 仓库 / 时段 / 语言 / git 习惯 / 配置扫描）
与基于它的 **skill 建议**（`skills/ai-usage-html-report/`）。

产品重心已经转移：用户真正需要的是「**帮我用好 Claude Code / Codex、给我使用建议**」，
而不是保活。保活功能（launchd/crontab 调度、provider ping、schedule、config、logging）
与这个新定位无关，反而增大了二进制面与维护成本。

## 决策

### D1 — 移除保活，只保留用量分析与建议

删除以下 Go 包（仅服务于保活）：
`internal/platform`（launchd/crontab）、`internal/provider`（codex/claude ping）、
`internal/schedule`（`5h10m` 间隔）、`internal/config`（起始时间/target）、
`internal/logging`（触发日志），以及只用于编排它们的 `internal/app`。

保留并作为核心：`internal/codexreport`（独立、零外部依赖）与 `internal/cli`。

### D2 — 更名为 ccoach（完整鲁班）

二进制、go module、`cmd/` 目录、README、skill 内的命令调用全部由 `autofresh` 改为 `ccoach`。
`ccoach` = AI coding **coach**：定位为「本机 AI 用量教练」。

### D3 — `report` 作为默认命令

裸命令即出报告：`ccoach [--json --days N --since … --date … --by-repo]`，不必再打 `report`。
为兼容习惯与现有 skill 调用，保留**可选**的前导 `report` token（`ccoach report …` 仍可用）。

### D4 — 删除保活相关命令

移除 `set` / `delete` / `plan` / `trigger` / `run` / `doctor` / `logs`；CLI 只剩报告这一条路径。

## 后果

- 好处：定位清晰（用量分析 + 建议）；二进制更瘦、依赖面更小；CLI 与测试大幅简化。
- 代价：保活用户需另寻方案（本工具不再提供）；README / skill / 文档需同步改名。
- 兼容：`go build ./... && go test ./...` 通过；`ccoach` 裸命令与 `ccoach report …` 行为一致。
- 影响：PRD 定位段、README/README_EN、`skills/ai-usage-html-report/` 内的 `autofresh report`
  调用、docs 全面改为 ccoach。
</content>

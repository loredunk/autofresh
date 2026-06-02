# TODO — ccoach

> 约定：`[ ]` 待办 · `[~]` 进行中 · `[x]` 完成。优先级 P0 > P1 > P2。
> 最近更新：2026-06-02

---

## T1 · AI 用量分析（skills 化）（P0）— ✅ 已完成

> 决策：[`adr/0004-skills-based-analysis.md`](adr/0004-skills-based-analysis.md)（已接受）
> · skill 在已上线的 `skills/ai-usage-html-report/` 上演进。

- [x] 复用 `codexreport.Build()` 产出 Codex 侧聚合。
- [x] Claude Code 数据源：`~/.claude/projects/**/*.jsonl`（`collect_claude_behavior.py`）。
- [x] `ccoach --json` 字段**自描述**：`report.go` 新增 `glossary`（各指标口径 + 仅本机 / 不含配额声明）。
- [x] 采集逻辑与模型解耦，`go test` 覆盖；skill 输出模板「结论/依据/行动项/风险」+ 空态护栏（见 SKILL.md）。
- [ ]（可选，未做）更偏「喂模型」的 `--digest`：当前 `--json` + `glossary` 已够 agent 消费。

## T5 · 三层分析（会话 / 项目 / 全局）（P0）— ✅ 已完成

> 决策：[`adr/0005-tiered-analysis-and-signals.md`](adr/0005-tiered-analysis-and-signals.md)（已接受）

- [x] `collect_claude_behavior.py --scope {global,project,session}`：global 聚合 / `projects[]` / `sessions_detail[]`。
- [x] 会话级当前会话直接分析；项目级定位 `~/.claude/projects/<编码目录>`；Codex 复用 `session_drilldown.py --repo`。
- [x] 信号只用 user prompt + permission + tool 调用，**不读 / 不导出 assistant 回复**。
- [x] prompt 边界：会话/项目层读 user prompt 仅产出聚合 `prompt_signals`（不落原文、脱敏）；全局层零 prompt 文本。

## T6 · 特性优先建议（P0）— ✅ 已完成

> 决策：[`adr/0006-feature-first-recommendations.md`](adr/0006-feature-first-recommendations.md)（已接受）

- [x] 新增 `references/feature-mapping.md`（finding → 产品特性表，覆盖 ADR 0006 D2）。
- [x] `insight-patterns.md` 与 SKILL.md「Analysis Guidance」接入：建议先点名特性、给配置建议前联网核对官方文档、只建议不自动改配置。

## T7 · 可分享成绩卡 + i18n（P1）— ✅ 已完成

> 决策：[`adr/0008-gamified-shareable-scorecard.md`](adr/0008-gamified-shareable-scorecard.md) / [`0009`](adr/0009-i18n-scorecard-copy.md)（均已接受）

- [x] 四轴段位与阈值（Prompt 功力 / 烧钱姿势 / 工程素养 / 勤奋度）：`scripts/scorecard.py`。
- [x] `references/scorecard-copy.json`：zh/en 段位名 + 吐槽语 + UI 标签（人工本地化，非直译）。
- [x] 渲染层封面成绩卡（竖版、可截图、置顶）：`render_dual_platform.py --scorecard --lang zh|en`。
- [x] 相对排名「超过 X% 用户」：本地估算并标注 estimate；分寸=只损习惯不伤人（文案表）。
- [ ]（后续）相对排名用真实大盘校准；称号整段由模型按语言现写（已在 SKILL.md 指明，运行时完成）。

## T2 · 边界澄清（P1）— ✅ 已完成
- [x] README / README_EN / PRD §2 统一「仅本机、不跨机器汇总、成本为估算、不含配额」措辞。

## T3 · 工程基建（P2）— ✅ 已完成
- [x] `tools/check_adrs.py`：ADR 编号唯一 / 状态字段 / docs 相对链接校验。
- [x] `tools/test_scorecard.py`：回归——四轴有段位、zh/en 本地化、估算标注、**不泄配额% / prompt 原文 / 密钥**。
- [x] `.github/workflows/ci.yml`：`go build/vet/test` + 上述两项检查。

---

## T4 · npm 分发（P0）— ⏸ 暂缓（需 NPM_TOKEN + GitHub Actions 执行）

> 决策：[`adr/0003-npm-distribution.md`](adr/0003-npm-distribution.md)（提议中）。
> 沙箱无法真正 `npm publish` / `npx` 端到端验证；待提供 npm 凭证与包名归属后落地。

### T4.1 仓库结构
- [ ] 落地 monorepo：`packages/cli/`（`ccoach`）+ `packages/skills/`（`@ccoach/skills`），npm workspaces。
- [ ] 终定包名 / scope（ADR 0003 OQ1）。

### T4.2 CLI 二进制分发
- [ ] CLI 主包 `bin` 薄包装：运行时解析并 exec 对应平台二进制。
- [ ] 平台专属子包 + `optionalDependencies` + `os`/`cpu`（esbuild 式），定平台矩阵（ADR 0003 OQ2）。
- [ ] CI 跨平台构建 Go 二进制并发布各平台子包；npm 与 Release 同源、带 checksum（ADR 0003 D4）。

### T4.3 skills 安装
- [ ] `@ccoach/skills` 可直接 `npm i` 安装。
- [ ] `ccoach skills install`：探测并复制到 Claude Code / Codex 的 skills 目录（ADR 0003 OQ3 / 0004 OQ1）。

---

## 已完成（历史）
- [x] 建立 `docs/`：PRD / ADR / TODO 体系（2026-06-02）。
- [x] 规划 npm 分发与 skills 化分析（ADR 0003 / 0004，2026-06-02）。
- [x] 规划三层分析与特性优先建议（ADR 0005 / 0006，2026-06-02）。
- [x] **剥离保活、更名 ccoach、`report` 改为默认命令**（ADR 0007，2026-06-02）。
- [x] 规划游戏化可分享成绩卡 + i18n（ADR 0008 / 0009，2026-06-02）。
- [x] **实现** T1/T5/T6/T7/T2/T3（glossary、三层 scope + prompt 信号、feature-mapping、成绩卡 + zh/en、CI 检查，2026-06-02）。
</content>

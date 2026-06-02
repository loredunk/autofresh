# ADR 0003 — 通过 npm 分发（npx / npm i -g），仓库内 CLI 与 skills 分开

> 状态：提议中 · 日期：2026-06-02 · 相关：[`PRD.md`](../PRD.md) §5、[`TODO.md`](../TODO.md) T4

## 背景

当前 autofresh 以「下载平台二进制 + `chmod +x`」的方式安装（见 README）。对很多
Claude Code / Codex 用户来说，更顺手的是 `npx autofresh` 或 `npm i -g autofresh`。
同时产品要分两块交付：**CLI**（产出用量数据）与 **skills**（教 agent 如何分析这些数据）。
需要定下：如何用 npm 分发一个 Go 二进制，以及 CLI 与 skills 在仓库/包层面如何组织。

## 决策

### D1 — 单仓库（monorepo），两个独立发布的 npm 包

仓库内物理分开，各自独立发版：

- `autofresh` — CLI 包，入口是对 Go 二进制的薄包装（`bin`）。
- `@autofresh/skills` — skills 包，纯内容（Markdown 指令 + 元数据），不含二进制。

> 包结构由实现方按「安装方便 + 仓库内明确分开」的目标自行拍定，此为采用方案。
> 建议目录：`packages/cli/`、`packages/skills/`（npm workspaces 统一管理）。

- **理由**：用户要求「CLI 和 skills 在一个仓库内肯定是分开的」；两者发布节奏、内容形态
  完全不同（二进制 vs 文档），独立包能各自升级、互不绑版本。

### D2 — Go 二进制走「平台专属 optionalDependencies」分发，对 npx 友好

CLI 主包不在 `postinstall` 里联网下载，而是把各平台预编译二进制拆成多个平台专属子包
（如 `@autofresh/cli-darwin-arm64`、`-linux-amd64` 等），用 `optionalDependencies` +
`os`/`cpu` 字段让 npm 只装当前平台那一个；主包的 `bin` 脚本在运行时解析到对应二进制并 exec。

- **理由**：这是 esbuild / @vscode/ripgrep 等成熟做法。相比 `postinstall` 下载：
  - 对 `npx` 更稳（无 postinstall 网络/防火墙/校验问题，CI 与离线缓存更友好）。
  - 安装即可用，版本与包锁定一致，可复现。
- **代价**：发版要同时构建并发布多个平台子包（交给 CI，见 TODO T4）。
- **备选**：`postinstall` 从 GitHub Releases 下载（实现简单但网络/校验风险高）——记为 fallback。

### D3 — skills 安装提供 CLI 便捷命令，降低落地门槛

除 `npm i @autofresh/skills` 外，CLI 提供 `autofresh skills install`，把 skills 复制到
Claude Code / Codex 各自的 skills 目录（路径在实现时确认，见 ADR 0004 OQ）。

- **理由**：用户最关心「安装方便」。手动找 skills 目录、放文件门槛高；让已装好的 CLI 代劳最顺。
- 同时保留「直接 `npm i` 那个包」的路径，满足不想用 CLI 的人。

### D4 — 版本与二进制来源单一可信

npm 上的二进制必须来自仓库 CI 的同一次构建（同 commit、可校验 checksum），
不与「手动 GitHub Release 上传」产生两套来源。

- **理由**：避免「npm 装到的」和「Release 下载的」行为不一致难排查。

## 后果

- 好处：`npx autofresh` / `npm i -g autofresh` 开箱即用；CLI 与 skills 解耦发布；安装路径多但都简单。
- 成本：引入 npm 包脚手架与多平台发布 CI；维护平台子包矩阵。
- 影响：README 安装一节需新增 npm 方式（保留二进制下载作为备选）。

## 待定（Open Questions）

- **OQ1**：包名/scope 终定（`autofresh` 是否可用、是否统一用 `@autofresh/*`）。
- **OQ2**：支持的平台矩阵（darwin arm64/amd64、linux amd64/arm64，是否含 musl/windows）。
- **OQ3**：`autofresh skills install` 的目标目录如何探测（与 ADR 0004 的 skills 落地路径联动）。
</content>

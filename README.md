# Autofresh

<p align="center"><a href="README_EN.md">English README</a></p>

![autofresh 工作原理](assets/autofresh.png)

跨平台（macOS / Linux）的 Codex & Claude 用量保活工具。

一个用 Go 写的小命令行工具，帮你**自动在工作时间内定时触发 Codex 和 Claude 的保活 ping**，把每个 5 小时计费窗口"卡"在你真正需要用的时段里，让有限的额度尽量都落在工作时间被你用上，而不是浪费在睡觉或下班时段。

- 设定一个起始时间（比如早上 6:00），后续按固定 `5h10m` 间隔自动触发，不跨午夜
- macOS 自动写入 `launchd`，Linux 自动写入 `crontab`，一条命令搞定
- 内置 `plan` / `trigger` / `logs` / `doctor` 等命令，方便查看计划、手动触发和排查
- Codex 走 `codex exec`，Claude 走 `claude -p`，纯保活 ping，不打扰你正常使用

## 安装

> 全程都在「终端」里操作。macOS 打开终端：按 `⌘ + 空格`，输入 `Terminal` 回车；Linux 一般是 `Ctrl + Alt + T`。

### macOS

**1. 下载**（推荐用下面的 `curl` 命令——这样下载的文件不带「隔离」标记，可跳过后面的解锁步骤）

不确定自己是什么芯片？点屏幕左上角  →「关于本机」，看「芯片 / 处理器」一栏。

```bash
# Apple Silicon（M1/M2/M3/M4 等）
curl -L -o autofresh https://github.com/loredunk/autofresh/releases/latest/download/autofresh-darwin-arm64

# Intel 芯片
curl -L -o autofresh https://github.com/loredunk/autofresh/releases/latest/download/autofresh-darwin-amd64
```

**2. 加上可执行权限**（下载下来的文件默认不能直接运行）

```bash
chmod +x autofresh
```

**3. 运行**

```bash
./autofresh report
```

> **如果你是从浏览器（Safari / Chrome）下载的**，macOS 会给文件打上「隔离」标记，第一次运行会弹「无法打开，因为无法验证开发者」。任选一种解锁方式：
>
> - 终端执行 `xattr -c autofresh` 清除隔离标记，再重新运行；或
> - 打开 **系统设置 → 隐私与安全性**，在页面底部找到被拦下的提示，点「仍要打开」，再回终端运行一次。
>
> （用上面的 `curl` 方式下载则没有这个标记，直接跳过本段。）

**（可选）装到全局**，之后在任意目录都能直接敲 `autofresh`：

```bash
sudo mv autofresh /usr/local/bin/
```

### Linux

```bash
curl -L -o autofresh https://github.com/loredunk/autofresh/releases/latest/download/autofresh-linux-amd64
chmod +x autofresh
./autofresh report

# （可选）装到全局
sudo mv autofresh /usr/local/bin/
```

### 从源码编译

本项目为标准 Go 模块，依据 [go.mod](go.mod) 进行编译，入口文件为 [cmd/autofresh/main.go](cmd/autofresh/main.go)。要求 Go 1.22 或更高版本。

```bash
go build -o autofresh ./cmd/autofresh
```

> 上面的下载链接用的是 `releases/latest/...`，会永远指向最新版本，不用手动改版本号。也可以前往 [Releases](https://github.com/loredunk/autofresh/releases) 页面手动下载。

## 快速上手

装好后，第一条命令可以先看看自己今天的 Codex 用量（纯读取、不改任何东西）：

```bash
./autofresh report
```

想开启保活定时，设一个每天的起始时间即可（例如早上 6 点）：

```bash
./autofresh set 06:00 --target all   # 之后按 5h10m 间隔自动触发，不跨午夜
./autofresh plan                     # 确认计划是否生效
```

> 如果上一步把二进制装进了 `/usr/local/bin`，把所有命令里的 `./autofresh` 换成 `autofresh` 即可。

## 命令

```bash
./autofresh set 06:00 --target all   # 给claude和codex设置一天的第一次fresh定时
./autofresh plan        # 查看当前计划
./autofresh trigger     # 尝试用autofresh给codex和claude发送打招呼
./autofresh trigger --target codex  # 用trigger给codex gpt-5.4-mini发一个ok
./autofresh logs        # 看所有的logs
./autofresh logs -n 10    # 看 10 行logs
./autofresh doctor    # 诊断当前计划
./autofresh delete    # 删除计划
./autofresh report    # 今天本机的 Codex 使用报告
```

手动执行 `trigger` 会把模型回复打印到 stdout，便于确认保活确实触发了。`plan` 会显示当前 provider 对应的 model 和 prompt，`logs` 会记录每次触发使用的 model。

### report：本机 Codex 使用报告

只读分析本机 `$CODEX_HOME`（默认 `~/.codex`）里的 rollout 记录，输出 Token 用量、估算成本、工具调用拆解，以及按来源、语言、仓库、时段等维度的统计。默认看今天：

```bash
./autofresh report                 # 今天本机的 Codex 使用报告
./autofresh report --date 2026-05-13   # 指定某一天
./autofresh report --since 2026-05-01  # 从某天起到今天
./autofresh report --days 7        # 最近 7 天
./autofresh report --by-repo       # 按 git 仓库展开（含分支）
./autofresh report --json          # 输出 JSON，脚本友好
```

说明与边界：

- **只反映这台电脑**：同一账号登录多台机器时，rollout 天然按机器隔离，本命令只读本机文件、不跨机器汇总。
- 不调用 `codex` 的 `/status`，也不读取 rollout 里的 `rate_limits`（CLI 下恒为 null，且配额百分比是账号级、跨机器的），因此**报告不输出任何配额百分比**。
- Token 取 `token_count` 事件里的累计值并按会话求增量去重；子代理（thread_spawn）会话会被排除，避免重复计数。
- 来源来自 Codex 本地 thread metadata，用于区分 CLI、Codex App、IDE 插件等入口；同一个 rollout 文件即使被多条 thread 记录引用也只计一次。
- 语言为本机仓库文件扩展名推断值，只用于辅助判断主要工作栈，不代表 Codex 服务端记录了语言字段。
- 仓库维度会补充主语言、语言文件分布、构建系统、测试命令、变更文件类型；`--json` 会输出完整字段，适合交给 Codex skill 生成日报。
- 配置痕迹只读取 `config.toml` 的非敏感键、MCP/profile 名称、全局/项目 AGENTS 文件是否存在和大小；不会输出指令正文、token、header、密钥值。
- 成本为**估算值**（token × 内置参考价），仅供参考，不等于实际账单。
- 时间窗口按本机时区的绝对日期边界划分，报告头部会标明时区。
- autofresh 自己的保活 ping 用了 `--ephemeral`、不写 rollout，因此不会出现在本报告中（符合预期）。
- 优先通过 `state_*.sqlite` 定位 rollout 文件；没有 sqlite（或存在未合并的 WAL）时自动退回扫描 `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`。

### 增强版 HTML 报告 Skill

仓库内置了可复用的 Codex skill：[skills/codex-usage-html-report](skills/codex-usage-html-report/SKILL.md)。它不会替代 `autofresh report`；它会先读取 `./autofresh report --json` 的事实数据，再由 Codex 写入更深的分析、建议和项目画像，最后渲染成增强版 HTML。增强报告会优先产出“指标 → 含义 → 影响 → 继续深入 → 建议动作”这种洞见，例如从高 `cached_input` 判断长会话和大上下文复用。

这个 skill 也支持逐步 drilldown：先从报告里找出高 Token / 高成本项目，让用户选择项目；再列出该项目下最重的 session；只有在用户明确同意后，才读取某个具体 session 的 user prompt 做 prompt 质量复盘。它不会读取隐藏的 Codex/OpenAI system prompt，也不会默认把 prompt 正文写进 HTML。

安装给 Codex 使用：

```bash
mkdir -p "${CODEX_HOME:-$HOME/.codex}/skills"
cp -R skills/codex-usage-html-report "${CODEX_HOME:-$HOME/.codex}/skills/"
```

然后重启 Codex 或开启新会话，直接说：

```text
使用 codex-usage-html-report 生成今天的 Codex 使用增强 HTML 报告。
```

也可以让它聚焦高消耗项目：

```text
使用 codex-usage-html-report 找出最近 7 天最费 token 的项目，然后让我选择一个 session 做 prompt 复盘。
```

如果只想手动跑底层流程：

```bash
./autofresh report --json > /tmp/codex-usage-report.json
# 让 Codex 按 skills/codex-usage-html-report/references/insights-schema.md 写 /tmp/codex-usage-insights.json
python3 skills/codex-usage-html-report/scripts/render_enriched_codex_report.py \
  --report /tmp/codex-usage-report.json \
  --insights /tmp/codex-usage-insights.json \
  --output codex-report.enriched.html

# 可选：先列出某个项目里的高 token session，不读取 prompt 正文
python3 skills/codex-usage-html-report/scripts/session_drilldown.py \
  --days 7 \
  --repo autofresh \
  --top 20 > /tmp/codex-session-candidates.json
```

本地生成的 `codex-report*.html` 已在 `.gitignore` 中忽略，避免误提交个人报告。

## 行为

- 每日调度从一个配置的时间点开始
- 间隔固定为 `5h10m`
- 时间不跨越午夜
- macOS 使用 `launchd`
- Linux 使用 `crontab`
- Codex 保活使用 `codex exec --model gpt-5.4-mini --skip-git-repo-check --ephemeral "ok"`
- Claude 保活使用 `claude --model haiku -p "ok"`
- `gpt-5.4-nano` 比 `gpt-5.4-mini` 更小，但目前只适用于 OpenAI API；Codex CLI 保活仍使用 Codex 可用的最小 GPT-5.4 系列模型 `gpt-5.4-mini`。Claude 的 `haiku` 是 Claude Code 的轻量模型别名，会随官方别名解析到对应 Haiku 模型。

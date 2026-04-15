# autofresh

跨平台（macOS / Linux）的 Codex & Claude 用量保活工具。

一个用 Go 写的小命令行工具，帮你**自动在工作时间内定时触发 Codex 和 Claude 的保活 ping**，把每个 5 小时计费窗口“卡”在你真正需要用的时段里，让有限的额度尽量都落在工作时间被你用上，而不是浪费在睡觉或下班时段。

- 设定一个起始时间（比如早上 8:00），后续按固定 `5h10m` 间隔自动触发，不跨午夜
- macOS 自动写入 `launchd`，Linux 自动写入 `crontab`，一条命令搞定
- 内置 `plan` / `trigger` / `logs` / `doctor` 等命令，方便查看计划、手动触发和排查
- Codex 走 `codex exec`，Claude 走 `claude -p`，纯保活 ping，不打扰你正常使用

## 编译

本项目为标准 Go 模块，依据 [go.mod](go.mod) 进行编译，入口文件为 [cmd/autofresh/main.go](cmd/autofresh/main.go)。

使用 Go 工具链直接编译：

```bash
# 编译生成可执行文件
go build -o autofresh ./cmd/autofresh

# 或直接运行
go run ./cmd/autofresh
```

要求 Go 1.22 或更高版本。

## 命令

```bash
./autofresh set 06:00 --target all   # 给claude和codex设置一天的第一次fresh定时
./autofresh plan        # 查看当前计划
./autofresh trigger     # 尝试用autofresh给codex和claude发送打招呼
./autofresh trigger --target codex  # 用trigger给codex gpt5.4 mini发一个ok
./autofresh logs        # 看所有的logs
./autofresh logs -n 10    # 看 10 行logs
./autofresh doctor    # 诊断当前计划
./autofresh delete    # 删除计划
```

手动执行 `trigger` 会把模型回复打印到 stdout，便于确认保活确实触发了。`plan` 会显示当前 provider 对应的 model 和 prompt，`logs` 会记录每次触发使用的 model。

## 行为

- 每日调度从一个配置的时间点开始
- 间隔固定为 `5h10m`
- 时间不跨越午夜
- macOS 使用 `launchd`
- Linux 使用 `crontab`
- Codex 保活使用 `codex exec --model gpt-5.4-mini --skip-git-repo-check --ephemeral "ok"`
- Claude 保活使用 `claude --model haiku -p "ok"`

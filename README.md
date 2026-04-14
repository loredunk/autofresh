# autofresh

用于调度每日 `codex` 和 `claude` 保活 ping 的小型 Go 命令行工具。

## 编译

本项目为标准 Go 模块，依据 [go.mod](go.mod) 进行编译，入口文件为 [cmd/autofresh/main.go](cmd/autofresh/main.go)。

使用 Go 工具链直接编译：

```bash
# 编译生成可执行文件
go build -o autofresh ./cmd/autofresh

# 或直接运行
go run ./cmd/autofresh

# 安装到 $GOPATH/bin 或 $GOBIN
go install ./cmd/autofresh
```

要求 Go 1.22 或更高版本。

## 命令

```bash
autofresh set 08:00 --target all
autofresh plan
autofresh trigger
autofresh trigger --target codex
autofresh logs
autofresh logs -n 10
autofresh doctor
autofresh delete
```

手动执行 `trigger` 会把模型回复打印到 stdout，便于确认保活确实触发了。

## 行为

- 每日调度从一个配置的时间点开始
- 间隔固定为 `5h10m`
- 时间不跨越午夜
- macOS 使用 `launchd`
- Linux 使用 `crontab`
- Codex 保活使用 `codex exec --skip-git-repo-check --ephemeral "ok"`
- Claude 保活使用 `claude -p "ok"`

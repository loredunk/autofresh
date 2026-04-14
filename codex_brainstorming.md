# Autofresh 设计稿

## 1. 目标

实现一个终端 CLI `autofresh`，帮助用户在每天固定起点后，按 `5 小时 10 分钟` 的间隔自动触发 `codex`、`claude` 或两者，从而尽量覆盖工作时段内更多 usage 时间窗口。

核心要求：

- 简单，稳定，代码精炼
- 支持 macOS 和 Linux
- 优先使用平台原生定时能力，而不是常驻 daemon
- 支持查看计划、修改计划、删除计划、手动触发

非目标：

- 不做 GUI
- 不做复杂多用户管理
- 不做跨天连续滚动调度
- 不依赖数据库或远程服务

## 2. 设计结论

采用 `Go + 本地配置文件 + 系统定时任务` 的方案。

原因：

- Go 适合输出单二进制，跨 macOS/Linux 稳定
- 平台定时由系统负责，工具本身不需要常驻进程
- 配置和计划都可以本地推导，结构足够简单
- 触发命令通过 `exec.CommandContext` 控制超时，比 shell 依赖 `timeout` 更稳，macOS 也无需额外安装 GNU coreutils

## 3. 命令模型

CLI 采用下面这组命令：

### `autofresh set <HH:MM> [--target codex|claude|all]`

作用：

- 保存每日起点时间
- 保存默认目标
- 自动替换旧计划
- 自动重装对应平台的定时任务

示例：

```bash
autofresh set 08:00 --target all
autofresh set 09:30 --target codex
```

### `autofresh delete`

作用：

- 删除本地配置
- 删除平台定时任务

### `autofresh plan`

作用：

- 输出当前配置
- 输出今天的触发计划
- 输出当前平台安装状态

示例输出：

```text
start time: 08:00
target: all
today:
  - 08:00 codex, claude
  - 13:10 codex, claude
  - 18:20 codex, claude
  - 23:30 codex, claude
job: installed
platform: darwin
```

### `autofresh trigger [--target codex|claude|all]`

作用：

- 立即主动触发一次
- 默认使用配置中的 target
- 如果命令行传了 `--target`，则覆盖默认值

### `autofresh run`

作用：

- 这是给系统定时任务调用的内部入口
- 读取配置并执行一次默认 target
- 正常用户一般不直接手动使用

### `autofresh doctor`

作用：

- 检查 `codex`、`claude` 是否在 PATH 中
- 检查配置文件是否存在
- 检查定时任务是否安装

这个命令不是强制，但很实用，建议一起做。

## 4. 调度规则

### 4.1 时间生成规则

用户设置一个当日起点，例如：

```text
08:00
```

系统生成当天计划：

```text
08:00
13:10
18:20
23:30
```

规则：

- 间隔固定为 `5h10m`
- 只保留同一天 `00:00-23:59` 内的时间点
- 不生成次日 `04:40`
- 第二天重新从起点时间开始

### 4.2 计划重建规则

每次 `set`：

- 覆盖配置
- 重新计算时间点
- 替换旧的系统定时任务

每次 `delete`：

- 删除配置
- 删除系统定时任务

### 4.3 触发目标规则

支持：

- `codex`
- `claude`
- `all`

含义：

- `codex`: 只触发 codex
- `claude`: 只触发 claude
- `all`: 顺序执行 codex，再执行 claude

执行 `all` 时顺序串行，避免同时启动两个 CLI 增加噪音和资源竞争。

## 5. 命令执行策略

### 5.1 Codex

默认执行：

```bash
codex exec "ok" --max-tokens 5
```

### 5.2 Claude

默认执行：

```bash
claude -p "ok" --max-tokens 5
```

### 5.3 执行要求

内部统一约束：

- 非交互
- 最小 prompt
- 最小 token
- 丢弃 stdout/stderr
- 强制超时
- 根据 exit code 判断成功失败

这里不直接依赖 shell 的 `timeout`，而是在 Go 里做：

- `context.WithTimeout(15 * time.Second)`
- `exec.CommandContext(...)`
- `Stdout/Stderr` 重定向到 `io.Discard`

这样跨平台更稳。

### 5.4 错误处理

单次运行时：

- 某个 provider 失败，不阻断另一个 provider
- 最终返回非零退出码，只要有一个失败

日志保留最小信息：

- 执行时间
- provider
- 成功/失败
- 失败原因摘要

## 6. 平台落地

### 6.1 macOS

使用 `launchd`。

做法：

- 在 `~/Library/LaunchAgents/` 写入 plist
- Label 使用固定名字，例如 `com.autofresh.runner`
- 用多个 `StartCalendarInterval` 条目表达当天多个时间点
- 执行命令指向 `autofresh run`

优点：

- 原生
- 用户级任务足够
- 不需要 root

### 6.2 Linux

使用 `crontab`。

做法：

- 写入带标记的 cron 条目
- 每个时间点生成一条记录
- 命令执行 `autofresh run`
- 更新时只替换 autofresh 管理的那部分，不破坏用户其他 cron

原因：

- 覆盖面广
- 依赖最少
- 比要求用户系统具备 `systemd --user` 更稳

### 6.3 二进制路径

定时任务必须写绝对路径。

因此安装任务时要解析：

- 当前 `autofresh` 可执行文件绝对路径
- 当前用户 HOME 目录

避免 PATH 在定时环境中不一致。

## 7. 配置设计

配置文件建议使用 JSON，位置：

- macOS: `~/.config/autofresh/config.json`
- Linux: `~/.config/autofresh/config.json`

统一使用 XDG 风格，简单够用。

结构：

```json
{
  "start_time": "08:00",
  "target": "all",
  "interval_minutes": 310,
  "timezone": "Local",
  "installed_job": true,
  "binary_path": "/absolute/path/to/autofresh"
}
```

说明：

- `interval_minutes` 固定写入，便于以后调整
- `timezone` 先固定本地时区
- `binary_path` 方便重建任务时使用

## 8. 模块划分

建议文件结构：

```text
cmd/autofresh/main.go
internal/app/app.go
internal/config/config.go
internal/schedule/schedule.go
internal/provider/provider.go
internal/provider/codex.go
internal/provider/claude.go
internal/platform/platform.go
internal/platform/darwin.go
internal/platform/linux.go
internal/cli/cli.go
internal/logging/logging.go
```

职责：

- `main.go`: 入口
- `cli`: 参数解析和命令分发
- `config`: 读写配置
- `schedule`: 计算日内时间点
- `provider`: 执行 codex/claude
- `platform`: 安装、替换、删除系统定时任务
- `logging`: 统一日志输出
- `app`: 串联业务流程

## 9. 用户行为流程

### 初次配置

```bash
autofresh set 08:00 --target all
```

系统执行：

1. 校验时间格式
2. 校验 target
3. 计算今日计划
4. 保存配置
5. 安装或替换定时任务
6. 输出计划摘要

### 查看计划

```bash
autofresh plan
```

输出：

- 当前起点
- 默认 target
- 今日时间点
- 平台任务安装状态

### 手动刷新

```bash
autofresh trigger
autofresh trigger --target codex
```

### 删除

```bash
autofresh delete
```

系统执行：

1. 删除平台任务
2. 删除配置文件
3. 输出删除结果

## 10. 边界与异常

### 时间格式

接受：

- `08:00`
- `8:00`

内部统一标准化为 `08:00`。

拒绝：

- `24:00`
- `08:60`
- 非法字符串

### 配置不存在

`plan`、`run`、`trigger` 如果没有配置：

- 返回清晰错误
- 提示先执行 `autofresh set <HH:MM> --target <...>`

### 命令不存在

如果本机没有 `codex` 或 `claude`：

- `doctor` 明确提示
- `trigger/run` 执行时返回 provider not found

### 定时环境 PATH 不一致

定时任务中建议注入一个保守 PATH：

```text
/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin
```

这样可以兼容常见 mac 和 Linux 安装路径。

## 11. 日志策略

目标是最小但够用。

建议日志文件：

- `~/.config/autofresh/autofresh.log`

记录：

- `timestamp`
- `provider`
- `mode=scheduled|manual`
- `result=success|failure`
- `message`

不做日志轮转，先保持简单。后续如果有需要，再限制文件大小。

## 12. 测试策略

重点测试 4 类：

### 12.1 调度计算

例如起点 `08:00`：

- 输出应为 `08:00, 13:10, 18:20, 23:30`

例如起点 `21:00`：

- 输出应为 `21:00`

### 12.2 配置读写

- 保存后可正确读取
- 修改后覆盖旧值
- 删除后不存在

### 12.3 provider 执行封装

通过 mock runner 或注入命令执行器，验证：

- 参数正确
- 超时生效
- 失败返回可识别

### 12.4 平台任务生成

- macOS plist 内容正确
- Linux cron 条目正确
- 替换 autofresh 任务时不影响其他 cron 内容

## 13. 实现建议

优先实现最小可用版本：

1. `set`
2. `plan`
3. `trigger`
4. `delete`
5. `run`
6. `doctor`
7. macOS/Linux 任务安装

这样能尽快形成闭环。

## 14. 推荐的最终行为

用户执行：

```bash
autofresh set 08:00 --target all
```

得到：

- 每天 `08:00`
- 每天 `13:10`
- 每天 `18:20`
- 每天 `23:30`

每次定时任务运行时：

- `codex exec "ok" --max-tokens 5`
- `claude -p "ok" --max-tokens 5`

都通过 Go 子进程执行，并带 15 秒超时和静默输出。

## 15. 结论

这个方案满足当前需求，重点在于：

- 不做 daemon
- 不做复杂调度
- 把逻辑压缩成一个清晰的 CLI + 本地配置 + 平台任务

实现成本低，故障面小，后续扩展也直接。

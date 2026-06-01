# Autofresh

![autofresh how it works](assets/autofresh.png)

A cross-platform (macOS / Linux) keep-alive tool for Codex & Claude usage.

A small CLI tool written in Go that **automatically sends keep-alive pings to Codex and Claude on a scheduled basis during working hours**, anchoring each 5-hour billing window to the time you actually need it. This ensures your limited quota is consumed during working hours rather than wasted while you're asleep or off work.

- Set a start time (e.g. 06:00), then automatically trigger at a fixed `5h10m` interval — never crossing midnight
- macOS auto-writes to `launchd`, Linux auto-writes to `crontab` — one command does it all
- Built-in `plan` / `trigger` / `logs` / `doctor` commands for viewing schedules, manual triggers, and diagnostics
- Codex uses `codex exec`, Claude uses `claude -p` — pure keep-alive pings that don't interrupt your normal usage

## Installation

**Option 1: Download prebuilt binary (recommended)**

Go to [Releases](https://github.com/loredunk/autofresh/releases) and download the executable for your platform:

| Platform | File |
|----------|------|
| macOS (Apple Silicon / M-series) | `autofresh-darwin-arm64` |
| macOS (Intel) | `autofresh-darwin-amd64` |
| Linux x86-64 | `autofresh-linux-amd64` |

```bash
chmod +x autofresh-darwin-arm64
# macOS: remove quarantine attribute
xattr -d com.apple.quarantine autofresh-darwin-arm64
```

**Option 2: Build from source**

This is a standard Go module. See [go.mod](go.mod) for dependencies; the entry point is [cmd/autofresh/main.go](cmd/autofresh/main.go).

```bash
go build -o autofresh ./cmd/autofresh
```

Requires Go 1.22 or higher.

## Commands

```bash
./autofresh set 06:00 --target all   # Set the first daily fresh time for both claude and codex
./autofresh plan        # View the current schedule
./autofresh trigger     # Send a keep-alive ping to both codex and claude
./autofresh trigger --target codex  # Send a ping to codex gpt-5.4-mini only
./autofresh logs        # View all logs
./autofresh logs -n 10    # View last 10 log entries
./autofresh doctor    # Diagnose the current schedule
./autofresh delete    # Delete the schedule
```

Running `trigger` manually prints the model response to stdout so you can confirm the keep-alive actually fired. `plan` shows the current provider's model and prompt; `logs` records the model used for each trigger.

The `report` command reads local Codex rollout records under `$CODEX_HOME` and includes token usage, estimated cost, tool calls, source breakdowns (CLI / Codex App / IDE plugin when available), inferred language breakdowns, repositories, and hourly distribution. Repository rows also include inferred primary language, language mix, build systems, test commands, and changed file types in JSON output. Language is inferred from local repository files and is only a heuristic; Codex rollouts do not currently store a first-class language field.

For richer AI-written HTML reports, install the reusable skill from [skills/codex-usage-html-report](skills/codex-usage-html-report/SKILL.md). The skill emphasizes deeper insight ladders that connect metrics to meaning, impact, drilldown, and concrete interventions, such as interpreting high `cached_input` as long-session large-context reuse. It can also drill down from high-token projects to candidate sessions; it only reads user prompts for a selected session after explicit approval, and it cannot read hidden Codex/OpenAI system prompts.

```bash
mkdir -p "${CODEX_HOME:-$HOME/.codex}/skills"
cp -R skills/codex-usage-html-report "${CODEX_HOME:-$HOME/.codex}/skills/"
```

Then restart Codex or start a new session and ask: `Use codex-usage-html-report to generate today's enriched Codex usage HTML report.` You can also ask: `Use codex-usage-html-report to find the highest-token projects from the last 7 days, then let me choose one session for prompt review.`

## Behavior

- Daily scheduling starts from a configured time
- Interval is fixed at `5h10m`
- Never crosses midnight
- macOS uses `launchd`
- Linux uses `crontab`
- Codex keep-alive: `codex exec --model gpt-5.4-mini --skip-git-repo-check --ephemeral "ok"`
- Claude keep-alive: `claude --model haiku -p "ok"`
- `gpt-5.4-nano` is smaller than `gpt-5.4-mini`, but it is currently API-only; Codex CLI keep-alive stays on `gpt-5.4-mini`, the smallest GPT-5.4-series model available in Codex. Claude's `haiku` is the Claude Code lightweight-model alias and resolves through Claude Code's official alias mapping.

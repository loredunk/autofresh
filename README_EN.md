# ccoach

> A local AI usage coach (macOS / Linux). Read-only analysis of how you use **Codex / Claude Code** —
> where your tokens go, what's wasted, and how to use the tools better — turned into a **shareable scorecard**.
>
> Formerly **autofresh** (a Codex/Claude keep-alive tool). Keep-alive has been removed; the project now
> focuses on usage analysis and advice, and is renamed **ccoach**. Design & decisions live in [`docs/`](docs/).

## What it does

- **Usage report**: read-only parse of local `~/.codex` rollouts — tokens, estimated cost, tool calls,
  and breakdowns by repo / hour / source / language / git habits / config scan. Read-only; changes nothing.
- **Advice** (skill): teaches Claude Code / Codex to interpret this data and give **feature-first** advice —
  whenever a finding can be solved with a native feature (CLAUDE.md/AGENTS.md, subagents, hooks, plan mode,
  permission settings, model/effort tiers…), it names the feature. Supports **session / project / global** scopes.
- **Shareable scorecard**: grade your usage / habits / prompts across four axes (Prompt Skill, Spending
  Style, Engineering Sense, Diligence) into a screenshot-friendly card at the top of the HTML report
  (bilingual zh/en, rendered by the skill).

> **Privacy**: all analysis runs locally — your prompts never leave your machine.

## Install

### Build from source (available today)

Standard Go module, entry point [cmd/ccoach/main.go](cmd/ccoach/main.go), requires Go 1.22+:

```bash
go build -o ccoach ./cmd/ccoach
```

> npm distribution (`npx ccoach` / `npm i -g ccoach`) and prebuilt binaries are planned — see [docs/TODO.md](docs/TODO.md) T4.

## Usage

The report is the default command — a bare invocation prints today's usage report:

```bash
./ccoach                      # today's local usage report
./ccoach --date 2026-05-13    # a specific day
./ccoach --since 2026-05-01   # from a day through today
./ccoach --days 7             # the last 7 days
./ccoach --by-repo            # per-repository breakdown (with branches)
./ccoach --json               # JSON output, script / agent friendly
```

> For familiarity, `./ccoach report --json …` still works (`report` is an optional prefix).

## Advice skill

For richer AI-written HTML reports, use the reusable skill
[skills/ai-usage-html-report](skills/ai-usage-html-report/SKILL.md): it uses `ccoach report --json`
(Codex) + ccusage (Claude Code) local data to produce a dual-platform HTML report and behavior profile,
and can drill from high-token projects down to candidate sessions. It reads a selected session's user
prompts only after explicit approval, and never reads hidden system prompts.

## Notes & boundaries

- **Local machine only**: rollouts are per-machine; this tool reads only local files and never aggregates across machines.
- **No quota percentages**: `rate_limits` is always null under the CLI, and quota is account-level / cross-machine.
- **Cost is an estimate** (token × built-in reference price), not your actual bill.
- Time windows use absolute local-timezone day boundaries; the report header states the timezone.
</content>

---
name: ai-usage-html-report
description: Generate an enriched dual-platform (Claude Code + Codex) HTML report of local AI coding usage, built from ccusage and `ccoach report --json` data. Use when the user wants a deeper AI-written review of how they use Claude Code and/or Codex — cost, token, cache, model and active-hours breakdown, habits analysis, project-by-project recommendations, high-token project/session drilldown, or a richer HTML dashboard than the raw text/JSON reports.
when_to_use: 'Trigger when the user wants to review, analyze, or visualize their local AI coding usage across Claude Code and Codex — for example "how much did I spend on Claude Code / Codex", "how much did I use AI today", "generate an AI usage report", "build an HTML dashboard of my Claude Code and Codex usage", "which projects burned the most tokens", "compare my Claude Code vs Codex usage", "review my most expensive or unclear Codex sessions", "analyze my AI coding habits", or an explicit /ai-usage-html-report invocation.'
argument-hint: "[YYYY-MM-DD | N (days back)]"
arguments: period
allowed-tools: Read Write WebSearch WebFetch Bash(ccoach *) Bash(./ccoach *) Bash(python3 *) Bash(npx *) Bash(ccusage *)
---

# AI Usage HTML Report (Claude Code + Codex)

## Purpose

Build a dual-platform local AI usage report. Use authoritative on-disk facts as the data source, then write AI interpretation into an insights file and render a polished standalone HTML report covering both Claude Code and Codex.

Keep responsibilities separate:

- Data tools (ccusage, `ccoach report`) collect local facts.
- This skill interprets those facts and produces an enhanced HTML document.

### Data-layer rules (mandatory)

- **Claude Code data MUST come from `ccusage`.** ccusage reads each `~/.claude/projects/*.jsonl` line for the real per-message model and prices it with offline LiteLLM data, so its model/cost attribution is correct.
- **NEVER use `~/.claude/stats-cache.json`.** It is polluted by cc-switch swapping in third-party providers (kimi, etc.), so its model/cost attribution is wrong, and it stopped updating after 2026-04-09 — stale and inaccurate. This was confirmed against real data when comparing approaches. Do not read or fall back to it under any circumstances.
- **Codex data** comes from `ccoach report --since <date> --json` (token *and* behavior: repos/hours/tools/git_habits/project_management/languages/sources over the window) plus `ccusage codex daily` (token/cost history).
- **Claude Code behavior** comes from `scripts/collect_claude_behavior.py`, which parses `~/.claude/projects/**/*.jsonl` locally (pure stdlib, offline) into the same behavior shape so both platforms render symmetrically. It emits only aggregates: Bash → first word / git subcommand only, repo → cwd basename, file → extension only. Never prompt text, file contents, full commands, or absolute paths.

## Workflow

### Daily dual-platform report

1. Locate / build `ccoach`:
   - Prefer `./ccoach` in the current repo.
   - Otherwise use `ccoach` from `PATH`.
   - If neither exists but this is the ccoach source repo, run `GOCACHE=/tmp/ccoach-go-cache go build -o ccoach ./cmd/ccoach`.
   - Generate the Codex report **over a window** (not just today), so ccoach emits its behavior dimensions (repos/hours/tools/git_habits/project_management/languages/sources):
     - `./ccoach report --since <START> --json > /tmp/codex-usage-report.json`
     - Choose `<START>` as the earliest Claude Code activity date or a sensible floor (e.g. start of the current year). ccoach supports `--since YYYY-MM-DD`, `--days N`, and `--date YYYY-MM-DD` (mutually exclusive).
     - If invoked with an argument (`$period`): a date like `2026-06-01` maps to `--date $period`; a bare integer like `7` maps to `--days $period`; empty means a wide window via `--since`.
2. Pull data with `ccusage` (offline, no network, no upload):
   - `npx ccusage@latest claude daily --json --offline --breakdown > /tmp/cc-daily.json`
   - `npx ccusage@latest claude session --json --offline > /tmp/cc-session.json`
   - `npx ccusage@latest codex daily --json --offline > /tmp/cc-codex.json`
   - Use `ccusage ...` directly if it is already on `PATH`; otherwise `npx ccusage@latest ...`.
3. Collect the Claude Code **behavior** profile (local stdlib parser, offline) over the same window:
   - `python3 ${CLAUDE_SKILL_DIR}/scripts/collect_claude_behavior.py --since <START> --output /tmp/claude-behavior.json`
   - Match `<START>`/`--days`/`--date` to the ccoach window so both platforms cover the same span. Default (no flag) is full history.
   - Add `--scope project` / `--scope session` for per-project / per-session breakdowns (see "Analysis scopes" below). The script also emits `prompt_signals` — numeric prompt-quality aggregates (length, structured/constraint/file-ref ratios, correction rate); **never prompt text, never assistant replies** — which power the scorecard's Prompt Skill axis.
4. Merge into one dual-platform JSON (both platforms get a unified `behavior` block):
   - `python3 ${CLAUDE_SKILL_DIR}/scripts/merge_dual_platform.py --cc-daily /tmp/cc-daily.json --cc-session /tmp/cc-session.json --cc-behavior /tmp/claude-behavior.json --codex-report /tmp/codex-usage-report.json --codex-ccusage /tmp/cc-codex.json --output /tmp/ai-usage.json`
   - `--cc-behavior` is optional: if omitted, the Claude Code behavior panel degrades gracefully and the Codex behavior panel (from ccoach) still renders.
5. Read `/tmp/ai-usage.json`, then write `/tmp/ai-usage-insights.json` following `references/dual-insights-schema.md`.
   - Write the rich AI-interpretation layer the dual renderer expects:
     - `executive_summary` — a prominent paragraph (or short list) covering both platforms.
     - `recommendations` — a list; each item is a string or `{title, text, evidence}` (include `evidence` grounded in the merged numbers).
     - `insights` — a list; each item is a string or `{title, detail}`.
   - All fields are optional and backward-compatible: a flat `{"insights": ["string", ...]}` still renders. Use Chinese unless the user asks otherwise.
   - Behavior data is in `platforms.<plat>.behavior` (tools / top_commands / git_habits / languages / repos / hours / sources / extras) for both platforms — ground at least one insight per platform in these behavior numbers.
   - For richer interpretation patterns (evidence → meaning → impact → drilldown → intervention), read `references/insight-patterns.md`; distill those ladders into the `recommendations`/`insights` fields described in `references/dual-insights-schema.md`.
6. (Optional) Build the shareable **scorecard** (see "Shareable scorecard" below):
   - `python3 ${CLAUDE_SKILL_DIR}/scripts/scorecard.py --data /tmp/ai-usage.json --lang zh --output /tmp/scorecard.json`
   - Pick `--lang zh|en` per the user's language. Then write the personality-summary
     sentence yourself (in the user's language) into the insights `executive_summary` —
     the fixed tier names / roasts come from the localized copy table, not from you.
7. Render HTML:
   - `python3 ${CLAUDE_SKILL_DIR}/scripts/render_dual_platform.py --data /tmp/ai-usage.json --insights /tmp/ai-usage-insights.json --scorecard /tmp/scorecard.json --lang zh --output ai-usage-report.html`
   - `--scorecard` is optional; include it to put the screenshot-friendly cover card at the top. Match `--lang` to the scorecard's.
   - Use the user-specified output path if given; otherwise `ai-usage-report.html`.

### Fallback when ccusage is unavailable

If `node`/`npx` is missing or `ccusage` fails to run, degrade to a **Codex-only** report from ccoach data:

- Run `./ccoach report --json > /tmp/codex-usage-report.json`, write `/tmp/codex-usage-insights.json` per `references/insights-schema.md`, and render with `python3 ${CLAUDE_SKILL_DIR}/scripts/render_enriched_codex_report.py --report /tmp/codex-usage-report.json --insights /tmp/codex-usage-insights.json --output codex-report.enriched.html`.
- Tell the user the Claude Code half was skipped and that installing Node/ccusage (`npx ccusage@latest`) enables the dual-platform report.
- **Never** substitute `~/.claude/stats-cache.json` for the missing Claude Code data — it is wrong and stale (see data-layer rules).

### Project and session drilldown (Codex)

Use this when the user wants to find expensive or unclear Codex sessions.

1. Generate/read `/tmp/codex-usage-report.json`.
2. Identify candidate projects from `repos`, sorted by token count, estimated cost, and session count.
3. Show the user a short candidate list and ask which project to inspect. Do not read prompt contents yet.
4. Generate session candidates for the selected project:
   - `python3 ${CLAUDE_SKILL_DIR}/scripts/session_drilldown.py --days N --repo REPO --top 20 > /tmp/codex-session-candidates.json`
   - Or for one day: `python3 ${CLAUDE_SKILL_DIR}/scripts/session_drilldown.py --date YYYY-MM-DD --repo REPO --top 20 > /tmp/codex-session-candidates.json`
5. Summarize the candidate sessions by token count, tools, time span, model, source, branch, and rollout path.
6. Ask the user to choose a session before doing any prompt-content review.
7. Only after explicit user approval, rerun the script for the selected session with user prompts included:
   - `python3 ${CLAUDE_SKILL_DIR}/scripts/session_drilldown.py --rollout /path/to/rollout.jsonl --include-user-prompts > /tmp/codex-session-review-source.json`
   - Or `python3 ${CLAUDE_SKILL_DIR}/scripts/session_drilldown.py --session-id SESSION_ID --include-user-prompts > /tmp/codex-session-review-source.json`
8. Read `references/session-prompt-review.md`, then summarize the session review findings. Fold the key diagnoses into the `insights` strings of `/tmp/ai-usage-insights.json` (or, in the Codex-only fallback, into `session_reviews` of `/tmp/codex-usage-insights.json` per the schema).
9. Render the HTML again.

## Analysis scopes (session / project / global)

Pick the scope from what the user asks (ADR 0005):

- **Global** (default): cross-project overview. `collect_claude_behavior.py` (no `--scope`) + `ccoach report --json`.
- **Project**: one project across its sessions. `collect_claude_behavior.py --scope project` (keyed by cwd basename under `~/.claude/projects/`); Codex side use `session_drilldown.py --repo <name>`.
- **Session**: the current/just-finished session. `collect_claude_behavior.py --scope session` emits `sessions_detail[]`; if the skill is invoked mid-session, analyze the live session directly.

Signal model for every scope: analyze **user prompts + permissions + tool calls only — never assistant replies**. User-prompt analysis is numeric (`prompt_signals`); if you ever quote a prompt (Codex opt-in drilldown only), paraphrase + redact (see `references/session-prompt-review.md`). Global scope stays purely aggregate (no prompt text).

## Shareable scorecard

`scripts/scorecard.py` grades four independent axes — Prompt Skill, Spending Style, Engineering Sense, Diligence — into tier labels + roast lines from `references/scorecard-copy.json` (hand-localized zh/en, ADR 0008/0009). The renderer shows it as a screenshot-friendly cover card.

- Tiers/roasts/UI labels are **fixed localized copy** (the table) — do not translate them yourself; pick `--lang zh|en`.
- The personality-summary sentence and any deeper roast IS yours to write, in the user's language.
- Tone: tease changeable **habits**, never ability or the person. The relative rank ("beats X%") is a local **estimate** — keep it labelled as such.
- Privacy is a selling point: state that all analysis is local and prompt content never leaves the machine.

## Analysis Guidance

Base all claims on the merged JSON / report data. If a claim is an inference, phrase it as an inference.

**Feature-first recommendations**: whenever a finding can be solved with a native Claude Code / Codex feature, recommend that feature by name before any generic habit advice. Use `references/feature-mapping.md` for the finding → feature table (ADR 0006).

Before recommending any platform configuration or feature (CLAUDE.md/AGENTS.md, skills, subagents, hooks, sandbox/model/effort settings, permission/settings), search the web for the latest official Claude Code and Codex documentation and verify the suggestion against it. These tools change quickly, so confirm every configuration recommendation against current docs rather than relying on prior knowledge. Only suggest — never auto-change the user's config.

Prioritize:

- Cross-platform comparison: how Claude Code vs Codex split cost, tokens, cache reuse, and active days.
- Usage patterns across time, source, language, and project.
- Git habits: review cadence, status/diff checks, commit/push behavior, branch spread.
- Project management habits: build systems, tests, CI, docs/planning/config changes.
- Cost and token efficiency: high-token projects, cache hit rate, reasoning ratio.
- Actionable configuration suggestions: CLAUDE.md/AGENTS.md, sandbox/model/effort settings, test commands.
- Insight ladders: connect metrics to meaning, impact, next drilldown, and habit/config changes. Use `references/insight-patterns.md`.
- Prompt quality only for sessions the user explicitly selected and authorized for prompt review. Use the framework in `references/session-prompt-review.md`.

Avoid:

- Reading or quoting prompt contents, AGENTS contents, secrets, auth files, or private code unless the user explicitly asks.
- Reading hidden OpenAI/Codex system or developer prompts. This skill can only analyze local user-owned traces and files it is allowed to read.
- Dumping all prompts for a project. Prompt review must be session-scoped and opt-in.
- Treating estimated cost as a bill. (Claude Code cost is real offline LiteLLM pricing; Codex per-model cost is partial/best-effort.)
- Claiming OpenAI/Anthropic server-side account usage; this is local-machine evidence only.

## Output Expectations

The final HTML should be useful as a daily dual-platform review artifact:

- Start with an executive summary that covers both Claude Code and Codex.
- Include evidence-backed recommendations.
- Include at least 2-4 deeper insights when the data supports them, especially around long sessions, cache reuse, high-token projects, and repeated tool loops.
- Highlight risks and next actions.
- Keep raw metrics visible but secondary to interpretation.
- Preserve local privacy: do not embed secret values or instruction-file contents.
- For prompt reviews, prefer paraphrased diagnoses and rewritten prompt examples over verbatim prompt quotes.

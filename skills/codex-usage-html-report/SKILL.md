---
name: codex-usage-html-report
description: Generate an enriched HTML report from local `autofresh report --json` Codex usage data. Use when the user wants a deeper AI-written Codex usage review, habits analysis, project-by-project recommendations, high-token project/session drilldown, prompt-quality review, or a richer HTML dashboard based on local Codex traces rather than the built-in autofresh text/JSON report.
---

# Codex Usage HTML Report

## Purpose

Use `autofresh report --json` as the factual data source, then write AI interpretation into an insights file and render a polished standalone HTML report.

Keep responsibilities separate:

- `autofresh report` collects local facts.
- This skill interprets those facts and produces an enhanced HTML document.

## Workflow

### Daily HTML report

1. Locate `autofresh`:
   - Prefer `./autofresh` in the current repo.
   - Otherwise use `autofresh` from `PATH`.
   - If neither exists but this is the autofresh source repo, run `GOCACHE=/tmp/autofresh-go-cache go build -o autofresh ./cmd/autofresh`.
2. Generate structured data:
   - Today: `./autofresh report --json > /tmp/codex-usage-report.json`
   - Specific day: `./autofresh report --date YYYY-MM-DD --json > /tmp/codex-usage-report.json`
   - Range: `./autofresh report --days N --json > /tmp/codex-usage-report.json`
3. Read the JSON report.
4. Write `/tmp/codex-usage-insights.json` using the schema in `references/insights-schema.md`.
   - For deeper interpretation, read `references/insight-patterns.md` and include `insight_ladder`.
5. Render HTML:
   - `python3 <skill>/scripts/render_enriched_codex_report.py --report /tmp/codex-usage-report.json --insights /tmp/codex-usage-insights.json --output codex-report.enriched.html`
6. If the user asked to overwrite an existing HTML file, use that output path.

### Project and session drilldown

Use this when the user wants to find expensive or unclear Codex sessions.

1. Generate/read `/tmp/codex-usage-report.json`.
2. Identify candidate projects from `repos`, sorted by token count, estimated cost, and session count.
3. Show the user a short candidate list and ask which project to inspect. Do not read prompt contents yet.
4. Generate session candidates for the selected project:
   - `python3 <skill>/scripts/session_drilldown.py --days N --repo REPO --top 20 > /tmp/codex-session-candidates.json`
   - Or for one day: `python3 <skill>/scripts/session_drilldown.py --date YYYY-MM-DD --repo REPO --top 20 > /tmp/codex-session-candidates.json`
5. Summarize the candidate sessions by token count, tools, time span, model, source, branch, and rollout path.
6. Ask the user to choose a session before doing any prompt-content review.
7. Only after explicit user approval, rerun the script for the selected session with user prompts included:
   - `python3 <skill>/scripts/session_drilldown.py --rollout /path/to/rollout.jsonl --include-user-prompts > /tmp/codex-session-review-source.json`
   - Or `python3 <skill>/scripts/session_drilldown.py --session-id SESSION_ID --include-user-prompts > /tmp/codex-session-review-source.json`
8. Read `references/session-prompt-review.md`, then write session review findings into `/tmp/codex-usage-insights.json` under `session_reviews`.
9. Render the HTML again.

## Analysis Guidance

Base all claims on the JSON report. If a claim is an inference, phrase it as an inference.

Prioritize:

- Usage patterns across time, source, language, and project.
- Git habits: review cadence, status/diff checks, commit/push behavior, branch spread.
- Project management habits: build systems, tests, CI, docs/planning/config changes.
- Cost and token efficiency: high-token projects, cache hit rate, reasoning ratio.
- Actionable configuration suggestions: AGENTS.md, sandbox/model settings, test commands.
- Insight ladders: connect metrics to meaning, impact, next drilldown, and habit/config changes. Use `references/insight-patterns.md`.
- Prompt quality only for sessions the user explicitly selected and authorized for prompt review. Use the framework in `references/session-prompt-review.md`.

Avoid:

- Reading or quoting prompt contents, AGENTS contents, secrets, auth files, or private code unless the user explicitly asks.
- Reading hidden OpenAI/Codex system or developer prompts. This skill can only analyze local user-owned traces and files it is allowed to read.
- Dumping all prompts for a project. Prompt review must be session-scoped and opt-in.
- Treating estimated cost as a bill.
- Claiming OpenAI server-side account usage; this is local-machine evidence only.

## Output Expectations

The final HTML should be useful as a daily review artifact:

- Start with an executive summary.
- Include evidence-backed recommendations.
- Include at least 2-4 deeper insights when the data supports them, especially around long sessions, cache reuse, high-token projects, and repeated tool loops.
- Highlight risks and next actions.
- Keep raw metrics visible but secondary to interpretation.
- Preserve local privacy: do not embed secret values or instruction-file contents.
- For prompt reviews, prefer paraphrased diagnoses and rewritten prompt examples over verbatim prompt quotes.

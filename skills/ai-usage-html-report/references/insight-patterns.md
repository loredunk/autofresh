# Insight Patterns

Use this reference when generating `insight_ladder` for the enriched HTML report.

## Insight Ladder

Each useful insight should move from observation to action:

1. Evidence: exact metric(s) from the report.
2. Meaning: what the metric likely represents in actual Codex use.
3. Impact: why it matters for cost, speed, quality, or user workflow.
4. Drilldown: which project/session/time/tool pattern should be inspected next.
5. Intervention: what to change in prompts, project docs, AGENTS.md, skills, or work splitting.

Do not stop at "high tokens" or "many tool calls". Explain the operating pattern behind the metric.

## High-Value Insight Patterns

### Long Session / Large Context Reuse

Signals:

- Very high total tokens.
- High `cached_input` and high `cache_hit_rate`.
- Few sessions relative to token count.
- One project dominates tokens.

Interpretation:

- The user likely worked in long sessions with a large repeated context.
- Prompt cache is working, so this is not necessarily full-price repeated reading.
- Cost can still accumulate because a large cached prefix is carried across many turns.

Recommended wording:

> High total tokens plus high cached input suggests long context reuse: the session kept carrying a large shared context across many turns. Cache reduced repeated-processing cost, but the repeated large prefix still accumulated meaningful usage.

Next drilldown:

- Identify the dominant project.
- List highest-token sessions for that project.
- Inspect whether the session crossed phases: exploration, implementation, report writing, validation, and docs.

Interventions:

- Split long work into phase-specific sessions.
- Ask for a checkpoint summary before switching phases.
- Persist stable rules in AGENTS.md or a project skill instead of keeping them only in conversation context.
- Generate factual JSON first, then do lightweight analysis in a separate step.

### Low Cache / Repeated Reloading

Signals:

- High total tokens.
- Low `cache_hit_rate`.
- Many sessions or many source entries for the same project.

Interpretation:

- The user may be restarting context often, switching entry points, or causing different prompts/files to miss the cache.

Interventions:

- Keep related work in one session until a phase boundary.
- Use a short handoff summary when starting a new session.
- Put stable setup instructions in AGENTS.md.

### Tool Loop / Exploration Drift

Signals:

- Shell calls or web searches are high relative to file changes.
- `rg`, `sed`, `nl`, `find`, browser/search tools dominate.
- Few tests or no final validation.

Interpretation:

- The agent may be spending tokens rediscovering context or chasing broad evidence.

Interventions:

- Ask for a bounded discovery pass.
- Require the agent to state what it has learned before expanding scope.
- Provide file paths, failing commands, or acceptance criteria earlier.

### Implementation Without Verification

Signals:

- File changes are present.
- No test commands observed for a repo with a known build/test system.
- CI exists but local test commands are absent.

Interpretation:

- The session may have produced changes without a tight local feedback loop.

Interventions:

- Add the canonical fast test/typecheck command to AGENTS.md.
- Ask Codex to run verification before final summary.

### Human-Gated Git Flow

Signals:

- Many `git status` and `git diff` commands.
- No `git commit` or `git push`.

Interpretation:

- This may be an intentional workflow where Codex prepares and reviews changes but humans commit.

Interventions:

- Document whether Codex should commit automatically or only summarize diffs.
- Ask for final `git status --short` and a commit-message suggestion.

## Writing Rules

- Phrase insights as likely interpretations, not absolute truth, unless the report directly proves them.
- Prefer "this suggests..." or "likely..." for behavioral inference.
- Connect each insight to an observable next step.
- Avoid generic advice that would apply to any report.
- **Feature-first**: when an intervention can be a native Claude Code / Codex feature, name it.
  See `feature-mapping.md` for the finding → feature table (verify against current official docs first).

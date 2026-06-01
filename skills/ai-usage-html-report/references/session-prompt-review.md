# Session Prompt Review

Use this reference only after the user has selected a specific session and explicitly approved reading user prompts from that session.

## Privacy Boundary

- Analyze user-authored prompts and user-owned instruction files only when authorized.
- Do not attempt to read hidden OpenAI/Codex system prompts, developer prompts, chain of thought, secrets, auth files, or private source code unrelated to the selected session.
- Do not quote full prompts by default. Paraphrase the issue and include short rewritten examples.
- If the user asks for quotes, keep excerpts short and redact secrets, paths, hostnames, tokens, and personal data.

## Matt Pocock / AI Hero Lens

Review the session as a context-management problem:

- Context: Did the prompt provide the repo, files, error, desired behavior, constraints, and examples needed for the agent to act?
- Direction: Was the target outcome clear, or did the prompt force the agent to infer too much?
- Scope: Was the request too broad for one session, or too narrow/prescriptive before diagnosis?
- Plan mode: Would the task have benefited from first asking the agent to inspect, plan, or ask clarifying questions?
- Verification: Did the prompt define how success should be checked: tests, screenshots, logs, commands, acceptance criteria?
- Iteration: Did later prompts correct course with concrete evidence, or only restate dissatisfaction?
- Session boundary: Should the work have been split into a discovery session, implementation session, and review session?

## Findings To Produce

For each reviewed session, write:

- What likely consumed tokens: ambiguity, repeated context loading, broad search, failed commands, rework, tool loops, or over-scoped asks.
- Prompt failure modes: missing acceptance criteria, missing files, unclear ownership, premature solution, conflicting constraints, vague bug report, too many goals, or no verification target.
- Better first prompt: a concise rewrite the user could have started with.
- Better follow-up prompt: a rewrite for steering the agent after the first result.
- Skill/config suggestion: whether this should become an AGENTS.md rule, a project skill, a checklist, or a reusable command.

## Output Shape

Add `session_reviews` to the insights JSON:

```json
[
  {
    "repo": "repo-name",
    "session_id": "optional-id",
    "rollout_path": "/path/to/rollout.jsonl",
    "summary": "Short diagnosis",
    "token_drivers": ["why this session likely became expensive"],
    "prompt_issues": ["specific prompt quality issue"],
    "better_first_prompt": "rewritten prompt",
    "better_followup_prompt": "rewritten follow-up",
    "next_action": "what to change in prompts, project docs, or skills"
  }
]
```

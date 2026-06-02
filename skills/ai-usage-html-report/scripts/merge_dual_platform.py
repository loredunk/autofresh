#!/usr/bin/env python3
"""Merge Claude Code data (from ccusage) + Codex data (from autofresh
report --json, with ccusage codex as historical fallback) into one dual-platform
usage JSON.

Inputs (all JSON files produced beforehand):
  --cc-daily        ccusage claude daily --json --offline --breakdown
  --cc-session      ccusage claude session --json --offline
  --cc-behavior     collect_claude_behavior.py output (Claude Code behavior)
  --codex-report    ./autofresh report --since <date> --json  (Codex behavior+tokens)
  --codex-ccusage   ccusage codex daily --json --offline   (historical Codex, fallback)
  --output          merged dual-platform JSON path

Both platforms expose a unified `behavior` block (tools / git_habits /
languages / repos / hours / sources) so the renderer can show them symmetrically.

Privacy: only aggregate counts/costs are read. No prompt text, session content,
file paths beyond project basenames, or secrets are touched.
"""
import argparse
import datetime
import json
from pathlib import Path


def load(p):
    return json.loads(Path(p).read_text())


def aggregate_cc_models(cc_daily):
    """Aggregate Claude Code per-model totals across all days."""
    models = {}
    for day in cc_daily.get("daily", []):
        for b in day.get("modelBreakdowns", []):
            name = b.get("modelName", "unknown")
            m = models.setdefault(name, dict(cost=0.0, input=0, output=0,
                                              cache_read=0, cache_create=0))
            m["cost"] += b.get("cost", 0)
            m["input"] += b.get("inputTokens", 0)
            m["output"] += b.get("outputTokens", 0)
            m["cache_read"] += b.get("cacheReadTokens", 0)
            m["cache_create"] += b.get("cacheCreationTokens", 0)
    return [dict(model=k, **v) for k, v in
            sorted(models.items(), key=lambda kv: -kv[1]["cost"])]


def cache_hit_rate(cache_read, total_input_like):
    denom = cache_read + total_input_like
    return (cache_read / denom) if denom else 0.0


# --------------------------------------------------------------------------
# Unified behavior block. Both platforms emit the same shape so the renderer
# can show symmetric panels:
#   behavior = {
#     generated_for, sessions, total_tool_calls,
#     tools_by_name:   [{name, count}],
#     top_commands:    [{command, count}],
#     tool_categories: {shell, web, file, ...},
#     git_habits:      [{command, count}],
#     languages:       [{name, count}],   # files (Claude) / sessions (Codex)
#     languages_unit:  "文件" | "会话",
#     repos:           [{repo, sessions, tokens, tool_calls}],
#     hours:           [{hour, tokens, count}],
#     sources:         [{name, count}],
#     extras:          [ "标签: 值", ... ],   # platform-specific signals
#   }
# --------------------------------------------------------------------------

# Known git subcommands; anything else (e.g. a leaked path captured by
# autofresh's parser) is dropped from the rendered git-habits list so no
# absolute path or arbitrary token reaches the report.
GIT_SUBCMDS = {
    "add", "commit", "push", "pull", "fetch", "diff", "status", "log",
    "checkout", "branch", "merge", "rebase", "stash", "show", "reset",
    "clone", "switch", "restore", "tag", "cherry-pick", "revert",
    "rev-parse", "remote", "init", "blame",
}


def _clean_commands(cmds):
    """Reduce command labels to a safe basename token (strip any path), drop
    anything still containing a slash so no absolute path reaches the report."""
    out = []
    for c in cmds or []:
        base = (c.get("command") or "").strip().rsplit("/", 1)[-1]
        if not base or "/" in base:
            continue
        out.append(dict(command=base, count=c.get("count", 0)))
    return out


def _clean_git(subs):
    """Keep only recognised git subcommands (privacy: drop leaked paths/tokens)."""
    out = []
    for s in subs or []:
        cmd = (s.get("command") or "").lower()
        if cmd in GIT_SUBCMDS:
            out.append(dict(command=cmd, count=s.get("count", 0)))
    return out


def _norm_hours(hours, count_key=None):
    """Normalize hour list to [{hour, tokens, count}] for 0..23 sparse->kept."""
    out = []
    for h in hours:
        out.append(dict(hour=h.get("hour", 0),
                        tokens=h.get("tokens", 0),
                        count=h.get("count", h.get("tokens", 0) if count_key is None
                              else h.get(count_key, 0))))
    return out


def claude_behavior(cb):
    """Normalize collect_claude_behavior.py output into the unified shape."""
    if not cb:
        return None
    tools = cb.get("tools", {})
    repos = [dict(repo=r.get("repo"), sessions=r.get("sessions", 0),
                  tokens=r.get("tokens", 0), tool_calls=r.get("tool_calls", 0))
             for r in cb.get("repos", [])][:10]
    sources = list(cb.get("sources", {}).get("entrypoints", []))
    sc = cb.get("sources", {})
    extras = []
    pm = sc.get("permission_modes", [])
    if pm:
        extras.append("权限模式: " + "、".join(
            f"{p['name']}×{p['count']}" for p in pm[:4]))
    if sc.get("subagent_calls"):
        extras.append(f"子代理调用 {sc['subagent_calls']} 次"
                      f"（占记录 {sc.get('subagent_share', 0) * 100:.1f}%）")
    return dict(
        generated_for=cb.get("generated_for"),
        sessions=cb.get("sessions", 0),
        total_tool_calls=tools.get("total_calls", 0),
        tools_by_name=tools.get("by_name", []),
        top_commands=_clean_commands(tools.get("top_commands", [])),
        tool_categories=tools.get("categories", {}),
        git_habits=_clean_git(cb.get("git_habits", {}).get("top_subcommands", [])),
        languages=[dict(name=l.get("name"), count=l.get("files", 0))
                   for l in cb.get("languages", [])][:10],
        languages_unit="文件",
        repos=repos,
        hours=_norm_hours(cb.get("hours", [])),
        sources=sources,
        extras=extras,
    )


def codex_behavior(r):
    """Normalize autofresh report --json into the unified behavior shape."""
    if not r:
        return None
    tools = r.get("tools", {})
    cats = dict(
        shell=tools.get("shell_calls", 0),
        web=tools.get("web_searches", 0),
        file=tools.get("file_changes", 0),
    )
    repos = [dict(repo=x.get("repo"), sessions=x.get("sessions", 0),
                  tokens=x.get("tokens", 0),
                  tool_calls=0)  # autofresh repo has no per-repo tool count
             for x in r.get("repos", [])][:10]
    git = r.get("git_habits", {})
    pm = r.get("project_management", {})
    # Drop any signal containing a path-like token so no absolute path leaks.
    def _safe(sigs, n):
        return [s for s in (sigs or []) if "/" not in s][:n]
    extras = []
    extras += _safe(git.get("review_signals"), 3)
    extras += _safe(git.get("risk_signals"), 2)
    extras += _safe(pm.get("signals"), 3)
    if r.get("reasoning_ratio"):
        extras.append(f"推理 token 占比 {r['reasoning_ratio'] * 100:.1f}%")
    return dict(
        generated_for=r.get("generated_for"),
        sessions=r.get("sessions", 0),
        total_tool_calls=tools.get("total_calls", 0),
        tools_by_name=[],  # autofresh doesn't break tools down by name
        top_commands=_clean_commands(tools.get("top_commands", [])),
        tool_categories=cats,
        git_habits=_clean_git(git.get("top_subcommands", [])),
        languages=[dict(name=l.get("name"), count=l.get("sessions", 0))
                   for l in r.get("languages", [])][:10],
        languages_unit="会话",
        repos=repos,
        hours=_norm_hours(r.get("hours", [])),
        sources=[dict(name=s.get("name"), count=s.get("sessions", 0))
                 for s in r.get("sources", [])],
        extras=extras,
    )


def build_claude(cc_daily, cc_session, cc_behavior=None):
    t = cc_daily.get("totals", {})
    daily = cc_daily.get("daily", [])
    sessions = cc_session.get("sessions", [])
    models = aggregate_cc_models(cc_daily)
    cache_read = t.get("cacheReadTokens", 0)
    input_t = t.get("inputTokens", 0)
    # cache hit rate = cache_read / (cache_read + fresh input)
    chr_ = cache_hit_rate(cache_read, input_t)
    # daily cost series for sparkline
    series = [dict(date=d.get("date"), cost=round(d.get("totalCost", 0), 2),
                   tokens=d.get("totalTokens", 0)) for d in daily]
    # top sessions by cost
    top_sessions = sorted(sessions, key=lambda s: -s.get("totalCost", 0))[:5]
    top = [dict(project=s.get("projectPath", "").replace("-Users-mac-", "~/"),
                last=s.get("lastActivity"),
                cost=round(s.get("totalCost", 0), 2),
                tokens=s.get("totalTokens", 0),
                models=s.get("modelsUsed", [])) for s in top_sessions]
    return dict(
        platform="Claude Code",
        source="ccusage (LiteLLM offline pricing)",
        active_days=len(daily),
        sessions=len(sessions),
        date_range=[daily[0]["date"], daily[-1]["date"]] if daily else [],
        tokens=dict(
            input=input_t,
            output=t.get("outputTokens", 0),
            cache_read=cache_read,
            cache_create=t.get("cacheCreationTokens", 0),
            total=t.get("totalTokens", 0),
        ),
        cost_usd=round(t.get("totalCost", 0), 2),
        cost_is_real=True,
        cache_hit_rate=round(chr_, 4),
        models=models,
        daily_series=series,
        top_sessions=top,
        behavior=claude_behavior(cc_behavior),
        prompt_signals=(cc_behavior or {}).get("prompt_signals", {}),
    )


def build_codex(codex_report, codex_ccusage):
    """Codex from autofresh (today) + ccusage codex (history fallback)."""
    r = codex_report
    tok = r.get("tokens", {})
    af_total = tok.get("total", 0)
    af_sessions = r.get("sessions", 0)

    # historical from ccusage codex
    cx = codex_ccusage
    ct = cx.get("totals", {})
    cdaily = cx.get("daily", [])
    # per-model aggregate
    cmodels = {}
    for day in cdaily:
        for name, b in (day.get("models") or {}).items():
            m = cmodels.setdefault(name, dict(cost=0.0, input=0, output=0,
                                              cached=0, reasoning=0))
            m["cost"] += b.get("costUSD", 0)
            m["input"] += b.get("inputTokens", 0)
            m["output"] += b.get("outputTokens", 0)
            m["cached"] += b.get("cachedInputTokens", 0)
            m["reasoning"] += b.get("reasoningOutputTokens", 0)
    models = [dict(model=k, **v) for k, v in
              sorted(cmodels.items(), key=lambda kv: -kv[1]["input"])]
    cached = ct.get("cachedInputTokens", 0)
    input_t = ct.get("inputTokens", 0)
    chr_ = cache_hit_rate(cached, input_t)
    series = [dict(date=d.get("date"), cost=round(d.get("costUSD", 0), 2),
                   tokens=d.get("totalTokens", 0)) for d in cdaily]
    return dict(
        platform="Codex",
        # autofresh = authoritative for "today"; ccusage codex = history.
        autofresh_today=dict(
            generated_for=r.get("generated_for"),
            timezone=r.get("timezone"),
            sessions=af_sessions,
            tokens=tok,
            cost_usd=r.get("estimated_cost_usd", 0),
            cache_hit_rate=r.get("cache_hit_rate", 0),
            empty=(af_total == 0),
        ),
        source="autofresh report --json (today) + ccusage codex (history)",
        active_days=len(cdaily),
        date_range=[cdaily[0]["date"], cdaily[-1]["date"]] if cdaily else [],
        tokens=dict(
            input=input_t,
            output=ct.get("outputTokens", 0),
            cache_read=cached,
            reasoning=ct.get("reasoningOutputTokens", 0),
            total=ct.get("totalTokens", 0),
        ),
        cost_usd=round(ct.get("costUSD", 0), 2),
        # ccusage prices Codex at the day level via gpt-5.x fallback; per-model
        # costUSD is 0, so cost is "best-effort" not fully model-attributed.
        cost_is_real="partial",
        cache_hit_rate=round(chr_, 4),
        models=models,
        daily_series=series,
        behavior=codex_behavior(r),
    )


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--cc-daily", required=True)
    ap.add_argument("--cc-session", required=True)
    ap.add_argument("--cc-behavior", default="")
    ap.add_argument("--codex-report", required=True)
    ap.add_argument("--codex-ccusage", required=True)
    ap.add_argument("--output", required=True)
    a = ap.parse_args()

    cc_behavior = load(a.cc_behavior) if a.cc_behavior else None
    claude = build_claude(load(a.cc_daily), load(a.cc_session), cc_behavior)
    codex = build_codex(load(a.codex_report), load(a.codex_ccusage))

    merged = dict(
        title="双平台 AI 使用报告",
        generated_at=datetime.date.today().isoformat(),
        platforms=dict(claude_code=claude, codex=codex),
        combined=dict(
            total_cost_usd=round(claude["cost_usd"] + codex["cost_usd"], 2),
            total_tokens=claude["tokens"]["total"] + codex["tokens"]["total"],
            total_sessions=claude["sessions"],  # codex session count not in ccusage daily
            prompt_signals=(cc_behavior or {}).get("prompt_signals", {}),
        ),
    )
    Path(a.output).write_text(json.dumps(merged, indent=2, ensure_ascii=False))
    print(f"wrote {a.output}")
    print(f"  Claude Code: {claude['sessions']} sessions, "
          f"${claude['cost_usd']}, {claude['tokens']['total']:,} tokens")
    print(f"  Codex: {codex['active_days']} days, "
          f"${codex['cost_usd']}, {codex['tokens']['total']:,} tokens "
          f"(autofresh today empty={codex['autofresh_today']['empty']})")


if __name__ == "__main__":
    main()

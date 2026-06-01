#!/usr/bin/env python3
"""Merge Claude Code data (from ccusage) + Codex data (from autofresh
report --json, with ccusage codex as historical fallback) into one dual-platform
usage JSON.

Inputs (all JSON files produced beforehand):
  --cc-daily      ccusage claude daily --json --offline --breakdown
  --cc-session    ccusage claude session --json --offline
  --codex-report  ./autofresh report --json   (today-only Codex snapshot)
  --codex-ccusage ccusage codex daily --json --offline   (historical Codex, fallback)
  --output        merged dual-platform JSON path

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


def build_claude(cc_daily, cc_session):
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
    )


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--cc-daily", required=True)
    ap.add_argument("--cc-session", required=True)
    ap.add_argument("--codex-report", required=True)
    ap.add_argument("--codex-ccusage", required=True)
    ap.add_argument("--output", required=True)
    a = ap.parse_args()

    claude = build_claude(load(a.cc_daily), load(a.cc_session))
    codex = build_codex(load(a.codex_report), load(a.codex_ccusage))

    merged = dict(
        title="双平台 AI 使用报告",
        generated_at=datetime.date.today().isoformat(),
        platforms=dict(claude_code=claude, codex=codex),
        combined=dict(
            total_cost_usd=round(claude["cost_usd"] + codex["cost_usd"], 2),
            total_tokens=claude["tokens"]["total"] + codex["tokens"]["total"],
            total_sessions=claude["sessions"],  # codex session count not in ccusage daily
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

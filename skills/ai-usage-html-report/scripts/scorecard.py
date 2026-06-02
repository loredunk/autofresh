#!/usr/bin/env python3
"""Compute a gamified, shareable scorecard from the merged dual-platform JSON.

Reads the merged JSON (from merge_dual_platform.py) plus the i18n copy table
(references/scorecard-copy.json) and grades four independent axes — Prompt Skill,
Spending Style, Engineering Sense, Diligence — into tier labels + roast lines in
the chosen language. The personality-summary paragraph is NOT produced here; the
model writes that in the user's language (ADR 0008 D3 / 0009).

Pure python3 stdlib, offline. Tier scoring is heuristic and deterministic.
Relative rank is a LOCAL ESTIMATE (labelled as such), never a real percentile.

Privacy: consumes only aggregate numbers (incl. prompt_signals); no prompt text.
"""
import argparse
import json
import os
from pathlib import Path

HERE = Path(__file__).resolve().parent
DEFAULT_COPY = HERE.parent / "references" / "scorecard-copy.json"


def clamp(x, lo, hi):
    return max(lo, min(hi, x))


def _f(d, *path, default=0.0):
    """Defensive nested getter returning a number."""
    cur = d
    for p in path:
        if not isinstance(cur, dict):
            return default
        cur = cur.get(p)
    if isinstance(cur, (int, float)):
        return cur
    return default


def score_prompt(ps):
    """Return tier index 0..4 (0 best) for Prompt Skill."""
    n = int(ps.get("prompts", 0) or 0)
    if n == 0:
        return 2  # no data -> neutral 'Apprentice', avoid unfair roast
    q = (0.30 * ps.get("structured_ratio", 0)
         + 0.25 * ps.get("constraint_ratio", 0)
         + 0.20 * ps.get("file_ref_ratio", 0)
         + 0.25 * (1 - ps.get("correction_rate", 0)))
    avg = ps.get("avg_len", 0) or 0
    if avg > 1500 or avg < 12:
        q -= 0.12
    if q >= 0.62:
        return 0
    if q >= 0.45:
        return 1
    if q >= 0.30:
        return 2
    if q >= 0.16:
        return 3
    return 4


def opus_share(cc):
    """Cost share of opus-class models on Claude Code, 0..1."""
    models = cc.get("models") or []
    tot = sum(_f(m, "cost") for m in models if isinstance(m, dict))
    if tot <= 0:
        return 0.0
    opus = sum(_f(m, "cost") for m in models
               if isinstance(m, dict) and "opus" in str(m.get("model", "")).lower())
    return opus / tot


def score_spending(combined, cc):
    """Return tier index 0..3 (0 best) for Spending Style."""
    cost = _f(combined, "total_cost_usd")
    tokens = _f(combined, "total_tokens")
    sessions = _f(combined, "total_sessions") or 1
    avg_tok = tokens / sessions if sessions else tokens
    # Opus-on-trivial: expensive model dominates but per-session work is small.
    if opus_share(cc) >= 0.6 and avg_tok < 8000:
        return 3
    if cost >= 30:
        return 2
    if cost >= 5:
        return 1
    return 0


def score_engineering(cc):
    """Return tier index 0..3 (0 best) for Engineering Sense."""
    beh = cc.get("behavior") or {}
    cats = beh.get("tool_categories") or {}
    file_ops = _f(cats, "file")
    loop = _f(cats, "shell") + _f(cats, "web") + _f(cats, "search")
    loop_ratio = loop / (file_ops + 1)
    repos = len(beh.get("repos") or [])
    sessions = _f(cc, "sessions") or 1
    repos_per_session = repos / sessions if sessions else repos
    if repos_per_session >= 2.0 or (repos >= 4 and sessions <= 2):
        return 3  # Archaeologist
    if loop_ratio >= 4:
        return 2  # Cowboy
    if loop_ratio >= 1.5:
        return 1  # Engineer
    return 0      # Architect


def score_diligence(combined, cc):
    """Return tier index 0..3 (0 best) for Diligence."""
    beh = cc.get("behavior") or {}
    hours = beh.get("hours") or []
    total = sum(_f(h, "count") for h in hours) or 0
    late = sum(_f(h, "count") for h in hours
               if isinstance(h, dict) and (h.get("hour", 12) >= 22
                                           or h.get("hour", 12) <= 5))
    late_share = (late / total) if total else 0.0
    active_days = _f(cc, "active_days")
    sessions = _f(combined, "total_sessions")
    if active_days <= 1 and sessions <= 2:
        return 3  # Weekend Warrior (low activity)
    if late_share >= 0.35:
        return 1  # Crunch Lord
    if active_days >= 5:
        return 0  # Workhorse
    return 2      # Zen Coder


def pick(copy, axis_key, idx, lang):
    tiers = copy["axes"][axis_key]["tiers"]
    idx = clamp(idx, 0, len(tiers) - 1)
    t = tiers[idx]
    name = t["en_name"] if lang == "en" else t["zh_name"]
    roast = t["en_roast"] if lang == "en" else t["zh_roast"]
    return name, roast, idx, len(tiers)


def build(data, copy, lang):
    lang = "en" if lang == "en" else "zh"
    ui = copy["ui"].get(lang, copy["ui"]["zh"])
    platforms = data.get("platforms") or {}
    cc = platforms.get("claude_code") or {}
    combined = data.get("combined") or {}
    ps = cc.get("prompt_signals") or combined.get("prompt_signals") or {}

    axes_spec = [
        ("prompt", "axis_prompt", score_prompt(ps)),
        ("spending", "axis_spending", score_spending(combined, cc)),
        ("engineering", "axis_engineering", score_engineering(cc)),
        ("diligence", "axis_diligence", score_diligence(combined, cc)),
    ]

    axes = []
    goodness = []
    names = []
    for key, ui_label, idx in axes_spec:
        name, roast, i, count = pick(copy, key, idx, lang)
        axes.append(dict(key=key, label=ui[ui_label], tier=name, roast=roast,
                         tier_index=i, tier_count=count))
        goodness.append((count - 1 - i) / (count - 1) if count > 1 else 1.0)
        names.append(name)

    good = sum(goodness) / len(goodness) if goodness else 0.0
    rank_pct = int(clamp(round(good * 100), 3, 97))

    return dict(
        lang=lang,
        ui=ui,
        scorecard_label=ui["scorecard"],
        title_label=ui["title_label"],
        axes=axes,
        # placeholder composite title; the model may rewrite into a sentence
        title=" × ".join(names),
        rank_pct=rank_pct,
        rank_label=ui["beats_pct"].format(pct=rank_pct),
        rank_is_estimate=True,
        estimate_note=ui["estimate_note"],
        privacy_note=ui["local_privacy_note"],
    )


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--data", required=True, help="merged dual-platform JSON")
    ap.add_argument("--copy", default=str(DEFAULT_COPY))
    ap.add_argument("--lang", choices=["zh", "en"], default="zh")
    ap.add_argument("--output", default="")
    a = ap.parse_args()

    data = json.loads(Path(a.data).read_text(encoding="utf-8"))
    copy = json.loads(Path(a.copy).read_text(encoding="utf-8"))
    card = build(data, copy, a.lang)

    out = json.dumps(card, indent=2, ensure_ascii=False)
    if a.output:
        Path(a.output).write_text(out, encoding="utf-8")
        print(f"wrote {a.output}")
        for ax in card["axes"]:
            print(f"  {ax['label']}: {ax['tier']}")
        print(f"  {card['rank_label']} ({'estimate' if card['rank_is_estimate'] else ''})")
    else:
        print(out)


if __name__ == "__main__":
    main()

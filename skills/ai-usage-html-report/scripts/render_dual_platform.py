#!/usr/bin/env python3
"""Render a dual-platform (Claude Code + Codex) AI usage HTML report
from the merged JSON produced by merge_dual_platform.py.

  --data <dual_platform.json> --insights <insights.json> --output <out.html>

Self-contained: no external libs, no network. Privacy: renders only aggregate
counts/costs/model names + project basenames already present in --data.
"""
import argparse
import html
import json
from pathlib import Path


def esc(v):
    return html.escape(str(v if v is not None else ""))


def comma(v):
    try:
        return f"{int(v):,}"
    except Exception:
        return esc(v)


def money(v):
    try:
        return f"${float(v):,.2f}"
    except Exception:
        return "$0.00"


def pct(v):
    try:
        return f"{float(v) * 100:.1f}%"
    except Exception:
        return "0.0%"


def metric(label, value, sub=""):
    s = f"<span class='sub'>{esc(sub)}</span>" if sub else ""
    return f"<div class='metric'><span>{esc(label)}</span><b>{esc(value)}</b>{s}</div>"


def bar_row(label, value, total, fmt=comma):
    share = (value / total * 100) if total else 0
    return (f"<div class='bar'><div class='blabel'>{esc(label)}</div>"
            f"<div class='btrack'><div class='bfill' style='width:{share:.1f}%'></div></div>"
            f"<div class='bval'>{fmt(value)} <span class='muted'>{share:.0f}%</span></div></div>")


def compare_metric(label, a, b, fmt=comma, a_name="Claude Code", b_name="Codex"):
    total = (a or 0) + (b or 0)
    sa = (a / total * 100) if total else 0
    return (
        f"<div class='cmp'><div class='cmp-label'>{esc(label)}</div>"
        f"<div class='cmp-bar'>"
        f"<div class='cmp-a' style='width:{sa:.1f}%' title='{esc(a_name)}'></div>"
        f"<div class='cmp-b' style='width:{100 - sa:.1f}%' title='{esc(b_name)}'></div>"
        f"</div>"
        f"<div class='cmp-vals'><span class='ca'>{fmt(a)}</span>"
        f"<span class='cb'>{fmt(b)}</span></div></div>"
    )


def model_table(models, kind):
    rows = []
    if kind == "claude":
        rows.append("<tr><th>模型</th><th>成本</th><th>输入</th><th>输出</th><th>缓存读</th></tr>")
        for m in models:
            rows.append(
                f"<tr><td><b>{esc(m['model'])}</b></td><td>{money(m['cost'])}</td>"
                f"<td>{comma(m['input'])}</td><td>{comma(m['output'])}</td>"
                f"<td>{comma(m['cache_read'])}</td></tr>")
    else:
        rows.append("<tr><th>模型</th><th>输入</th><th>输出</th><th>缓存输入</th><th>reasoning</th></tr>")
        for m in models:
            rows.append(
                f"<tr><td><b>{esc(m['model'])}</b></td>"
                f"<td>{comma(m['input'])}</td><td>{comma(m['output'])}</td>"
                f"<td>{comma(m['cached'])}</td><td>{comma(m['reasoning'])}</td></tr>")
    return "<table>" + "".join(rows) + "</table>"


def mini_bars(items, label_key, val_key, color, unit="", top=8):
    """Horizontal mini bar list for top_commands / git / languages / repos."""
    items = [i for i in (items or []) if i.get(val_key)][:top]
    if not items:
        return "<p class='muted'>无数据</p>"
    mx = max(i.get(val_key, 0) for i in items) or 1
    rows = []
    for it in items:
        v = it.get(val_key, 0)
        share = v / mx * 100
        label = esc(it.get(label_key, ""))
        vtxt = comma(v) + (f" {unit}" if unit else "")
        rows.append(
            f"<div class='mbar'><div class='mlabel' title='{label}'>{label}</div>"
            f"<div class='mtrack'><div class='mfill' style='width:{share:.1f}%;"
            f"background:{color}'></div></div>"
            f"<div class='mval'>{vtxt}</div></div>")
    return "".join(rows)


def hours_chart(hours, color):
    """24h activity columns using the `count` (message/tool activity)."""
    if not hours:
        return "<p class='muted'>无活跃时段数据</p>"
    by_h = {h.get("hour"): h for h in hours}
    vals = [by_h.get(h, {}).get("count", 0) for h in range(24)]
    mx = max(vals) or 1
    cols = []
    for h in range(24):
        v = vals[h]
        ht = (v / mx * 100) if v else 2
        cols.append(
            f"<div class='hcol' title='{h:02d}:00 · {v}'>"
            f"<div class='hbar' style='height:{ht:.0f}%;background:{color}'></div>"
            f"<div class='hlab'>{h if h % 6 == 0 else ''}</div></div>")
    return f"<div class='hours'>{''.join(cols)}</div>"


def behavior_panel(beh, color, platform):
    """Symmetric behavior block for one platform."""
    if not beh:
        return (f"<div class='panel'><h2>{esc(platform)} · 使用行为</h2>"
                f"<p class='muted'>无行为数据。</p></div>")
    p = [f"<div class='panel'><h2>{esc(platform)} · 使用行为</h2>"]
    p.append(f"<p class='muted'>窗口 {esc(beh.get('generated_for'))} · "
             f"{beh.get('sessions', 0)} 会话 · "
             f"{comma(beh.get('total_tool_calls', 0))} 次工具调用</p>")

    # tool categories chips
    cats = beh.get("tool_categories") or {}
    cat_zh = {"shell": "命令行", "web": "网络", "file": "文件",
              "search": "搜索", "mcp": "MCP", "other": "其他"}
    if cats:
        chips = "".join(
            f"<span class='chip'>{esc(cat_zh.get(k, k))} {comma(v)}</span>"
            for k, v in sorted(cats.items(), key=lambda kv: -kv[1]) if v)
        p.append(f"<div class='chips'>{chips}</div>")

    # tools by name (Claude only) else top commands
    if beh.get("tools_by_name"):
        p.append("<h3>工具 Top</h3>")
        p.append(mini_bars(beh["tools_by_name"], "name", "count", color))
    p.append("<h3>命令 Top（仅命令首词）</h3>")
    p.append(mini_bars(beh.get("top_commands"), "command", "count", color))

    p.append("<h3>Git 习惯（子命令次数）</h3>")
    p.append(mini_bars(beh.get("git_habits"), "command", "count", color))

    unit = beh.get("languages_unit", "")
    p.append(f"<h3>语言分布（{esc(unit)}）</h3>")
    p.append(mini_bars(beh.get("languages"), "name", "count", color, unit=unit))

    p.append("<h3>Repo 排行（按 Token）</h3>")
    p.append(mini_bars(beh.get("repos"), "repo", "tokens", color))

    p.append("<h3>活跃时段（本地时区，按活动计数）</h3>")
    p.append(hours_chart(beh.get("hours"), color))

    src = beh.get("sources") or []
    if src:
        p.append("<h3>来源</h3>")
        p.append(mini_bars(src, "name", "count", color))

    extras = beh.get("extras") or []
    if extras:
        p.append("<h3>行为信号</h3><ul class='sig'>")
        for e in extras:
            p.append(f"<li>{esc(e)}</li>")
        p.append("</ul>")
    p.append("</div>")
    return "".join(p)


def sparkline(series, color):
    if not series:
        return ""
    vals = [s.get("cost", 0) for s in series]
    mx = max(vals) or 1
    w, h = 280, 46
    n = len(vals)
    if n == 1:
        pts = f"0,{h} {w},{h - (vals[0] / mx) * h}"
    else:
        step = w / (n - 1)
        pts = " ".join(f"{i * step:.1f},{h - (v / mx) * h:.1f}" for i, v in enumerate(vals))
    return (f"<svg class='spark' viewBox='0 0 {w} {h}' preserveAspectRatio='none'>"
            f"<polyline points='{pts}' fill='none' stroke='{color}' stroke-width='2'/></svg>")


def scorecard_html(sc):
    """Render the shareable cover scorecard (vertical, screenshot-friendly).

    `sc` is the JSON from scripts/scorecard.py; fully bilingual via its own copy.
    """
    if not sc:
        return ""
    parts = ["<section class='scorecard'>"]
    parts.append(f"<span class='sc-kicker'>{esc(sc.get('scorecard_label', ''))} · "
                 f"{esc(sc.get('title_label', ''))}</span>")
    parts.append(f"<h2 class='sc-title'>{esc(sc.get('title', ''))}</h2>")
    if sc.get("rank_label"):
        parts.append(f"<p class='sc-rank'>{esc(sc['rank_label'])}</p>")
    for ax in sc.get("axes", []):
        parts.append(
            "<div class='sc-axis'>"
            f"<span class='sc-ax-label'>{esc(ax.get('label'))}</span>"
            f"<span class='sc-tier'>{esc(ax.get('tier'))}</span>"
            f"<span class='sc-roast'>{esc(ax.get('roast'))}</span>"
            "</div>")
    note = " · ".join(x for x in (sc.get("privacy_note"), sc.get("estimate_note")) if x)
    if note:
        parts.append(f"<p class='sc-note'>{esc(note)}</p>")
    parts.append("</section>")
    return "".join(parts)


def render(data, insights, scorecard=None, lang="zh"):
    cc = data["platforms"]["claude_code"]
    cx = data["platforms"]["codex"]
    comb = data["combined"]
    title = data.get("title", "双平台 AI 使用报告")
    htmllang = "en" if lang == "en" else "zh-CN"

    p = [
        f"<!doctype html><html lang='{htmllang}'><head><meta charset='utf-8'>",
        "<meta name='viewport' content='width=device-width, initial-scale=1'>",
        f"<title>{esc(title)}</title><style>", CSS, "</style></head><body><main>",
        f"<header><h1>{esc(title)}</h1>"
        f"<p>生成于 {esc(data.get('generated_at'))} · 数据来源：ccusage(Claude Code, 离线 LiteLLM 定价) + ccoach/ccusage(Codex)</p></header>",
    ]

    # shareable cover scorecard (top of the report, screenshot-friendly)
    if scorecard:
        p.append(scorecard_html(scorecard))

    # combined headline metrics
    p.append("<section class='metrics'>")
    p.append(metric("合计成本", money(comb["total_cost_usd"]), "两平台真实/估算成本"))
    p.append(metric("合计 Token", comma(comb["total_tokens"])))
    p.append(metric("Claude Code 成本", money(cc["cost_usd"]), f"{cc['active_days']} 活跃天"))
    p.append(metric("Codex 成本", money(cx["cost_usd"]), f"{cx['active_days']} 活跃天"))
    p.append("</section>")

    # AI executive summary (prominent, near the top)
    exec_summary = insights.get("executive_summary")
    if exec_summary:
        p.append("<section class='panel focus'><h2>执行摘要</h2>")
        if isinstance(exec_summary, str):
            for para in [s for s in exec_summary.split("\n") if s.strip()]:
                p.append(f"<p>{esc(para.strip())}</p>")
        else:
            p.append("<ul>")
            for item in exec_summary:
                p.append(f"<li>{esc(item)}</li>")
            p.append("</ul>")
        p.append("</section>")

    # AI recommendations (each may be a string or {text, evidence})
    recs = insights.get("recommendations")
    if recs:
        p.append("<section class='panel'><h2>AI 建议</h2><div class='cards'>")
        for rec in recs:
            if isinstance(rec, str):
                p.append(f"<article class='card rec'><p>{esc(rec)}</p></article>")
            else:
                title_html = (f"<strong>{esc(rec.get('title'))}</strong>"
                              if rec.get("title") else "")
                text = rec.get("text") or rec.get("action") or ""
                ev = rec.get("evidence")
                ev_html = f"<p class='muted'>证据：{esc(ev)}</p>" if ev else ""
                p.append(f"<article class='card rec'>{title_html}<p>{esc(text)}</p>{ev_html}</article>")
        p.append("</div></section>")

    # AI insights (each may be a string or {title, detail})
    items = insights.get("insights", [])
    p.append("<section class='panel focus'><h2>AI 洞见（基于真实数字）</h2>")
    if items:
        p.append("<ul>")
        for it in items:
            if isinstance(it, str):
                p.append(f"<li>{esc(it)}</li>")
            else:
                title = it.get("title")
                detail = it.get("detail", "")
                if title:
                    p.append(f"<li><b>{esc(title)}</b>{('：' + esc(detail)) if detail else ''}</li>")
                else:
                    p.append(f"<li>{esc(detail)}</li>")
        p.append("</ul>")
    elif not exec_summary and not recs:
        p.append("<ul><li>未提供洞见。</li></ul>")
    p.append("</section>")

    # head-to-head comparison bars
    p.append("<section class='panel'><h2>两平台对比</h2><div class='legend'>"
             "<span class='ldot a'></span>Claude Code"
             "<span class='ldot b'></span>Codex</div>")
    p.append(compare_metric("总成本 (USD)", cc["cost_usd"], cx["cost_usd"], money))
    p.append(compare_metric("总 Token", cc["tokens"]["total"], cx["tokens"]["total"]))
    p.append(compare_metric("输入 Token", cc["tokens"]["input"], cx["tokens"]["input"]))
    p.append(compare_metric("输出 Token", cc["tokens"]["output"], cx["tokens"]["output"]))
    p.append(compare_metric("缓存读取 Token", cc["tokens"]["cache_read"], cx["tokens"]["cache_read"]))
    p.append(compare_metric("缓存命中率", cc["cache_hit_rate"], cx["cache_hit_rate"], pct))
    p.append(compare_metric("活跃天数", cc["active_days"], cx["active_days"]))
    p.append("</div></section>" if False else "</section>")

    # two platform panels side by side
    p.append("<section class='grid2'>")

    # Claude Code panel
    p.append("<div class='panel'><h2>Claude Code</h2>")
    p.append(f"<p class='muted'>{esc(cc['source'])} · {cc['date_range'][0]}→{cc['date_range'][-1]} · "
             f"{cc['sessions']} 会话 · 成本真实</p>")
    p.append(sparkline(cc["daily_series"], "#0f766e"))
    p.append("<h3>模型分布</h3>")
    p.append(model_table(cc["models"], "claude"))
    p.append("<h3>Top 会话（按成本）</h3><table><tr><th>项目</th><th>成本</th><th>Token</th><th>模型</th></tr>")
    for s in cc["top_sessions"]:
        p.append(f"<tr><td>{esc(s['project'])}<br><span class='muted'>{esc(s['last'])}</span></td>"
                 f"<td>{money(s['cost'])}</td><td>{comma(s['tokens'])}</td>"
                 f"<td>{esc(', '.join(s['models']))}</td></tr>")
    p.append("</table></div>")

    # Codex panel
    p.append("<div class='panel'><h2>Codex</h2>")
    af = cx["autofresh_today"]
    empty_note = "（autofresh 今日报告为空：今天无 Codex 活动）" if af["empty"] else ""
    p.append(f"<p class='muted'>{esc(cx['source'])} · {cx['date_range'][0]}→{cx['date_range'][-1]} · "
             f"成本：部分按日估算（per-model 缺定价）{esc(empty_note)}</p>")
    p.append(sparkline(cx["daily_series"], "#b45309"))
    p.append("<h3>模型分布</h3>")
    p.append(model_table(cx["models"], "codex"))
    p.append(f"<h3>autofresh 今日快照（{esc(af['generated_for'])} {esc(af['timezone'])}）</h3>")
    p.append(f"<p>会话 <b>{af['sessions']}</b> · token <b>{comma(af['tokens'].get('total', 0))}</b> · "
             f"成本 <b>{money(af['cost_usd'])}</b> · 缓存命中 <b>{pct(af['cache_hit_rate'])}</b></p>")
    if af["empty"]:
        p.append("<p class='muted'>说明：autofresh <code>report --json</code> 只统计当天；"
                 "今天无 Codex 会话，故快照为 0。历史 Codex 数据由 ccusage codex 提供（上方）。</p>")
    p.append("</div>")
    p.append("</section>")

    # token composition per platform
    p.append("<section class='grid2'>")
    p.append("<div class='panel'><h2>Claude Code Token 构成</h2>")
    cct = cc["tokens"]
    p.append(bar_row("缓存读取", cct["cache_read"], cct["total"]))
    p.append(bar_row("输出", cct["output"], cct["total"]))
    p.append(bar_row("缓存写入", cct.get("cache_create", 0), cct["total"]))
    p.append(bar_row("输入", cct["input"], cct["total"]))
    p.append("</div>")
    p.append("<div class='panel'><h2>Codex Token 构成</h2>")
    cxt = cx["tokens"]
    p.append(bar_row("缓存输入", cxt["cache_read"], cxt["total"]))
    p.append(bar_row("输入", cxt["input"], cxt["total"]))
    p.append(bar_row("输出", cxt["output"], cxt["total"]))
    p.append(bar_row("reasoning", cxt.get("reasoning", 0), cxt["total"]))
    p.append("</div></section>")

    # symmetric behavior panels (tools / git / languages / repos / hours)
    p.append("<section><h2 class='section-h'>使用行为画像（两平台对称）</h2>"
             "<div class='grid2'>")
    p.append(behavior_panel(cc.get("behavior"), "#0f766e", "Claude Code"))
    p.append(behavior_panel(cx.get("behavior"), "#b45309", "Codex"))
    p.append("</div></section>")

    # data provenance / privacy
    p.append("<section class='panel'><h2>数据来源与隐私</h2><ul>")
    p.append("<li>Claude Code：<code>ccusage claude daily/session --json --offline --breakdown</code>，"
             "成本由 ccusage 内置 LiteLLM 定价离线计算（真实）。</li>")
    p.append("<li>Codex：<code>autofresh report --json</code>（当天快照）+ "
             "<code>ccusage codex daily --json --offline</code>（历史）。Codex 成本为按日估算，"
             "ccusage 对 Codex per-model 不输出 costUSD。</li>")
    p.append("<li>隐私：仅读取聚合 token/成本/模型名/项目目录名；不嵌入任何 prompt 原文、对话内容或密钥。"
             "ccusage 离线模式不联网、不上传。</li>")
    p.append("</ul></section>")

    p.append("</main></body></html>")
    return "".join(p)


CSS = r"""
:root{color-scheme:light dark;--bg:#f6f7f5;--fg:#202322;--muted:#68706c;--panel:#fff;--line:#d8ddd9;--a:#0f766e;--b:#b45309}
@media(prefers-color-scheme:dark){:root{--bg:#111413;--fg:#edf1ef;--muted:#a7b0ab;--panel:#1a1f1d;--line:#313936;--a:#5eead4;--b:#fbbf24}}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--fg);font:14px/1.5 system-ui,-apple-system,sans-serif}
main{max-width:1180px;margin:0 auto;padding:30px 18px 56px}header{margin-bottom:22px}h1{font-size:27px;margin:0 0 6px}
h2{font-size:17px;margin:0 0 12px}h3{font-size:13px;margin:16px 0 6px;color:var(--muted)}header p,.muted{color:var(--muted)}
.metrics{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px;margin-bottom:16px}
.metric,.panel,.card{background:var(--panel);border:1px solid var(--line);border-radius:8px;padding:14px}
.cards{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;margin-top:6px}
.card.rec{border-left:4px solid var(--a)}.card strong{display:block;margin-bottom:5px}.card p{margin:5px 0}
.metric span{display:block;color:var(--muted);font-size:12px}.metric b{display:block;font-size:22px;margin-top:5px}.metric .sub{font-size:11px;margin-top:3px}
.grid2{display:grid;grid-template-columns:1fr 1fr;gap:14px;margin-top:16px}.focus{border-left:4px solid var(--a);margin-bottom:8px}
section{margin-top:18px}table{width:100%;border-collapse:collapse;margin-top:6px}td,th{text-align:left;border-bottom:1px solid var(--line);padding:7px 8px;font-size:13px}th{color:var(--muted);font-size:11px}tr:last-child td{border-bottom:0}
code{font-family:ui-monospace,Menlo,monospace;font-size:12px}ul{margin:0;padding-left:18px}
.cmp{margin:10px 0}.cmp-label{font-size:12px;color:var(--muted);margin-bottom:3px}
.cmp-bar{display:flex;height:18px;border-radius:5px;overflow:hidden;background:var(--line)}
.cmp-a{background:var(--a)}.cmp-b{background:var(--b)}
.cmp-vals{display:flex;justify-content:space-between;font-size:12px;margin-top:2px}.ca{color:var(--a);font-weight:600}.cb{color:var(--b);font-weight:600}
.legend{font-size:12px;color:var(--muted);margin-bottom:8px}.ldot{display:inline-block;width:10px;height:10px;border-radius:2px;margin:0 5px 0 12px;vertical-align:middle}.ldot.a{background:var(--a)}.ldot.b{background:var(--b)}
.bar{display:grid;grid-template-columns:90px 1fr 150px;gap:8px;align-items:center;margin:5px 0}.blabel{font-size:12px;color:var(--muted)}
.btrack{height:14px;background:var(--line);border-radius:4px;overflow:hidden}.bfill{height:100%;background:var(--a)}.bval{font-size:12px;text-align:right}
.spark{width:100%;height:46px;margin:6px 0}
.section-h{margin:18px 0 10px;font-size:18px}
.chips{display:flex;flex-wrap:wrap;gap:6px;margin:4px 0 8px}
.chip{font-size:11px;background:var(--line);border-radius:10px;padding:2px 9px;color:var(--fg)}
.mbar{display:grid;grid-template-columns:120px 1fr 96px;gap:8px;align-items:center;margin:3px 0}
.mlabel{font-size:12px;color:var(--fg);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.mtrack{height:12px;background:var(--line);border-radius:4px;overflow:hidden}.mfill{height:100%}
.mval{font-size:11px;text-align:right;color:var(--muted)}
.hours{display:flex;align-items:flex-end;gap:2px;height:70px;margin:8px 0 2px}
.hcol{flex:1;display:flex;flex-direction:column;justify-content:flex-end;align-items:center;height:100%}
.hbar{width:70%;border-radius:2px 2px 0 0;min-height:2px}
.hlab{font-size:9px;color:var(--muted);margin-top:2px;height:11px}
ul.sig{margin:4px 0;padding-left:18px}ul.sig li{font-size:12px;color:var(--muted);margin:2px 0}
.scorecard{max-width:430px;margin:14px auto 22px;padding:20px 22px;border-radius:16px;background:linear-gradient(160deg,#1b2440,#0f1530);color:#fff;box-shadow:0 8px 30px rgba(0,0,0,.25)}
.scorecard .sc-kicker{font-size:11px;letter-spacing:.12em;text-transform:uppercase;opacity:.65}
.scorecard .sc-title{margin:4px 0 2px;font-size:23px;line-height:1.25;font-weight:800}
.scorecard .sc-rank{margin:0 0 12px;font-size:13px;opacity:.85}
.sc-axis{display:flex;flex-direction:column;padding:10px 0;border-top:1px solid rgba(255,255,255,.12)}
.sc-axis .sc-ax-label{font-size:11px;opacity:.6}
.sc-axis .sc-tier{font-size:17px;font-weight:700;margin:1px 0}
.sc-axis .sc-roast{font-size:12px;opacity:.82}
.scorecard .sc-note{margin:12px 0 0;font-size:10px;opacity:.55}
@media(max-width:900px){.metrics,.grid2,.cards{grid-template-columns:1fr}.mbar{grid-template-columns:90px 1fr 70px}}
"""


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--data", required=True)
    ap.add_argument("--insights", required=True)
    ap.add_argument("--output", required=True)
    ap.add_argument("--scorecard", default="",
                    help="scorecard JSON from scorecard.py (optional cover card)")
    ap.add_argument("--lang", choices=["zh", "en"], default="zh")
    a = ap.parse_args()
    data = json.loads(Path(a.data).read_text())
    insights = json.loads(Path(a.insights).read_text())
    scorecard = json.loads(Path(a.scorecard).read_text()) if a.scorecard else None
    Path(a.output).write_text(render(data, insights, scorecard=scorecard,
                                     lang=a.lang), encoding="utf-8")
    print(f"wrote {a.output}")


if __name__ == "__main__":
    main()

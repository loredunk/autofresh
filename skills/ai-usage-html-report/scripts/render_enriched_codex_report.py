#!/usr/bin/env python3
import argparse
import html
import json
from pathlib import Path


def esc(value):
    return html.escape(str(value if value is not None else ""))


def comma(value):
    try:
        return f"{int(value):,}"
    except Exception:
        return esc(value)


def money(value):
    try:
        return f"${float(value):.2f}"
    except Exception:
        return "$0.00"


def pct(value):
    try:
        return f"{float(value) * 100:.1f}%"
    except Exception:
        return "0.0%"


def pills(items):
    if not items:
        return '<span class="muted">-</span>'
    return "".join(f'<span class="pill">{esc(item)}</span>' for item in items)


def priority_class(priority):
    p = str(priority or "").lower()
    if p == "high":
        return " high"
    if p == "low":
        return " low"
    return ""


def render(report, insights):
    tokens = report.get("tokens", {})
    tools = report.get("tools", {})
    repos = report.get("repos", [])
    sources = report.get("sources", [])
    languages = report.get("languages", [])
    git = report.get("git_habits", {})
    project = report.get("project_management", {})

    title = insights.get("title") or "Codex 使用深度报告"
    subtitle = insights.get("subtitle") or f'{report.get("generated_for", "")} · {report.get("timezone", "")}'

    parts = [
        "<!doctype html><html lang='zh-CN'><head><meta charset='utf-8'>",
        "<meta name='viewport' content='width=device-width, initial-scale=1'>",
        f"<title>{esc(title)}</title>",
        "<style>",
        CSS,
        "</style></head><body><main>",
        f"<header><h1>{esc(title)}</h1><p>{esc(subtitle)}</p></header>",
        "<section class='metrics'>",
        metric("会话", report.get("sessions", 0)),
        metric("Token", comma(tokens.get("total", 0))),
        metric("估算成本", money(report.get("estimated_cost_usd", 0))),
        metric("活跃时长", report.get("duration", "0m")),
        metric("缓存命中", pct(report.get("cache_hit_rate", 0))),
        metric("Reasoning 占比", pct(report.get("reasoning_ratio", 0))),
        "</section>",
    ]

    parts.append("<section class='panel focus'><h2>执行摘要</h2><ul>")
    for item in insights.get("executive_summary", []):
        parts.append(f"<li>{esc(item)}</li>")
    if not insights.get("executive_summary"):
        parts.append("<li>未提供 AI 摘要；请在 insights JSON 中补充 executive_summary。</li>")
    parts.append("</ul></section>")

    parts.append("<section><h2>AI 建议</h2><div class='cards'>")
    for rec in insights.get("recommendations", []):
        parts.append(
            f"<article class='card rec{priority_class(rec.get('priority'))}'>"
            f"<strong>{esc(rec.get('title'))}</strong>"
            f"<p class='muted'>证据：{esc(rec.get('evidence'))}</p>"
            f"<p>{esc(rec.get('action'))}</p>"
            "</article>"
        )
    parts.append("</div></section>")

    insight_ladder = insights.get("insight_ladder", [])
    if insight_ladder:
        parts.append("<section><h2>深度洞见</h2><div class='insights'>")
        for item in insight_ladder:
            parts.append(
                "<article class='card insight'>"
                f"<strong>{esc(item.get('title'))}</strong>"
                "<h3>证据</h3>"
                f"{signal_list(item.get('evidence', []))}"
                f"<p><b>代表什么</b><br>{esc(item.get('meaning', ''))}</p>"
                f"<p><b>为什么重要</b><br>{esc(item.get('impact', ''))}</p>"
                f"<p><b>继续深入</b><br>{esc(item.get('drilldown', ''))}</p>"
                f"<p><b>建议动作</b><br>{esc(item.get('intervention', ''))}</p>"
                "</article>"
            )
        parts.append("</div></section>")

    parts.append("<section class='grid2'>")
    parts.append(usage_table("来源", sources, tokens.get("total", 0)))
    parts.append(usage_table("语言", languages, tokens.get("total", 0)))
    parts.append("</section>")

    parts.append("<section class='grid2'>")
    parts.append("<div class='panel'><h2>Git 习惯</h2>")
    parts.append(f"<p><b>{git.get('command_count', 0)}</b> 次 git 命令 · <b>{git.get('branch_count', 0)}</b> 个分支上下文</p>")
    parts.append("<p>" + pills([f"{x.get('command')} {x.get('count')}" for x in git.get("top_subcommands", [])]) + "</p>")
    parts.append(signal_list(git.get("review_signals", []) + git.get("risk_signals", [])))
    parts.append("</div>")

    parts.append("<div class='panel'><h2>项目管理习惯</h2>")
    project_counts = [
        f"有测试 {project.get('repos_with_tests', 0)}",
        f"有构建 {project.get('repos_with_build_system', 0)}",
        f"有 CI {project.get('repos_with_ci', 0)}",
    ]
    change_counts = [
        f"文档/计划 {project.get('documentation_changes', 0)}",
        f"配置 {project.get('config_changes', 0)}",
    ]
    parts.append(f"<p>{pills(project_counts)}</p>")
    parts.append(f"<p>{pills(change_counts)}</p>")
    parts.append(signal_list(project.get("signals", [])))
    parts.append("</div></section>")

    for section in insights.get("sections", []):
        parts.append(f"<section class='panel'><h2>{esc(section.get('title'))}</h2><ul>")
        for bullet in section.get("bullets", []):
            parts.append(f"<li>{esc(bullet)}</li>")
        parts.append("</ul></section>")

    session_reviews = insights.get("session_reviews", [])
    if session_reviews:
        parts.append("<section><h2>Session Prompt 复盘</h2><div class='cards wide'>")
        for review in session_reviews:
            parts.append(
                "<article class='card'>"
                f"<strong>{esc(review.get('repo'))}</strong>"
                f"<p class='muted'>{esc(review.get('session_id') or review.get('rollout_path'))}</p>"
                f"<p>{esc(review.get('summary'))}</p>"
                "<h3>Token 驱动</h3>"
                f"{signal_list(review.get('token_drivers', []))}"
                "<h3>Prompt 问题</h3>"
                f"{signal_list(review.get('prompt_issues', []))}"
                f"<p><b>更好的起始 prompt</b><br>{esc(review.get('better_first_prompt', ''))}</p>"
                f"<p><b>更好的追问</b><br>{esc(review.get('better_followup_prompt', ''))}</p>"
                f"<p class='muted'>{esc(review.get('next_action', ''))}</p>"
                "</article>"
            )
        parts.append("</div></section>")

    parts.append("<section><h2>项目画像</h2><table><thead><tr><th>项目</th><th>语言</th><th>构建/测试</th><th>变更</th><th>Token</th><th>AI 备注</th></tr></thead><tbody>")
    notes = {n.get("repo"): n for n in insights.get("project_notes", [])}
    for repo in repos:
        note = notes.get(repo.get("repo"), {})
        changes = ", ".join(f"{x.get('type')} {x.get('count')}" for x in repo.get("file_change_types", [])) or "-"
        build = pills((repo.get("build_systems") or []) + (repo.get("test_commands") or []))
        parts.append(
            "<tr>"
            f"<td><b>{esc(repo.get('repo'))}</b><br><span class='muted'>{repo.get('sessions', 0)} 会话 · {money(repo.get('estimated_cost_usd', 0))}</span></td>"
            f"<td>{esc(repo.get('language', '-'))}</td>"
            f"<td>{build}</td>"
            f"<td>{esc(changes)}</td>"
            f"<td>{comma(repo.get('tokens', 0))}</td>"
            f"<td>{esc(note.get('summary', ''))}<br><span class='muted'>{esc(note.get('next_action', ''))}</span></td>"
            "</tr>"
        )
    parts.append("</tbody></table></section>")

    parts.append("<section class='panel'><h2>原始报告来源</h2>")
    parts.append(f"<p><code>{esc(report.get('codex_home'))}</code> · source: <code>{esc(report.get('source'))}</code></p>")
    parts.append(f"<p>shell {tools.get('shell_calls', 0)} · web {tools.get('web_searches', 0)} · file changes {tools.get('file_changes', 0)}</p>")
    parts.append("</section>")

    parts.append("</main></body></html>")
    return "".join(parts)


def metric(label, value):
    return f"<div class='metric'><span>{esc(label)}</span><b>{esc(value)}</b></div>"


def usage_table(title, rows, total):
    out = [f"<div class='panel'><h2>按{esc(title)}</h2><table><tbody>"]
    for row in rows:
        share = 0.0
        if total:
            share = float(row.get("tokens", 0)) / float(total) * 100
        out.append(f"<tr><td>{esc(row.get('name'))}</td><td>{share:.1f}%</td><td>{comma(row.get('tokens', 0))}</td></tr>")
    out.append("</tbody></table></div>")
    return "".join(out)


def signal_list(items):
    if not items:
        return "<p class='muted'>暂无明显信号</p>"
    return "<ul>" + "".join(f"<li>{esc(item)}</li>" for item in items) + "</ul>"


CSS = r"""
:root{color-scheme:light dark;--bg:#f6f7f5;--fg:#202322;--muted:#68706c;--panel:#fff;--line:#d8ddd9;--accent:#0f766e;--high:#b42318;--low:#4d7c0f}
@media(prefers-color-scheme:dark){:root{--bg:#111413;--fg:#edf1ef;--muted:#a7b0ab;--panel:#1a1f1d;--line:#313936;--accent:#5eead4;--high:#fca5a5;--low:#bef264}}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--fg);font:14px/1.5 system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}main{max-width:1220px;margin:0 auto;padding:30px 18px 56px}header{margin-bottom:22px}h1{font-size:28px;margin:0 0 6px}h2{font-size:17px;margin:0 0 12px}h3{font-size:13px;margin:12px 0 6px}header p,.muted{color:var(--muted)}.metrics{display:grid;grid-template-columns:repeat(6,minmax(0,1fr));gap:10px;margin-bottom:16px}.metric,.panel,.card{background:var(--panel);border:1px solid var(--line);border-radius:8px;padding:14px}.metric span{display:block;color:var(--muted);font-size:12px}.metric b{display:block;font-size:21px;margin-top:5px}.grid2{display:grid;grid-template-columns:1fr 1fr;gap:14px;margin-top:16px}.cards{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}.cards.wide{grid-template-columns:repeat(2,minmax(0,1fr))}.insights{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.insight{border-left:4px solid var(--accent)}.focus{border-left:4px solid var(--accent);margin-bottom:16px}.rec{border-left:4px solid var(--accent)}.rec.high{border-left-color:var(--high)}.rec.low{border-left-color:var(--low)}section{margin-top:18px}table{width:100%;border-collapse:collapse;background:var(--panel);border:1px solid var(--line);border-radius:8px;overflow:hidden}td,th{text-align:left;vertical-align:top;border-bottom:1px solid var(--line);padding:9px 10px}tr:last-child td{border-bottom:0}th{color:var(--muted);font-size:12px}.pill{display:inline-block;border:1px solid var(--line);border-radius:999px;padding:2px 7px;margin:0 4px 4px 0;color:var(--muted)}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px}ul{margin:0;padding-left:18px}@media(max-width:900px){.metrics,.grid2,.cards,.cards.wide,.insights{grid-template-columns:1fr}main{padding:18px 12px}.metric b{font-size:18px}}
"""


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--report", required=True)
    parser.add_argument("--insights", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    report = json.loads(Path(args.report).read_text())
    insights = json.loads(Path(args.insights).read_text())
    Path(args.output).write_text(render(report, insights), encoding="utf-8")


if __name__ == "__main__":
    main()
